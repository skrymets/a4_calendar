package render

import (
	"strings"
	"testing"

	"a4calendar/internal/dsl"
	"a4calendar/internal/period"
	"a4calendar/internal/timeline"
)

// buildTimeline renders a 5-month A4 page (wide columns) with the given
// declarations onto a fake canvas.
func buildTimeline(t *testing.T, dates string) *fakeCanvas {
	t.Helper()
	start := period.Date(2027, 1, 1)
	cfg := testConfig(start, period.Period{N: 5, Unit: 'm'})
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
	cfg.Timeline = &Timeline{Layout: l}
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
