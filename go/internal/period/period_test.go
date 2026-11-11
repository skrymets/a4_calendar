package period

import (
	"errors"
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	ok := []struct {
		in   string
		want Period
	}{
		{"90d", Period{90, 'd'}},
		{" 12W ", Period{12, 'w'}},
		{"6m", Period{6, 'm'}},
		{"1y", Period{1, 'y'}},
	}
	for _, c := range ok {
		got, err := ParsePeriod(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParsePeriod(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"0d", "90", "abc", "", "-1d", "1.5m", "3x"} {
		if _, err := ParsePeriod(in); err == nil {
			t.Errorf("ParsePeriod(%q): expected error", in)
		}
	}
	_, err := ParsePeriod("90")
	want := "invalid period '90': use <N>d, <N>w, <N>m or 1y (N >= 1)"
	if err == nil || err.Error() != want {
		t.Errorf("error text = %v; want %q", err, want)
	}
}

func TestPeriodString(t *testing.T) {
	if s := (Period{90, 'd'}).String(); s != "90d" {
		t.Errorf("String() = %q", s)
	}
	if !FullYear.IsFullYear() || (Period{12, 'm'}).IsFullYear() {
		t.Error("IsFullYear wrong")
	}
}

func TestAddMonths(t *testing.T) {
	cases := []struct {
		from time.Time
		n    int
		want time.Time
	}{
		{Date(2026, 11, 7), 3, Date(2027, 2, 7)},
		{Date(2027, 1, 31), 1, Date(2027, 3, 1)},  // Feb 31 missing -> Mar 1
		{Date(2028, 2, 29), 12, Date(2029, 3, 1)}, // Feb 29 -> Mar 1
		{Date(2026, 12, 31), 2, Date(2027, 3, 1)},
		{Date(2026, 1, 15), 0, Date(2026, 1, 15)},
	}
	for _, c := range cases {
		if got := AddMonths(c.from, c.n); !got.Equal(c.want) {
			t.Errorf("AddMonths(%s, %d) = %s; want %s", c.from.Format("2006-01-02"), c.n, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}
}

func TestAddMonthsClamped(t *testing.T) {
	cases := []struct {
		from time.Time
		n    int
		want time.Time
	}{
		{Date(2027, 1, 31), 1, Date(2027, 2, 28)},
		{Date(2027, 1, 31), 2, Date(2027, 3, 31)},
		{Date(2027, 1, 31), 3, Date(2027, 4, 30)},
		{Date(2028, 1, 31), 1, Date(2028, 2, 29)}, // leap year
		{Date(2028, 2, 29), 12, Date(2029, 2, 28)},
		{Date(2027, 3, 31), -1, Date(2027, 2, 28)},
		{Date(2027, 1, 15), -1, Date(2026, 12, 15)},
		{Date(2027, 1, 15), -13, Date(2025, 12, 15)},
		{Date(2026, 12, 31), 2, Date(2027, 2, 28)},
		{Date(2026, 1, 15), 0, Date(2026, 1, 15)},
	}
	for _, c := range cases {
		if got := AddMonthsClamped(c.from, c.n); !got.Equal(c.want) {
			t.Errorf("AddMonthsClamped(%s, %d) = %s; want %s", c.from.Format("2006-01-02"), c.n, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}
}

func TestPeriodEnd(t *testing.T) {
	s := Date(2026, 11, 7)
	cases := []struct {
		start time.Time
		p     Period
		want  time.Time
	}{
		{s, Period{90, 'd'}, Date(2027, 2, 4)},
		{s, Period{2, 'w'}, Date(2026, 11, 20)},
		{s, Period{3, 'm'}, Date(2027, 2, 6)},
		{s, Period{6, 'm'}, Date(2027, 5, 6)},
		{s, Period{1, 'y'}, Date(2027, 11, 6)},
		{s, Period{12, 'm'}, Date(2027, 11, 6)},
		{s, Period{1, 'd'}, s},
		{Date(2027, 1, 31), Period{1, 'm'}, Date(2027, 2, 28)},
		{Date(2028, 2, 29), Period{1, 'y'}, Date(2029, 2, 28)},
		{Date(2026, 1, 1), Period{3, 'm'}, Date(2026, 3, 31)},
	}
	for _, c := range cases {
		got, err := PeriodEnd(c.start, c.p)
		if err != nil || !got.Equal(c.want) {
			t.Errorf("PeriodEnd(%s, %s) = %s, %v; want %s", c.start.Format("2006-01-02"), c.p, got.Format("2006-01-02"), err, c.want.Format("2006-01-02"))
		}
	}
	for _, p := range []Period{{999999, 'm'}, {9999999999, 'd'}} {
		if _, err := PeriodEnd(s, p); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("PeriodEnd(%s) err = %v; want ErrOutOfRange", p, err)
		}
	}
}

func TestMaxEnd(t *testing.T) {
	cases := map[time.Time]int{ // start -> days in the 1-year maximum
		Date(2026, 11, 7): 365,
		Date(2026, 3, 1):  365,
		Date(2027, 3, 1):  366, // spans Feb 29 2028
		Date(2027, 11, 7): 366,
	}
	for start, days := range cases {
		if got := DaysBetween(start, MaxEnd(start)) + 1; got != days {
			t.Errorf("MaxEnd(%s): %d days; want %d", start.Format("2006-01-02"), got, days)
		}
	}
}

func TestDaysIn(t *testing.T) {
	if DaysIn(2028, 2) != 29 || DaysIn(2027, 2) != 28 || DaysIn(2026, 12) != 31 {
		t.Error("DaysIn wrong")
	}
}
