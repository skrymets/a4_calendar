package render

import (
	"fmt"
	"strconv"
	"time"

	"pinplan/internal/canvas"
	"pinplan/internal/period"
	"pinplan/internal/timeline"
)

// Config is the validated input of Build (one field per CLI option).
type Config struct {
	Start        time.Time
	Period       period.Period
	WeekMode     string // relative | iso | rolling
	WeekStart    int    // 0=Mon..6=Sun
	WeekLabels   string // all | first
	Weekend      map[int]bool
	Title        string // "" draws no title
	DayLetters   []rune // 7, Monday first
	MonthNames   []string
	Font         canvas.Font
	PaperW       float64
	PaperH       float64
	Footer       bool
	NoteGuide    bool
	NoteLines    int
	WorkdayColor canvas.RGB
	WeekendColor canvas.RGB
	MarginMM     float64
	Header       string // compact | full
	ShowOutside  bool
	Timeline     *Timeline // nil without --file/--milestone/--interval: page unchanged (REQ-CLI-8)
}

// Timeline is the timeline drawn on top of the calendar.
type Timeline struct {
	Layout     *timeline.Layout
	Legend     bool     // --timeline-legend full
	Files      []string // --file paths in load order
	Inline     bool     // --milestone or --interval given
	Milestones int      // declarations, hidden ones included (REQ-PG-3)
	Intervals  int
}

// GridWidth is the width of the day grid in pt: the page minus its margins.
func (cfg Config) GridWidth() float64 { return cfg.PaperW - 2*(cfg.MarginMM*MM) }

// Result describes the rendered period.
type Result struct {
	Start, End time.Time
	TotalDays  int
	// LegendOverflow is the full legend, one entry per line, when it did
	// not fit the page and ends in "+N more" (REQ-LG-5); nil otherwise.
	LegendOverflow []string
}

