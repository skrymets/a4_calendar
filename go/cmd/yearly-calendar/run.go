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
	"a4calendar/internal/dsl"
	"a4calendar/internal/fonts"
	"a4calendar/internal/period"
	"a4calendar/internal/render"
)

//go:embed THIRD_PARTY_LICENSES.txt
var thirdPartyLicenses string

// usageError is a command-line error: exit code 2.
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
		if slices.ContainsFunc(fs.Args(), func(a string) bool { return strings.ContainsAny(a, `";`) }) {
			fmt.Fprintln(stderr, "Hint: a --milestone or --interval value was split at a space. Windows PowerShell 5.1 "+
				"does this to values with double quotes unless --% comes before the arguments; or use --file.")
		}
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

	cfg, output, err := configure(o, fs.Changed, stderr)
	var dslErrs dsl.Errors
	if errors.As(err, &dslErrs) {
		printErrors(stderr, dslErrs)
		return 2
	}
	if err == nil {
		var res render.Result
		if res, err = buildAndSave(cfg, output); err == nil {
			if len(res.LegendOverflow) > 0 { // REQ-LG-5
				fmt.Fprintln(stderr, "Warning: the legend does not fit the page; all entries:")
				for _, l := range res.LegendOverflow {
					fmt.Fprintln(stderr, "  "+l)
				}
			}
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
	c.SetCreator(fmt.Sprintf("yearly-calendar %s", version))
	if res, err = render.Build(cfg, c); err != nil {
		return res, err
	}
	return res, c.Save(output)
}

// choice is an option with a fixed set of values.
type choice struct {
	flag, val string
	allowed   []string
}

// configure validates o in a fixed order: conversions and
// choices, then the 1-year maximum (both usage errors, exit 2), then the
// checks build() makes (exit 1), then the timeline inputs (dsl.Errors,
// exit 2). changed reports whether a flag was given.
func configure(o *options, changed func(string) bool, stderr io.Writer) (render.Config, string, error) {
	var cfg render.Config
	today, err := parseDate("--today", o.today, time.Time{})
	if err != nil {
		return cfg, "", err
	}
	start, err := parseDate("--start", o.start, today)
	if err != nil {
		return cfg, "", err
	}
	lengths := 0
	for _, f := range []string{"period", "end", "until"} {
		if changed(f) {
			lengths++
		}
	}
	if lengths > 1 {
		return cfg, "", usagef("--period, --end and --until are mutually exclusive")
	}
	p, err := period.ParsePeriod(o.period)
	if err != nil {
		return cfg, "", usagef("argument --period: %s", err)
	}
	endFlag, endText := "", ""
	switch {
	case changed("end"):
		endFlag, endText = "--end", o.end
	case changed("until"):
		endFlag, endText = "--until", o.until
	}
	var endDate time.Time
	if endFlag != "" {
		if endText == "" {
			return cfg, "", usagef("argument %s: expected a date", endFlag)
		}
		if endDate, err = parseDate(endFlag, endText, time.Time{}); err != nil {
			return cfg, "", err
		}
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
	choices := append([]choice{
		{"--week-mode", o.weekMode, []string{"relative", "iso", "rolling"}},
		{"--week-labels", o.weekLabels, []string{"all", "first"}},
		{"--paper", o.paper, []string{"a4", "letter"}},
		{"--header", o.header, []string{"compact", "full"}},
	}, timelineChoices(o)...)
	for _, ch := range choices {
		if !slices.Contains(ch.allowed, ch.val) {
			return cfg, "", usagef("argument %s: invalid choice: '%s' (choose from '%s')",
				ch.flag, ch.val, strings.Join(ch.allowed, "', '"))
		}
	}
	lanes, err := parseLanes(o.tlLanes)
	if err != nil {
		return cfg, "", err
	}
	if changed("timeline-compact") && o.tlLayout != "flow" { // REQ-CLI-4
		fmt.Fprintln(stderr, "Warning: --timeline-compact is ignored with --timeline-layout packed")
	}

	end, err := pageEnd(start, &p, endFlag, endText, endDate)
	if err != nil {
		return cfg, "", err
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
	if !changed("title") {
		title = render.DefaultTitle(p)
	}
	if !changed("output") {
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
	cfg.Timeline, err = loadTimeline(o, cfg, today, end, lanes, stderr)
	return cfg, output, err
}

// pageEnd returns the last day of the page. Given --end or --until (flag,
// as written in text, resolved to date) it sets *p to the matching period
// (period.FromEnd); otherwise the end follows from *p. Either way the page
// must start no later than it ends and stay within one year.
func pageEnd(start time.Time, p *period.Period, flag, text string, date time.Time) (time.Time, error) {
	var (
		end  time.Time
		err  error
		what = flag + " " + text
	)
	switch flag {
	case "":
		end, err = period.PeriodEnd(start, *p)
		what = "--period " + p.String()
	case "--end":
		end = date
	case "--until":
		end = period.AddDays(date, -1)
	}
	if flag != "" && end.Before(start) {
		rel := "is before"
		if flag == "--until" {
			rel = "is not after"
		}
		return end, usagef("argument %s: %s %s the start %s", flag, text, rel, start.Format(time.DateOnly))
	}
	maxEnd := period.MaxEnd(start)
	if err != nil || end.After(maxEnd) {
		length := ""
		if err == nil {
			length = fmt.Sprintf(" (%d days)", period.DaysBetween(start, end)+1)
		}
		return end, usagef("%s%s exceeds the 1-year maximum (%d days from %s)",
			what, length, period.DaysBetween(start, maxEnd)+1, start.Format(time.DateOnly))
	}
	if flag != "" {
		*p = period.FromEnd(start, end)
	}
	return end, nil
}

// parseDate reads a YYYY-MM-DD option; empty means def, or the local
// date when def is zero.
func parseDate(flag, s string, def time.Time) (time.Time, error) {
	if s == "" {
		if def.IsZero() {
			now := time.Now()
			def = period.Date(now.Year(), now.Month(), now.Day())
		}
		return def, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil || t.Year() < 1 {
		return time.Time{}, usagef("argument %s: invalid fromisoformat value: '%s'", flag, s)
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
