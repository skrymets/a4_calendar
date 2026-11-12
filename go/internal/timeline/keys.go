// Package timeline turns resolved Date DSL declarations into layout
// decisions for the calendar page (docs/timeline/01-03): which interval
// takes which lane, which labels, keys and milestone marks appear on which
// day, and the legend. It is pure: no PDF drawing happens here.
package timeline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pinplan/internal/dsl"
)

// Mark is a milestone's glyph (REQ-KEY-3).
type Mark int

const (
	Diamond Mark = iota
	Star
	Dot
	Ring // circles the day number instead of a glyph
)

// Glyph is the mark's character in debug output and legend keys
// ("" for Ring; the legend shows ◯ for it).
func (m Mark) Glyph() string { return [...]string{"◆", "★", "●", ""}[m] }

// Style is how an interval is drawn (REQ-KEY-5).
type Style int

const (
	Solid Style = iota
	Dashed
	Hatch // hatches the cells; no rail, no lane
)

// Layer separates rails from margin lines (REQ-KEY-8).
type Layer int

const (
	Front Layer = iota
	Back
)

// Priority orders packing (REQ-KEY-9); lower values pack first.
type Priority int

const (
	High Priority = iota
	Normal
	Low
)

// LabelMode decides which occurrences get a text label (REQ-KEY-12).
type LabelMode int

const (
	LabelFirst LabelMode = iota
	LabelEach
	LabelNone
)

// LengthMode appends a length to the first label and the legend (REQ-KEY-7).
type LengthMode int

const (
	NoLength LengthMode = iota
	Days
	Workdays
)

// Palette is the named timeline colours (02 §4); grey and gray are equal.
var Palette = map[string]string{
	"ink": "#1F1F1F", "red": "#8E1B1B", "teal": "#2E6E62", "violet": "#5E4A8F",
	"orange": "#A04A14", "steel": "#3D6A99", "ochre": "#8A6D1F", "magenta": "#9C3D6B",
	"grey": "#7A7A7A", "gray": "#7A7A7A",
}

const defaultColor = "#1F1F1F"

// Milestone is a milestone declaration with its rendering keys read.
type Milestone struct {
	*dsl.Milestone
	Title     string // title, else the identifier
	Color     string // "#RRGGBB"
	Mark      Mark
	Tentative bool
	Countdown int // 0: none; ignored on a recurring milestone
	Hidden    bool
	Label     LabelMode
}

// Interval is an interval declaration with its rendering keys read.
type Interval struct {
	*dsl.Interval
	Key       string // 1-based position among all intervals (REQ-LB-4)
	Title     string // title, else the identifier, else ""
	Color     string
	Style     Style // a tentative solid interval is Dashed (REQ-KEY-6)
	Tentative bool
	Length    LengthMode
	Layer     Layer
	Priority  Priority
	Group     string // lower-case; "" for none
	Hidden    bool
	Label     LabelMode
}

// Decls is every declaration of a Model with its rendering keys.
type Decls struct {
	Milestones []*Milestone
	Intervals  []*Interval
}

// Warning is a problem that does not stop the run.
type Warning struct {
	Source string
	Line   int
	Msg    string
}

func (w Warning) String() string {
	return (&dsl.Error{Source: w.Source, Line: w.Line, Msg: "warning: " + w.Msg}).Error()
}

var groupRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// intervalOnlyKeys and milestoneOnlyKeys are rendering keys of one kind;
// on the other kind they are reported as having no effect (any other
// unknown key is plain data, REQ-KEY-13).
var (
	intervalOnlyKeys  = []string{"style", "length", "layer", "priority", "group"}
	milestoneOnlyKeys = []string{"mark", "countdown"}
)

