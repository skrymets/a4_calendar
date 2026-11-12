package dsl

import (
	"a4calendar/internal/period"
	"errors"
	"strings"
	"testing"
	"time"
)

var today = time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func file(text string) []Source { return []Source{{Name: "t.dates", Text: text}} }

func mustParse(t *testing.T, text string) *Model {
	t.Helper()
	m, err := Parse(file(text), today)
	if err != nil {
		t.Fatalf("Parse(%q): %v", text, err)
	}
	return m
}

func mustExpand(t *testing.T, text, windowEnd string) *Resolved {
	t.Helper()
	r, err := Expand(mustParse(t, text), d(windowEnd))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return r
}

func TestMilestoneDates(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"2027-03-15", "2027-03-15"},
		{"today", "2026-11-11"},
		{"TODAY", "2026-11-11"},
		{"+2w", "2026-11-25"},
		{"-3d", "2026-11-08"},
		{"today+1m", "2026-12-11"},
		{"2027-03-15-2w", "2027-03-01"},
		{"2027-03-15+2D", "2027-03-17"},
		{"2027-03-15+1M", "2027-04-15"},
		{"2027-03-15+2Y", "2029-03-15"},
		{"2027-01-31+1m", "2027-02-28"}, // clamped
		{"2028-02-29+1y", "2029-02-28"},
		{"2027-03-31-1m", "2027-02-28"},
	}
	for _, c := range cases {
		m := mustParse(t, "x="+c.expr)
		if got := iso(m.Milestones[0].Date); got != c.want {
			t.Errorf("%s = %s; want %s", c.expr, got, c.want)
		}
	}
}

func TestReferences(t *testing.T) {
	m := mustParse(t, `
release = 2027-03-15
freeze  = @release-2w
testing = @freeze-1w
review  = @release+1w;title="Release Review"
after   = @Freeze+3d
`)
	want := map[string]string{"release": "2027-03-15", "freeze": "2027-03-01", "testing": "2027-02-22",
		"review": "2027-03-22", "after": "2027-03-04"}
	for _, ms := range m.Milestones {
		if got := iso(ms.Date); got != want[ms.ID] {
			t.Errorf("%s = %s; want %s", ms.ID, got, want[ms.ID])
		}
	}
}

func TestCaseInsensitivity(t *testing.T) {
	m := mustParse(t, `RELEASE=2027-03-15
freeze=@Release-2W
QA=@FREEZE/@release;TITLE="Final QA";Owner=Alice`)
	if got := iso(m.Milestones[1].Date); got != "2027-03-01" {
		t.Errorf("freeze = %s", got)
	}
	iv := m.Intervals[0]
	if iv.ID != "QA" || iso(iv.Start) != "2027-03-01" || iso(iv.End) != "2027-03-15" {
		t.Errorf("QA = %+v", iv)
	}
	if v, _ := iv.Props.Get("title"); v != "Final QA" {
		t.Errorf("title = %q", v)
	}
	if v, _ := iv.Props.Get("owner"); v != "Alice" {
		t.Errorf("owner = %q (values keep their case)", v)
	}
}

func TestIntervalForms(t *testing.T) {
	m := mustParse(t, `
release = 2027-03-15
freeze  = @release-2w
2027-01-01/2027-02-01
qa          = @freeze/@release
support     = @release/1m
preparation = 2w/@freeze
audit       = @release+1w/@release+3w
monthEnd    = 2027-01-31/1M
`)
	want := []struct{ id, s, e string }{
		{"", "2027-01-01", "2027-02-01"},
		{"qa", "2027-03-01", "2027-03-15"},
		{"support", "2027-03-15", "2027-04-15"},
		{"preparation", "2027-02-15", "2027-03-01"},
		{"audit", "2027-03-22", "2027-04-05"},
		{"monthEnd", "2027-01-31", "2027-02-28"},
	}
	for i, w := range want {
		iv := m.Intervals[i]
		if iv.ID != w.id || iso(iv.Start) != w.s || iso(iv.End) != w.e {
			t.Errorf("interval %d = %s %s..%s; want %s %s..%s", i, iv.ID, iso(iv.Start), iso(iv.End), w.id, w.s, w.e)
		}
	}
}

type isoSpan struct{ s, e string }

