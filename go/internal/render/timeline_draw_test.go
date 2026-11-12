package render

import (
	"strconv"
	"strings"
	"testing"

	"a4calendar/internal/dsl"
	"a4calendar/internal/period"
	"a4calendar/internal/timeline"
)

// buildTimeline renders a 5-month A4 page (wide columns) with the given
// declarations onto a fake canvas, without the legend.
func buildTimeline(t *testing.T, dates string) *fakeCanvas {
	t.Helper()
	return buildTimelineWith(t, dates, func(tl *Timeline) { tl.Legend = false })
}

// buildTimelineWith is buildTimeline with a hook to change the Timeline.
func buildTimelineWith(t *testing.T, dates string, edit func(*Timeline)) *fakeCanvas {
	t.Helper()
	return buildTimelinePage(t, period.Period{N: 5, Unit: 'm'}, dates, edit)
}

// buildTimelinePage renders an A4 page from 1 January 2027 for period p.
func buildTimelinePage(t *testing.T, p period.Period, dates string, edit func(*Timeline)) *fakeCanvas {
	t.Helper()
	start := period.Date(2027, 1, 1)
	cfg := testConfig(start, p)
	end, _ := period.PeriodEnd(start, cfg.Period)
	m, err := dsl.Parse([]dsl.Source{{Name: "t.dates", Text: dates}}, start)
	if err != nil {
		t.Fatal(err)
	}
	r, err := dsl.Expand(m, end)
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := timeline.ReadKeys(m)
	if err != nil {
		t.Fatal(err)
	}
	l := timeline.Build(d, r, timeline.Options{Start: start, End: end, GridWidth: cfg.GridWidth(),
		Weekend: cfg.Weekend, MonthNames: cfg.MonthNames})
	cfg.Timeline = &Timeline{Layout: l, Legend: true, Files: []string{"dir/t.dates"}, Milestones: len(m.Milestones), Intervals: len(m.Intervals)}
	if edit != nil {
		edit(cfg.Timeline)
	}
	c := &fakeCanvas{}
	if _, err := Build(cfg, c); err != nil {
		t.Fatal(err)
	}
	return c
}

func (f *fakeCanvas) count(op string) int {
	n := 0
	for _, d := range f.out {
		if d.op == op {
			n++
		}
	}
	return n
}

func TestDrawTimelineTexts(t *testing.T) {
	c := buildTimeline(t, `exam = 2027-02-19; title="Mock exam"; mark=ring; countdown=3; color=orange
call = 2027-02-04; title="Tutor call"; mark=dot
rev  = 2027-02-15/2w; title="Revision"; length=days; color=orange`)
	texts := strings.Join(append(c.texts("string"), c.texts("spaced")...), "|")
	for _, want := range []string{"Mock exam", "Tutor call", "REVISION · 14 D", "T–3", "T–1"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in %s", want, texts)
		}
	}
	if strings.Contains(texts, "T–4") {
		t.Error("countdown longer than countdown=3")
	}
	// one ring (stroked pill), one dot, a rail per covered day
	if n := c.count("roundrect-stroke"); n != 1 {
		t.Errorf("rings = %d; want 1", n)
	}
	if n := c.count("circle"); n != 1 {
		t.Errorf("dots = %d; want 1", n)
	}
	if n := c.count("roundrect"); n != 14 {
		t.Errorf("rail segments = %d; want 14", n)
	}
}

// A one-day interval is one rail segment, labelled, and named once in the
// legend.
func TestDrawOneDayInterval(t *testing.T) {
	const one = `o = 2027-02-10/2027-02-10; title="One"; length=days`
	if n := buildTimeline(t, one).count("roundrect"); n != 1 { // without the legend's swatch
		t.Errorf("rail segments = %d; want 1", n)
	}
	c := buildTimelineWith(t, one, nil)
	texts := strings.Join(append(c.texts("string"), c.texts("spaced")...), "|")
	for _, want := range []string{"ONE · 1 D", "One|10 Feb · 1 d"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in %s", want, texts)
		}
	}
}

