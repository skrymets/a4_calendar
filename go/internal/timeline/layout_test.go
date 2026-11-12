package timeline

import (
	"strings"
	"testing"
	"time"
)

// layoutOf builds text on an A4 page for start..end (YYYY-MM-DD).
func layoutOf(t *testing.T, text, start, end string, tweak func(*Options)) *Layout {
	t.Helper()
	o := Options{Start: d(start), End: d(end), GridWidth: a4Grid, Weekend: defaultWeekend}
	if tweak != nil {
		tweak(&o)
	}
	_, _, l := build(t, "t.dates", text, o, today)
	return l
}

func d(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

// day returns the Day of date s.
func day(t *testing.T, l *Layout, s string) *Day {
	t.Helper()
	date := d(s)
	for _, c := range l.Columns {
		if c.Year == date.Year() && c.Month == date.Month() {
			return c.Days[date.Day()-1]
		}
	}
	t.Fatalf("no column for %s", s)
	return nil
}

func column(t *testing.T, l *Layout, month time.Month) *Column {
	t.Helper()
	for _, c := range l.Columns {
		if c.Month == month {
			return c
		}
	}
	t.Fatalf("no column %s", month)
	return nil
}

func tops(dy *Day) string {
	var s []string
	for _, it := range dy.Top {
		s = append(s, it.Text)
	}
	return strings.Join(s, "|")
}

func TestPageClasses(t *testing.T) {
	cases := []struct {
		start, end string
		grid       float64
		narrow     bool
		laneCap    int
	}{
		{"2027-01-01", "2027-01-31", a4Grid, false, 6},     // 1 column
		{"2027-01-01", "2027-05-31", a4Grid, false, 6},     // 5
		{"2027-01-01", "2027-06-30", a4Grid, false, 4},     // 6: 31.67 mm
		{"2027-01-01", "2027-07-31", a4Grid, false, 4},     // 7: 27.1 mm
		{"2027-01-01", "2027-08-31", a4Grid, true, 3},      // 8
		{"2027-01-01", "2027-12-31", a4Grid, true, 3},      // 12
		{"2026-11-11", "2027-11-10", a4Grid, true, 3},      // 13
		{"2027-01-01", "2027-07-31", 195.9 * mm, false, 4}, // Letter, 7: 28.0 mm
		{"2027-01-01", "2027-08-31", 195.9 * mm, true, 3},  // Letter, 8
	}
	for _, c := range cases {
		l := layoutOf(t, "m=2027-01-05", c.start, c.end, func(o *Options) { o.GridWidth = c.grid })
		if l.Narrow != c.narrow || l.LaneCap != c.laneCap {
			t.Errorf("%s..%s (%.0f pt): narrow=%v cap=%d; want %v, %d", c.start, c.end, c.grid, l.Narrow, l.LaneCap, c.narrow, c.laneCap)
		}
	}
	l := layoutOf(t, "m=2027-01-05", "2027-01-01", "2027-12-31", func(o *Options) { o.LaneCap = 8 })
	if l.LaneCap != 8 {
		t.Errorf("--timeline-lanes 8: cap %d", l.LaneCap)
	}
}

func TestRailCapsAndContinuation(t *testing.T) {
	l := layoutOf(t, "a=2027-01-30/2027-02-02\nb=2027-02-03/2027-02-04", "2027-01-01", "2027-05-31", nil)
	jan30, jan31, feb1, feb2 := day(t, l, "2027-01-30"), day(t, l, "2027-01-31"), day(t, l, "2027-02-01"), day(t, l, "2027-02-02")
	if r := jan30.Rails[0]; !r.TopCap || r.BotCap {
		t.Errorf("start day: %+v", r)
	}
	if r := jan31.Rails[0]; r.TopCap || r.BotCap {
		t.Errorf("month end continues flush: %+v", r)
	}
	if r := feb1.Rails[0]; r.TopCap || r.BotCap {
		t.Errorf("day 1 continues flush: %+v", r)
	}
	if r := feb2.Rails[0]; r.TopCap || !r.BotCap {
		t.Errorf("last day (the END, included): %+v", r)
	}
	feb3 := day(t, l, "2027-02-03")
	if len(feb3.Rails) != 1 || !feb3.Rails[0].TopCap {
		t.Errorf("back-to-back: Feb 3 holds only b, capped: %+v", feb3.Rails)
	}
	// clipped by the period start: no cap at the clipped end
	l = layoutOf(t, "a=2027-01-10/2027-01-19", "2027-01-15", "2027-06-14", nil)
	if r := day(t, l, "2027-01-15").Rails[0]; r.TopCap {
		t.Errorf("clipped start must not be capped: %+v", r)
	}
	// interval ending exactly on the period end
	l = layoutOf(t, "a=2027-05-25/2027-05-31", "2027-01-01", "2027-05-31", nil)
	if r := day(t, l, "2027-05-31").Rails[0]; !r.BotCap {
		t.Errorf("last drawn day = period end: %+v", r)
	}
}

func TestPackingPriorityAndSpans(t *testing.T) {
	text := `a=2027-03-01/2027-03-09;priority=low
b=2027-03-02/2027-03-11
c=2027-03-03/2027-03-07;priority=high
d=2027-03-20/2027-03-24`
	l := layoutOf(t, text, "2027-01-01", "2027-05-31", func(o *Options) { o.LaneCap = 2 })
	mar5 := day(t, l, "2027-03-05")
	if len(mar5.Rails) != 2 || mar5.OverflowLane != 2 {
		t.Fatalf("Mar 5: %d rails, overflow lane %d; want 2 rails and the overflow rail in lane 2", len(mar5.Rails), mar5.OverflowLane)
	}
	// high (c) takes lane 0, normal (b) lane 1, low (a) overflows
	if got := tops(day(t, l, "2027-03-01")); got != "A|+1" {
		t.Errorf("Mar 1 top row = %q; want label A and +1 (a is overflowed from its first day)", got)
	}
	if col := column(t, l, time.March); col.K != 3 || col.Labels != TextLabels {
		t.Errorf("March: k=%d labels=%v; want 3 incl. overflow, text", col.K, col.Labels)
	}
	// d reuses lane 0 after c ended: a unit occupies only its spans
	if r := day(t, l, "2027-03-21").Rails; len(r) != 1 || r[0].Lane != 0 {
		t.Errorf("Mar 21 rails = %+v", r)
	}
}

func TestOverflowChipOnlyWhenCountChanges(t *testing.T) {
	text := "a=2027-03-01/2027-03-19\nb=2027-03-01/2027-03-19\nc=2027-03-02/2027-03-09\nd=2027-03-05/2027-03-11"
	l := layoutOf(t, text, "2027-01-01", "2027-12-31", nil) // narrow: cap 3
	want := map[string]string{"2027-03-04": "", "2027-03-05": "4|+1", "2027-03-06": "", "2027-03-13": ""}
	for date, w := range want {
		if got := tops(day(t, l, date)); got != w {
			t.Errorf("%s top = %q; want %q", date, got, w)
		}
	}
}

func TestSeriesKeepsOneLane(t *testing.T) {
	l := layoutOf(t, "a=2027-03-01/2027-03-30\ns=2027-03-02/2d;every=1w", "2027-01-01", "2027-05-31", nil)
	for _, date := range []string{"2027-03-02", "2027-03-09", "2027-03-16"} {
		rails := day(t, l, date).Rails
		if len(rails) != 2 || rails[1].Lane != 1 {
			t.Errorf("%s rails = %+v", date, rails)
		}
	}
	if got := tops(day(t, l, "2027-03-09")); got != "#2" {
		t.Errorf("second occurrence label = %q; want #2", got)
	}
}

func TestTextLabels(t *testing.T) {
	text := `m=2027-03-01
a=2027-03-01/2027-04-04;title="Alpha";length=workdays
b=2027-03-01/2027-03-04;title="Beta";status=tentative;length=days
n=2027-03-10/2027-03-11;title="Hidden label";label=none
s=2027-03-15/1d;every=1w;count=3;title="Sync";label=each
2027-03-20/2d`
	l := layoutOf(t, text, "2027-01-01", "2027-05-31", func(o *Options) { o.Weekend = map[int]bool{4: true, 5: true} })
	want := map[string]string{
		"2027-03-01": "",              // milestone day: no label
		"2027-03-02": "ALPHA · 25 WD", // Fri+Sat weekend: 35 days, 10 not worked
		"2027-03-03": "BETA ? · 4 D",
		"2027-03-10": "",        // label=none
		"2027-03-22": "SYNC #2", // label=each
		"2027-03-20": "",        // anonymous, untitled: rail only
		"2027-04-01": "ALPHA (CONT.)",
	}
	for date, w := range want {
		if got := tops(day(t, l, date)); got != w {
			t.Errorf("%s = %q; want %q", date, got, w)
		}
	}
}

func TestKeyLabels(t *testing.T) {
	text := "a=2026-12-20/2027-02-09\ns=2026-12-28/3d;every=1w\n2027-01-05/2d"
	// a narrow page: keys only, and never at the top of a column
	l := layoutOf(t, text, "2026-12-01", "2027-11-30", nil)
	if got := tops(day(t, l, "2027-01-01")); got != "" {
		t.Errorf("narrow page: no column-top keys, got %q", got)
	}
	if got := tops(day(t, l, "2027-01-05")); got != "3" {
		t.Errorf("anonymous interval key = %q; want 3", got)
	}
	if got := tops(day(t, l, "2027-01-04")); got != "2" {
		t.Errorf("series key on each occurrence start = %q; want 2", got)
	}
}

func TestColumnTopKeysOnWidePages(t *testing.T) {
	text := "a=2027-01-20/2027-02-09\nb=2027-01-20/2027-02-09\nc=2027-01-20/2027-02-09\nd=2027-01-20/2027-02-09\ns=2027-01-25/10d;every=1w"
	l := layoutOf(t, text, "2027-01-01", "2027-05-31", nil)
	if col := column(t, l, time.February); col.Labels != KeyLabels {
		t.Fatalf("February should use keys (k=%d)", col.K)
	}
	if got := tops(day(t, l, "2027-02-01")); got != "1|2|3|4|5" {
		t.Errorf("Feb 1 = %q; want 1|2|3|4 continuing plus series 5 starting", got)
	}
	if got := tops(day(t, l, "2027-02-01")); strings.Count(got, "5") != 1 {
		t.Errorf("a series continuing into a column gets no column-top key: %q", got)
	}
}

func TestMergedGroups(t *testing.T) {
	text := `carol=2027-03-10/2027-03-20;color=grey;style=dashed;group=people
alice=2027-03-18/2027-03-26;color=teal;group=people
pat=2027-03-01/2027-03-04;color=red;group=Pond`
	l := layoutOf(t, text, "2027-01-01", "2027-12-31", nil) // narrow: merged
	if !l.GroupsMerged || len(l.Groups) != 2 || l.Groups[0].Key != "P" || l.Groups[1].Key != "PO" {
		t.Fatalf("groups = %+v", l.Groups)
	}
	r := day(t, l, "2027-03-12").Rails[0]
	if !r.Dashed || r.Color != "#7A7A7A" || r.Thick {
		t.Errorf("one member active: member style: %+v", r)
	}
	r = day(t, l, "2027-03-19").Rails[0]
	if r.Dashed || r.Color != "#7A7A7A" || !r.Thick {
		t.Errorf("two members active: thick, group colour: %+v", r)
	}
	if r := day(t, l, "2027-03-18").Rails[0]; len(r.Ticks) != 1 || !r.Ticks[0].AtTop {
		t.Errorf("alice starts inside the rail: tick at top: %+v", r)
	}
	if r := day(t, l, "2027-03-20").Rails[0]; len(r.Ticks) != 1 || r.Ticks[0].AtTop {
		t.Errorf("carol ends inside the rail: tick at bottom: %+v", r)
	}
	if got := day(t, l, "2027-03-18").Top; len(got) != 1 || got[0].Text != "2" || got[0].Color != "#2E6E62" {
		t.Errorf("member key in member colour: %+v", got)
	}
	var groupEntry *LegendEntry
	for i := range l.Legend {
		if l.Legend[i].Swatch == SwatchGroup && l.Legend[i].Key == "P" {
			groupEntry = &l.Legend[i]
		}
	}
	if groupEntry == nil || groupEntry.When != "= 1 + 2" {
		t.Errorf("group legend entry = %+v", groupEntry)
	}
}

func TestFlowDailyAndWeekly(t *testing.T) {
	text := "a=2027-03-01/2027-03-19\nb=2027-03-01/2027-03-04\nc=2027-03-02/2027-03-19"
	// daily: b ends Mar 4 (last day), c slides inward on Mar 5 with a jog
	l := layoutOf(t, text, "2027-01-01", "2027-05-31", func(o *Options) { o.Flow = true })
	m4, m5 := day(t, l, "2027-03-04"), day(t, l, "2027-03-05")
	if len(m4.Rails) != 3 || m4.Rails[2].Lane != 2 {
		t.Fatalf("Mar 4 rails = %+v", m4.Rails)
	}
	if r := m5.Rails[1]; r.Lane != 1 || r.JogFrom != 2 {
		t.Errorf("Mar 5: c moves inward with a jog: %+v", r)
	}
	if m4.Zone <= m5.Zone {
		t.Errorf("per-day zone narrows: %v -> %v", m4.Zone, m5.Zone)
	}
	// weekly, week starting Sunday (6): the gap stays until Sun Mar 7
	l = layoutOf(t, text, "2027-01-01", "2027-05-31", func(o *Options) { o.Flow, o.WeeklyCompact, o.WeekStart = true, true, 6 })
	if r := day(t, l, "2027-03-06").Rails; len(r) != 2 || r[1].Lane != 2 {
		t.Errorf("Sat Mar 6 keeps the empty lane: %+v", r)
	}
	if r := day(t, l, "2027-03-07").Rails; r[1].Lane != 1 || r[1].JogFrom != 2 {
		t.Errorf("Sun Mar 7 closes up: %+v", r)
	}
}

func TestFlowNoJogOnDay1AndStickyOverflow(t *testing.T) {
	text := "a=2027-03-25/2027-04-02\nb=2027-03-20/2027-04-09\nx=2027-03-26/2027-04-04\ny=2027-03-27/2027-04-01"
	l := layoutOf(t, text, "2027-01-01", "2027-05-31", func(o *Options) { o.Flow = true; o.LaneCap = 2 })
	// a arrives after b (lane 1); x and y overflow and stay there even after a ends
	if dy := day(t, l, "2027-04-03"); dy.OverflowLane != 1 || len(dy.Rails) != 1 {
		t.Errorf("Apr 3 (a ended): rails %+v, overflow lane %d; overflow stays sticky", dy.Rails, dy.OverflowLane)
	}
	l = layoutOf(t, "a=2027-03-25/2027-04-02\nb=2027-03-20/2027-04-09", "2027-01-01", "2027-05-31", func(o *Options) { o.Flow = true })
	for _, r := range day(t, l, "2027-04-01").Rails {
		if r.JogFrom != -1 {
			t.Errorf("no jog on day 1 of a column: %+v", r)
		}
	}
}

func TestMilestonesPerDay(t *testing.T) {
	text := `a=2027-03-10;title="Alpha";mark=ring;color=teal
b=2027-03-10;title="Beta";mark=star;label=none
c=2027-03-10;title="Gamma";mark=dot;status=tentative
w=2027-03-01;title="Weekly";every=1w
e=2027-01-03;title="Exam";countdown=5`
	l := layoutOf(t, text, "2027-01-01", "2027-05-31", nil)
	m := day(t, l, "2027-03-10").Milestones
	if len(m.Marks) != 3 || m.Ring != "#2E6E62" || m.Title != "Alpha +2" {
		t.Errorf("Mar 10 = %+v", m)
	}
	if got := day(t, l, "2027-03-08").Milestones.Title; got != "" {
		t.Errorf("a series titles its first occurrence in a column only: %q", got)
	}
	if got := day(t, l, "2027-04-05").Milestones.Title; got != "Weekly" {
		t.Errorf("first occurrence in April = %q", got)
	}
	// countdown clipped by the period start: only Jan 1 and 2 exist
	if c := day(t, l, "2027-01-01").Countdown; c == nil || c.N != 2 {
		t.Errorf("Jan 1 countdown = %+v; want T-2", c)
	}
	l = layoutOf(t, text, "2027-01-01", "2027-12-31", nil)
	if day(t, l, "2027-01-01").Countdown != nil {
		t.Errorf("no countdown on narrow pages")
	}
}

// A one-day interval is a rail one cell long: both caps on that day,
// nothing on the days around it, its label on that day.
func TestOneDayInterval(t *testing.T) {
	for _, layer := range []string{"", ";style=dashed", ";style=hatch", ";layer=back"} {
		l := layoutOf(t, `o=2027-03-10/2027-03-10;title="One";length=days`+layer, "2027-01-01", "2027-05-31", nil)
		on, before, after := day(t, l, "2027-03-10"), day(t, l, "2027-03-09"), day(t, l, "2027-03-11")
		switch layer {
		case ";style=hatch":
			if !on.Hatch || before.Hatch || after.Hatch {
				t.Errorf("hatch: on %v, before %v, after %v", on.Hatch, before.Hatch, after.Hatch)
			}
		case ";layer=back":
			if len(on.Back) != 1 || !on.Back[0].TopCap || !on.Back[0].BotCap || len(before.Back)+len(after.Back) != 0 {
				t.Errorf("back line: %+v / %+v / %+v", before.Back, on.Back, after.Back)
			}
		default:
			if len(on.Rails) != 1 || !on.Rails[0].TopCap || !on.Rails[0].BotCap || len(before.Rails)+len(after.Rails) != 0 {
				t.Errorf("%s rail: %+v / %+v / %+v", layer, before.Rails, on.Rails, after.Rails)
			}
			if on.Rails[0].Dashed != (layer == ";style=dashed") {
				t.Errorf("%s: dashed = %v", layer, on.Rails[0].Dashed)
			}
			if got := tops(on); got != "ONE · 1 D" {
				t.Errorf("%s: label %q; want ONE · 1 D", layer, got)
			}
		}
	}
}

func TestLegend(t *testing.T) {
	text := `old=2026-06-01;title="Old"
m=2027-02-01;title="Launch";mark=ring
r=2027-01-04;title="Review";every=2w
b=2027-01-01/2027-03-31;title="Q1";layer=back
x=2027-01-10/3d;gap=4d;title="Sprint"
t=2027-01-04/2027-01-08;status=tentative;length=workdays
h=2027-02-01/2027-02-07;style=hatch;title="Holiday"
o=2027-03-10/2027-03-10;title="One day"
w=2027-03-13/1d;title="Saturday";length=workdays`
	l := layoutOf(t, text, "2027-01-01", "2027-03-31", nil)
	var got []string
	for _, e := range l.Legend {
		got = append(got, e.Key+"|"+e.Title+"|"+e.When)
	}
	want := []string{
		"1|Q1 · back|1 Jan–31 Mar · 90 d",
		"|Review|4 Jan–29 Mar · every 2w · ×7",
		"3|t ?|4 Jan–8 Jan · 5 wd",
		"2|Sprint|10 Jan–30 Mar · gap 4d · ×12",
		"|Launch|1 Feb",
		"4|Holiday|1 Feb–7 Feb · 7 d",
		"5|One day|10 Mar · 1 d", // one day: named once
		"6|Saturday|13 Mar · 0 wd",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("legend:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !l.Legend[4].Milestone || l.Legend[4].Mark != Ring || l.Legend[0].Swatch != SwatchBack || l.Legend[5].Swatch != SwatchHatch || l.Legend[2].Swatch != SwatchDashed {
		t.Errorf("legend kinds = %+v", l.Legend)
	}
}

// REQ-LG-2: an interval that started before the period sorts by its first
// in-period day, so a milestone on that day (declared first) comes first.
func TestLegendSortsByFirstInPeriodDay(t *testing.T) {
	l := layoutOf(t, `m=2027-02-01;title="Start"
x=2027-01-20/20d;title="Running"`, "2027-02-01", "2027-03-31", nil)
	if len(l.Legend) != 2 || l.Legend[0].Title != "Start" || l.Legend[1].Title != "Running" {
		t.Errorf("legend order = %+v", l.Legend)
	}
}
