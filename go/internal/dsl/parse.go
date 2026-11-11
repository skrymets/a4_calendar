package dsl

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"a4calendar/internal/period"
)

// utf8BOM is skipped at the start of a file.
const utf8BOM = "\xef\xbb\xbf"

// msRef is what a later declaration can see of a milestone name.
type msRef struct {
	date   time.Time
	series bool // recurring: not usable as a scalar date
	broken bool // declared with errors: references fail silently
}

type parser struct {
	today      time.Time
	milestones map[string]msRef // lower-case id
	intervals  map[string]bool  // lower-case id of named intervals
	model      *Model
	errs       Errors
}

// Parse reads the sources in order (files, then inline milestones, then
// inline intervals, as the caller supplies them) and resolves each
// declaration's first date against today. It reports every error found;
// a declaration that only fails because it refers to a failed one is not
// reported twice. The Model is complete only when the error is nil.
func Parse(sources []Source, today time.Time) (*Model, error) {
	p := &parser{
		today:      today,
		milestones: map[string]msRef{},
		intervals:  map[string]bool{},
		model:      &Model{},
	}
	for _, src := range sources {
		if src.inline() {
			p.declaration(src, 0, strings.TrimSpace(src.Text), nil)
			continue
		}
		seenInterval := false
		text := strings.TrimPrefix(src.Text, utf8BOM)
		for i, raw := range strings.Split(text, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || line[0] == '#' {
				continue
			}
			p.declaration(src, i+1, line, &seenInterval)
		}
	}
	if len(p.errs) > 0 {
		return p.model, p.errs
	}
	return p.model, nil
}

// declaration parses one line and records its error, if any.
func (p *parser) declaration(src Source, line int, text string, seenInterval *bool) {
	if err := p.parseDecl(src, line, text, seenInterval); err != nil && !errors.Is(err, errBroken) {
		p.errs = append(p.errs, &Error{Source: src.Name, Line: line, Msg: err.Error()})
	}
}

func (p *parser) parseDecl(src Source, line int, text string, seenInterval *bool) error {
	if text == "" {
		return errorf("empty declaration")
	}
	if !utf8.ValidString(text) {
		return errorf("not valid UTF-8")
	}
	parts, err := splitTop(text, ';')
	if err != nil {
		return err
	}
	head := strings.TrimSpace(parts[0])
	sides, err := splitTop(head, '/')
	if err != nil {
		return err
	}
	isInterval := len(sides) > 1

	name, expr := "", head
	if before, after, ok := strings.Cut(head, "="); ok {
		name, expr = strings.TrimSpace(before), strings.TrimSpace(after)
		if !identRE.MatchString(name) {
			return errorf("invalid identifier %q (letters, digits and _, not starting with a digit)", name)
		}
	}

	lname := strings.ToLower(name)
	_, dup := p.milestones[lname]
	dup = dup || p.intervals[lname]
	// broken registers a failed declaration's name, so references to it
	// are not reported again; asInterval is the kind the user meant.
	broken := func(asInterval bool) {
		if name == "" || dup || lname == "today" {
			return
		}
		if asInterval {
			p.intervals[lname] = true
		} else {
			p.milestones[lname] = msRef{broken: true}
		}
	}

	switch {
	case src.Expect == KindMilestone && isInterval:
		broken(false)
		return errorf("a milestone cannot contain '/'; declare intervals with --interval")
	case src.Expect == KindInterval && !isInterval:
		broken(true)
		return errorf("an interval needs START/END, START/DURATION or DURATION/END")
	case !isInterval && name == "":
		return errorf("a milestone needs a name: NAME=DATE")
	case !isInterval && seenInterval != nil && *seenInterval:
		broken(false)
		return errorf("milestones must be declared before the first interval")
	}
	if isInterval && seenInterval != nil {
		*seenInterval = true
	}

	if name != "" {
		if lname == "today" {
			return errorf(`"today" is reserved and cannot be an identifier`)
		}
		if dup {
			return errorf("duplicate identifier %q", name)
		}
	}

	var ok bool
	if isInterval {
		ok = p.interval(src, line, name, expr, parts[1:], &err)
	} else {
		ok = p.milestone(src, line, name, expr, parts[1:], &err)
	}
	if !ok {
		broken(isInterval)
	}
	return err
}

func (p *parser) milestone(src Source, line int, name, expr string, propParts []string, errp *error) bool {
	fail := func(err error) bool { *errp = err; return false }
	props, err := parseProps(propParts)
	if err != nil {
		return fail(err)
	}
	date, err := p.date(expr)
	if err != nil {
		return fail(err)
	}
	rec, err := p.recurrence(props, false)
	if err != nil {
		return fail(err)
	}
	if rec != nil && rec.HasUntil && rec.Until.Before(date) {
		return fail(errorf("until %s is before the first occurrence %s", iso(rec.Until), iso(date)))
	}
	p.model.Milestones = append(p.model.Milestones, &Milestone{
		ID: name, Date: date, Rec: rec, Props: props, Source: src.Name, Line: line, Expr: expr,
	})
	p.milestones[strings.ToLower(name)] = msRef{date: date, series: rec != nil}
	return true
}

