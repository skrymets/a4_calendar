package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"pinplan/internal/canvas"
	"pinplan/internal/period"
)

// Align is the horizontal anchor of DrawSpaced's x.
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// SpacedWidth is the width of s with letter-spacing.
func SpacedWidth(c canvas.Canvas, s string, f canvas.Font, size, spacing float64) float64 {
	return c.StringWidth(s, f, size) + spacing*float64(utf8.RuneCountInString(s)-1)
}

// DrawSpaced draws letter-spaced text anchored at x.
func DrawSpaced(c canvas.Canvas, s string, x, y float64, f canvas.Font, size, spacing float64, a Align) {
	w := SpacedWidth(c, s, f, size, spacing)
	switch a {
	case AlignRight:
		x -= w
	case AlignCenter:
		x -= w / 2
	}
	c.SetFont(f, size)
	c.DrawSpaced(x, y, s, spacing)
}

// FitText truncates text with "..." so that width(text) <= maxW.
func FitText(text string, maxW float64, width func(string) float64) string {
	return fitText(text, maxW, "...", width)
}

// fitText truncates text with ell so that width(text) <= maxW; "" if
// nothing fits.
func fitText(text string, maxW float64, ell string, width func(string) float64) string {
	if width(text) <= maxW {
		return text
	}
	r := []rune(text)
	for len(r) > 0 {
		r = r[:len(r)-1]
		cut := strings.TrimRightFunc(string(r), unicode.IsSpace) + ell
		if width(cut) <= maxW {
			return cut
		}
	}
	return ""
}

// PeriodLabel is the top-right label: the bare year for a Jan 1 - Dec 31
// period, otherwise the date range.
func PeriodLabel(start, end time.Time, monthNames []string) string {
	if start.Month() == 1 && start.Day() == 1 && end.Equal(period.Date(start.Year(), 12, 31)) {
		return strconv.Itoa(start.Year())
	}
	return fmt.Sprintf("%02d %s %d  –  %02d %s %d",
		start.Day(), monthNames[start.Month()-1], start.Year(),
		end.Day(), monthNames[end.Month()-1], end.Year())
}

var unitNames = map[byte]string{'d': "DAY", 'w': "WEEK", 'm': "MONTH", 'y': "YEAR"}

// DefaultTitle is the title used when --title is not given.
func DefaultTitle(p period.Period) string {
	if p.IsFullYear() {
		return "YEARLY CALENDAR"
	}
	return fmt.Sprintf("%d-%s CALENDAR", p.N, unitNames[p.Unit])
}

// DefaultOutput is the file name used when -o is not given.
func DefaultOutput(start time.Time, p period.Period) string {
	suffix := ""
	if !p.IsFullYear() {
		suffix = "_" + p.String()
	}
	return fmt.Sprintf("calendar_%s%s.pdf", start.Format("2006-01-02"), suffix)
}
