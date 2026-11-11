package render

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"a4calendar/internal/canvas"
	"a4calendar/internal/timeline"
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
	for i, e := range p.entries {
		x := x0 + float64(i%p.cols)*(cw+lgColGap)
		y := top - lgTopGap - float64(i/p.cols)*lgRowH - lgRowH*0.72 // baseline
		t.legendEntry(e, x, y, cw, p.brief)
	}
	if p.more > 0 {
		i := len(p.entries)
		x := x0 + float64(i%p.cols)*(cw+lgColGap) + lgSwatchW + lgGap + lgKeyW + lgGap
		y := top - lgTopGap - float64(i/p.cols)*lgRowH - lgRowH*0.72
		t.c.SetFillColor(InkSoft)
		t.c.SetFont(t.bold, lgSize)
		t.c.DrawString(x, y, fmt.Sprintf("+%d more", p.more))
	}
}

// legendEntry draws swatch, key, title and dates in a cell of width w.
func (t *tlDrawer) legendEntry(e timeline.LegendEntry, x, y, w float64, brief bool) {
	c := t.c
	col := hexColor(e.Color)
	sy := y + lgSize*0.36 - lgSwatchH/2 // swatch centred on the text
	c.SetFillColor(col)
	switch e.Swatch {
	case timeline.SwatchRail, timeline.SwatchGroup:
		c.RoundedRect(x, sy, lgSwatchW, lgSwatchH, tlCapR, tlCapR, true)
	case timeline.SwatchDashed:
		t.dashes(x, lgSwatchW, sy, sy+lgSwatchH, 1.5, 1.1)
	case timeline.SwatchBack:
		c.Rect(x+(lgSwatchW-tlBackW)/2, sy, tlBackW, lgSwatchH)
	case timeline.SwatchHatch:
		t.hatchLines(x, sy, lgHatchW, lgSwatchH, false)
		c.SetStrokeColor(lgOutline)
		c.SetLineWidth(0.4)
		c.RoundedRect(x, sy, lgHatchW, lgSwatchH, 0, 0, false)
	}

	keyR := x + lgSwatchW + lgGap + lgKeyW // keys are right-aligned here
	if e.Milestone {
		size := lgSize * 1.1
		c.SetFillColor(col)
		if e.Mark == timeline.Ring {
			r := 0.3 * size
			c.SetStrokeColor(col)
			c.SetLineWidth(0.6)
			c.RoundedRect(keyR-2*r, y+0.33*size-r, 2*r, 2*r, r, r, false)
		} else {
			gw := glyphWidth(e.Mark, size)
			drawGlyph(c, e.Mark, keyR-gw, y, size)
		}
	} else {
		c.SetFillColor(col)
		c.SetFont(t.bold, lgSize)
		c.DrawString(keyR-c.StringWidth(e.Key, t.bold, lgSize), y, e.Key)
	}

	tx := keyR + lgGap
	right := x + w
	when, whenW := e.When, 0.0
	if !brief && when != "" {
		whenW = c.StringWidth(when, t.font, lgSize)
		if whenW > right-tx { // dates alone are too wide: shorten them
			when = fitText(when, right-tx, tlEllipsis, func(s string) float64 { return c.StringWidth(s, t.font, lgSize) })
			whenW = c.StringWidth(when, t.font, lgSize)
		}
	}
	maxTitle := right - tx
	if whenW > 0 {
		maxTitle -= whenW + lgGap
	}
	title := fitText(e.Title, maxTitle, tlEllipsis, func(s string) float64 { return c.StringWidth(s, t.bold, lgSize) })
	c.SetFillColor(col)
	c.SetFont(t.bold, lgSize)
	c.DrawString(tx, y, title)
	if whenW > 0 {
		wx := tx + lgGap
		if title != "" {
			wx += c.StringWidth(title, t.bold, lgSize)
		}
		c.SetFillColor(InkSoft)
		c.SetFont(t.font, lgSize)
		c.DrawString(wx, y, when)
	}
}

// glyphWidth is the width drawGlyph uses for m at size.
func glyphWidth(m timeline.Mark, size float64) float64 {
	switch m {
	case timeline.Diamond:
		return 0.6 * size
	case timeline.Star:
		return 2 * 0.38 * size * math.Cos(math.Pi/10)
	case timeline.Dot:
		return 0.48 * size
	}
	return 0
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