func intervalSpans(r *Resolved) []isoSpan {
	var out []isoSpan
	for _, o := range r.Intervals {
		out = append(out, isoSpan{iso(o.Start), iso(o.End)})
	}
	return out
}

func milestoneDates(r *Resolved) []string {
	var out []string
	for _, o := range r.Milestones {
		out = append(out, iso(o.Date))
	}
	return out
}

func equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRecurringIntervals(t *testing.T) {
	cases := []struct {
		name, text, window string
		want               []isoSpan
	}{
		{"every (spec 6.3)", "sprint=2027-01-01/2w;every=3w;count=3", "2027-12-31",
			[]isoSpan{{"2027-01-01", "2027-01-15"}, {"2027-01-22", "2027-02-05"}, {"2027-02-12", "2027-02-26"}}},
		{"gap (spec 6.4)", "work=2027-01-04/5d;gap=2d;count=3", "2027-12-31",
			[]isoSpan{{"2027-01-04", "2027-01-09"}, {"2027-01-11", "2027-01-16"}, {"2027-01-18", "2027-01-23"}}},
		{"overlapping occurrences allowed", "x=2027-01-01/2w;every=1w;count=2", "2027-12-31",
			[]isoSpan{{"2027-01-01", "2027-01-15"}, {"2027-01-08", "2027-01-22"}}},
		{"START/END keeps its days", "x=2027-01-30/2027-02-02;every=1m;count=3", "2027-12-31",
			[]isoSpan{{"2027-01-30", "2027-02-02"}, {"2027-02-28", "2027-03-03"}, {"2027-03-30", "2027-04-02"}}},
		{"START/DURATION keeps the calendar duration", "x=2027-01-31/1m;every=1m;count=3", "2027-12-31",
			[]isoSpan{{"2027-01-31", "2027-02-28"}, {"2027-02-28", "2027-03-28"}, {"2027-03-31", "2027-04-30"}}},
		{"DURATION/END applies the period forward", "x=1m/2027-03-31;every=1m;count=2", "2027-12-31",
			[]isoSpan{{"2027-02-28", "2027-03-31"}, {"2027-03-28", "2027-04-28"}}},
		{"gap from the actual end", "x=2027-01-31/1m;gap=1d;count=2", "2027-12-31",
			[]isoSpan{{"2027-01-31", "2027-02-28"}, {"2027-03-01", "2027-04-01"}}},
		{"until applies to the start, inclusive", "x=2027-01-01/1w;every=1w;until=2027-01-15", "2027-12-31",
			[]isoSpan{{"2027-01-01", "2027-01-08"}, {"2027-01-08", "2027-01-15"}, {"2027-01-15", "2027-01-22"}}},
		{"unbounded stops at the window end", "x=2027-01-01/1d;every=1w", "2027-01-20",
			[]isoSpan{{"2027-01-01", "2027-01-02"}, {"2027-01-08", "2027-01-09"}, {"2027-01-15", "2027-01-16"}}},
		{"count beyond the window", "x=2027-01-01/1d;every=1w;count=10", "2027-01-08",
			[]isoSpan{{"2027-01-01", "2027-01-02"}, {"2027-01-08", "2027-01-09"}}},
	}
	for _, c := range cases {
		r := mustExpand(t, c.text, c.window)
		if got := intervalSpans(r); !equal(got, c.want) {
			t.Errorf("%s: got %v; want %v", c.name, got, c.want)
		}
		for i, o := range r.Intervals {
			if o.Idx != i+1 || o.N != len(c.want) {
				t.Errorf("%s: occurrence %d has Idx/N %d/%d", c.name, i, o.Idx, o.N)
			}
		}
	}
}

