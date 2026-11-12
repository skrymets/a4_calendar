package render

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"pinplan/internal/canvas"
	"pinplan/internal/timeline"
)

// Legend and page budget (REQ-LG-1, -5, -6, docs/timeline/03 §8). Sizes in pt.
const (
	lgRowH    = 7.5
	lgPad     = 6.0 // top + bottom gap around the rows
	lgTopGap  = 4.0 // bottom rule to the first row
	lgColGap  = 9.0
	lgSize    = 4.9
	lgSwatchW = 2.25
	lgSwatchH = 6.0
	lgHatchW  = 6.0 // hatch swatch is a small box
	lgKeyW    = 6.75
	lgGap     = 3.0 // swatch, key, title, dates
	lgMinRow  = 6.0 * MM
	lgCols    = 4
	lgColsFit = 5 // fallback: key + title only

	lgKeyRight   = lgSwatchW + lgGap + lgKeyW // keys are right-aligned here
	lgTextX      = lgKeyRight + lgGap         // title start
	lgTextMid    = 0.36                       // swatch centre above the baseline, × lgSize
	lgDashOn     = 1.5                        // dashed swatch
	lgDashOff    = 1.1
	lgOutlineW   = 0.4 // hatch swatch outline
	lgGlyphScale = 1.1 // milestone mark, × lgSize
	lgRingR      = 0.3 // ◯, × mark size
	lgRingW      = 0.6
)

var lgOutline = canvas.MustHex("#8C8C8C")

// legendPlan is how the legend fits the page.
type legendPlan struct {
	entries []timeline.LegendEntry // drawn, in order
	cols    int
	brief   bool // key + title only (REQ-LG-5)
	more    int  // entries left out; the last cell reads "+N more"
	rows    int
	height  float64
}

// planLegend chooses the legend layout for entries, given the grid height
// without a legend: 4 columns, else 5 columns without dates, else as many
// rows as keep the grid rows at 6 mm, the last entry "+N more" (REQ-LG-5).
func planLegend(entries []timeline.LegendEntry, gridH float64) legendPlan {
	n := len(entries)
	if n == 0 {
		return legendPlan{}
	}
	height := func(rows int) float64 { return lgPad + float64(rows)*lgRowH }
	fits := func(rows int) bool { return (gridH-height(rows))/31 >= lgMinRow }
	ceil := func(a, b int) int { return (a + b - 1) / b }

	p := legendPlan{entries: entries, cols: lgCols, rows: ceil(n, lgCols)}
	if !fits(p.rows) {
		p.cols, p.brief, p.rows = lgColsFit, true, ceil(n, lgColsFit)
	}
	if !fits(p.rows) {
		p.rows = max(1, int(math.Floor((gridH-31*lgMinRow-lgPad)/lgRowH)))
		keep := p.rows*p.cols - 1
		p.entries, p.more = entries[:keep], n-keep
	}
	p.height = height(p.rows)
	return p
}

// overflowLines is the full legend for stderr when entries were left out.
func (p legendPlan) overflowLines(all []timeline.LegendEntry) []string {
	if p.more == 0 {
		return nil
	}
	lines := make([]string, len(all))
	for i, e := range all {
		key := e.Key
		if e.Milestone {
			key = e.Mark.Glyph()
			if key == "" {
				key = "◯"
			}
		}
		lines[i] = strings.TrimSpace(fmt.Sprintf("%3s  %s  %s", key, e.Title, e.When))
	}
	return lines
}

// legend draws the plan below the grid's bottom rule at y = top.
func (t *tlDrawer) legend(p legendPlan, x0, top, gridW float64) {
	if p.rows == 0 {
		return
	}
	cw := (gridW - float64(p.cols-1)*lgColGap) / float64(p.cols)
	cell := func(i int) (x, y float64) { // left edge and baseline of entry i
		return x0 + float64(i%p.cols)*(cw+lgColGap), top - lgTopGap - float64(i/p.cols)*lgRowH - lgRowH*tlCapHeight
	}
	for i, e := range p.entries {
		x, y := cell(i)
		t.legendEntry(e, x, y, cw, p.brief)
	}
	if p.more > 0 {
		x, y := cell(len(p.entries))
		t.c.SetFillColor(InkSoft)
		t.c.SetFont(t.bold, lgSize)
		t.c.DrawString(x+lgTextX, y, fmt.Sprintf("+%d more", p.more))
	}
}