func pick(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

// Build draws the calendar page onto c.
func Build(cfg Config, c canvas.Canvas) (Result, error) {
	start := cfg.Start
	end, err := period.PeriodEnd(start, cfg.Period)
	if err != nil {
		return Result{}, err
	}
	totalDays := period.DaysBetween(start, end) + 1
	font := cfg.Font
	marginX := cfg.MarginMM * MM
	marginBottom, marginTop := marginX, marginX
	compact := cfg.Header == "compact"
	titleSize := pick(compact, 10, TitleSize)
	pageW, pageH := cfg.PaperW, cfg.PaperH
	c.SetTitle(fmt.Sprintf("%s %s – %s", cfg.Title, start.Format("2006-01-02"), end.Format("2006-01-02")))

	months := period.MonthsBetween(start, end)
	multiYear := months[0].Year != months[len(months)-1].Year
	nCols := len(months)
	gridW := cfg.GridWidth()
	colW := gridW / float64(nCols)

	// --- title + period label
	titleY := pageH - marginTop - titleSize*0.75
	c.SetFillColor(Ink)
	label := PeriodLabel(start, end, cfg.MonthNames)
	labelSize := pick(compact, 8, 9)
	DrawSpaced(c, label, pageW-marginX, titleY, font, labelSize, 0.8, AlignRight)
	if cfg.Title != "" {
		titleMaxW := gridW - SpacedWidth(c, label, font, labelSize, 0.8) - 12
		title := FitText(cfg.Title, titleMaxW, func(s string) float64 {
			return SpacedWidth(c, s, font, titleSize, 1.2)
		})
		DrawSpaced(c, title, marginX, titleY, font, titleSize, 1.2, AlignLeft)
	}

	// --- header
	headerTop := titleY - pick(compact, 6, 14)
	hdrH := pick(compact, 16, HeaderH)
	headerBot := headerTop - hdrH
	c.SetStrokeColor(Rule)
	c.SetLineWidth(0.8)
	c.Line(marginX, headerTop, pageW-marginX, headerTop)
	c.SetFillColor(HeaderBG)
	c.Rect(marginX, headerBot, gridW, hdrH)
	c.SetLineWidth(0.4)
	c.Line(marginX, headerBot, pageW-marginX, headerBot)

	hdrSize := pick(compact, 7, 8)
	if nCols <= 12 {
		hdrSize = pick(compact, 7.5, 9)
	}
	for i, ym := range months {
		cx := marginX + colW*(float64(i)+0.5)
		name := cfg.MonthNames[ym.Month-1]
		c.SetFillColor(Ink)
		if multiYear && (i == 0 || ym.Month == 1 || i == nCols-1) {
			if compact { // "NOV '26" on one line
				DrawSpaced(c, fmt.Sprintf("%s '%02d", name, ym.Year%100), cx,
					headerBot+hdrH/2-hdrSize*0.35, font, hdrSize, 0.6, AlignCenter)
				continue
			}
			DrawSpaced(c, name, cx, headerBot+hdrH*0.52, font, hdrSize, 0.8, AlignCenter)
			c.SetFillColor(InkSoft)
			DrawSpaced(c, strconv.Itoa(ym.Year), cx, headerBot+3.5, font, 5, 0.4, AlignCenter)
		} else {
			DrawSpaced(c, name, cx, headerBot+hdrH/2-hdrSize*0.35, font, hdrSize, 0.8, AlignCenter)
		}
	}

	// --- day grid
	gridTop := headerBot - pick(compact, 1.5, 3)
	gridBot := marginBottom + pick(cfg.Footer, 8, 2)
	footerY := gridBot - 8
	tl := newTLDrawer(c, cfg.Timeline, font, pageH) // nil without a timeline
	var legend legendPlan
	if tl != nil && cfg.Timeline.Legend { // the grid rows shrink to make room (REQ-LG-1)
		legend = planLegend(tl.l.Legend, gridTop-gridBot)
		gridBot += legend.height
	}
	rowH := (gridTop - gridBot) / 31
	pad := CellPad
	// note guides start right of the widest possible label ("28W"), same x in every column
	labelW := 0.0
	for _, ch := range cfg.DayLetters {
		labelW = max(labelW, c.StringWidth("28"+string(ch), font, MainSize))
	}
	labelW += 0.4
	smallW := c.StringWidth("000/00", font, SmallSize)
	guideX0Off := pad + max(labelW, smallW) + 1.5

	for i, ym := range months {
		x0 := marginX + colW*float64(i)
		for d := 1; d <= period.DaysIn(ym.Year, ym.Month); d++ {
			day := period.Date(ym.Year, ym.Month, d)
			yt := gridTop - rowH*float64(d-1) // cell top
			yb := yt - rowH
			inRange := !day.Before(start) && !day.After(end)
			wd := period.Weekday(day)
			isWeekend := cfg.Weekend[wd]

			if isWeekend && inRange {
				c.SetFillColor(WeekendBG)
				c.Rect(x0, yb, colW, rowH)
			}
			tl.hatch(i, d, x0, yb, colW, rowH, isWeekend && inRange)
			if !inRange && !cfg.ShowOutside {
				continue
			}

			// main "7W": number in ink, weekday letter in work/weekend colour
			mainY := yb + rowH*0.22
			num := strconv.Itoa(d)
			c.SetFont(font, MainSize)
			c.SetFillColor(pickColor(inRange, Ink, InkFaint))
			c.DrawString(x0+pad, mainY, num)
			letterColor := InkFaint
			if inRange {
				letterColor = pickColor(isWeekend, cfg.WeekendColor, cfg.WorkdayColor)
			}
			c.SetFillColor(letterColor)
			numW := c.StringWidth(num, font, MainSize)
			letter, letterX := string(cfg.DayLetters[wd]), x0+pad+numW+0.4
			c.DrawString(letterX, mainY, letter)

			if !inRange {
				continue
			}

			// hairline dotted note guide(s) in the free right part of the cell
			gx0, gx1 := x0+guideX0Off, x0+colW-pad
			if day := tl.day(i, d); day != nil {
				gx1 = x0 + colW - day.Zone // the note line ends at the rail zone
			}
			if cfg.NoteGuide && cfg.NoteLines > 0 && (tl == nil || gx1 > gx0) { // a full rail zone leaves no note line
				c.SetStrokeColor(Guide)
				c.SetLineWidth(0.25)
				c.SetDash(0.4, 1.2)
				c.SetRoundCap(true)
				step := (yt - mainY) / float64(cfg.NoteLines) // evenly upward from the label baseline
				for k := 0; k < cfg.NoteLines; k++ {
					gy := mainY + float64(k)*step
					c.Line(gx0, gy, gx1, gy)
				}
				c.ClearDash()
				c.SetRoundCap(false)
			}

			// tiny "ddd/ww" above it
			idx := period.DaysBetween(start, day) + 1
			wk := period.WeekNumber(day, start, cfg.WeekMode, cfg.WeekStart)
			showWeek := cfg.WeekLabels == "all" || day.Equal(start) ||
				(cfg.WeekMode == "rolling" && (idx-1)%7 == 0) ||
				(cfg.WeekMode != "rolling" && wd == cfg.WeekStart) || d == 1
			c.SetFillColor(InkSoft)
			c.SetFont(font, SmallSize)
			small := fmt.Sprintf("%03d", idx)
			if showWeek {
				small = fmt.Sprintf("%03d/%02d", idx, wk)
			}
			c.DrawString(x0+pad, mainY+MainSize*0.85+1.2, small)

			if tl != nil {
				tl.cell(i, d, tlCell{
					x0: x0, w: colW, yt: yt, yb: yb, mainY: mainY, guideX: x0 + guideX0Off,
					numX: x0 + pad, numW: numW,
					letterEnd: letterX + c.StringWidth(letter, font, MainSize),
				})
			}
		}
	}

	// column dividers + bottom rule
	c.SetStrokeColor(Divider)
	c.SetLineWidth(0.35)
	for i := 1; i < nCols; i++ {
		x := marginX + colW*float64(i)
		c.Line(x, headerBot+4, x, headerTop-4)
		c.Line(x, gridBot, x, gridTop)
	}
	c.SetStrokeColor(Rule)
	c.SetLineWidth(0.8)
	c.Line(marginX, gridBot-2, pageW-marginX, gridBot-2)
	res := Result{Start: start, End: end, TotalDays: totalDays}
	if tl != nil {
		tl.legend(legend, marginX, gridBot-2, gridW)
		res.LegendOverflow = legend.overflowLines(tl.l.Legend)
		c.SetSubject("timeline: " + cfg.Timeline.sourceNames()) // REQ-PG-4
	}

	if cfg.Footer {
		c.SetFillColor(InkSoft)
		c.SetFont(font, 5.5)
		weeksTotal := period.WeekNumber(end, start, cfg.WeekMode, cfg.WeekStart)
		footer := fmt.Sprintf("%d days · week numbering: %s", totalDays, cfg.WeekMode)
		if cfg.WeekMode != "iso" {
			footer += fmt.Sprintf(" (%d weeks)", weeksTotal)
		}
		if tl != nil { // REQ-PG-3
			footer = fitText(footer+cfg.Timeline.footerSuffix(), gridW, tlEllipsis,
				func(s string) float64 { return c.StringWidth(s, font, 5.5) })
		}
		c.DrawString(marginX, footerY, footer)
	}
	return res, nil
}

func pickColor(cond bool, a, b canvas.RGB) canvas.RGB {
	if cond {
		return a
	}
	return b
}
