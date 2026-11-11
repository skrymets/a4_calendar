package dsl

import (
	"fmt"
	"time"
)

// MaxOccurrences bounds one declaration's occurrences up to the window end.
const MaxOccurrences = 100_000

// Expand lists every occurrence that starts on or before windowEnd: all of a
// non-recurring declaration's single occurrence, and a recurrence's
// occurrences until its count, its until date or windowEnd, whichever comes
// first. Occurrences before the window start are included (REQ-RES-2); N
// counts the occurrences listed.
//
// every: occurrence i starts at the original start + i periods (no drift).
// gap: each start is the previous (exclusive) end + the gap.
// An interval keeps its length as written (§6.7): START/END its days,
// START/DURATION and DURATION/END the duration from each start.
func Expand(m *Model, windowEnd time.Time) (*Resolved, error) {
	r := &Resolved{}
	var errs Errors
	tooMany := func(source string, line int) {
		errs = append(errs, &Error{Source: source, Line: line, Msg: fmt.Sprintf(
			"the recurrence has more than %d occurrences up to %s; start it later or limit it with count or until",
			MaxOccurrences, iso(windowEnd))})
	}

	for mi, ms := range m.Milestones {
		if ms.Rec == nil {
			r.Milestones = append(r.Milestones, MilestoneOcc{Decl: ms, Index: mi, Date: ms.Date, Idx: 1, N: 1})
			continue
		}
		var occ []MilestoneOcc
		for i := 0; ; i++ {
			t := shift(ms.Date, i*ms.Rec.Period.N, ms.Rec.Period.Unit)
			if stop(ms.Rec, i, t, windowEnd) {
				break
			}
			if len(occ) == MaxOccurrences {
				tooMany(ms.Source, ms.Line)
				break
			}
			occ = append(occ, MilestoneOcc{Decl: ms, Index: mi, Date: t, Idx: i + 1})
		}
		for i := range occ {
			occ[i].N = len(occ)
		}
		r.Milestones = append(r.Milestones, occ...)
	}

	for xi, iv := range m.Intervals {
		if iv.Rec == nil {
			r.Intervals = append(r.Intervals, IntervalOcc{Decl: iv, Index: xi, Start: iv.Start, End: iv.End, Idx: 1, N: 1})
			continue
		}
		var occ []IntervalOcc
		s, e := iv.Start, iv.End
		for i := 0; ; i++ {
			if i > 0 {
				p := iv.Rec.Period
				if iv.Rec.Mode == Every {
					s = shift(iv.Start, i*p.N, p.Unit)
				} else {
					s = shift(e, p.N, p.Unit)
				}
				e = iv.endFrom(s)
			}
			if stop(iv.Rec, i, s, windowEnd) {
				break
			}
			if len(occ) == MaxOccurrences {
				tooMany(iv.Source, iv.Line)
				break
			}
			occ = append(occ, IntervalOcc{Decl: iv, Index: xi, Start: s, End: e, Idx: i + 1})
		}
		for i := range occ {
			occ[i].N = len(occ)
		}
		r.Intervals = append(r.Intervals, occ...)
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

// endFrom is the exclusive end of a later occurrence starting at s.
func (iv *Interval) endFrom(s time.Time) time.Time {
	if iv.DurKind == StartEnd {
		return s.AddDate(0, 0, iv.Days)
	}
	return shift(s, iv.Duration.N, iv.Duration.Unit)
}
