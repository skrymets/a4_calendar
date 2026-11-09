package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"a4calendar/internal/canvas"
	"a4calendar/internal/fonts"
	"a4calendar/internal/period"
	"a4calendar/internal/render"
)

//go:embed THIRD_PARTY_LICENSES.txt
var thirdPartyLicenses string

// usageError is a command-line error: exit code 2, like argparse's error().
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

func run(args []string, stdout, stderr io.Writer) int {
	fs, o, help := newFlagSet()
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			fmt.Fprint(stdout, help())
			return 0
		}
		fmt.Fprintf(stderr, "Error: %s\n", err)
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "Error: unrecognized arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if o.version {
		fmt.Fprintf(stdout, "yearly-calendar %s\n", version)
		return 0
	}
	if o.licenses {
		fmt.Fprint(stdout, "yearly-calendar "+version+"\n"+
			"Copyright (c) 2026 Sergii Krymets. Released under the MIT License;\n"+
			"the full text is in the LICENSE file of the source and of every release archive.\n\n"+
			"yearly-calendar embeds the PT Sans Narrow font, licensed under the\n"+
			"SIL Open Font License, Version 1.1:\n\n"+fonts.OFL+
			"\n\nThird-party software linked into this binary:\n\n"+thirdPartyLicenses)
		return 0
	}

	cfg, output, err := configure(o, fs.Changed("title"), fs.Changed("output"))
	if err == nil {
		var res render.Result
		if res, err = buildAndSave(cfg, output); err == nil {
			fmt.Fprintf(stdout, "Wrote %s: %s → %s (%d days)\n", output,
				res.Start.Format("2006-01-02"), res.End.Format("2006-01-02"), res.TotalDays)
			return 0
		}
	}
	fmt.Fprintf(stderr, "Error: %s\n", err)
	var ue usageError
	if errors.As(err, &ue) {
		return 2
	}
	return 1
}

// buildAndSave renders and saves the PDF; as a last line of defence it turns
// any panic from the PDF library into an ordinary error.
func buildAndSave(cfg render.Config, output string) (res render.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cannot render PDF: %v", r)
		}
	}()
	c := canvas.New(cfg.PaperW, cfg.PaperH)
	c.SetCreator(fmt.Sprintf("yearly-calendar %s (Go)", version))
	if res, err = render.Build(cfg, c); err != nil {
		return res, err
	}
	return res, c.Save(output)
}