func TestDrawTimelineClipsWithEllipsis(t *testing.T) {
	long := strings.Repeat("Very long milestone title ", 4)
	c := buildTimeline(t, `m = 2027-02-10; title="`+long+`"
x = 2027-03-01/3d; title="`+strings.Repeat("Long interval label ", 4)+`"`)
	var title, label string
	for _, d := range c.out {
		if strings.HasPrefix(d.text, "Very long") {
			title = d.text
		}
		if strings.HasPrefix(d.text, "LONG INTERVAL") {
			label = d.text
		}
	}
	if !strings.HasSuffix(title, "…") || len(title) >= len(long) {
		t.Errorf("title %q not clipped with …", title)
	}
	if !strings.HasSuffix(label, "…") {
		t.Errorf("label %q not clipped with …", label)
	}
}

// Narrow pages show milestone titles too, smaller and clipped (REQ-MS-3),
// but leave out a title that would keep fewer than three characters.
func TestNarrowPageMilestoneTitles(t *testing.T) {
	noLegend := func(tl *Timeline) { tl.Legend = false }
	titles := func(c *fakeCanvas) string { return strings.Join(c.texts("string"), "|") }

	eight := buildTimelinePage(t, period.Period{N: 8, Unit: 'm'}, `kickoff = 2027-01-05; title="Kick-off"`, noLegend)
	if !strings.Contains(titles(eight), "|Kick-off|") {
		t.Errorf("8 columns: no Kick-off title in %q", titles(eight))
	}

	year := buildTimelinePage(t, period.Period{N: 1, Unit: 'y'}, `m = 2027-01-05; title="Leave quota resets"`, noLegend)
	if !strings.Contains(titles(year), "|Leave q…|") {
		t.Errorf("12 columns: want the title cut to Leave q…")
	}

	// beside a full rail zone four letters stay; with two more marks
	// before it, fewer than three would, so the title is left out
	rails := "\na = 2027-01-01/1w\nb = 2027-01-01/1w\nc = 2027-01-01/1w\nd = 2027-01-01/1w"
	crowded := buildTimelinePage(t, period.Period{N: 1, Unit: 'y'}, `m = 2027-01-05; title="Leave quota resets"`+rails, noLegend)
	if !strings.Contains(titles(crowded), "|Leav…|") {
		t.Errorf("crowded cell: want the title cut to Leav…")
	}
	marks := buildTimelinePage(t, period.Period{N: 1, Unit: 'y'}, `m = 2027-01-05; title="Leave quota resets"
n = 2027-01-05
o = 2027-01-05`+rails, noLegend)
	if s := titles(marks); strings.Contains(s, "…") {
		t.Errorf("three marks: a title cut below three characters should be left out: %q", s[:80])
	}
	if n := marks.count("polygon"); n != 3 {
		t.Errorf("polygons = %d; want three diamonds", n)
	}
}

func TestDrawTimelineGlyphsAndHiddenDays(t *testing.T) {
	c := buildTimeline(t, `a = 2027-01-05; mark=diamond
b = 2027-01-05; mark=star
c = 2026-12-20; title="Before the period"`)
	if n := c.count("polygon"); n != 2 {
		t.Errorf("polygons = %d; want diamond + star", n)
	}
	if strings.Contains(strings.Join(c.texts("string"), "|"), "Before the period") {
		t.Error("milestone outside the period drawn")
	}
}

func entries(n int) []timeline.LegendEntry {
	es := make([]timeline.LegendEntry, n)
	for i := range es {
		es[i] = timeline.LegendEntry{Key: strconv.Itoa(i + 1), Title: "T", When: "1 Jan", Color: "#1F1F1F"}
	}
	return es
}

