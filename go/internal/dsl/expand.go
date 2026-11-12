package dsl

import (
	"fmt"
	"time"
)

// MaxOccurrences bounds one declaration's occurrences up to the window end.
const MaxOccurrences = 100_000

// span is one occurrence [s, e); a milestone's e equals s.
type span struct{ s, e time.Time }

// occurrences lists the spans of a recurrence that starts with first, up to
// its count, its until date, windowEnd or the year 9999, and at most limit
// of them. every: occurrence i starts at the first start + i periods (no
// drift); gap: each start is the previous (exclusive) end + the gap. endFrom
// gives the end of a later occurrence from its start.
func occurrences(first span, rec *Recurrence, endFrom func(time.Time) time.Time, windowEnd time.Time, limit int) []span {
	var out []span
	sp := first
	for i := 0; len(out) < limit; i++ {
		if i > 0 {
			p := rec.Period
			if rec.Mode == Every {
				sp.s = shift(first.s, i*p.N, p.Unit)
			} else {
				sp.s = shift(sp.e, p.N, p.Unit)
			}
			sp.e = endFrom(sp.s)
		}
		if stop(rec, i, sp.s, windowEnd) {
			break
		}
		out = append(out, sp)
	}
	return out
}

// Expand lists every occurrence that starts on or before windowEnd: all of a
// non-recurring declaration's single occurrence, and a recurrence's
// occurrences until its count, its until date or windowEnd, whichever comes
// first. Occurrences before the window start are included (REQ-RES-2); N
// counts the occurrences listed.
//
// An interval keeps its length as written (§6.7): START/END its days,
// START/DURATION and DURATION/END the duration from each start.
func Expand(m *Model, windowEnd time.Time) (*Resolved, error) {
	r := &Resolved{}
	var errs Errors
	// list expands one recurrence, reporting one that has too many.
	list := func(first span, rec *Recurrence, endFrom func(time.Time) time.Time, source string, line int) []span {
		occ := occurrences(first, rec, endFrom, windowEnd, MaxOccurrences+1)
		if len(occ) > MaxOccurrences {
			errs = append(errs, &Error{Source: source, Line: line, Msg: fmt.Sprintf(
				"the recurrence has more than %d occurrences up to %s; start it later or limit it with count or until",
				MaxOccurrences, iso(windowEnd))})
			occ = occ[:MaxOccurrences]
		}
		return occ
	}

	for mi, ms := range m.Milestones {
		if ms.Rec == nil {
			r.Milestones = append(r.Milestones, MilestoneOcc{Decl: ms, Index: mi, Date: ms.Date, Idx: 1, N: 1})
			continue
		}
		occ := list(span{ms.Date, ms.Date}, ms.Rec, sameDay, ms.Source, ms.Line)
		for i, o := range occ {
			r.Milestones = append(r.Milestones, MilestoneOcc{Decl: ms, Index: mi, Date: o.s, Idx: i + 1, N: len(occ)})
		}
	}

	for xi, iv := range m.Intervals {
		if iv.Rec == nil {
			r.Intervals = append(r.Intervals, IntervalOcc{Decl: iv, Index: xi, Start: iv.Start, End: iv.End, Idx: 1, N: 1})
			continue
		}
		occ := list(span{iv.Start, iv.End}, iv.Rec, iv.endFrom, iv.Source, iv.Line)
		for i, o := range occ {
			r.Intervals = append(r.Intervals, IntervalOcc{Decl: iv, Index: xi, Start: o.s, End: o.e, Idx: i + 1, N: len(occ)})
		}
	}
	if len(errs) > 0 {
		return r, errs
	}
	return r, nil
}

// stop reports whether occurrence i, starting at t, is past the recurrence's
// count, its until date, the window end or the year 9999.
func stop(rec *Recurrence, i int, t, windowEnd time.Time) bool {
	return (rec.Count > 0 && i >= rec.Count) ||
		(rec.HasUntil && t.After(rec.Until)) ||
		t.After(windowEnd) || !inRange(t)
}

func sameDay(t time.Time) time.Time { return t }

// endFrom is the exclusive end of a later occurrence starting at s.
func (iv *Interval) endFrom(s time.Time) time.Time {
	if iv.DurKind == StartEnd {
		return s.AddDate(0, 0, iv.Days)
	}
	return iv.Duration.forward(s)
}