// configure validates o in Python's order: argparse conversions and
// choices, then the 1-year maximum (both usage errors, exit 2), then the
// checks build() makes (exit 1).
func configure(o *options, titleSet, outputSet bool) (render.Config, string, error) {
	var cfg render.Config
	start, err := parseStart(o.start)
	if err != nil {
		return cfg, "", err
	}
	p, err := period.ParsePeriod(o.period)
	if err != nil {
		return cfg, "", usagef("argument --period: %s", err)
	}
	weekStart, err := parseInt("--week-start", o.weekStart)
	if err != nil {
		return cfg, "", err
	}
	noteLines, err := parseInt("--note-lines", o.noteLines)
	if err != nil {
		return cfg, "", err
	}
	margin, err := strconv.ParseFloat(strings.TrimSpace(o.margin), 64)
	if err != nil {
		return cfg, "", usagef("argument --margin: invalid float value: '%s'", o.margin)
	}
	for _, ch := range []struct {
		flag, val string
		allowed   []string
	}{
		{"--week-mode", o.weekMode, []string{"relative", "iso", "rolling"}},
		{"--week-labels", o.weekLabels, []string{"all", "first"}},
		{"--paper", o.paper, []string{"a4", "letter"}},
		{"--header", o.header, []string{"compact", "full"}},
	} {
		if !slices.Contains(ch.allowed, ch.val) {
			return cfg, "", usagef("argument %s: invalid choice: '%s' (choose from '%s')",
				ch.flag, ch.val, strings.Join(ch.allowed, "', '"))
		}
	}

	end, endErr := period.PeriodEnd(start, p)
	maxEnd := period.MaxEnd(start)
	if endErr != nil || end.After(maxEnd) {
		length := ""
		if endErr == nil {
			length = fmt.Sprintf(" (%d days)", period.DaysBetween(start, end)+1)
		}
		return cfg, "", usagef("--period %s%s exceeds the 1-year maximum (%d days from %s)",
			p, length, period.DaysBetween(start, maxEnd)+1, start.Format("2006-01-02"))
	}

	letters := []rune(o.dayLetters)
	if len(letters) != 7 {
		return cfg, "", errors.New("--day-letters must have exactly 7 characters (Mon..Sun)")
	}
	months := strings.Split(o.monthNames, ",")
	for i := range months {
		months[i] = strings.TrimSpace(months[i])
	}
	if len(months) != 12 {
		return cfg, "", errors.New("--month-names must have 12 comma-separated names")
	}
	weekend := map[int]bool{}
	for _, s := range strings.Split(o.weekend, ",") {
		wd, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return cfg, "", errors.New("--weekend must be comma-separated weekday indexes (0=Mon..6=Sun)")
		}
		weekend[wd] = true
	}
	font, err := loadFont(o.font)
	if err != nil {
		return cfg, "", err
	}
	workday, err := canvas.Hex(o.workdayColor)
	if err != nil {
		return cfg, "", fmt.Errorf("--workday-color: %w", err)
	}
	weekendColor, err := canvas.Hex(o.weekendColor)
	if err != nil {
		return cfg, "", fmt.Errorf("--weekend-color: %w", err)
	}

	title, output := o.title, o.output
	if !titleSet {
		title = render.DefaultTitle(p)
	}
	if !outputSet {
		output = render.DefaultOutput(start, p)
	}
	paperW, paperH := render.A4W, render.A4H
	if o.paper == "letter" {
		paperW, paperH = render.LetterW, render.LetterH
	}
	cfg = render.Config{
		Start: start, Period: p,
		WeekMode: o.weekMode, WeekStart: weekStart, WeekLabels: o.weekLabels,
		Weekend: weekend, Title: title, DayLetters: letters, MonthNames: months,
		Font: font, PaperW: paperW, PaperH: paperH,
		Footer: !o.noFooter, NoteGuide: !o.noNoteGuide, NoteLines: noteLines,
		WorkdayColor: workday, WeekendColor: weekendColor,
		MarginMM: margin, Header: o.header, ShowOutside: !o.hideOutside,
	}
	return cfg, output, nil
}

func parseStart(s string) (time.Time, error) {
	if s == "" {
		now := time.Now()
		return period.Date(now.Year(), now.Month(), now.Day()), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil || t.Year() < 1 {
		return time.Time{}, usagef("argument --start: invalid fromisoformat value: '%s'", s)
	}
	return period.Date(t.Year(), t.Month(), t.Day()), nil
}

func parseInt(flag, s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, usagef("argument %s: invalid int value: '%s'", flag, s)
	}
	return n, nil
}

// isTrueType reports whether data starts with a TrueType sfnt signature
// (0x00010000) or the legacy Apple 'true' tag.
func isTrueType(data []byte) bool {
	return len(data) >= 4 && (bytes.Equal(data[:4], []byte{0x00, 0x01, 0x00, 0x00}) || string(data[:4]) == "true")
}

func loadFont(spec string) (canvas.Font, error) {
	switch spec {
	case "":
		return canvas.Helvetica, nil
	case fonts.PTSansNarrowKeyword:
		return canvas.TTF("PTSansNarrow", fonts.PTSansNarrow), nil
	}
	data, err := os.ReadFile(spec)
	if err != nil {
		return canvas.Font{}, fmt.Errorf("cannot read --font: %w", err)
	}
	if !isTrueType(data) {
		return canvas.Font{}, fmt.Errorf("--font: '%s' is not a TrueType (.ttf) font", spec)
	}
	if err := canvas.CheckTTF(data); err != nil {
		return canvas.Font{}, fmt.Errorf("--font: '%s' is not a usable TrueType (.ttf) font", spec)
	}
	return canvas.TTF(strings.TrimSuffix(filepath.Base(spec), filepath.Ext(spec)), data), nil
}
