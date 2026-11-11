package canvas

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// Widths are the standard Helvetica AFM metrics.
func TestHelveticaWidths(t *testing.T) {
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
	for _, want := range []string{"%PDF-", "0.80 Tc", "[0.40 1.20] 0.00 d", "[] 0.00 d", "/Helvetica"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("PDF lacks %q", want)
		}
	}
	// Whole-line matches so that "0 J" cannot match inside "10 J" and "0 Tc"
	// cannot match inside "0.80 Tc".
	for _, want := range []string{"\n1 J\n", "\n0 J\n"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("PDF lacks line %q", want)
		}
	}
	// Bottom-left coordinates reach the content stream unchanged (fpdf's
	// top-down y is flipped back internally): baseline y=100 -> "100.00".
	if !bytes.Contains(data, []byte("BT 10.00 100.00 Td")) {
		t.Error("text not placed with bottom-left coordinates")
	}
	// Tc must be set before the spaced text and reset to 0 after it.
	i := bytes.Index(data, []byte("\n0.80 Tc\n"))
	if i < 0 {
		t.Fatal("PDF lacks line \"\\n0.80 Tc\\n\"")
	}
	j := bytes.Index(data[i:], []byte("BT 10.00 100.00 Td"))
	if j < 0 {
		t.Fatal("spaced text not found after its Tc")
	}
	j += i
	k := bytes.Index(data[j:], []byte("\n0 Tc\n"))
	if k < 0 {
		t.Fatal("PDF lacks line \"\\n0 Tc\\n\" after the spaced text")
	}
	k += j
	if !(i < j && j < k) {
		t.Error("Tc must be set before the spaced text and reset to 0 after it")
	}
}

func TestNonBMPRuneOnTTF(t *testing.T) {
	c := New(595, 842)
	pt := TTF("PTSansNarrow", fonts.PTSansNarrow)
	c.SetFont(pt, 10)
	for _, s := range []string{"😀", "a𐀀b", "􏿿"} {
		if w := c.StringWidth(s, pt, 10); w <= 0 {
			t.Errorf("StringWidth(%q) = %v", s, w)
		}
		c.DrawString(10, 10, s)
	}
	out := filepath.Join(t.TempDir(), "e.pdf")
	if err := c.Save(out); err != nil {
		t.Fatal(err)
	}
}

func TestSaveFailureLeavesNoFile(t *testing.T) {
	c := New(595, 842)
	out := filepath.Join(t.TempDir(), "nodir", "x.pdf")
	if err := c.Save(out); err == nil {
		t.Error("expected error")
	}
}

func TestBoldHelvetica(t *testing.T) {
	c := New(595.2755905511812, 841.8897637795277)
	c.pdf.SetCompression(false)
	bold := Helvetica.Bolder()
	// Helvetica-Bold AFM: "A" 722, "B" 722; regular: 667, 667
	if got := c.StringWidth("AB", bold, 10); math.Abs(got-14.44) > 1e-3 {
		t.Errorf("bold width = %.4f; want 14.44", got)
	}
	c.SetFont(bold, 10)
	c.DrawString(10, 10, "AB")
	var buf bytes.Buffer
	if err := c.pdf.Output(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/BaseFont /Helvetica-Bold")) {
		t.Error("Helvetica-Bold not used")
	}
}

// Synthetic bold and the raw shapes must leave the graphics state balanced.
func TestSyntheticBoldAndShapes(t *testing.T) {
	c := New(595.2755905511812, 841.8897637795277)
	c.pdf.SetCompression(false)
	pt := TTF("PTSansNarrow", fonts.PTSansNarrow).Bolder()
	c.SetFillColor(MustHex("#8E1B1B"))
	c.SetFont(pt, 8)
	c.DrawString(10, 10, "Bold")
	c.RoundedRect(10, 20, 3, 30, 0.75, 0, true)
	c.RoundedRect(20, 20, 10, 8, 4, 4, false)
	c.Polygon([]Point{{1, 1}, {2, 3}, {3, 1}})
	c.Circle(50, 50, 2)
	var buf bytes.Buffer
	if err := c.pdf.Output(&buf); err != nil {
		t.Fatal(err)
	}
	pdf := buf.String()
	for _, want := range []string{"q 0.557 0.106 0.106 RG 0.280 w 2 Tr", "10.750 50.000 m", "1.000 1.000 m"} {
		if !strings.Contains(pdf, want) {
			t.Errorf("content lacks %q", want)
		}
	}
	q := regexp.MustCompile(`(?m)(^|\s)q(\s|$)`).FindAllStringIndex(pdf, -1)
	Q := regexp.MustCompile(`(?m)(^|\s)Q(\s|$)`).FindAllStringIndex(pdf, -1)
	if len(q) != len(Q) {
		t.Errorf("unbalanced graphics state: %d q, %d Q", len(q), len(Q))
	}
}
