package render

import (
	"unicode/utf8"

	"a4calendar/internal/canvas"
)

type drawn struct {
	op, text string
	x, y     float64
	size     float64
	x2       float64 // lines: end x
}

// fakeCanvas records text output; widths are 0.5*size per rune.
type fakeCanvas struct {
	size    float64
	title   string
	subject string
	out     []drawn
}

func (f *fakeCanvas) SetFillColor(canvas.RGB)                 {}
func (f *fakeCanvas) SetStrokeColor(canvas.RGB)               {}
func (f *fakeCanvas) SetLineWidth(float64)                    {}
func (f *fakeCanvas) SetDash(float64, float64)                {}
func (f *fakeCanvas) ClearDash()                              {}
func (f *fakeCanvas) SetRoundCap(bool)                        {}
func (f *fakeCanvas) Rect(float64, float64, float64, float64) {}
func (f *fakeCanvas) Line(x1, y1, x2, _ float64) {
	f.out = append(f.out, drawn{op: "line", x: x1, y: y1, x2: x2})
}
func (f *fakeCanvas) SetFont(_ canvas.Font, size float64) { f.size = size }

func (f *fakeCanvas) RoundedRect(x, y, w, h, _, _ float64, fill bool) {
	op := "roundrect-stroke"
	if fill {
		op = "roundrect"
	}
	f.out = append(f.out, drawn{op: op, x: x, y: y})
}

func (f *fakeCanvas) Polygon(pts []canvas.Point) {
	f.out = append(f.out, drawn{op: "polygon", x: pts[0].X, y: pts[0].Y})
}

func (f *fakeCanvas) Circle(x, y, _ float64) { f.out = append(f.out, drawn{op: "circle", x: x, y: y}) }
func (f *fakeCanvas) SetTitle(s string)      { f.title = s }
func (f *fakeCanvas) SetSubject(s string)    { f.subject = s }

func (f *fakeCanvas) DrawString(x, y float64, s string) {
	f.out = append(f.out, drawn{op: "string", text: s, x: x, y: y, size: f.size})
}

func (f *fakeCanvas) DrawSpaced(x, y float64, s string, _ float64) {
	f.out = append(f.out, drawn{op: "spaced", text: s, x: x, y: y, size: f.size})
}

func (f *fakeCanvas) StringWidth(s string, _ canvas.Font, size float64) float64 {
	return float64(utf8.RuneCountInString(s)) * size * 0.5
}

func (f *fakeCanvas) texts(op string) []string {
	var r []string
	for _, d := range f.out {
		if d.op == op {
			r = append(r, d.text)
		}
	}
	return r
}
