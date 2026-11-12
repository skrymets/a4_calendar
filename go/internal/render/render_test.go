package render

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"pinplan/internal/canvas"
	"pinplan/internal/period"
)

const months = "JAN,FEB,MAR,APR,MAY,JUN,JUL,AUG,SEP,OCT,NOV,DEC"

func testConfig(start time.Time, p period.Period) Config {
	return Config{
		Start: start, Period: p,
		WeekMode: "relative", WeekStart: 0, WeekLabels: "first",
		Weekend:    map[int]bool{5: true, 6: true},
		Title:      DefaultTitle(p),
		DayLetters: []rune("MTWTFSS"),
		MonthNames: strings.Split(months, ","),
		Font:       canvas.Helvetica,
		PaperW:     A4W, PaperH: A4H,
		Footer: true, NoteGuide: true, NoteLines: 1,
		WorkdayColor: canvas.MustHex("#1A3A7A"), WeekendColor: canvas.MustHex("#8E1B1B"),
		MarginMM: 10, Header: "compact", ShowOutside: true,
	}
}

func TestFitText(t *testing.T) {
	w := func(s string) float64 { return float64(len(s)) }
	cases := []struct {
		in   string
		max  float64
		want string
	}{
		{"ABCDEFGHIJ", 10, "ABCDEFGHIJ"},
		{"ABCDEFGHIJ", 7, "ABCD..."},
		{"AB CDEF", 5, "AB..."}, // trailing space trimmed before "..."
		{"ABCDEFGHIJ", 2, ""},
	}
	for _, c := range cases {
		if got := FitText(c.in, c.max, w); got != c.want {
			t.Errorf("FitText(%q, %v) = %q; want %q", c.in, c.max, got, c.want)
		}
	}
}

func TestPeriodLabel(t *testing.T) {
	m := strings.Split(months, ",")
	cases := []struct {
		s, e time.Time
		want string
	}{
		{period.Date(2026, 1, 1), period.Date(2026, 12, 31), "2026"},
		{period.Date(2026, 1, 1), period.Date(2026, 3, 31), "01 JAN 2026  –  31 MAR 2026"},
		{period.Date(2026, 11, 7), period.Date(2027, 11, 6), "07 NOV 2026  –  06 NOV 2027"},
	}
	for _, c := range cases {
		if got := PeriodLabel(c.s, c.e, m); got != c.want {
			t.Errorf("PeriodLabel = %q; want %q", got, c.want)
		}
	}
}

func TestDefaults(t *testing.T) {
	s := period.Date(2026, 11, 7)
	titles := map[period.Period]string{
		period.FullYear:    "YEARLY CALENDAR",
		{N: 90, Unit: 'd'}: "90-DAY CALENDAR",
		{N: 12, Unit: 'w'}: "12-WEEK CALENDAR",
		{N: 12, Unit: 'm'}: "12-MONTH CALENDAR",
	}
	for p, want := range titles {
		if got := DefaultTitle(p); got != want {
			t.Errorf("DefaultTitle(%s) = %q; want %q", p, got, want)
		}
	}
	if got := DefaultOutput(s, period.FullYear); got != "calendar_2026-11-07.pdf" {
		t.Errorf("DefaultOutput(1y) = %q", got)
	}
	if got := DefaultOutput(s, period.Period{N: 90, Unit: 'd'}); got != "calendar_2026-11-07_90d.pdf" {
		t.Errorf("DefaultOutput(90d) = %q", got)
	}
}

var smallLabel = regexp.MustCompile(`^\d{3}(/\d{2})?$`)
var dayNum = regexp.MustCompile(`^\d{1,2}$`)

func count(texts []string, re *regexp.Regexp) int {
	n := 0
	for _, s := range texts {
		if re.MatchString(s) {
			n++
		}
	}
	return n
}

func TestBuild90Days(t *testing.T) {
	f := &fakeCanvas{}
	res, err := Build(testConfig(period.Date(2026, 11, 7), period.Period{N: 90, Unit: 'd'}), f)
	if err != nil {
		t.Fatal(err)
	}
	if !res.End.Equal(period.Date(2027, 2, 4)) || res.TotalDays != 90 {
		t.Errorf("result = %+v", res)
	}
	if f.title != "90-DAY CALENDAR 2026-11-07 – 2027-02-04" {
		t.Errorf("metadata title = %q", f.title)
	}
	strs := f.texts("string")
	if n := count(strs, smallLabel); n != 90 {
		t.Errorf("%d small labels; want 90", n)
	}
	if n := count(strs, dayNum); n != 30+31+31+28 { // faint outside days too
		t.Errorf("%d day numbers; want 120", n)
	}
	for _, want := range []string{"001/01", "002", "003/02", "025/05", "087/14",
		"90 days · week numbering: relative (14 weeks)"} {
		if !slices.Contains(strs, want) {
			t.Errorf("missing text %q", want)
		}
	}
	spaced := f.texts("spaced")
	for _, want := range []string{"90-DAY CALENDAR", "07 NOV 2026  –  04 FEB 2027",
		"NOV '26", "DEC", "JAN '27", "FEB '27"} {
		if !slices.Contains(spaced, want) {
			t.Errorf("missing spaced text %q (have %q)", want, spaced)
		}
	}
}

func TestBuildHideOutside(t *testing.T) {
	cfg := testConfig(period.Date(2026, 11, 7), period.Period{N: 90, Unit: 'd'})
	cfg.ShowOutside = false
	f := &fakeCanvas{}
	if _, err := Build(cfg, f); err != nil {
		t.Fatal(err)
	}
	if n := count(f.texts("string"), dayNum); n != 90 {
		t.Errorf("%d day numbers; want 90", n)
	}
}

func TestBuildFullYearJan1(t *testing.T) {
	f := &fakeCanvas{}
	if _, err := Build(testConfig(period.Date(2026, 1, 1), period.FullYear), f); err != nil {
		t.Fatal(err)
	}
	spaced := f.texts("spaced")
	for _, want := range append([]string{"YEARLY CALENDAR", "2026"}, strings.Split(months, ",")...) {
		if !slices.Contains(spaced, want) {
			t.Errorf("missing spaced text %q", want)
		}
	}
	if !slices.Contains(f.texts("string"), "365 days · week numbering: relative (53 weeks)") {
		t.Error("footer missing")
	}
}

func TestBuildTruncatesLongTitle(t *testing.T) {
	cfg := testConfig(period.Date(2026, 11, 7), period.Period{N: 90, Unit: 'd'})
	cfg.Title = strings.Repeat("A", 200)
	f := &fakeCanvas{}
	if _, err := Build(cfg, f); err != nil {
		t.Fatal(err)
	}
	title := f.texts("spaced")[1] // [0] is the period label, drawn first
	if !strings.HasSuffix(title, "...") || len(title) >= 200 {
		t.Errorf("title not truncated: %q", title)
	}
	if !strings.HasPrefix(f.title, strings.Repeat("A", 200)) {
		t.Error("metadata title must stay untruncated")
	}
}

func TestBuildNoTitleNoFooter(t *testing.T) {
	cfg := testConfig(period.Date(2026, 11, 7), period.FullYear)
	cfg.Title, cfg.Footer = "", false
	f := &fakeCanvas{}
	if _, err := Build(cfg, f); err != nil {
		t.Fatal(err)
	}
	for _, s := range f.texts("string") {
		if strings.Contains(s, "week numbering") {
			t.Error("footer drawn although disabled")
		}
	}
}
