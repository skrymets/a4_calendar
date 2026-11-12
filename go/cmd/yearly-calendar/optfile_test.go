package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"a4calendar/internal/period"
)

// writeFiles writes name → text into dir and returns dir.
func writeFiles(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSplitOption(t *testing.T) {
	cases := []struct {
		line, name, value string
		hasValue          bool
		err               string
	}{
		{"--start = 2027-01-01", "start", "2027-01-01", true, ""},
		{"--start=2027-01-01", "start", "2027-01-01", true, ""},
		{"--start 2027-01-01", "start", "2027-01-01", true, ""},
		{"--start\t=\t@kick+1w", "start", "@kick+1w", true, ""},
		{"--hide-outside", "hide-outside", "", false, ""},
		{"--hide-outside = false", "hide-outside", "false", true, ""},
		{"--title = Prepare for Exams", "title", "Prepare for Exams", true, ""},
		{`--title = "Plan #2; \"final\" \\ ok"`, "title", `Plan #2; "final" \ ok`, true, ""},
		{`--title ""`, "title", "", true, ""},
		{"--title =", "title", "", true, ""},
		{`--title = "x" y`, "", "", false, "--title: text after the closing quote: y"},
		{`--title = "x`, "", "", false, "--title: the closing quote is missing"},
		{"--Start = x", "", "", false, `invalid option line "--Start = x"`},
		{"-- start", "", "", false, "invalid option line"},
	}
	for _, c := range cases {
		name, value, hasValue, err := splitOption(c.line)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%q: err %v; want %q", c.line, err, c.err)
			}
			continue
		}
		if err != nil || name != c.name || value != c.value || hasValue != c.hasValue {
			t.Errorf("%q = %q, %q, %v, %v; want %q, %q, %v", c.line, name, value, hasValue, err, c.name, c.value, c.hasValue)
		}
	}
}

