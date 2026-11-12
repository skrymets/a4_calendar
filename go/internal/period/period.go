// Package period holds the calendar's date logic: period parsing, period
// end, week numbers, months, and month arithmetic.
//
// Dates are time.Time values at midnight UTC (see Date), so day
// arithmetic is exact and never affected by DST.
package period

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Date returns the calendar day y-m-d as midnight UTC. Out-of-range
// months/days are normalised the way time.Date does.
func Date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AddDays returns d shifted by n days.
func AddDays(d time.Time, n int) time.Time { return d.AddDate(0, 0, n) }

// DaysBetween returns b - a in days.
func DaysBetween(a, b time.Time) int { return int((b.Unix() - a.Unix()) / 86400) }

// DaysIn returns the number of days in month m of year y.
func DaysIn(y int, m time.Month) int { return Date(y, m+1, 0).Day() }

// Period is a calendar length: N units of d(ays), w(eeks), m(onths) or y(ears).
type Period struct {
	N    int
	Unit byte
}

// FullYear is the default (and maximum) period.
var FullYear = Period{N: 1, Unit: 'y'}

func (p Period) String() string   { return strconv.Itoa(p.N) + string(p.Unit) }
func (p Period) IsFullYear() bool { return p == FullYear }

var periodRE = regexp.MustCompile(`^(\d+)([dwmy])$`)

// ParsePeriod parses "90d", "12w", "6m" or "1y" (case-insensitive,
// surrounding whitespace ignored). Numbers too large for int become
// math.MaxInt; PeriodEnd reports those as ErrOutOfRange.
func ParsePeriod(s string) (Period, error) {
	m := periodRE.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return Period{}, invalidPeriod(s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil { // digits only, so the sole possible error is overflow
		n = math.MaxInt
	}
	if n < 1 {
		return Period{}, invalidPeriod(s)
	}
	return Period{N: n, Unit: m[2][0]}, nil
}

func invalidPeriod(s string) error {
	return fmt.Errorf("invalid period '%s': use <N>d, <N>w, <N>m or 1y (N >= 1)", s)
}

// ErrOutOfRange reports a period ending after year 9999.
var ErrOutOfRange = errors.New("period end out of range")

// maxUnits exceeds the number of days between years 1 and 9999, so any
// larger N is out of range and the arithmetic below can never overflow.
const maxUnits = 4_000_000

// AddMonths returns the same day n (>= 0) months later. A day missing in
// the target month rolls forward to the 1st of the following month
// (Jan 31 + 1 month = Mar 1; Feb 29 + 12 months = Mar 1).
func AddMonths(d time.Time, n int) time.Time {
	total := int(d.Month()) - 1 + n
	y, m := d.Year()+total/12, time.Month(total%12+1)
	if d.Day() <= DaysIn(y, m) {
		return Date(y, m, d.Day())
	}
	return Date(y, m+1, 1) // m < 12 here: December has 31 days
}

// AddMonthsClamped returns d shifted by n months (n may be negative). A day
// missing in the target month is clamped to that month's last day
// (Jan 31 + 1 month = Feb 28; Feb 29 + 12 months = Feb 28). This is the
// Date DSL's calendar arithmetic; --period uses AddMonths.
func AddMonthsClamped(d time.Time, n int) time.Time {
	total := int(d.Month()) - 1 + n
	y, m := d.Year()+floorDiv(total, 12), time.Month(mod(total, 12)+1)
	return Date(y, m, min(d.Day(), DaysIn(y, m)))
}

// PeriodEnd returns the inclusive last day of period p starting at start.
func PeriodEnd(start time.Time, p Period) (time.Time, error) {
	if p.N > maxUnits {
		return time.Time{}, ErrOutOfRange
	}
	var end time.Time
	switch p.Unit {
	case 'd':
		end = AddDays(start, p.N-1)
	case 'w':
		end = AddDays(start, 7*p.N-1)
	case 'm':
		end = AddDays(AddMonths(start, p.N), -1)
	default: // 'y'
		end = AddDays(AddMonths(start, 12*p.N), -1)
	}
	if end.Year() > 9999 {
		return time.Time{}, ErrOutOfRange
	}
	return end, nil
}

// FromEnd returns the shortest period from start whose last day is end
// (inclusive, not before start): 1y, else <N>m, else <N>w, else <N>d.
// It does not check the 1-year maximum.
func FromEnd(start, end time.Time) Period {
	for _, p := range append([]Period{FullYear}, months(11)...) {
		if e, err := PeriodEnd(start, p); err == nil && e.Equal(end) {
			return p
		}
	}
	days := DaysBetween(start, end) + 1
	if days%7 == 0 {
		return Period{N: days / 7, Unit: 'w'}
	}
	return Period{N: days, Unit: 'd'}
}

// months returns the periods 1m … nm.
func months(n int) []Period {
	ps := make([]Period, n)
	for i := range ps {
		ps[i] = Period{N: i + 1, Unit: 'm'}
	}
	return ps
}

// MaxEnd is the last day of the longest allowed period: one year.
func MaxEnd(start time.Time) time.Time { return AddDays(AddMonths(start, 12), -1) }