// legendEntry draws swatch, key, title and dates in a cell of width w; y is
// the text baseline.
func (t *tlDrawer) legendEntry(e timeline.LegendEntry, x, y, w float64, brief bool) {
	col := hexColor(e.Color)
	t.c.SetFillColor(col) // swatch, key and title; only the dates are grey
	t.legendSwatch(e, x, y)
	t.legendKey(e, col, x+lgKeyRight, y)
	t.legendText(e, x+lgTextX, y, x+w, brief)
}

// legendSwatch draws the small sample of the entry's rail or hatch.
func (t *tlDrawer) legendSwatch(e timeline.LegendEntry, x, y float64) {
	c := t.c
	sy := y + lgSize*lgTextMid - lgSwatchH/2 // centred on the text
	switch e.Swatch {
	case timeline.SwatchRail, timeline.SwatchGroup:
		c.RoundedRect(x, sy, lgSwatchW, lgSwatchH, tlCapR, tlCapR, true)
	case timeline.SwatchDashed:
		t.dashes(x, lgSwatchW, sy, sy+lgSwatchH, lgDashOn, lgDashOff)
	case timeline.SwatchBack:
		c.Rect(x+(lgSwatchW-tlBackW)/2, sy, tlBackW, lgSwatchH)
	case timeline.SwatchHatch:
		t.hatchLines(x, sy, lgHatchW, lgSwatchH, false)
		c.SetStrokeColor(lgOutline)
		c.SetLineWidth(lgOutlineW)
		c.RoundedRect(x, sy, lgHatchW, lgSwatchH, 0, 0, false)
	}
}

// legendKey draws the interval or group key, or the milestone's mark (◯ for
// a ring), right-aligned at right.
func (t *tlDrawer) legendKey(e timeline.LegendEntry, col canvas.RGB, right, y float64) {
	c := t.c
	if !e.Milestone {
		c.SetFont(t.bold, lgSize)
		c.DrawString(right-c.StringWidth(e.Key, t.bold, lgSize), y, e.Key)
		return
	}
	size := lgSize * lgGlyphScale
	if e.Mark != timeline.Ring {
		drawGlyph(c, e.Mark, right-glyphWidth(e.Mark, size), y, size)
		return
	}
	r := lgRingR * size
	c.SetStrokeColor(col)
	c.SetLineWidth(lgRingW)
	c.RoundedRect(right-2*r, y+glyphMid*size-r, 2*r, 2*r, r, r, false)
}

// legendText draws the bold title and the grey dates between x and right;
// the title is shortened first, the dates only when they alone are too wide.
func (t *tlDrawer) legendText(e timeline.LegendEntry, x, y, right float64, brief bool) {
	c := t.c
	boldW := func(s string) float64 { return c.StringWidth(s, t.bold, lgSize) }
	fontW := func(s string) float64 { return c.StringWidth(s, t.font, lgSize) }
	when := ""
	if !brief {
		when = fitText(e.When, right-x, tlEllipsis, fontW)
	}
	maxTitle := right - x
	if when != "" {
		maxTitle -= fontW(when) + lgGap
	}
	title := fitText(e.Title, maxTitle, tlEllipsis, boldW)
	c.SetFont(t.bold, lgSize)
	c.DrawString(x, y, title)
	if when != "" {
		c.SetFillColor(InkSoft)
		c.SetFont(t.font, lgSize)
		c.DrawString(x+boldW(title)+lgGap, y, when)
	}
}

// sourceNames names the timeline inputs: file names, then "command line"
// for --milestone/--interval declarations.
func (tl *Timeline) sourceNames() string {
	var names []string
	for _, f := range tl.Files {
		names = append(names, filepath.Base(f))
	}
	if tl.Inline {
		names = append(names, "command line")
	}
	return strings.Join(names, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// footerSuffix is REQ-PG-3's addition to the footer.
func (tl *Timeline) footerSuffix() string {
	layout := "packed"
	if tl.Layout.Flow {
		layout = "flow"
		if tl.Layout.Weekly {
			layout += " (weekly)"
		}
	}
	return fmt.Sprintf(" · timeline: %s — %s, %s · layout: %s", tl.sourceNames(),
		plural(tl.Milestones, "milestone"), plural(tl.Intervals, "interval"), layout)
}
