package timeline

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"a4calendar/internal/dsl"
)

// Swatch is the small sample drawn before a legend entry.
type Swatch int

const (
	SwatchNone   Swatch = iota // milestone: the key is its glyph
	SwatchRail                 // solid rail
	SwatchDashed               // dashed rail
	SwatchHatch                // hatched box
	SwatchBack                 // thin margin line
	SwatchGroup                // merged group rail
)

// LegendEntry is one line of the legend (REQ-LG-2…4).
type LegendEntry struct {
	Key       string // interval or group key; "" for a milestone
	Milestone bool
	Mark      Mark // milestone glyph (Ring is shown as a circle)
	Title     string
	When      string // grey date text
	Color     string
	Swatch    Swatch
	sortDay   int
}

// legend builds the entries (03 §7): milestones then intervals in
// declaration order, stably sorted by first in-period date; merged groups
// with an in-period member follow.
func (e *engine) legend() []LegendEntry {
	months := e.o.MonthNames
	if len(months) != 12 {
		months = defaultMonths
	}
	when := func(t int) string {
		d := dayDate(t)
		return strconv.Itoa(d.Day()) + " " + capitalize(months[d.Month()-1])
	}
	var out []LegendEntry
	for _, m := range e.allMs {
		if m.Hidden {
			continue
		}
		var occ []*msOcc
		for _, o := range e.ms {
			if o.Milestone == m && o.t >= e.start && o.t <= e.end {
				occ = append(occ, o)
			}
		}
		if len(occ) == 0 {
			continue
		}
		w := when(occ[0].t)
		if m.Rec != nil {
			w += "–" + when(occ[len(occ)-1].t) + " · every " + m.Rec.Period.String() + " · ×" + strconv.Itoa(len(occ))
		}
		title := m.Title
		if m.Tentative {
			title += " ?"
		}
		out = append(out, LegendEntry{Milestone: true, Mark: m.Mark, Title: title, When: w, Color: m.Color, sortDay: occ[0].t})
	}
	for _, x := range e.allIv {
		if x.Hidden {
			continue
		}
		var occ []*ivOcc
		for _, o := range e.iv {
			if o.Interval == x && o.e >= e.start && o.s <= e.end {
				occ = append(occ, o)
			}
		}
		if len(occ) == 0 {
			continue
		}
		first, last := occ[0], occ[len(occ)-1]
		w := when(first.s) + "–"
		switch {
		case x.Rec != nil:
			mode := "every "
			if x.Rec.Mode == dsl.Gap {
				mode = "gap "
			}
			w += when(last.e) + " · " + mode + x.Rec.Period.String() + " · ×" + strconv.Itoa(len(occ))
		case x.Length == Workdays:
			w += when(first.e) + " · " + strconv.Itoa(e.workdays(first.s, first.e)) + " wd"
		default:
			w += when(first.e) + " · " + strconv.Itoa(first.e-first.s+1) + " d"
		}
		title := x.Title
		if title == "" {
			title = "(untitled)"
		}
		if x.Tentative {
			title += " ?"
		}
		if x.Layer == Back && x.Style != Hatch {
			title += " · back"
		}
		if g := e.group(x); g != nil {
			title += " · " + g.Key
		}
		sw := SwatchRail
		switch {
		case x.Style == Hatch:
			sw = SwatchHatch
		case x.Layer == Back:
			sw = SwatchBack
		case x.Style == Dashed:
			sw = SwatchDashed
		}
		out = append(out, LegendEntry{Key: x.Key, Title: title, When: w, Color: x.Color, Swatch: sw, sortDay: first.s})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].sortDay < out[j].sortDay })
	for _, g := range e.groups {
		inPeriod := false
		for _, m := range g.members {
			inPeriod = inPeriod || (m.e >= e.start && m.s <= e.end)
		}
		if inPeriod {
			out = append(out, LegendEntry{Key: g.Key, Title: g.Name, When: "= " + strings.Join(g.MemberKeys(), " + "),
				Color: g.Color, Swatch: SwatchGroup})
		}
	}
	return out
}

// group is the merged group that interval x belongs to, if any.
func (e *engine) group(x *Interval) *Group {
	for _, g := range e.groups {
		for _, m := range g.members {
			if m.Interval == x {
				return g
			}
		}
	}
	return nil
}

// capitalize turns "JAN" or "січ" into "Jan" / "Січ" for legend dates.
func capitalize(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + strings.ToLower(s[n:])
}
