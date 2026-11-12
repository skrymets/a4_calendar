package render

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"a4calendar/internal/canvas"
	"a4calendar/internal/timeline"
)

// Timeline drawing (docs/timeline/02 §2-5). Sizes in pt.
var (
	tlGrey = canvas.MustHex("#7A7A7A") // overflow rail and "+N"
	// hatch: ink at 16 % opacity, as in the reference pictures, over white
	// (#DBDBDB, 02 §3) and over weekend grey
	tlHatch       = canvas.MustHex("#DBDBDB")
	tlHatchOnGrey = canvas.MustHex("#C6C6C6")
)

const (
	tlCapR          = 0.75 // rail end radius
	tlDashOn        = 2.25 // dashed rail
	tlDashOff       = 1.5
	tlOverflowW     = 1.1 // overflow rail width and dots
	tlBackW         = 0.75
	tlBackOff       = 1.5  // margin line: left of guideStart
	tlBackStep      = 2.25 // 2nd and 3rd margin line
	tlBackDot       = 1.5
	tlTickH         = 1.1
	tlTickExtra     = 2.25
	tlHatchW        = 0.75
	tlHatchPitch    = 3.75
	tlRingW         = 0.75
	tlTopFrom       = 2.25 // top row from the cell top
	tlTextGap       = 1.5  // note area inset from guideStart
	tlItemGap       = 1.9  // between top-row items
	tlLabelSize     = 4.5
	tlLabelTrack    = 0.06 // em
	tlKeySizeN      = 4.1  // narrow keys
	tlKeyLineN      = 5.25
	tlTitleSize     = 5.6
	tlTitleSizeN    = 4.9 // narrow pages
	tlTitleGapN     = 1.0 // narrow pages: between the glyphs and the title
	tlTitleMinChars = 3   // a milestone title cut shorter is left out
	tlGlyphWide     = 4.9
	tlGlyphNarrow   = 5.25
	tlCountSize     = 4.1
	tlCountTrack    = 0.04 // em
	tlEllipsis      = "…"  // timeline text; the page title keeps "..."
	tlSeam          = 0.15 // a running rail overlaps the next cell, so no hairline gap shows
	tlGlyphGap      = 1.5  // after the weekday letter
	tlMarkGap       = 0.5  // between glyphs, and between the glyphs and a title
	tlGlyphInsetN   = 0.75 // narrow page: glyph inset from guideStart
	tlZoneGap       = 0.75 // text stops this far before the rail zone
	tlRingPad       = 0.75 // ring: 1 px each side of the day number (02 §3)
	tlRingBelow     = 1.6  // ring bottom below the label baseline
	tlRingH         = 1.18 // ring height, × MainSize
	tlCapHeight     = 0.72 // cap height as a share of the font size
)

// Vector glyphs (REQ-PG-5), as shares of the font size.
const (
	glyphMid     = 0.33 // centre above the baseline (diamond, star)
	diamondHalfW = 0.3
	diamondHalfH = 0.36
	starOuterR   = 0.38
	starInnerR   = 0.382 // of the outer radius
	dotR         = 0.24
	dotMid       = 0.3 // dot centre above the baseline
)

// tlDrawer draws a timeline.Layout into the calendar's cells.
type tlDrawer struct {
	c          canvas.Canvas
	tl         *Timeline
	l          *timeline.Layout
	font, bold canvas.Font
	anchor     float64 // y every dash pattern is aligned to, so dashes run on across cells
}

// tlCell is the geometry of one cell, from Build.
type tlCell struct {
	x0, w, yt, yb float64
	mainY         float64 // label baseline = note line
	guideX        float64 // guideStart
	numX, numW    float64 // day number
	letterEnd     float64 // right of the weekday letter
}

// noteRight is where timeline text in the cell must stop: before the rail zone.
func (g tlCell) noteRight(day *timeline.Day) float64 { return g.x0 + g.w - day.Zone - tlZoneGap }

// span is the vertical extent of a rail or margin line in the cell (REQ-IV-2):
// a capped end stops CapInset inside the cell, rounded with radius r; a
// running bottom end overlaps the next cell by tlSeam.
func (g tlCell) span(topCap, botCap bool, r float64) (top, bot, rTop, rBot float64) {
	top, bot = g.yt, g.yb-tlSeam
	if topCap {
		top, rTop = g.yt-timeline.CapInset, r
	}
	if botCap {
		bot, rBot = g.yb+timeline.CapInset, r
	}
	return top, bot, rTop, rBot
}

func newTLDrawer(c canvas.Canvas, t *Timeline, font canvas.Font, pageH float64) *tlDrawer {
	if t == nil || t.Layout == nil {
		return nil
	}
	return &tlDrawer{c: c, tl: t, l: t.Layout, font: font, bold: font.Bolder(), anchor: pageH}
}

