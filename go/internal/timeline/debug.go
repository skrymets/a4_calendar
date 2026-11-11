package timeline

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"a4calendar/internal/dsl"
)

// DebugInfo names the inputs in the export; Options is free-form.
type DebugInfo struct {
	Fixture string            `json:"fixture"`
	Options map[string]string `json:"options"`
}

type debugDoc struct {
	DebugInfo
	Resolved debugResolved `json:"resolved"`
	Layout   debugLayout   `json:"layout"`
}

type debugResolved struct {
	Milestones []debugMilestone `json:"milestones"`
	Intervals  []debugInterval  `json:"intervals"`
}

type debugMilestone struct {
	ID         string `json:"id"`
	Date       string `json:"date"`
	Occurrence string `json:"occurrence,omitempty"`
	Hidden     bool   `json:"hidden,omitempty"`
}

type debugInterval struct {
	ID           *string `json:"id"`
	Key          string  `json:"key"`
	Start        string  `json:"start"`
	EndExclusive string  `json:"endExclusive"`
	LastDrawnDay string  `json:"lastDrawnDay"`
	Occurrence   string  `json:"occurrence,omitempty"`
}

type debugLayout struct {
	Geometry     string        `json:"geometry"`
	LaneCap      int           `json:"laneCap"`
	GroupsMerged bool          `json:"groupsMerged"`
	Layout       string        `json:"layout"`
	Columns      []debugColumn `json:"columns"`
}

type debugColumn struct {
	Month     string       `json:"month"`
	RailLanes int          `json:"railLanes"`
	LabelMode string       `json:"labelMode"`
	Labels    []debugLabel `json:"labels"`
}

type debugLabel struct {
	Day       int      `json:"day"`
	Items     []string `json:"items,omitempty"`
	Milestone *string  `json:"milestone,omitempty"`
}

// DebugJSON exports the resolved dates and the layout decisions in the
// shape of docs/timeline/examples/*.expected.json (05 §1).
func DebugJSON(info DebugInfo, d *Decls, r *dsl.Resolved, l *Layout) ([]byte, error) {
	doc := debugDoc{DebugInfo: info}
	doc.Resolved.Milestones = []debugMilestone{}
	doc.Resolved.Intervals = []debugInterval{}
	for _, o := range r.Milestones {
		m := d.Milestones[o.Index]
		doc.Resolved.Milestones = append(doc.Resolved.Milestones, debugMilestone{
			ID: m.ID, Date: o.Date.Format(time.DateOnly), Occurrence: occurrence(m.Rec != nil, o.Idx, o.N), Hidden: m.Hidden,
		})
	}
	for _, o := range r.Intervals {
		x := d.Intervals[o.Index]
		var id *string
		if x.ID != "" {
			id = &x.ID
		}
		doc.Resolved.Intervals = append(doc.Resolved.Intervals, debugInterval{
			ID: id, Key: x.Key, Start: o.Start.Format(time.DateOnly), EndExclusive: o.End.Format(time.DateOnly),
			LastDrawnDay: o.End.AddDate(0, 0, -1).Format(time.DateOnly), Occurrence: occurrence(x.Rec != nil, o.Idx, o.N),
		})
	}

	dl := &doc.Layout
	dl.Geometry, dl.Layout = "wide", "packed"
	if l.Narrow {
		dl.Geometry = "narrow"
	}
	if l.Flow {
		dl.Layout = "flow"
	}
	dl.LaneCap, dl.GroupsMerged = l.LaneCap, l.GroupsMerged
	dl.Columns = []debugColumn{}
	for _, c := range l.Columns {
		dc := debugColumn{Month: c.Name, RailLanes: c.K, LabelMode: "text", Labels: []debugLabel{}}
		if c.Labels == KeyLabels {
			dc.LabelMode = "keys"
		}
		for i, day := range c.Days {
			if day == nil {
				continue
			}
			if len(day.Top) > 0 {
				items := make([]string, len(day.Top))
				for k, it := range day.Top {
					items[k] = it.Text
				}
				dc.Labels = append(dc.Labels, debugLabel{Day: i + 1, Items: items})
			}
			if m := day.Milestones; m != nil {
				var glyphs strings.Builder
				for _, mk := range m.Marks {
					glyphs.WriteString(mk.Mark.Glyph())
				}
				s := glyphs.String()
				if m.Title != "" {
					s += " " + m.Title
				}
				dc.Labels = append(dc.Labels, debugLabel{Day: i + 1, Milestone: &s})
			}
		}
		dl.Columns = append(dl.Columns, dc)
	}
	return json.MarshalIndent(doc, "", " ")
}

func occurrence(series bool, idx, n int) string {
	if !series {
		return ""
	}
	return strconv.Itoa(idx) + "/" + strconv.Itoa(n)
}
