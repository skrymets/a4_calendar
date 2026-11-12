package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"
)

const (
	defaultLetters = "MTWTFSS"
	defaultMonths  = "JAN,FEB,MAR,APR,MAY,JUN,JUL,AUG,SEP,OCT,NOV,DEC"
)

// options holds raw flag values. Values that need converting (int, float,
// date, choices) are kept as strings and validated in configure.
type options struct {
	start, period, end, until, output                     string
	weekMode, weekStart, weekLabels, weekend              string
	title, dayLetters, monthNames, font, paper, noteLines string
	workdayColor, weekendColor, margin, header            string
	noFooter, noNoteGuide, hideOutside, version, licenses bool

	// timeline (docs/timeline, REQ-CLI-*)
	files, milestones, intervals        []string
	today, tlLayout, tlCompact, tlLanes string
	tlGroups, tlLegend, debugJSON       string
}

// newFlagSet defines the CLI and returns a function rendering the help text.
func newFlagSet() (*pflag.FlagSet, *options, func() string) {
	o := &options{}
	fs := pflag.NewFlagSet("yearly-calendar", pflag.ContinueOnError)
	fs.SetOutput(io.Discard) // run() prints errors and help itself
	fs.Usage = func() {}
	fs.SortFlags = false

	rows := [][2]string{{"-h, --help", "show this help message and exit"}}
	str := func(p *string, name, short, def, metavar, help string) {
		fs.StringVarP(p, name, short, def, help)
		left := "--" + name + " " + metavar
		if short != "" {
			left = "-" + short + " " + metavar + ", " + left
		}
		rows = append(rows, [2]string{left, help})
	}
	flag := func(p *bool, name, help string) {
		fs.BoolVar(p, name, false, help)
		rows = append(rows, [2]string{"--" + name, help})
	}

	// repeatable; StringArray keeps commas inside a declaration intact
	list := func(p *[]string, name, metavar, help string) {
		fs.StringArrayVar(p, name, nil, help)
		rows = append(rows, [2]string{"--" + name + " " + metavar, help})
	}

	str(&o.start, "start", "", "", "START", "first day of the period: YYYY-MM-DD or a date expression such as @kickoff or today+1w (default: --today)")
	str(&o.period, "period", "", "1y", "PERIOD", "length of the period: <N>d, <N>w, <N>m or 1y (default 1y, max 1 year)")
	str(&o.end, "end", "", "", "END", "last day of the period, a date as for --start; instead of --period")
	str(&o.until, "until", "", "", "UNTIL", "the day after the period, a date as for --start; instead of --period")
	str(&o.output, "output", "o", "", "OUTPUT", "output PDF path")
	str(&o.weekMode, "week-mode", "", "relative", "{relative,iso,rolling}",
		"relative: w01 = calendar week containing start (default); iso: ISO-8601 week of year; rolling: blocks of 7 days from start")
	str(&o.weekStart, "week-start", "", "0", "WEEK_START", "first weekday for 'relative' mode, 0=Mon..6=Sun (default 0)")
	str(&o.weekLabels, "week-labels", "", "first", "{all,first}", "show week number on every day, or only on the week's first day")
	str(&o.weekend, "weekend", "", "5,6", "WEEKEND", "weekday indexes to shade (default 5,6 = Sat,Sun)")
	str(&o.title, "title", "", "", "TITLE",
		"page title; truncated with '...' if too long (default: YEARLY CALENDAR for 1y, else e.g. 90-DAY CALENDAR)")
	str(&o.dayLetters, "day-letters", "", defaultLetters, "DAY_LETTERS", "7 letters, Monday first")
	str(&o.monthNames, "month-names", "", defaultMonths, "MONTH_NAMES", "12 comma-separated names")
	str(&o.font, "font", "", "", "FONT",
		"path to a .ttf (needed for Cyrillic etc.), or 'pt-sans-narrow' for the built-in PT Sans Narrow")
	str(&o.paper, "paper", "", "a4", "{a4,letter}", "")
	flag(&o.noFooter, "no-footer", "")
	flag(&o.noNoteGuide, "no-note-guide", "disable the dotted hairline note guide (enabled by default)")
	str(&o.noteLines, "note-lines", "", "1", "NOTE_LINES", "number of dotted guide lines per cell (default 1)")
	str(&o.workdayColor, "workday-color", "", "#1A3A7A", "WORKDAY_COLOR", "weekday-letter colour, workdays")
	str(&o.weekendColor, "weekend-color", "", "#8E1B1B", "WEEKEND_COLOR", "weekday-letter colour, weekends")
	str(&o.margin, "margin", "", "10", "MARGIN", "page margin in mm (default 10)")
	str(&o.header, "header", "", "compact", "{compact,full}", "compact: small one-line title, slim month row (default)")
	flag(&o.hideOutside, "hide-outside", "leave days outside the period blank instead of faint")
	list(&o.files, "file", "PATH", "read milestones, intervals and options from a .dates file (repeatable; a path without --file is read the same way)")
	list(&o.milestones, "milestone", "DECL", `add a milestone, e.g. 'release=2027-03-15;title="Release"' (repeatable)`)
	list(&o.intervals, "interval", "DECL", "add an interval, e.g. 'qa=@release-2w/@release' (repeatable)")
	str(&o.today, "today", "", "", "TODAY", "evaluation date for 'today' in declarations, YYYY-MM-DD (default: today)")
	str(&o.tlLayout, "timeline-layout", "", "packed", "{packed,flow}",
		"packed: interval lanes chosen per month (default); flow: lanes kept across months, closed up daily or weekly")
	str(&o.tlCompact, "timeline-compact", "", "daily", "{daily,weekly}",
		"flow layout: close up lanes every day (default) or on the --week-start day")
	str(&o.tlLanes, "timeline-lanes", "", "auto", "LANES",
		"auto (default) or 1-12: interval lanes per column before the overflow rail")
	str(&o.tlGroups, "timeline-groups", "", "auto", "{auto,merge,separate}",
		"draw intervals with the same group= as one rail; auto: merge on narrow pages only (default)")
	str(&o.tlLegend, "timeline-legend", "", "full", "{full,off}", "list milestones and intervals under the grid (default full)")
	fs.StringVar(&o.debugJSON, "debug-timeline-json", "", "") // hidden: layout decisions as JSON (docs/timeline/05 §1)
	flag(&o.version, "version", "print the version and exit")
	flag(&o.licenses, "licenses", "print the licences of the embedded font and bundled libraries and exit")

	help := func() string {
		var b strings.Builder
		b.WriteString("usage: yearly-calendar [options] [FILE.dates ...]\n\nGenerate a one-page yearly calendar PDF.\n\noptions:\n")
		for _, r := range rows {
			switch {
			case r[1] == "":
				fmt.Fprintf(&b, "  %s\n", r[0])
			case len(r[0]) <= 22:
				fmt.Fprintf(&b, "  %-22s  %s\n", r[0], r[1])
			default:
				fmt.Fprintf(&b, "  %s\n  %-22s  %s\n", r[0], "", r[1])
			}
		}
		return b.String()
	}
	return fs, o, help
}
