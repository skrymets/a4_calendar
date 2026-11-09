package period

import (
	"testing"
	"time"
)

func TestWeekNumber(t *testing.T) {
	s := Date(2026, 11, 7)
	cases := []struct {
		day       time.Time
		start     time.Time
		mode      string
		weekStart int
		want      int
	}{
		{Date(2026, 11, 7), s, "relative", 0, 1},
		{Date(2026, 11, 9), s, "relative", 0, 2},
		{Date(2026, 12, 1), s, "relative", 0, 5},
		{Date(2027, 2, 4), s, "relative", 0, 14},
		{Date(2026, 11, 8), s, "relative", 6, 2},
		{Date(2026, 11, 13), s, "rolling", 0, 1},
		{Date(2026, 11, 14), s, "rolling", 0, 2},
		{Date(2026, 11, 7), s, "iso", 0, 45},
		{Date(2027, 1, 1), s, "iso", 0, 53},
		{Date(2027, 1, 4), s, "iso", 0, 1},
		{Date(2026, 12, 31), Date(2026, 1, 1), "relative", 0, 53},
		{Date(2026, 1, 1), Date(2026, 1, 1), "iso", 0, 1},
	}
	for _, c := range cases {
		if got := WeekNumber(c.day, c.start, c.mode, c.weekStart); got != c.want {
			t.Errorf("WeekNumber(%s, %s, %s, %d) = %d; want %d", c.day.Format("2006-01-02"), c.start.Format("2006-01-02"), c.mode, c.weekStart, got, c.want)
		}
	}
}

func TestWeekday(t *testing.T) {
	if Weekday(Date(2026, 11, 9)) != 0 || Weekday(Date(2026, 11, 8)) != 6 {
		t.Error("Weekday must be Monday=0..Sunday=6")
	}
}

func TestMonthsBetween(t *testing.T) {
	m := MonthsBetween(Date(2026, 11, 7), Date(2027, 11, 6))
	if len(m) != 13 || m[0] != (YM{2026, 11}) || m[12] != (YM{2027, 11}) {
		t.Errorf("mid-year: %v", m)
	}
	if n := len(MonthsBetween(Date(2026, 1, 1), Date(2026, 12, 31))); n != 12 {
		t.Errorf("Jan-1 year: %d months", n)
	}
	if n := len(MonthsBetween(Date(2026, 11, 7), Date(2026, 11, 7))); n != 1 {
		t.Errorf("1d: %d months", n)
	}
}