// A plan file carries the options; paths in it are relative to it, wherever
// the command runs; a path without a flag is a --file.
func TestRunOptionFile(t *testing.T) {
	dir := writeFiles(t, t.TempDir(), map[string]string{
		"plans/all.dates": "# the whole command\n--start = @kick\n--until = @exam+1w\n--title \"Plan #2; final\"\n" +
			"--paper letter\n--hide-outside\n--output = out/plan.pdf\n--file = common.dates\nexam = @kick+8w; title=\"Exam\"\n",
		"plans/common.dates": "kick = 2027-01-04; title=\"Kick-off\"\n",
		"plans/out/.keep":    "",
	})
	t.Chdir(t.TempDir())
	plan := filepath.Join(dir, "plans", "all.dates")
	code, stdout, stderr := call(plan)
	want := "Wrote " + filepath.Join(dir, "plans", "out", "plan.pdf") + ": 2027-01-04 → 2027-03-07 (63 days)\n"
	if code != 0 || stdout != want {
		t.Fatalf("exit %d, stdout %q, stderr %q; want %q", code, stdout, stderr, want)
	}

	// the command line wins, --until over the file's --until and --end alike
	out := filepath.Join(t.TempDir(), "x.pdf")
	code, stdout, stderr = call("--file", plan, "--end", "2027-01-31", "-o", out)
	if code != 0 || !strings.Contains(stdout, out+": 2027-01-04 → 2027-01-31 (28 days)") {
		t.Errorf("CLI override: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRunOptionFileErrors(t *testing.T) {
	dir := writeFiles(t, t.TempDir(), map[string]string{
		"a.dates":        "--paper = a4\n--period 90d\n",
		"b.dates":        "--paper letter\n--end 2027-02-01\n",
		"bad.dates":      "--version\n--milestone x=2027-01-01\n--colour red\n--start\n--title \"x\" y\n--hide-outside maybe\n--file\n",
		"loop1.dates":    "--file loop2.dates\n",
		"loop2.dates":    "--file loop1.dates\n",
		"inc.dates":      "--file = sub/part.dates\nb = @a+1d\nc = @nope\n",
		"sub/part.dates": "a = 2027-01-01\n",
		"choice.dates":   "--paper a3\n",
		"start.dates":    "\n--start = @nope\n",
	})
	at := func(name string) string { return filepath.Join(dir, name) }
	out := filepath.Join(t.TempDir(), "x.pdf")
	cases := []struct {
		args  []string
		code  int
		lines []string
	}{
		{[]string{at("a.dates"), at("b.dates")}, 2, []string{
			at("b.dates") + ":1: --paper is already set in " + at("a.dates") + ":1",
			at("b.dates") + ":2: --end and --period in " + at("a.dates") + ":2 are mutually exclusive",
		}},
		{[]string{at("bad.dates")}, 2, []string{
			at("bad.dates") + ":1: --version cannot be set in a file",
			at("bad.dates") + ":2: write the declaration itself, without --milestone",
			at("bad.dates") + ":3: unknown option --colour",
			at("bad.dates") + ":4: --start needs a value",
			at("bad.dates") + ":5: --title: text after the closing quote: y",
			at("bad.dates") + `:6: invalid value "maybe" for --hide-outside`,
			at("bad.dates") + ":7: --file needs a path",
		}},
		{[]string{at("loop1.dates")}, 2, []string{
			at("loop2.dates") + ":1: " + at("loop1.dates") + " is already read (from the command line)",
		}},
		{[]string{at("a.dates"), "--file", at("a.dates")}, 2, []string{
			"Error: --file " + at("a.dates") + ": the file is already read (from the command line)",
		}},
		// declarations after an included file keep their line numbers
		{[]string{at("inc.dates")}, 2, []string{
			at("inc.dates") + ":3: unknown name @nope (only milestones and intervals declared earlier can be referenced)",
		}},
		{[]string{at("choice.dates")}, 2, []string{
			"Error: " + at("choice.dates") + ":1: argument --paper: invalid choice: 'a3' (choose from 'a4', 'letter')",
		}},
		{[]string{at("start.dates")}, 2, []string{
			"Error: " + at("start.dates") + ":2: argument --start: unknown name @nope",
		}},
		{[]string{at("missing.dates")}, 1, []string{"Error: cannot read --file: open " + at("missing.dates")}},
	}
	for _, c := range cases {
		code, _, stderr := call(append(c.args, "-o", out)...)
		got := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
		if code != c.code || len(got) != len(c.lines) {
			t.Errorf("%v: exit %d, stderr %q; want exit %d with %d lines", c.args, code, stderr, c.code, len(c.lines))
			continue
		}
		for i, w := range c.lines {
			if !strings.HasPrefix(got[i], w) {
				t.Errorf("%v: line %d = %q; want prefix %q", c.args, i+1, got[i], w)
			}
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("%v: PDF written despite errors", c.args)
		}
	}
}

// A file that only sets options gives the PDF the same flags give, byte
// for byte (REQ-CLI-8): no timeline, no footer suffix, no Subject.
func TestRunOptionOnlyFileIsLikeFlags(t *testing.T) {
	dir := writeFiles(t, t.TempDir(), map[string]string{
		"opts.dates":  "--start = 2027-01-01\n--period = 90d\n--paper = letter\n# no declarations\n",
		"empty.dates": "",
	})
	render := func(args ...string) []byte {
		t.Helper()
		out := filepath.Join(t.TempDir(), "x.pdf")
		if code, _, stderr := call(append(args, "-o", out)...); code != 0 {
			t.Fatalf("%v: exit %d, %s", args, code, stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return regexp.MustCompile(`/(CreationDate|ModDate) \(D:[0-9]+\)`).ReplaceAll(data, nil)
	}
	flags := render("--start", "2027-01-01", "--period", "90d", "--paper", "letter")
	if !bytes.Equal(render(filepath.Join(dir, "opts.dates")), flags) {
		t.Error("an options-only file renders differently from the same flags")
	}
	if !bytes.Equal(render(filepath.Join(dir, "empty.dates"), "--start", "2027-01-01", "--period", "90d", "--paper", "letter"), flags) {
		t.Error("an empty .dates file changes the page")
	}
}

// The footer names the files that declare something, an included one too.
func TestRunFooterNamesDeclaringFiles(t *testing.T) {
	dir := writeFiles(t, t.TempDir(), map[string]string{
		"main.dates": "--start 2027-01-01\n--file = part.dates\n--file = opts.dates\n",
		"part.dates": "kick = 2027-01-04\n",
		"opts.dates": "--paper letter\n",
	})
	out := filepath.Join(t.TempDir(), "x.pdf")
	fs, _, _ := newFlagSet()
	files, err := loadFiles(fs, []string{filepath.Join(dir, "main.dates")})
	if err != nil || len(files.errs) > 0 {
		t.Fatal(err, files.errs)
	}
	in := readTimeline(&options{}, files.srcs, period.Date(2027, 1, 1))
	if got := in.declFiles(); len(got) != 1 || got[0] != filepath.Join(dir, "part.dates") {
		t.Errorf("declFiles = %q", got)
	}
	if code, _, stderr := call(filepath.Join(dir, "main.dates"), "-o", out); code != 0 {
		t.Errorf("exit %d, %s", code, stderr)
	}
}