func TestRecurringMilestones(t *testing.T) {
	cases := []struct {
		name, text, window string
		want               []string
	}{
		{"month end clamps from the original date (spec 6.7)", "report=2027-01-31;every=1m;count=4", "2027-12-31",
			[]string{"2027-01-31", "2027-02-28", "2027-03-31", "2027-04-30"}},
		{"leap year", "x=2028-01-31;every=1m;count=3", "2028-12-31",
			[]string{"2028-01-31", "2028-02-29", "2028-03-31"}},
		{"every 3 days", "backup=today;every=3d;count=3", "2027-12-31",
			[]string{"2026-11-11", "2026-11-14", "2026-11-17"}},
		{"until inclusive", "audit=2027-01-01;every=1m;until=2027-03-01", "2027-12-31",
			[]string{"2027-01-01", "2027-02-01", "2027-03-01"}},
		{"unbounded weekly", "meeting=2027-01-01;every=1w", "2027-01-21",
			[]string{"2027-01-01", "2027-01-08", "2027-01-15"}},
		{"yearly", "x=2024-02-29;every=1y", "2027-12-31",
			[]string{"2024-02-29", "2025-02-28", "2026-02-28", "2027-02-28"}},
	}
	for _, c := range cases {
		if got := milestoneDates(mustExpand(t, c.text, c.window)); !equal(got, c.want) {
			t.Errorf("%s: got %v; want %v", c.name, got, c.want)
		}
	}
}

func TestOccurrencesBeforeTheWindowAreKept(t *testing.T) {
	r := mustExpand(t, "x=2026-01-01;every=1m", "2026-03-15")
	if got := milestoneDates(r); !equal(got, []string{"2026-01-01", "2026-02-01", "2026-03-01"}) {
		t.Errorf("got %v", got)
	}
}

func TestProperties(t *testing.T) {
	m := mustParse(t, `x = 2027-03-15 ; title = "A; \"quoted\" \\ title" ; owner=alice;;Priority=high;`)
	p := m.Milestones[0].Props
	want := Props{{"title", `A; "quoted" \ title`}, {"owner", "alice"}, {"priority", "high"}}
	if !equal(p, want) {
		t.Errorf("props = %q; want %q", p, want)
	}
	m = mustParse(t, `x=2027-03-15;title="";note="a\b"`)
	if v, ok := m.Milestones[0].Props.Get("title"); !ok || v != "" {
		t.Errorf("empty quoted title = %q, %v", v, ok)
	}
	if v, _ := m.Milestones[0].Props.Get("note"); v != `a\b` {
		t.Errorf("other backslashes stay literal: %q", v)
	}
}

func TestFileFormat(t *testing.T) {
	text := utf8BOM + "# comment\r\n\r\n  a = 2027-01-01 ; title=\"A\"  \r\n# intervals\r\nx=@a/2d\r\n"
	m := mustParse(t, text)
	if len(m.Milestones) != 1 || len(m.Intervals) != 1 {
		t.Fatalf("got %d milestones, %d intervals", len(m.Milestones), len(m.Intervals))
	}
	if m.Milestones[0].Line != 3 || m.Intervals[0].Line != 5 {
		t.Errorf("lines = %d, %d; want 3, 5", m.Milestones[0].Line, m.Intervals[0].Line)
	}
}

