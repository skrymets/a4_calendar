package canvas

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-pdf/fpdf"
)

// FPDF is a single-page Canvas backed by go-pdf/fpdf.
type FPDF struct {
	pdf    *fpdf.Fpdf
	pageH  float64
	cp1252 func(string) string
	cur    Font
	size   float64
	loaded map[string]bool
}

var _ Canvas = (*FPDF)(nil)

// New starts a one-page document; sizes are in points.
func New(pageW, pageH float64) *FPDF {
	p := fpdf.NewCustom(&fpdf.InitType{UnitStr: "pt", Size: fpdf.SizeType{Wd: pageW, Ht: pageH}})
	p.SetMargins(0, 0, 0)
	p.SetAutoPageBreak(false, 0)
	p.AddPage()
	return &FPDF{
		pdf:    p,
		pageH:  pageH,
		cp1252: p.UnicodeTranslatorFromDescriptor(""),
		loaded: map[string]bool{},
	}
}

// family registers f on first use and returns its fpdf family name.
func (c *FPDF) family(f Font) string {
	if f.TTF == nil {
		return "helvetica"
	}
	fam := "ttf-" + f.Name // prefix: never collide with core font names
	if !c.loaded[fam] {
		c.pdf.AddUTF8FontFromBytes(fam, "", f.TTF)
		c.loaded[fam] = true
	}
	return fam
}

// enc converts s to what fpdf expects for f: cp1252 bytes for core fonts.
func (c *FPDF) enc(f Font, s string) string {
	if f.TTF == nil {
		return c.cp1252(s)
	}
	return replaceNonBMP(s)
}

// replaceNonBMP swaps runes >= U+10000 for U+FFFD: fpdf's UTF-8 font tables
// only cover the BMP and panic on anything beyond it.
func replaceNonBMP(s string) string {
	if !strings.ContainsFunc(s, func(r rune) bool { return r >= 0x10000 }) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r >= 0x10000 {
			return 0xFFFD
		}
		return r
	}, s)
}

// CheckTTF reports whether data can be used as an fpdf TrueType font. fpdf
// panics on corrupt fonts, so it does a trial load, draw and render on a
// throwaway document and converts any panic into an error.
func CheckTTF(data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	p := fpdf.New("P", "pt", "A4", "")
	p.AddPage()
	p.AddUTF8FontFromBytes("probe", "", data)
	p.SetFont("probe", "", 10)
	p.GetStringWidth("A")
	p.Text(10, 10, "A")
	if err := p.Error(); err != nil {
		return err
	}
	if err := p.Output(io.Discard); err != nil {
		return err
	}
	return p.Error()
}

func (c *FPDF) SetFillColor(col RGB) {
	r, g, b := int(col.R), int(col.G), int(col.B)
	c.pdf.SetFillColor(r, g, b)
	c.pdf.SetTextColor(r, g, b) // same colour: Text() then adds no q/Q wrapper
}

func (c *FPDF) SetStrokeColor(col RGB)  { c.pdf.SetDrawColor(int(col.R), int(col.G), int(col.B)) }
func (c *FPDF) SetLineWidth(w float64)  { c.pdf.SetLineWidth(w) }
func (c *FPDF) SetDash(on, off float64) { c.pdf.SetDashPattern([]float64{on, off}, 0) }
func (c *FPDF) ClearDash()              { c.pdf.SetDashPattern(nil, 0) }

func (c *FPDF) SetRoundCap(round bool) {
	if round {
		c.pdf.SetLineCapStyle("round")
	} else {
		c.pdf.SetLineCapStyle("butt")
	}
}

func (c *FPDF) Rect(x, y, w, h float64) { c.pdf.Rect(x, c.pageH-y-h, w, h, "F") }

func (c *FPDF) Line(x1, y1, x2, y2 float64) { c.pdf.Line(x1, c.pageH-y1, x2, c.pageH-y2) }

func (c *FPDF) SetFont(f Font, size float64) {
	c.cur, c.size = f, size
	c.pdf.SetFont(c.family(f), "", size)
}

func (c *FPDF) DrawString(x, y float64, s string) {
	c.pdf.Text(x, c.pageH-y, c.enc(c.cur, s))
}

// DrawSpaced draws with letter-spacing via the Tc operator and resets it,
// since Tc persists in the text state (as draw_spaced in Python).
func (c *FPDF) DrawSpaced(x, y float64, s string, spacing float64) {
	c.pdf.RawWriteStr(fmt.Sprintf("%.2f Tc", spacing)) // RawWriteStr appends "\n"
	c.DrawString(x, y, s)
	c.pdf.RawWriteStr("0 Tc")
}

// StringWidth measures s in f at size without changing the current font.
func (c *FPDF) StringWidth(s string, f Font, size float64) float64 {
	c.pdf.SetFont(c.family(f), "", size)
	w := c.pdf.GetStringWidth(c.enc(f, s))
	if c.size > 0 {
		c.pdf.SetFont(c.family(c.cur), "", c.size)
	}
	return w
}

func (c *FPDF) SetTitle(s string) { c.pdf.SetTitle(s, true) }

// Save renders the PDF in memory and only then writes path, so a failure
// (including an fpdf panic) never creates or truncates the output file.
func (c *FPDF) Save(path string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cannot render PDF: %v", r)
		}
	}()
	var buf bytes.Buffer
	if err := c.pdf.Output(&buf); err != nil {
		return err
	}
	if err := c.pdf.Error(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
