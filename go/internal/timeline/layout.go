package timeline

import (
	"time"

	"pinplan/internal/dsl"
	"pinplan/internal/period"
)

// GroupMode controls group merging (REQ-CLI-6).
type GroupMode int

const (
	GroupsAuto GroupMode = iota // merge on narrow pages only
	GroupsMerge
	GroupsSeparate
)

// Options are the page facts and CLI settings the layout depends on. The
// layout never depends on the row height, only on the column width, so it
// can be computed before the page budget (legend height) is known.
type Options struct {
	Start, End    time.Time // the printed period, inclusive
	GridWidth     float64   // pt; divided equally among the month columns
	LaneCap       int       // --timeline-lanes; 0 = auto (02 §3)
	Groups        GroupMode
	Flow          bool // --timeline-layout flow
	WeeklyCompact bool // --timeline-compact weekly (flow only)
	WeekStart     int  // 0 = Monday; weekly compaction day
	Weekend       map[int]bool
	MonthNames    []string // 12 names; nil = JAN..DEC
}

// Page classes and geometry (02 §3). Sizes in pt.
const (
	wideMinMM     = 26.5  // narrower columns make a narrow page
	sixLanesMinMM = 31.75 // wide columns from here on get lane cap 6
	mm            = 72 / 25.4

	JogHeight  = 4.5  // flow: diagonal at the cell top
	CapInset   = 3.0  // rail end inset from the cell edge
	ThickExtra = 0.75 // merged group with >= 2 members active
	emptyZone  = 0.75 // rail zone of a column without rails, besides pad
)

// ColumnLabels is how a column labels intervals (REQ-LB-1).
type ColumnLabels int

const (
	TextLabels ColumnLabels = iota
	KeyLabels
)

// Layout is every decision needed to draw the timeline.
type Layout struct {
	Narrow       bool
	LaneCap      int
	GroupsMerged bool
	Flow         bool
	Weekly       bool
	Pad          float64 // rail zone edge and inner gap
	Columns      []*Column
	Groups       []*Group // merged groups, in order of first member
	Legend       []LegendEntry
}

// Column is one month.
type Column struct {
	Year   int
	Month  time.Month
	Name   string // month name, + " 'yy" on year-marker columns
	K      int    // rail lanes incl. the overflow rail (02 §3 "k")
	RailW  float64
	Step   float64
	Labels ColumnLabels
	Days   [31]*Day // Days[d-1]; nil past the month end or outside the period
}

// Day is the timeline content of one in-period cell.
type Day struct {
	Date         time.Time
	Zone         float64 // rail zone width from the column's right edge
	Hatch        bool
	Back         []BackLine
	Rails        []Rail
	OverflowLane int // -1: no overflow rail
	Top          []TopItem
	Milestones   *DayMilestones
	Countdown    *Countdown
}

// Rail is one interval rail segment in one cell.
type Rail struct {
	Lane    int
	Color   string
	Dashed  bool
	Thick   bool // merged group with >= 2 members active: RailW + ThickExtra
	TopCap  bool // starts CapInset below the cell top, rounded
	BotCap  bool
	JogFrom int // flow: lane of yesterday when it moved; -1 for none
	Ticks   []Tick
}

// Tick marks a group member starting or ending inside a running group rail.
type Tick struct{ AtTop bool }

// BackLine is a layer=back margin line (REQ-IV-10); Slot 0..2.
type BackLine struct {
	Slot   int
	Color  string
	TopCap bool
	BotCap bool
}

// Dotted reports whether the margin line is drawn dotted (2nd and 3rd).
func (b BackLine) Dotted() bool { return b.Slot > 0 }

// TopKind tells text labels from keys and the overflow chip.
type TopKind int

const (
	TopLabel TopKind = iota
	TopKey
	TopMore // "+N"
)

// TopItem is one text in a cell's top row, in drawing order.
type TopItem struct {
	Text  string
	Color string
	Kind  TopKind
}

// DayMilestones is everything drawn for the milestones of one day.
type DayMilestones struct {
	Marks      []MarkItem // all of the day's milestones, declaration order
	Title      string     // "" when none is due a title
	TitleColor string
	Ring       string // colour of the first ring milestone; "" for none
}

// MarkItem is one milestone glyph.
type MarkItem struct {
	Mark  Mark
	Color string
}

// Countdown is "T–N" on a day before a milestone (REQ-MS-5).
type Countdown struct {
	N     int
	Color string
}

// Group is a merged group of intervals (REQ-GR-*).
type Group struct {
	Name    string // lower-case
	Key     string // REQ-GR-4
	Color   string // first member's
	members []*ivOcc
}

// MemberKeys lists the members' interval keys, unique, in declaration order.
func (g *Group) MemberKeys() []string {
	var keys []string
	seen := map[string]bool{}
	for _, m := range g.members {
		if !seen[m.Key] {
			seen[m.Key] = true
			keys = append(keys, m.Key)
		}
	}
	return keys
}

// railSize is the rail width and lane step for k lanes (02 §3).
func railSize(narrow bool, k int) (w, step float64) {
	switch {
	case !narrow && k <= 4:
		return 2.25, 3.75
	case !narrow && k <= 7:
		return 1.5, 2.6
	case !narrow:
		return 1.1, 2.25
	case k <= 3:
		return 1.5, 2.6
	case k <= 5:
		return 1.1, 1.9
	default:
		return 0.75, 1.5
	}
}

// zoneWidth is the rail zone for n lanes in use (REQ-PK-5, -6).
func zoneWidth(pad, step float64, n int) float64 {
	if n == 0 {
		return pad + emptyZone
	}
	return pad + float64(n)*step + pad
}

var defaultMonths = []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}

// Build computes the layout of decls' occurrences r on the period.
func Build(decls *Decls, r *dsl.Resolved, o Options) *Layout {
	months := period.MonthsBetween(o.Start, o.End)
	colWmm := o.GridWidth / float64(len(months)) / mm
	l := &Layout{Narrow: colWmm < wideMinMM, Flow: o.Flow, Weekly: o.Flow && o.WeeklyCompact}
	switch {
	case o.LaneCap > 0:
		l.LaneCap = o.LaneCap
	case l.Narrow:
		l.LaneCap = 3
	case colWmm >= sixLanesMinMM:
		l.LaneCap = 6
	default:
		l.LaneCap = 4
	}
	l.Pad = 2.25
	if l.Narrow {
		l.Pad = 1.5
	}
	l.GroupsMerged = o.Groups == GroupsMerge || (o.Groups == GroupsAuto && l.Narrow)

	e := newEngine(decls, r, o, l)
	if o.Flow {
		e.flow()
	}
	names := o.MonthNames
	if len(names) != 12 {
		names = defaultMonths
	}
	multiYear := months[0].Year != months[len(months)-1].Year
	for ci, ym := range months {
		col := &Column{Year: ym.Year, Month: ym.Month, Name: names[ym.Month-1]}
		if multiYear && (ci == 0 || ci == len(months)-1 || ym.Month == time.January) {
			col.Name += " '" + twoDigits(ym.Year%100)
		}
		e.column(col)
		l.Columns = append(l.Columns, col)
	}
	l.Groups = e.groups
	l.Legend = e.legend()
	return l
}

func twoDigits(n int) string { return string([]byte{byte('0' + n/10), byte('0' + n%10)}) }
