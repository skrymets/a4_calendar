package main

import (
	"cmp"
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

// timelineSources reads the DSL inputs in the order of REQ-CLI-1: files,
// then inline milestones, then inline intervals.
func timelineSources(o *options) ([]dsl.Source, error) {
	var srcs []dsl.Source
	for _, path := range o.files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read --file: %w", err)
		}
		srcs = append(srcs, dsl.Source{Name: path, Text: string(data)})
	}
	for i, d := range o.milestones {
		srcs = append(srcs, dsl.Source{Name: fmt.Sprintf("--milestone[%d]", i+1), Text: d, Expect: dsl.KindMilestone})
	}
	for i, d := range o.intervals {
		srcs = append(srcs, dsl.Source{Name: fmt.Sprintf("--interval[%d]", i+1), Text: d, Expect: dsl.KindInterval})
	}
	return srcs, nil
}

// loadTimeline parses, resolves and lays out the timeline for cfg. Without
// any input it returns nil, so the page stays as it was (REQ-CLI-8); the
// debug export is still written when asked for. DSL and key errors come back
// as dsl.Errors (REQ-ER-1); warnings go to stderr.
func loadTimeline(o *options, cfg render.Config, today, end time.Time, lanes int, stderr io.Writer) (*render.Timeline, error) {
	srcs, err := timelineSources(o)
	if err != nil || (len(srcs) == 0 && o.debugJSON == "") {
		return nil, err
	}
	// keys are checked on what parsed, so one run reports both kinds of error
	model, parseErr := dsl.Parse(srcs, today)
	decls, warnings, keyErr := timeline.ReadKeys(model)
	if parseErr != nil || keyErr != nil {
		return nil, joinErrors(srcs, parseErr, keyErr)
	}
	resolved, err := dsl.Expand(model, end)
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
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
	if len(srcs) == 0 {
		return nil, nil
	}
	return &render.Timeline{
		Layout: layout, Legend: o.tlLegend == "full",
		Files: o.files, Inline: len(o.milestones)+len(o.intervals) > 0,
		Milestones: len(model.Milestones), Intervals: len(model.Intervals),
	}, nil
}

// writeDebugJSON writes the layout decisions in the golden-file format
// (docs/timeline/05 §1). Options list start, end and today, plus layout,
// compact, cap and groups when they differ from the defaults.
func writeDebugJSON(o *options, srcs []dsl.Source, today time.Time, opts timeline.Options,
	d *timeline.Decls, r *dsl.Resolved, l *timeline.Layout) error {
	names := make([]string, len(srcs))
	for i, s := range srcs {
		names[i] = s.Name
	}
	info := timeline.DebugInfo{Fixture: strings.Join(names, ", "), Options: map[string]string{
		"start": opts.Start.Format(isoDate), "end": opts.End.Format(isoDate), "today": today.Format(isoDate),
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

// joinErrors merges dsl.Errors into source order: by source as loaded, then
// by line; errors on the same line keep their order.
func joinErrors(srcs []dsl.Source, errs ...error) dsl.Errors {
	rank := map[string]int{}
	for i, s := range srcs {
		rank[s.Name] = i
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
