package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"a4calendar/internal/dsl"
	"a4calendar/internal/render"
	"a4calendar/internal/timeline"
)

// timelineChoices are the timeline options with a fixed set of values.
func timelineChoices(o *options) []choice {
	return []choice{
		{"--timeline-layout", o.tlLayout, []string{"packed", "flow"}},
		{"--timeline-compact", o.tlCompact, []string{"daily", "weekly"}},
		{"--timeline-groups", o.tlGroups, []string{"auto", "merge", "separate"}},
		{"--timeline-legend", o.tlLegend, []string{"full", "off"}},
	}
}

// parseLanes reads --timeline-lanes: "auto" (0) or 1-12 (REQ-CLI-5).
func parseLanes(s string) (int, error) {
	if s == "auto" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 12 {
		return 0, usagef("argument --timeline-lanes: invalid value: '%s' (use auto or 1-12)", s)
	}
	return n, nil
}

// timelineSources lists the DSL inputs in the order of REQ-CLI-1: files
// (as loadFiles read them), then inline milestones, then inline intervals.
func timelineSources(o *options, files []dsl.Source) []dsl.Source {
	srcs := slices.Clone(files)
	for i, d := range o.milestones {
		srcs = append(srcs, dsl.Source{Name: fmt.Sprintf("--milestone[%d]", i+1), Text: d, Expect: dsl.KindMilestone})
	}
	for i, d := range o.intervals {
		srcs = append(srcs, dsl.Source{Name: fmt.Sprintf("--interval[%d]", i+1), Text: d, Expect: dsl.KindInterval})
	}
	return srcs
}

// tlInput is the timeline input of one run: the sources, the declarations
// parsed from them and their rendering keys. err holds every DSL and key
// error (dsl.Errors, REQ-ER-1); it is reported after the other options
// are checked, unless a date option refers to a declaration with errors.
type tlInput struct {
	srcs     []dsl.Source
	today    time.Time
	model    *dsl.Model
	decls    *timeline.Decls
	warnings []timeline.Warning
	err      error
}

// readTimeline parses the declarations of the files and of the command
// line. Keys are checked on what parsed, so one run reports both kinds of
// error.
func readTimeline(o *options, files []dsl.Source, today time.Time) *tlInput {
	srcs := timelineSources(o, files)
	in := &tlInput{srcs: srcs, today: today}
	var parseErr, keyErr error
	in.model, parseErr = dsl.Parse(srcs, today)
	in.decls, in.warnings, keyErr = timeline.ReadKeys(in.model)
	if parseErr != nil || keyErr != nil {
		in.err = joinErrors(srcs, parseErr, keyErr)
	}
	return in
}

// declFiles lists the files that declare something, in reading order: an
// included file part way through another comes after it.
func (in *tlInput) declFiles() []string {
	used := map[string]bool{}
	for _, m := range in.model.Milestones {
		used[m.Source] = true
	}
	for _, iv := range in.model.Intervals {
		used[iv.Source] = true
	}
	var files []string
	for _, s := range in.srcs {
		if s.Expect == dsl.KindAny && used[s.Name] && !slices.Contains(files, s.Name) {
			files = append(files, s.Name)
		}
	}
	return files
}

// date resolves the value of a date option (--start, --end, --until): any
// date expression, resolved against every declaration. One that refers to
// a declaration with errors reports the declaration errors instead.
func (in *tlInput) date(flag, expr string) (time.Time, error) {
	t, err := in.model.Date(expr)
	switch {
	case errors.Is(err, dsl.ErrBroken):
		return t, in.err
	case err != nil:
		return t, usagef("argument %s: %v", flag, err)
	}
	return t, nil
}

