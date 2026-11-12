package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/spf13/pflag"

	"a4calendar/internal/dsl"
	"a4calendar/internal/fonts"
)

// Options in .dates files: a line starting with -- sets an option as on
// the command line (--start = 2027-01-01, --start 2027-01-01, a bare
// --hide-outside). --file includes another file at that line. Paths are
// relative to the file. The command line wins over files; an option set
// twice in files is an error.

// fileOpt is one option line of a .dates file.
type fileOpt struct {
	name, value string
	src         string // the file, as named in messages
	line        int
}

func (op fileOpt) origin() string { return fmt.Sprintf("%s:%d", op.src, op.line) }

// notInFiles are the options a file cannot set, with the reason.
var notInFiles = map[string]string{
	"help":      "--help cannot be set in a file",
	"version":   "--version cannot be set in a file",
	"licenses":  "--licenses cannot be set in a file",
	"milestone": "write the declaration itself, without --milestone",
	"interval":  "write the declaration itself, without --interval",
}

// pathOpts are the options whose value is a path, read relative to the file.
var pathOpts = map[string]bool{"output": true, "font": true, "debug-timeline-json": true}

var optNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var utf8BOM = string(rune(0xFEFF)) // skipped at the start of a file

// fileSet is what the .dates files of one run hold: their declarations in
// reading order, their option lines, and the errors found in those lines.
type fileSet struct {
	fs   *pflag.FlagSet
	srcs []dsl.Source
	opts []fileOpt
	errs dsl.Errors
	seen map[string]string // file → where it was first named
}

