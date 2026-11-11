// Package canvas is the drawing surface used by render. Coordinates are
// PDF-native: points, origin at the bottom-left corner.
package canvas

import (
	"fmt"
	"strconv"
	"strings"
)

// RGB is an 8-bit colour.
type RGB struct{ R, G, B uint8 }

// Hex parses "#RRGGBB" or "0xRRGGBB".
func Hex(s string) (RGB, error) {
	h := s
	switch {
	case strings.HasPrefix(h, "#"):
		h = h[1:]
	case strings.HasPrefix(strings.ToLower(h), "0x"):
		h = h[2:]
	default:
		h = ""
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if len(h) != 6 || err != nil {
		return RGB{}, fmt.Errorf("invalid colour '%s': use #RRGGBB or 0xRRGGBB", s)
	}
	return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}

// MustHex is Hex for constants; it panics on malformed input.
func MustHex(s string) RGB {
	c, err := Hex(s)
	if err != nil {
		panic(err)
	}
	return c
}

// Font is either the built-in Helvetica (TTF == nil) or an embedded TrueType font.
type Font struct {
	Name string
	TTF  []byte
}

// Helvetica is the PDF core font, not embedded, WinAnsi encoded.
var Helvetica = Font{Name: "Helvetica"}

// TTF returns a TrueType font; name identifies it inside the document.
func TTF(name string, data []byte) Font { return Font{Name: name, TTF: data} }

// Canvas is the drawing surface the calendar uses.
type Canvas interface {
	SetFillColor(c RGB) // also the text colour
	SetStrokeColor(c RGB)
	SetLineWidth(w float64)
	SetDash(on, off float64)
	ClearDash()
	SetRoundCap(round bool)
	Rect(x, y, w, h float64) // filled, no stroke
	Line(x1, y1, x2, y2 float64)
	SetFont(f Font, size float64)
	DrawString(x, y float64, s string)
	DrawSpaced(x, y float64, s string, spacing float64) // letter-spacing via Tc
	StringWidth(s string, f Font, size float64) float64
	SetTitle(s string)
}