func TestInlineSourcesAndOrder(t *testing.T) {
	srcs := []Source{
		{Name: "roadmap.dates", Text: "release=2027-03-15\nqa=2027-03-01/@release"},
		{Name: "--milestone[1]", Text: "extra_review=@release+3w", Expect: KindMilestone},
		{Name: "--interval[1]", Text: "extra_qa=@release/@extra_review", Expect: KindInterval},
	}
	m, err := Parse(srcs, today)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Milestones) != 2 || len(m.Intervals) != 2 || iso(m.Intervals[1].End) != "2027-04-05" {
		t.Errorf("model = %d milestones, %d intervals, extra_qa end %s", len(m.Milestones), len(m.Intervals), iso(m.Intervals[1].End))
	}
	if m.Milestones[1].Line != 0 || m.Milestones[1].Source != "--milestone[1]" {
		t.Errorf("inline milestone at %s:%d", m.Milestones[1].Source, m.Milestones[1].Line)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"syntax", "x=next friday", `t.dates:1: invalid date expression "next friday"`},
		{"invalid date", "x=2027-02-30", "t.dates:1: invalid date 2027-02-30"},
		{"year 0", "x=0000-01-01", "invalid date 0000-01-01"},
		{"out of range", "x=9999-12-31+1d", "outside the years 0001–9999"},
		{"huge offset", "x=today+99999999d", "number 99999999 is too large"},
		{"chained offsets (phase 2)", "a=2027-01-01\nb=@a+1d+2d", `t.dates:2: invalid date expression "@a+1d+2d"`},
		{"unknown reference", "x=@nope", "t.dates:1: unknown name @nope (only milestones and intervals declared earlier"},
		{"forward reference", "x=@y\ny=2027-01-01", "t.dates:1: unknown name @y"},
		{"recurring reference", "r=2027-01-01;every=1w\nx=@r+1d", "t.dates:2: @r is recurring; pick one occurrence, e.g. @r[1] or @r[-1]"},
		{"recurring interval boundary", "q=2027-01-01/2d;every=1w\nx=@q.end", "t.dates:2: @q is recurring; pick one occurrence, e.g. @q[1].end or @q[-1].end"},
		{"interval without boundary", "a=2027-01-01\nq=@a/2d\nr=@q/2d", "t.dates:3: @q is an interval; use @q.start or @q.end"},
		{"occurrence without boundary", "q=2027-01-01/2d;every=1w\nx=@q[2]", "t.dates:2: @q is an interval; use @q[2].start or @q[2].end"},
		{"milestone boundary", "a=2027-01-01\nb=@a.end", "t.dates:2: @a is a milestone; .start and .end apply to intervals"},
		{"index on a single date", "a=2027-01-01\nb=@a[1]", "t.dates:2: @a is not recurring; leave out [1]"},
		{"index zero", "a=2027-01-01;every=1w\nb=@a[0]", "t.dates:2: @a[0]: an occurrence number is 1 to 100000, or -1 to -100000"},
		{"index too large", "a=2027-01-01;every=1w\nb=@a[100001]", "@a[100001]: an occurrence number is 1 to 100000"},
		{"index past count", "a=2027-01-01;every=1w;count=8\nb=@a[9]", "t.dates:2: @a[9]: the series has only 8 occurrences"},
		{"index past until", "a=2027-01-01;every=1w;until=2027-01-20\nb=@a[-4]", "@a[-4]: the series has only 3 occurrences"},
		{"index past year 9999", "a=9999-12-01;every=1w\nb=@a[6]", "@a[6]: the series has only 5 occurrences"},
		{"negative index unbounded", "a=2027-01-01;every=1w\nb=@a[-1]", "@a[-1]: counting back from the last occurrence needs a series limited by count or until"},
		{"negative index too many", "a=1700-01-01;every=1d;until=9000-01-01\nb=@a[-1]", "@a[-1]: the series has more than 100000 occurrences"},
		{"boundary offset out of range", "q=9999-12-01/9999-12-31\nb=@q.end+1d", "outside the years 0001–9999"},
		{"chained boundary offsets (phase 2)", "q=2027-01-01/2d\nb=@q.end+1d-2d", `invalid date expression "@q.end+1d-2d"`},
		{"unknown boundary", "q=2027-01-01/2d\nb=@q.middle", `invalid date expression "@q.middle"`},
		{"duplicate milestone", "a=2027-01-01\nA=2027-01-02", `t.dates:2: duplicate identifier "A"`},
		{"duplicate across kinds", "a=2027-01-01\na=@a/2d", `t.dates:2: duplicate identifier "a"`},
		{"today reserved", "Today=2027-01-01", `"today" is reserved`},
		{"identifier starts with digit", "2release=2027-01-01", `invalid identifier "2release"`},
		{"identifier with dash", "code-freeze=2027-01-01", `invalid identifier "code-freeze"`},
		{"identifier with space", "release date=2027-01-01", `invalid identifier "release date"`},
		{"anonymous milestone", "2027-01-01", "a milestone needs a name"},
		{"zero duration", "x=2027-01-01/0d", "invalid date expression \"0d\""},
		{"two durations", "x=2w/3d", "only one side of an interval may be a duration"},
		{"two slashes", "x=2027-01-01/2027-01-05/2027-01-09", "exactly one '/'"},
		{"empty interval", "x=2027-01-01/2027-01-01", "interval start 2027-01-01 must be before its end 2027-01-01"},
		{"reversed interval", "x=2027-01-05/2027-01-01", "must be before its end"},
		{"every and gap", "x=2027-01-01/2d;every=1w;gap=1d", "every and gap are mutually exclusive"},
		{"gap on a milestone", "x=2027-01-01;gap=1w", "gap is only valid for intervals"},
		{"count and until", "x=2027-01-01;every=1w;count=2;until=2027-02-01", "count and until are mutually exclusive"},
		{"count alone", "x=2027-01-01;count=2", "count and until need every or gap"},
		{"until alone", "x=2027-01-01/2d;until=2027-02-01", "count and until need every or gap"},
		{"count zero", "x=2027-01-01;every=1w;count=0", "count must be a positive integer"},
		{"count text", "x=2027-01-01;every=1w;count=many", "count must be a positive integer"},
		{"every zero", "x=2027-01-01;every=0w", `every/gap needs a positive period such as 2w, got "0w"`},
		{"every composite (phase 2)", "x=2027-01-01;every=1m2w", "every/gap needs a positive period"},
		{"until before first milestone", "x=2027-03-01;every=1w;until=2027-02-01", "until 2027-02-01 is before the first occurrence 2027-03-01"},
		{"until before first interval", "x=2027-03-01/2d;every=1w;until=2027-02-01", "until 2027-02-01 is before the first occurrence"},
		{"duplicate property", "x=2027-01-01;title=a;TITLE=b", `duplicate property "title"`},
		{"property without value", "x=2027-01-01;title", `property without value: "title"`},
		{"empty value", "x=2027-01-01;title=", "empty property value"},
		{"unquoted whitespace", "x=2027-01-01;title=Final QA", "unquoted value contains whitespace"},
		{"unterminated quote", `x=2027-01-01;title="Final QA`, "unterminated quoted value"},
		{"bad quoted value", `x=2027-01-01;title="a" b`, "bad quoted value"},
		{"invalid property key", "x=2027-01-01;team-name=x", `invalid property key "team-name"`},
	}
	for _, c := range cases {
		_, err := Parse(file(c.text), today)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v; want it to contain %q", c.name, err, c.want)
		}
		var es Errors
		if !errors.As(err, &es) || len(es) != 1 {
			t.Errorf("%s: want exactly one error, got %v", c.name, err)
		}
	}
}