// REQ-LG-1, -5: 4 columns; 5 columns without dates when the grid rows
// would drop below 6 mm; then "+N more".
func TestPlanLegend(t *testing.T) {
	gridH := 31*lgMinRow + lgPad + 10*lgRowH + 0.01 // room for 10 rows
	cases := []struct {
		n, cols, rows, more int
		brief               bool
	}{
		{0, 0, 0, 0, false},
		{40, 4, 10, 0, false},
		{41, 5, 9, 0, true},
		{50, 5, 10, 0, true},
		{51, 5, 10, 2, true}, // 49 entries + "+2 more"
	}
	for _, c := range cases {
		p := planLegend(entries(c.n), gridH)
		if p.cols != c.cols || p.rows != c.rows || p.more != c.more || p.brief != c.brief {
			t.Errorf("%d entries: cols %d rows %d more %d brief %v; want %d %d %d %v",
				c.n, p.cols, p.rows, p.more, p.brief, c.cols, c.rows, c.more, c.brief)
		}
		if c.more > 0 && len(p.overflowLines(entries(c.n))) != c.n {
			t.Errorf("%d entries: overflow list incomplete", c.n)
		}
	}
	if p := planLegend(entries(500), 31*lgMinRow); p.rows != 1 || p.more != 496 {
		t.Errorf("no room: rows %d more %d; want one row and +496 more", p.rows, p.more)
	}
}

func TestFooterSubjectAndLegendOff(t *testing.T) {
	const dates = "kick = 2027-01-04; title=\"Kick-off\"\nx = 2027-01-05/3d; title=\"Build\""
	c := buildTimelineWith(t, dates, nil)
	footer := c.out[len(c.out)-1].text
	if want := " · timeline: t.dates — 1 milestone, 1 interval · layout: packed"; !strings.HasSuffix(footer, want) {
		t.Errorf("footer %q; want suffix %q", footer, want)
	}
	if c.subject != "timeline: t.dates" {
		t.Errorf("subject %q", c.subject)
	}
	texts := strings.Join(c.texts("string"), "|")
	for _, want := range []string{"Kick-off|4 Jan", "Build|5 Jan–7 Jan · 3 d"} {
		if !strings.Contains(texts, want) {
			t.Errorf("legend lacks %q", want)
		}
	}
	gridBottom := func(c *fakeCanvas) float64 { // y of the "31" row label in January
		for _, d := range c.out {
			if d.text == "031" {
				return d.y
			}
		}
		return 0
	}
	off := buildTimeline(t, dates)
	if strings.Contains(strings.Join(off.texts("string"), "|"), "5 Jan–7 Jan") {
		t.Error("legend drawn with --timeline-legend off")
	}
	if gridBottom(off) >= gridBottom(c) {
		t.Error("the legend did not take room from the grid")
	}
}

// A rail zone as wide as the cell leaves no note line rather than one
// drawn backwards over the day label.
func TestFullRailZoneHasNoNoteLine(t *testing.T) {
	start := period.Date(2027, 1, 1)
	cfg := testConfig(start, period.Period{N: 1, Unit: 'y'}) // 12 columns
	end, _ := period.PeriodEnd(start, cfg.Period)
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		b.WriteString("2027-03-01/20d\n")
	}
	m, err := dsl.Parse([]dsl.Source{{Name: "t.dates", Text: b.String()}}, start)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := dsl.Expand(m, end)
	d, _, _ := timeline.ReadKeys(m)
	cfg.Timeline = &Timeline{Layout: timeline.Build(d, r, timeline.Options{Start: start, End: end,
		GridWidth: cfg.GridWidth(), LaneCap: 12, Weekend: cfg.Weekend})}
	c := &fakeCanvas{}
	if _, err := Build(cfg, c); err != nil {
		t.Fatal(err)
	}
	for _, l := range c.out {
		if l.op == "line" && l.x2 < l.x {
			t.Fatalf("line drawn right to left: x %.2f → %.2f at y %.2f", l.x, l.x2, l.y)
		}
	}
}
