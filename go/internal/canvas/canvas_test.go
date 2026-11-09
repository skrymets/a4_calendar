package canvas

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"a4calendar/internal/fonts"
)

func TestHex(t *testing.T) {
	ok := map[string]RGB{
		"#1A3A7A":  {0x1A, 0x3A, 0x7A},
		"0x8E1B1B": {0x8E, 0x1B, 0x1B},
		"0XcC6600": {0xCC, 0x66, 0x00},
	}
	for in, want := range ok {
		if got, err := Hex(in); err != nil || got != want {
			t.Errorf("Hex(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1A3A7A", "#12345", "#GGGGGG", "nope"} {
		if _, err := Hex(in); err == nil {
			t.Errorf("Hex(%q): expected error", in)
		}
	}
}

// Widths must equal ReportLab's pdfmetrics.stringWidth (values from Python).
func TestHelveticaWidthsMatchReportLab(t *testing.T) {
	c := New(595.2755905511812, 841.8897637795277)
	cases := []struct {
		s    string
		size float64
		want float64
	}{
		{"07 NOV 2026  –  04 FEB 2027 · x", 9, 134.0820},
		{"28W", 7, 14.3920},
		{"28T", 7, 12.0610},
		{"000/00", 3.9, 11.9262},
		{"YEARLY CALENDAR", 10, 96.6900},
	}
	for _, k := range cases {
		if got := c.StringWidth(k.s, Helvetica, k.size); math.Abs(got-k.want) > 1e-3 {
			t.Errorf("StringWidth(%q, %v) = %.4f; want %.4f", k.s, k.size, got, k.want)
		}
	}
}

func TestDrawAndSave(t *testing.T) {
	c := New(595.2755905511812, 841.8897637795277)
	c.pdf.SetCompression(false)
	pt := TTF("PTSansNarrow", fonts.PTSansNarrow)

	c.SetTitle("TEST 2026-11-07 – 2027-02-04")
	c.SetFillColor(MustHex("#E6E6E6"))
	c.Rect(10, 10, 100, 20)
	c.SetStrokeColor(MustHex("#8C8C8C"))
	c.SetLineWidth(0.25)
	c.SetDash(0.4, 1.2)
	c.SetRoundCap(true)
	c.Line(10, 50, 200, 50)
	c.ClearDash()
	c.SetRoundCap(false)
	c.SetFillColor(MustHex("#1F1F1F"))
	c.SetFont(Helvetica, 9)
	c.DrawSpaced(10, 100, "07 NOV 2026  –  04 FEB 2027", 0.8)
	c.SetFont(pt, 12)
	c.DrawString(10, 140, "ПВСЧПСН СІЧ")
	if w := c.StringWidth("ПВСЧПСН", pt, 12); w <= 0 {
		t.Errorf("TTF width = %v", w)
	}

	path := filepath.Join(t.TempDir(), "t.pdf")
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"%PDF-", "0.80 Tc", "0 Tc", "[0.40 1.20] 0.00 d", "[] 0.00 d", "1 J", "0 J", "/Helvetica"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("PDF lacks %q", want)
		}
	}
	// Bottom-left coordinates reach the content stream unchanged (fpdf's
	// top-down y is flipped back internally): baseline y=100 -> "100.00".
	if !bytes.Contains(data, []byte("BT 10.00 100.00 Td")) {
		t.Error("text not placed with bottom-left coordinates")
	}
}
