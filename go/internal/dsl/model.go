// Package dsl parses and evaluates the Date DSL (docs/timeline/
// date-dsl-specification.md, Phase 1 with recurrence): named milestones,
// intervals, relative dates, durations and recurrences, given inline or in
// .dates files.
//
// Parse turns sources into a Model of declarations with their first dates
// resolved; Expand turns the Model into dated occurrences up to a window
// end. Dates are civil dates: time.Time at midnight UTC, as in package
// period. Nothing here knows about rendering; rendering keys such as color
// or mark are carried as plain properties.
package dsl

import (
	"time"

	"a4calendar/internal/period"
)

// Kind restricts what a source may declare.
type Kind int

const (
	KindAny       Kind = iota // a .dates file: milestones, then intervals
	KindMilestone             // one --milestone argument
	KindInterval              // one --interval argument
)

// Source is one input: a file (KindAny, many lines) or a single inline
// declaration (KindMilestone/KindInterval). Name is used in error messages
// ("roadmap.dates", "--milestone[2]").
type Source struct {
	Name   string
	Text   string
	Expect Kind
}

func (s Source) inline() bool { return s.Expect != KindAny }

// Prop is one key=value property. Key is lower-cased; Value keeps its case.
type Prop struct {
	Key, Value string
}

// Props keeps a declaration's properties in their written order.
type Props []Prop

// Get returns the value of key (lower-case) and whether it is present.
func (p Props) Get(key string) (string, bool) {
	for _, kv := range p {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return "", false
}

// RecMode is the recurrence frequency kind.
type RecMode int

const (
	Every RecMode = iota + 1 // start-to-start
	Gap                      // end-to-start (intervals only)
)

// Recurrence is the every/gap + count/until configuration.
type Recurrence struct {
	Mode     RecMode
	Period   period.Period
	Count    int // 0: no count limit
	Until    time.Time
	HasUntil bool
}

// DurKind records how an interval was written, which decides how later
// occurrences of a recurring interval get their length (spec §6.7).
type DurKind int

const (
	StartEnd      DurKind = iota + 1 // START/END: fixed number of days
	StartDuration                    // START/DURATION: the duration from each start
	DurationEnd                      // DURATION/END: the duration from each later start
)

// Milestone is a declared milestone with its first (or only) date.
type Milestone struct {
	ID     string // as written; case-insensitive for references
	Date   time.Time
	Rec    *Recurrence
	Props  Props
	Source string
	Line   int // 0 for inline declarations
	Expr   string
}

// Interval is a declared interval with its first occurrence [Start, End).
type Interval struct {
	ID       string // "" for an anonymous interval
	Start    time.Time
	End      time.Time // exclusive
	DurKind  DurKind
	Duration period.Period // StartDuration, DurationEnd
	Days     int           // StartEnd: End - Start in days
	Rec      *Recurrence
	Props    Props
	Source   string
	Line     int
	Expr     string
}

// Model is every declaration in loading order.
type Model struct {
	Milestones []*Milestone
	Intervals  []*Interval
}

// MilestoneOcc is one dated occurrence of a milestone.
type MilestoneOcc struct {
	Decl  *Milestone
	Index int // position of Decl in Model.Milestones
	Date  time.Time
	Idx   int // 1-based occurrence number
	N     int // occurrences generated up to the window end
}

// IntervalOcc is one dated occurrence [Start, End) of an interval.
type IntervalOcc struct {
	Decl  *Interval
	Index int // position of Decl in Model.Intervals
	Start time.Time
	End   time.Time // exclusive
	Idx   int
	N     int
}

// Resolved is every occurrence up to the window end, in declaration order
// and then occurrence order.
type Resolved struct {
	Milestones []MilestoneOcc
	Intervals  []IntervalOcc
}
