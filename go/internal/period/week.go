package period

import "time"

// Weekday returns Monday=0 .. Sunday=6, like Python's date.weekday().
func Weekday(d time.Time) int { return (int(d.Weekday()) + 6) % 7 }

// WeekNumber numbers the week of day in a calendar starting at start:
//   - "iso":      ISO-8601 week of year
//   - "rolling":  7-day blocks counted from start
//   - "relative": week 1 is the calendar week containing start; weeks
//     begin on weekStart (0=Mon..6=Sun)
func WeekNumber(day, start time.Time, mode string, weekStart int) int {
	switch mode {
	case "iso":
		_, w := day.ISOWeek()
		return w
	case "rolling":
		return floorDiv(DaysBetween(start, day), 7) + 1
	}
	anchor := func(d time.Time) time.Time {
		return AddDays(d, -mod(Weekday(d)-weekStart, 7))
	}
	return floorDiv(DaysBetween(anchor(start), anchor(day)), 7) + 1
}

// mod and floorDiv follow Python's % and // semantics for negatives.
func mod(a, b int) int { return ((a % b) + b) % b }

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// YM is a calendar month.
type YM struct {
	Year  int
	Month time.Month
}

// MonthsBetween lists every month from start's to end's, inclusive.
func MonthsBetween(start, end time.Time) []YM {
	var out []YM
	y, m := start.Year(), start.Month()
	for y < end.Year() || (y == end.Year() && m <= end.Month()) {
		out = append(out, YM{y, m})
		if m == 12 {
			y, m = y+1, 1
		} else {
			m++
		}
	}
	return out
}
