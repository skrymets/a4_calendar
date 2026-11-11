// Package render draws the calendar page; it is a line-by-line port of
// build() in yearly_calendar.py and works in PDF-native coordinates.
package render

import "a4calendar/internal/canvas"

var (
	Ink       = canvas.MustHex("#1F1F1F")
	InkSoft   = canvas.MustHex("#7A7A7A")
	InkFaint  = canvas.MustHex("#C8C8C8")
	HeaderBG  = canvas.MustHex("#E6E6E6")
	WeekendBG = canvas.MustHex("#E6E6E6")
	Rule      = canvas.MustHex("#8C8C8C")
	Guide     = canvas.MustHex("#B4B4B4")
	Divider   = canvas.MustHex("#D0D0D0")
)

const (
	MM        = 72 / 25.4
	TitleSize = 20.0
	HeaderH   = 24.0
	MainSize  = 7.0 // "7W"
	SmallSize = 3.9 // "001/01"
	CellPad   = 1.8 // left padding inside a cell, pt
)

// Paper sizes in points.
const (
	A4W     = 210 * MM
	A4H     = 297 * MM
	LetterW = 612.0
	LetterH = 792.0
)