// loadFiles reads the files given on the command line and, at their --file
// lines, the files they include. A file named twice is an error; an
// unreadable file ends the run (exit 1).
func loadFiles(fs *pflag.FlagSet, paths []string) (*fileSet, error) {
	s := &fileSet{fs: fs, seen: map[string]string{}}
	for _, p := range paths {
		if err := s.read(p, fileOpt{}); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// fail records an error on line of src.
func (s *fileSet) fail(src string, line int, format string, a ...any) {
	s.errs = append(s.errs, &dsl.Error{Source: src, Line: line, Msg: fmt.Sprintf(format, a...)})
}

// read reads one file, named on the --file line from (its zero value for
// the command line).
func (s *fileSet) read(path string, from fileOpt) error {
	key, err := filepath.Abs(path)
	if err != nil {
		key = path
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	where := "the command line"
	if from.src != "" {
		where = from.origin()
	}
	if first, ok := s.seen[key]; ok {
		if from.src == "" {
			return usagef("--file %s: the file is already read (from %s)", path, first)
		}
		s.fail(from.src, from.line, "%s is already read (from %s)", path, first)
		return nil
	}
	s.seen[key] = where

	data, err := os.ReadFile(path)
	if err != nil {
		if from.src != "" {
			return fmt.Errorf("%s: cannot read --file: %w", where, err)
		}
		return fmt.Errorf("cannot read --file: %w", err)
	}
	lines := strings.Split(strings.TrimPrefix(string(data), utf8BOM), "\n")
	part := 0 // first line of the current part of declarations
	flush := func(end int) {
		if slices.ContainsFunc(lines[part:end], func(l string) bool { return strings.TrimSpace(l) != "" }) {
			s.srcs = append(s.srcs, dsl.Source{Name: path, Text: strings.Join(lines[part:end], "\n"), Offset: part})
		}
	}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "--") {
			continue
		}
		lines[i] = "" // not a declaration; the line numbers stay
		name, value, hasValue, err := splitOption(line)
		switch {
		case err != nil:
			s.fail(path, i+1, "%v", err)
		case name == "file":
			if value == "" {
				s.fail(path, i+1, "--file needs a path")
				continue
			}
			flush(i)
			part = i + 1
			if err := s.read(relTo(path, value), fileOpt{src: path, line: i + 1}); err != nil {
				return err
			}
		default:
			if value, err = s.optionValue(path, name, value, hasValue); err != nil {
				s.fail(path, i+1, "%v", err)
				continue
			}
			s.opts = append(s.opts, fileOpt{name: name, value: value, src: path, line: i + 1})
		}
	}
	flush(len(lines))
	return nil
}

// optionValue checks that a file may set option name and returns its value:
// true for a boolean given without one, a path made relative to file.
func (s *fileSet) optionValue(file, name, value string, hasValue bool) (string, error) {
	if why, ok := notInFiles[name]; ok {
		return "", errors.New(why)
	}
	f := s.fs.Lookup(name)
	if f == nil {
		return "", fmt.Errorf("unknown option --%s", name)
	}
	if !hasValue {
		if f.Value.Type() != "bool" {
			return "", fmt.Errorf("--%s needs a value", name)
		}
		return "true", nil
	}
	if pathOpts[name] && value != "" && !(name == "font" && value == fonts.PTSansNarrowKeyword) {
		value = relTo(file, value)
	}
	return value, nil
}

// apply sets the options read from the files on fs and returns where each
// came from ("--paper" → "plan.dates:3"), for error messages. An option the
// command line set is left as it is; --period, --end and --until count as
// one. One option set on two lines is an error naming both.
func (s *fileSet) apply() map[string]string {
	group := func(name string) string {
		if name == "end" || name == "until" {
			return "period"
		}
		return name
	}
	cli := map[string]bool{}
	s.fs.Visit(func(f *pflag.Flag) { cli[group(f.Name)] = true })
	set := map[string]fileOpt{}
	origin := map[string]string{}
	for _, op := range s.opts {
		g := group(op.name)
		if cli[g] {
			continue
		}
		if prev, ok := set[g]; ok {
			if prev.name == op.name {
				s.fail(op.src, op.line, "--%s is already set in %s", op.name, prev.origin())
			} else {
				s.fail(op.src, op.line, "--%s and --%s in %s are mutually exclusive", op.name, prev.name, prev.origin())
			}
			continue
		}
		set[g] = op
		if err := s.fs.Set(op.name, op.value); err != nil {
			s.fail(op.src, op.line, "invalid value %q for --%s", op.value, op.name)
			continue
		}
		origin["--"+op.name] = op.origin()
	}
	// errors by file (in the order first reported), then by line
	rank := map[string]int{}
	for i, e := range s.errs {
		if _, ok := rank[e.Source]; !ok {
			rank[e.Source] = i
		}
	}
	slices.SortStableFunc(s.errs, func(a, b *dsl.Error) int {
		return cmp.Or(cmp.Compare(rank[a.Source], rank[b.Source]), cmp.Compare(a.Line, b.Line))
	})
	return origin
}

// splitOption reads an option line: --name, then nothing, or = or
// whitespace and a value. A value in double quotes may hold anything, with
// \" and \\ escaped; otherwise it is the rest of the line.
func splitOption(line string) (name, value string, hasValue bool, err error) {
	rest := line[2:]
	n := strings.IndexFunc(rest, func(r rune) bool { return r == '=' || unicode.IsSpace(r) })
	if n < 0 {
		n = len(rest)
	}
	name, rest = rest[:n], strings.TrimSpace(rest[n:])
	if !optNameRE.MatchString(name) {
		return "", "", false, fmt.Errorf("invalid option line %q: use --name = value", line)
	}
	if rest == "" {
		return name, "", false, nil
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
	if strings.HasPrefix(rest, `"`) {
		value, err = unquoteValue(rest)
		if err != nil {
			err = fmt.Errorf("--%s: %w", name, err)
		}
		return name, value, true, err
	}
	return name, rest, true, nil
}

// unquoteValue reads a double-quoted value that ends the line.
func unquoteValue(s string) (string, error) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\'):
			i++
			b.WriteByte(s[i])
		case c == '"':
			if after := strings.TrimSpace(s[i+1:]); after != "" {
				return "", fmt.Errorf("text after the closing quote: %s", after)
			}
			return b.String(), nil
		default:
			b.WriteByte(c)
		}
	}
	return "", errors.New("the closing quote is missing")
}

// relTo returns path as seen from the directory of file.
func relTo(file, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(file), path)
}

var flagRE = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

// fromFile prefixes an error about an option set in a file with where it
// was set ("plan.dates:3: argument --paper: …"); the first option the
// message names decides.
func fromFile(err error, origin map[string]string) error {
	at := origin[flagRE.FindString(err.Error())]
	if at == "" {
		return err
	}
	var ue usageError
	if errors.As(err, &ue) {
		return usageError{at + ": " + ue.msg}
	}
	return fmt.Errorf("%s: %w", at, err)
}