// day is the timeline content of day d (1-based) in column ci, or nil.
func (t *tlDrawer) day(ci, d int) *timeline.Day {
	if t == nil || ci >= len(t.l.Columns) || d < 1 || d > 31 {
		return nil
	}
	return t.l.Columns[ci].Days[d-1]
}

// hexColor converts a layout colour; timeline.ReadKeys has validated every
// colour, so the fallback is never used.
func hexColor(s string) canvas.RGB {
	c, err := canvas.Hex(s)
	if err != nil {
		return Ink
	}
	return c
}

// hatch draws REQ-IV-7's 135° lines over the cell background. The lines are
// y = x + b in page coordinates, so they continue from cell to cell.
func (t *tlDrawer) hatch(ci, d int, x0, yb, w, h float64, onGrey bool) {
	if day := t.day(ci, d); day != nil && day.Hatch {
		t.hatchLines(x0, yb, w, h, onGrey)
	}
}

// hatchLines hatches the rectangle; also the legend's hatch swatch.
func (t *tlDrawer) hatchLines(x0, yb, w, h float64, onGrey bool) {
	c := t.c
	c.SetStrokeColor(tlHatch)
	if onGrey {
		c.SetStrokeColor(tlHatchOnGrey)
	}
	c.SetLineWidth(tlHatchW)
	step := tlHatchPitch * math.Sqrt2
	x1, yt := x0+w, yb+h
	for k := math.Ceil((yb - x1) / step); k*step <= yt-x0; k++ {
		b := k * step
		xa, xb := max(x0, yb-b), min(x1, yt-b)
		if xb > xa {
			c.Line(xa, xa+b, xb, xb+b)
		}
	}
}

// dashes fills x..x+w between bot and top with on/off dashes counted from
// the anchor, so a dashed line drawn cell by cell looks unbroken.
func (t *tlDrawer) dashes(x, w, bot, top, on, off float64) {
	p := on + off
	for n := math.Floor((t.anchor - top) / p); t.anchor-n*p > bot; n++ {
		hi := min(top, t.anchor-n*p)
		lo := max(bot, t.anchor-n*p-on)
		if hi > lo {
			t.c.Rect(x, lo, w, hi-lo)
		}
	}
}

// cell draws everything of one in-period day except the hatch.
func (t *tlDrawer) cell(ci, d int, g tlCell) {
	day := t.day(ci, d)
	if day == nil {
		return
	}
	col := t.l.Columns[ci]
	t.backLines(day, g)
	t.rails(col, day, g)
	marksEnd := t.milestoneMarks(day, g)
	countEnd := t.countdown(day, g)
	t.topRow(day, g, countEnd)
	t.milestoneTitle(day, g, marksEnd)
}

// backLines draws layer=back margin lines (REQ-IV-10).
func (t *tlDrawer) backLines(day *timeline.Day, g tlCell) {
	for _, b := range day.Back {
		x := g.guideX - tlBackOff + float64(b.Slot)*tlBackStep
		top, bot, rTop, rBot := g.span(b.TopCap, b.BotCap, tlBackW/2)
		t.c.SetFillColor(hexColor(b.Color))
		if b.Dotted() {
			t.dashes(x, tlBackW, bot, top, tlBackDot, tlBackDot)
		} else {
			t.c.RoundedRect(x, bot, tlBackW, top-bot, rTop, rBot, true)
		}
	}
}

// rails draws interval rails with caps, dashes, jogs and group ticks, and
// the overflow rail (REQ-IV-1…6, REQ-PK-4, REQ-FL-3, REQ-GR-2, -3).
func (t *tlDrawer) rails(col *timeline.Column, day *timeline.Day, g tlCell) {
	laneRight := func(lane int) float64 { return g.x0 + g.w - (t.l.Pad + float64(lane)*col.Step) }
	for _, r := range day.Rails {
		t.c.SetFillColor(hexColor(r.Color))
		t.rail(r, col.RailW, g, laneRight)
		t.ticks(r, col.RailW, g, laneRight(r.Lane))
	}
	if day.OverflowLane >= 0 {
		t.c.SetFillColor(tlGrey)
		t.dashes(laneRight(day.OverflowLane)-tlOverflowW, tlOverflowW, g.yb, g.yt, tlOverflowW, tlOverflowW)
	}
}