// ReadKeys validates the rendering keys of every declaration (REQ-KEY-*).
// All value errors are returned together as dsl.Errors, each naming the
// source, line, key and allowed values. Unknown keys are kept as data.
func ReadKeys(m *dsl.Model) (*Decls, []Warning, error) {
	var (
		d    = &Decls{}
		errs dsl.Errors
		warn []Warning
	)
	for _, ms := range m.Milestones {
		r := reader{props: ms.Props}
		x := &Milestone{Milestone: ms, Title: ms.ID}
		if t, ok := ms.Props.Get("title"); ok && t != "" {
			x.Title = t
		}
		x.Color = r.color()
		x.Mark = Mark(r.choice("mark", int(Diamond), "diamond", "star", "dot", "ring"))
		x.Tentative = r.choice("status", 0, "confirmed", "tentative") == 1
		x.Hidden = r.choice("hide", 1, "yes", "no") == 0
		x.Label = LabelMode(r.choice("label", int(LabelFirst), "first", "each", "none"))
		x.Countdown = r.countdown()
		if x.Countdown > 0 && ms.Rec != nil {
			warn = append(warn, Warning{ms.Source, ms.Line, "countdown is ignored on a recurring milestone"})
			x.Countdown = 0
		}
		warn = append(warn, r.misplaced(ms.Source, ms.Line, intervalOnlyKeys, "a milestone")...)
		errs = append(errs, r.errors(ms.Source, ms.Line)...)
		d.Milestones = append(d.Milestones, x)
	}
	for i, iv := range m.Intervals {
		r := reader{props: iv.Props}
		x := &Interval{Interval: iv, Key: strconv.Itoa(i + 1), Title: iv.ID}
		if t, ok := iv.Props.Get("title"); ok && t != "" {
			x.Title = t
		}
		x.Color = r.color()
		x.Style = Style(r.choice("style", int(Solid), "solid", "dashed", "hatch"))
		x.Tentative = r.choice("status", 0, "confirmed", "tentative") == 1
		if x.Tentative && x.Style == Solid {
			x.Style = Dashed
		}
		x.Length = LengthMode(r.choice("length", int(NoLength), "", "days", "workdays"))
		x.Layer = Layer(r.choice("layer", int(Front), "front", "back"))
		x.Priority = Priority(r.choice("priority", int(Normal), "high", "normal", "low"))
		x.Hidden = r.choice("hide", 1, "yes", "no") == 0
		x.Label = LabelMode(r.choice("label", int(LabelFirst), "first", "each", "none"))
		x.Group = r.group()
		warn = append(warn, r.misplaced(iv.Source, iv.Line, milestoneOnlyKeys, "an interval")...)
		errs = append(errs, r.errors(iv.Source, iv.Line)...)
		d.Intervals = append(d.Intervals, x)
	}
	if len(errs) > 0 {
		return d, warn, errs
	}
	return d, warn, nil
}

// reader reads one declaration's keys and collects its value errors.
type reader struct {
	props dsl.Props
	msgs  []string
}

func (r *reader) fail(format string, a ...any) { r.msgs = append(r.msgs, fmt.Sprintf(format, a...)) }

// choice returns the index of the key's value among allowed (case-
// insensitive), or def when the key is absent. An empty allowed entry is a
// placeholder that no value matches (it keeps index 0 for "none").
func (r *reader) choice(key string, def int, allowed ...string) int {
	v, ok := r.props.Get(key)
	if !ok {
		return def
	}
	var listed []string
	for i, a := range allowed {
		if a == "" {
			continue
		}
		if strings.EqualFold(v, a) {
			return i
		}
		listed = append(listed, a)
	}
	r.fail("%s=%s: expected one of %s", key, v, strings.Join(listed, ", "))
	return def
}

func (r *reader) color() string {
	v, ok := r.props.Get("color")
	if !ok {
		return defaultColor
	}
	if hex, named := Palette[strings.ToLower(v)]; named {
		return hex
	}
	if len(v) == 7 && v[0] == '#' {
		if _, err := strconv.ParseUint(v[1:], 16, 32); err == nil {
			return strings.ToUpper(v)
		}
	}
	r.fail("color=%s: expected one of ink, red, teal, violet, orange, steel, ochre, magenta, grey, gray or #RRGGBB", v)
	return defaultColor
}

func (r *reader) countdown() int {
	v, ok := r.props.Get("countdown")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 99 || strings.HasPrefix(v, "+") {
		r.fail("countdown=%s: expected an integer from 1 to 99", v)
		return 0
	}
	return n
}

func (r *reader) group() string {
	v, ok := r.props.Get("group")
	if !ok {
		return ""
	}
	if !groupRE.MatchString(v) {
		r.fail("group=%s: expected an identifier (letters, digits and _, not starting with a digit)", v)
		return ""
	}
	return strings.ToLower(v)
}

// misplaced warns about rendering keys that only apply to the other kind.
func (r *reader) misplaced(source string, line int, keys []string, kind string) []Warning {
	var w []Warning
	for _, k := range keys {
		if _, ok := r.props.Get(k); ok {
			w = append(w, Warning{source, line, fmt.Sprintf("%s has no effect on %s", k, kind)})
		}
	}
	return w
}

func (r *reader) errors(source string, line int) dsl.Errors {
	var es dsl.Errors
	for _, m := range r.msgs {
		es = append(es, &dsl.Error{Source: source, Line: line, Msg: m})
	}
	return es
}