func TestErrorsAreCollectedWithoutCascades(t *testing.T) {
	_, err := Parse(file("a=2027-02-30\nb=@a+1d\nc=@zzz\nd=2027-01-01\nq=@b/2d\nr=@d/2d"), today)
	var es Errors
	if !errors.As(err, &es) {
		t.Fatalf("err = %v", err)
	}
	got := es.Error()
	want := "t.dates:1: invalid date 2027-02-30\nt.dates:3: unknown name @zzz (only milestones and intervals declared earlier can be referenced)"
	if got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// A declaration rejected for its kind is registered as broken too, so a
// later reference to it is not a second error.
func TestStructuralErrorsDoNotCascade(t *testing.T) {
	srcs := []Source{
		{Name: "t.dates", Text: "a=2027-01-01/2d\nb=2027-02-30"},
		{Name: "--milestone[1]", Text: "m=2027-01-01/2d", Expect: KindMilestone},
		{Name: "--interval[1]", Text: "i=2027-01-01", Expect: KindInterval},
		{Name: "--interval[2]", Text: "@m/@b", Expect: KindInterval},
		{Name: "--interval[3]", Text: "i=2027-01-01/1d", Expect: KindInterval},
		{Name: "--interval[4]", Text: "@i.end/1d", Expect: KindInterval},
	}
	_, err := Parse(srcs, today)
	want := "t.dates:2: invalid date 2027-02-30\n" +
		"--milestone[1]: a milestone cannot contain '/'; declare intervals with --interval\n" +
		"--interval[1]: an interval needs START/END, START/DURATION or DURATION/END\n" +
		`--interval[3]: duplicate identifier "i"`
	if err == nil || err.Error() != want {
		t.Errorf("errors:\n%v\nwant:\n%s", err, want)
	}
}

func TestInlineErrors(t *testing.T) {
	srcs := []Source{
		{Name: "--milestone[1]", Text: "x=2027-01-01/2d", Expect: KindMilestone},
		{Name: "--interval[1]", Text: "y=2027-01-01", Expect: KindInterval},
		{Name: "--interval[2]", Text: "  ", Expect: KindInterval},
	}
	_, err := Parse(srcs, today)
	want := "--milestone[1]: a milestone cannot contain '/'; declare intervals with --interval\n" +
		"--interval[1]: an interval needs START/END, START/DURATION or DURATION/END\n" +
		"--interval[2]: empty declaration"
	if err == nil || err.Error() != want {
		t.Errorf("errors:\n%v\nwant:\n%s", err, want)
	}
	// Declarations may come in any order.
	srcs = []Source{
		{Name: "--interval[1]", Text: "2027-01-01/2d", Expect: KindInterval},
		{Name: "--milestone[1]", Text: "x=2027-01-01", Expect: KindMilestone},
	}
	if _, err := Parse(srcs, today); err != nil {
		t.Errorf("inline order: %v", err)
	}
}

// References to an interval's start or end (§18) and to one occurrence of a
// series (§17). An interval's .end is its exclusive end, the END as written.
func TestBoundaryAndOccurrenceReferences(t *testing.T) {
	m := mustParse(t, `
qa      = 2027-03-01/2027-03-15
done    = @qa.end; mark=star
review  = 2027-01-04; every=1w; count=8
labs    = 2027-01-11/2d; gap=1w; until=2027-03-01
last    = @review[-1]
third   = @review[3]+1d
follow  = @qa.end/1w
before  = 2w/@qa.start
lab2    = @labs[2].start/@labs[2].end
lablast = @labs[-1].end-1d
start   = @qa.START; title="case-insensitive boundary"
weekly  = @review[2]; every=1w; until=@review[-1]
`)
	got := map[string]string{}
	for _, ms := range m.Milestones {
		got[ms.ID] = iso(ms.Date)
	}
	for _, iv := range m.Intervals {
		got[iv.ID] = iso(iv.Start) + "/" + iso(iv.End)
	}
	want := map[string]string{
		"done":    "2027-03-15",
		"last":    "2027-02-22", // 8th Monday from 4 Jan
		"third":   "2027-01-19", // 18 Jan + 1d
		"follow":  "2027-03-15/2027-03-22",
		"before":  "2027-02-15/2027-03-01",
		"lab2":    "2027-01-20/2027-01-22", // 13 Jan (end) + 1w gap
		"lablast": "2027-02-26",            // last lab 25–26 Feb, exclusive end 27 Feb
		"start":   "2027-03-01",
		"weekly":  "2027-01-11",
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s = %s; want %s", id, got[id], w)
		}
	}
	if until := m.Milestones[len(m.Milestones)-1].Rec.Until; iso(until) != "2027-02-22" {
		t.Errorf("weekly until = %s; want 2027-02-22", iso(until))
	}
}