// rail draws one rail segment with its caps, dashes and flow jog.
func (t *tlDrawer) rail(r timeline.Rail, railW float64, g tlCell, laneRight func(int) float64) {
	w, right := railW, laneRight(r.Lane)
	if r.Thick { // grows towards the column edge
		w, right = w+timeline.ThickExtra, right+timeline.ThickExtra
	}
	top, bot, rTop, rBot := g.span(r.TopCap, r.BotCap, tlCapR)
	if r.JogFrom >= 0 && !r.TopCap { // diagonal from yesterday's lane at the cell top
		from := laneRight(r.JogFrom) + right - laneRight(r.Lane)
		top = g.yt - timeline.JogHeight
		t.c.Polygon([]canvas.Point{{X: from - w, Y: g.yt}, {X: from, Y: g.yt}, {X: right, Y: top}, {X: right - w, Y: top}})
	}
	if r.Dashed {
		t.dashes(right-w, w, bot, top, tlDashOn, tlDashOff)
	} else {
		t.c.RoundedRect(right-w, bot, w, top-bot, rTop, rBot, true)
	}
}

// ticks marks group members starting or ending inside a running group rail
// whose lane's right edge is at right (REQ-GR-3).
func (t *tlDrawer) ticks(r timeline.Rail, railW float64, g tlCell, right float64) {
	tw := railW + tlTickExtra
	for _, tk := range r.Ticks {
		y := g.yb + timeline.CapInset
		if tk.AtTop {
			y = g.yt - timeline.CapInset - tlTickH
		}
		t.c.Rect(right+tlTickExtra/2-tw, y, tw, tlTickH)
	}
}

// milestoneMarks draws the ring and the glyphs; it returns the x right of
// the last glyph on a wide page (REQ-MS-1…3).
func (t *tlDrawer) milestoneMarks(day *timeline.Day, g tlCell) float64 {
	m := day.Milestones
	if m == nil {
		return g.letterEnd
	}
	c := t.c
	if m.Ring != "" {
		h, w := MainSize*tlRingH, g.numW+2*tlRingPad
		c.SetStrokeColor(hexColor(m.Ring))
		c.SetLineWidth(tlRingW)
		c.RoundedRect(g.numX-tlRingPad, g.mainY-tlRingBelow, w, h, min(w, h)/2, min(w, h)/2, false)
	}
	x, y, size := g.letterEnd+tlGlyphGap, g.mainY, tlGlyphWide
	if t.l.Narrow {
		x, size = g.guideX+tlGlyphInsetN, tlGlyphNarrow
	}
	for _, mk := range m.Marks {
		if mk.Mark == timeline.Ring {
			continue
		}
		c.SetFillColor(hexColor(mk.Color))
		x += drawGlyph(c, mk.Mark, x, y, size) + tlMarkGap
	}
	return x
}

// glyphWidth is the width of m's vector glyph at size; 0 for a ring.
func glyphWidth(m timeline.Mark, size float64) float64 {
	switch m {
	case timeline.Diamond:
		return 2 * diamondHalfW * size
	case timeline.Star:
		return 2 * starOuterR * size * math.Cos(math.Pi/10)
	case timeline.Dot:
		return 2 * dotR * size
	}
	return 0
}

// drawGlyph draws ◆ ★ ● as vector shapes (REQ-PG-5) with the left edge at x
// and the baseline at y; it returns the width.
func drawGlyph(c canvas.Canvas, m timeline.Mark, x, y, size float64) float64 {
	w := glyphWidth(m, size)
	cy := y + glyphMid*size
	switch m {
	case timeline.Diamond:
		hh := diamondHalfH * size
		c.Polygon([]canvas.Point{{X: x + w/2, Y: cy + hh}, {X: x + w, Y: cy}, {X: x + w/2, Y: cy - hh}, {X: x, Y: cy}})
	case timeline.Star:
		ro := starOuterR * size
		pts := make([]canvas.Point, 10)
		for i := range pts {
			r := ro
			if i%2 == 1 {
				r = ro * starInnerR
			}
			a := math.Pi/2 + float64(i)*math.Pi/5
			pts[i] = canvas.Point{X: x + w/2 + r*math.Cos(a), Y: cy + r*math.Sin(a)}
		}
		c.Polygon(pts)
	case timeline.Dot:
		c.Circle(x+w/2, y+dotMid*size, w/2)
	}
	return w
}

// topY is the baseline of the top row (02 §2).
func topY(g tlCell, size float64) float64 { return g.yt - tlTopFrom - tlCapHeight*size }