// loadTimeline expands and lays out the timeline for cfg. Without any
// declaration it returns nil, so the page stays as it was (REQ-CLI-8), also
// for files that only set options; the debug export is still written when
// asked for. DSL and key errors come back as dsl.Errors (REQ-ER-1);
// warnings go to stderr.
func loadTimeline(o *options, in *tlInput, cfg render.Config, end time.Time, lanes int, stderr io.Writer) (*render.Timeline, error) {
	srcs, model, decls, today := in.srcs, in.model, in.decls, in.today
	if in.err != nil {
		return nil, in.err
	}
	empty := len(model.Milestones)+len(model.Intervals) == 0
	if empty && o.debugJSON == "" {
		return nil, nil
	}
	resolved, err := dsl.Expand(model, end)
	if err != nil {
		return nil, err
	}
	for _, w := range in.warnings {
		fmt.Fprintln(stderr, w)
	}

	opts := timeline.Options{
		Start: cfg.Start, End: end, GridWidth: cfg.GridWidth(), LaneCap: lanes,
		Flow: o.tlLayout == "flow", WeeklyCompact: o.tlCompact == "weekly",
		WeekStart: cfg.WeekStart, Weekend: cfg.Weekend, MonthNames: cfg.MonthNames,
	}
	switch o.tlGroups {
	case "merge":
		opts.Groups = timeline.GroupsMerge
	case "separate":
		opts.Groups = timeline.GroupsSeparate
	}
	layout := timeline.Build(decls, resolved, opts)

	if o.debugJSON != "" {
		if err := writeDebugJSON(o, srcs, today, opts, decls, resolved, layout); err != nil {
			return nil, err
		}
	}
	if empty {
		return nil, nil
	}
	return &render.Timeline{
		Layout: layout, Legend: o.tlLegend == "full",
		Files: in.declFiles(), Inline: len(o.milestones)+len(o.intervals) > 0,
		Milestones: len(model.Milestones), Intervals: len(model.Intervals),
	}, nil
}

// writeDebugJSON writes the layout decisions in the golden-file format
// (docs/timeline/05 §1). Options list start, end and today, plus layout,
// compact, cap and groups when they differ from the defaults.
func writeDebugJSON(o *options, srcs []dsl.Source, today time.Time, opts timeline.Options,
	d *timeline.Decls, r *dsl.Resolved, l *timeline.Layout) error {
	var names []string // a file read in parts is named once
	for _, s := range srcs {
		if !slices.Contains(names, s.Name) {
			names = append(names, s.Name)
		}
	}
	info := timeline.DebugInfo{Fixture: strings.Join(names, ", "), Options: map[string]string{
		"start": opts.Start.Format(time.DateOnly), "end": opts.End.Format(time.DateOnly), "today": today.Format(time.DateOnly),
	}}
	if opts.Flow {
		info.Options["layout"] = "flow"
		if opts.WeeklyCompact {
			info.Options["compact"] = "weekly"
		}
	}
	if opts.LaneCap > 0 {
		info.Options["cap"] = strconv.Itoa(opts.LaneCap)
	}
	if o.tlGroups != "auto" {
		info.Options["groups"] = o.tlGroups
	}
	out, err := timeline.DebugJSON(info, d, r, l)
	if err == nil {
		err = os.WriteFile(o.debugJSON, append(out, '\n'), 0o644)
	}
	if err != nil {
		return fmt.Errorf("cannot write --debug-timeline-json: %w", err)
	}
	return nil
}

// joinErrors merges dsl.Errors into source order: by source as first
// loaded (a file read in parts counts once), then by line; errors on the
// same line keep their order.
func joinErrors(srcs []dsl.Source, errs ...error) dsl.Errors {
	rank := map[string]int{}
	for i, s := range srcs {
		if _, ok := rank[s.Name]; !ok {
			rank[s.Name] = i
		}
	}
	var all dsl.Errors
	for _, err := range errs {
		if es, ok := err.(dsl.Errors); ok {
			all = append(all, es...)
		}
	}
	slices.SortStableFunc(all, func(a, b *dsl.Error) int {
		return cmp.Or(cmp.Compare(rank[a.Source], rank[b.Source]), cmp.Compare(a.Line, b.Line))
	})
	return all
}

// printErrors prints DSL and key errors one per line, as REQ-ER-1 asks.
func printErrors(w io.Writer, es dsl.Errors) {
	for _, e := range es {
		fmt.Fprintln(w, e)
	}
}