// A reference resolves the same whatever the page shows: a later occurrence
// is found even when the window that Expand draws ends before it.
func TestOccurrenceReferenceIgnoresWindow(t *testing.T) {
	m := mustParse(t, "r=2027-01-01;every=1m\nx=@r[24]")
	if got := iso(m.Milestones[1].Date); got != "2028-12-01" {
		t.Errorf("x = %s; want 2028-12-01", got)
	}
}

func TestTooManyOccurrences(t *testing.T) {
	m := mustParse(t, "x=1700-01-01;every=1d")
	_, err := Expand(m, d("2027-12-31"))
	if err == nil || !strings.Contains(err.Error(), "t.dates:1: the recurrence has more than 100000 occurrences") {
		t.Errorf("err = %v", err)
	}
}

// Day counts use calendar arithmetic, not time.Duration, which saturates
// after about 292 years.
func TestLongIntervalDays(t *testing.T) {
	m, err := Parse(file("x=0001-01-01/2400-01-01; every=1000y; count=2"), today)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Expand(m, period.Date(9999, 12, 31))
	if err != nil {
		t.Fatal(err)
	}
	// same day count (876,216) from 1001-01-01; one more leap day in between
	if got := r.Intervals[1].End.Format("2006-01-02"); got != "3399-12-31" {
		t.Errorf("second occurrence ends %s; want 3399-12-31", got)
	}
}

// Inline declarations get a hint when their quotes were probably removed by
// the shell; files do not.
func TestUnquotedWhitespaceHint(t *testing.T) {
	const hint = "Windows PowerShell 5.1 removes the quotes"
	_, err := Parse([]Source{{Name: "--interval[1]", Text: "2027-01-01/2d;title=Final QA", Expect: KindInterval}}, today)
	if err == nil || !strings.Contains(err.Error(), hint) {
		t.Errorf("inline: %v", err)
	}
	_, err = Parse(file("x=2027-01-01;title=Final QA"), today)
	if err == nil || strings.Contains(err.Error(), hint) {
		t.Errorf("file: %v", err)
	}
}