// countdown draws "T–N" top-left in the note area (REQ-MS-5); it returns
// the x where the top row may start.
func (t *tlDrawer) countdown(day *timeline.Day, g tlCell) float64 {
	x := g.guideX + tlTextGap
	cd := day.Countdown
	if cd == nil {
		return x
	}
	s := "T–" + strconv.Itoa(cd.N) // en dash: U+2212 is not in WinAnsi
	track := tlCountTrack * tlCountSize
	t.c.SetFillColor(hexColor(cd.Color))
	DrawSpaced(t.c, s, x, topY(g, tlLabelSize), t.bold, tlCountSize, track, AlignLeft)
	return x + SpacedWidth(t.c, s, t.bold, tlCountSize, track) + tlItemGap
}

// topRow draws labels, keys and the "+N" chip right-aligned before the rail
// zone (REQ-LB-2…6, REQ-PK-4); what does not fit is clipped.
func (t *tlDrawer) topRow(day *timeline.Day, g tlCell, left float64) {
	if len(day.Top) == 0 {
		return
	}
	right := g.noteRight(day)
	if t.l.Narrow {
		t.narrowKeys(day.Top, g, left, right)
		return
	}
	track := tlLabelTrack * tlLabelSize
	width := func(s string) float64 { return SpacedWidth(t.c, s, t.bold, tlLabelSize, track) }
	items := append([]timeline.TopItem(nil), day.Top...)
	total := func() float64 {
		w := tlItemGap * float64(len(items)-1)
		for _, it := range items {
			w += width(it.Text)
		}
		return w
	}
	shortened := false
	for len(items) > 0 && total() > right-left {
		if items[0].Kind == timeline.TopLabel && !shortened { // shorten the label once, then drop it
			shortened = true
			rest := total() - width(items[0].Text)
			if s := fitText(items[0].Text, right-left-rest, tlEllipsis, width); s != "" {
				items[0].Text = s
				continue
			}
		}
		items = items[1:]
	}
	x, y := right, topY(g, tlLabelSize)
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		t.c.SetFillColor(topColor(it))
		DrawSpaced(t.c, it.Text, x, y, t.bold, tlLabelSize, track, AlignRight)
		x -= width(it.Text) + tlItemGap
	}
}

// narrowKeys draws keys on up to two right-aligned lines (REQ-LB-5).
func (t *tlDrawer) narrowKeys(items []timeline.TopItem, g tlCell, left, right float64) {
	width := func(s string) float64 { return t.c.StringWidth(s, t.bold, tlKeySizeN) }
	var lines [][]timeline.TopItem
	var used float64
	for _, it := range items {
		w := width(it.Text)
		if len(lines) > 0 && used+tlItemGap+w <= right-left {
			lines[len(lines)-1] = append(lines[len(lines)-1], it)
			used += tlItemGap + w
			continue
		}
		if len(lines) == 2 || w > right-left {
			break
		}
		lines, used = append(lines, []timeline.TopItem{it}), w
	}
	y := topY(g, tlKeySizeN)
	t.c.SetFont(t.bold, tlKeySizeN)
	for _, line := range lines {
		x := right
		for i := len(line) - 1; i >= 0; i-- {
			w := width(line[i].Text)
			t.c.SetFillColor(topColor(line[i]))
			t.c.DrawString(x-w, y, line[i].Text)
			x -= w + tlItemGap
		}
		y -= tlKeyLineN
	}
}

func topColor(it timeline.TopItem) canvas.RGB {
	if it.Kind == timeline.TopMore {
		return tlGrey
	}
	return hexColor(it.Color)
}

// milestoneTitle draws the title on the note line, clipped before the rail
// zone (REQ-MS-1, -3, -4). It starts at guideStart, or after the glyphs
// when they reach past it: on a wide page when a wide day label pushes
// them, on a narrow page always. A title cut to fewer than tlTitleMinChars
// characters is left out; the mark and the legend still show the milestone.
func (t *tlDrawer) milestoneTitle(day *timeline.Day, g tlCell, marksEnd float64) {
	m := day.Milestones
	if m == nil || m.Title == "" {
		return
	}
	size := tlTitleSize
	x := max(g.guideX, marksEnd) // marksEnd already includes tlMarkGap
	if t.l.Narrow {
		size = tlTitleSizeN
		if marksEnd > g.guideX+tlGlyphInsetN { // a glyph was drawn
			x += tlTitleGapN - tlMarkGap
		}
	}
	right := g.noteRight(day)
	s := fitText(m.Title, right-x, tlEllipsis, func(s string) float64 { return t.c.StringWidth(s, t.bold, size) })
	if cut, ok := strings.CutSuffix(s, tlEllipsis); ok && s != m.Title && utf8.RuneCountInString(cut) < tlTitleMinChars {
		return
	}
	if s == "" {
		return
	}
	t.c.SetFillColor(hexColor(m.TitleColor))
	t.c.SetFont(t.bold, size)
	t.c.DrawString(x, g.mainY, s)
}