func (p *parser) interval(src Source, line int, name, expr string, propParts []string, errp *error) bool {
	fail := func(err error) bool { *errp = err; return false }
	props, err := parseProps(propParts)
	if err != nil {
		return fail(err)
	}
	sides, _ := splitTop(expr, '/') // quotes already checked on the whole line
	if len(sides) != 2 {
		return fail(errorf("an interval needs exactly one '/'"))
	}
	left, lIsDur, err := parsePeriod(sides[0])
	if err != nil {
		return fail(err)
	}
	right, rIsDur, err := parsePeriod(sides[1])
	if err != nil {
		return fail(err)
	}
	iv := &Interval{ID: name, Props: props, Source: src.Name, Line: line, Expr: expr}
	switch {
	case lIsDur && rIsDur:
		return fail(errorf("only one side of an interval may be a duration"))
	case rIsDur: // START/DURATION
		if iv.Start, err = p.date(sides[0]); err != nil {
			return fail(err)
		}
		iv.End, iv.DurKind, iv.Duration = shift(iv.Start, right.N, right.Unit), StartDuration, right
	case lIsDur: // DURATION/END
		if iv.End, err = p.date(sides[1]); err != nil {
			return fail(err)
		}
		iv.Start, iv.DurKind, iv.Duration = shift(iv.End, -left.N, left.Unit), DurationEnd, left
	default: // START/END
		if iv.Start, err = p.date(sides[0]); err != nil {
			return fail(err)
		}
		if iv.End, err = p.date(sides[1]); err != nil {
			return fail(err)
		}
		iv.DurKind = StartEnd
	}
	if !inRange(iv.Start) || !inRange(iv.End) {
		return fail(errorf("interval %s–%s is outside the years 0001–9999", iso(iv.Start), iso(iv.End)))
	}
	if !iv.Start.Before(iv.End) {
		return fail(errorf("interval start %s must be before its end %s", iso(iv.Start), iso(iv.End)))
	}
	iv.Days = period.DaysBetween(iv.Start, iv.End) // not Sub: Duration saturates after ~292 years
	if iv.Rec, err = p.recurrence(props, true); err != nil {
		return fail(err)
	}
	if iv.Rec != nil && iv.Rec.HasUntil && iv.Rec.Until.Before(iv.Start) {
		return fail(errorf("until %s is before the first occurrence %s", iso(iv.Rec.Until), iso(iv.Start)))
	}
	p.model.Intervals = append(p.model.Intervals, iv)
	if name != "" {
		p.intervals[strings.ToLower(name)] = true
	}
	return true
}

// date resolves a date expression: an anchor (ISO date, today, @milestone,
// or nothing = today) and at most one signed offset.
func (p *parser) date(expr string) (time.Time, error) {
	expr = strings.TrimSpace(expr)
	m := dateRE.FindStringSubmatch(expr)
	if m == nil || expr == "" {
		return time.Time{}, errorf("invalid date expression %q", expr)
	}
	var t time.Time
	switch {
	case m[1] != "":
		d, err := time.Parse("2006-01-02", m[1])
		if err != nil || d.Year() < 1 {
			return time.Time{}, errorf("invalid date %s", m[1])
		}
		t = d
	case m[3] != "":
		ref, ok := p.milestones[strings.ToLower(m[3])]
		switch {
		case !ok && p.intervals[strings.ToLower(m[3])]:
			return time.Time{}, errorf("@%s is an interval; only milestones can be referenced", m[3])
		case !ok:
			return time.Time{}, errorf("unknown milestone @%s (only milestones declared earlier can be referenced)", m[3])
		case ref.broken:
			return time.Time{}, errBroken
		case ref.series:
			return time.Time{}, errorf("@%s is recurring and cannot be used as a date", m[3])
		}
		t = ref.date
	default: // "today", or an offset alone
		t = p.today
	}
	if m[4] != "" {
		n, err := amount(m[5])
		if err != nil {
			return time.Time{}, err
		}
		if m[4] == "-" {
			n = -n
		}
		t = shift(t, n, strings.ToLower(m[6])[0])
	}
	if !inRange(t) {
		return time.Time{}, errorf("date expression %q is outside the years 0001–9999", expr)
	}
	return t, nil
}

// recurrence reads every/gap/count/until; nil when there is none.
func (p *parser) recurrence(props Props, isInterval bool) (*Recurrence, error) {
	every, hasEvery := props.Get("every")
	gap, hasGap := props.Get("gap")
	count, hasCount := props.Get("count")
	until, hasUntil := props.Get("until")
	switch {
	case !hasEvery && !hasGap:
		if hasCount || hasUntil {
			return nil, errorf("count and until need every or gap")
		}
		return nil, nil
	case hasEvery && hasGap:
		return nil, errorf("every and gap are mutually exclusive")
	case hasGap && !isInterval:
		return nil, errorf("gap is only valid for intervals")
	case hasCount && hasUntil:
		return nil, errorf("count and until are mutually exclusive")
	}
	r := &Recurrence{Mode: Every}
	freq := every
	if hasGap {
		r.Mode, freq = Gap, gap
	}
	per, ok, err := parsePeriod(freq)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errorf("every/gap needs a positive period such as 2w, got %q", freq)
	}
	r.Period = per
	if hasCount {
		if r.Count, err = parseCount(count); err != nil {
			return nil, err
		}
	}
	if hasUntil {
		if r.Until, err = p.date(until); err != nil {
			return nil, err
		}
		r.HasUntil = true
	}
	return r, nil
}
