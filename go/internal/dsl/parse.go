package dsl

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"a4calendar/internal/period"
)

// utf8BOM is skipped at the start of a file.
const utf8BOM = "\xef\xbb\xbf"

// ref is what a later declaration can see of a name: its milestone or its
// interval, or neither when it was declared with errors (references to it
// then fail without a second message).
type ref struct {
	ms     *Milestone
	iv     *Interval
	broken bool
}

type parser struct {
	today time.Time
	names map[string]ref // lower-case id
	model *Model
	errs  Errors
}

// Parse reads the sources in order (files, then inline milestones, then
// inline intervals, as the caller supplies them) and resolves each
// declaration's first date against today. It reports every error found;
// a declaration that only fails because it refers to a failed one is not
// reported twice. The Model is complete only when the error is nil.
func Parse(sources []Source, today time.Time) (*Model, error) {
	names := map[string]ref{}
	p := &parser{
		today: today,
		names: names,
		model: &Model{today: today, names: names},
	}
	for _, src := range sources {
		if src.inline() {
			p.declaration(src, 0, strings.TrimSpace(src.Text))
			continue
		}
		text := strings.TrimPrefix(src.Text, utf8BOM)
		for i, raw := range strings.Split(text, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || line[0] == '#' {
				continue
			}
			p.declaration(src, src.Offset+i+1, line)
		}
	}
	if len(p.errs) > 0 {
		return p.model, p.errs
	}
	return p.model, nil
}

// declaration parses one line and records its error, if any.
func (p *parser) declaration(src Source, line int, text string) {
	err := p.parseDecl(src, line, text)
	if err == nil || errors.Is(err, ErrBroken) {
		return
	}
	msg := err.Error()
	if src.inline() && errors.Is(err, errUnquotedSpace) {
		msg += "; Windows PowerShell 5.1 removes the quotes unless --% comes before the arguments, or use --file"
	}
	p.errs = append(p.errs, &Error{Source: src.Name, Line: line, Msg: msg})
}

func (p *parser) parseDecl(src Source, line int, text string) error {
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
	_, dup := p.names[lname]
	// broken registers a failed declaration's name, so references to it
	// are not reported again.
	broken := func() {
		if name != "" && !dup && lname != "today" {
			p.names[lname] = ref{broken: true}
		}
	}

	switch {
	case src.Expect == KindMilestone && isInterval:
		broken()
		return errorf("a milestone cannot contain '/'; declare intervals with --interval")
	case src.Expect == KindInterval && !isInterval:
		broken()
		return errorf("an interval needs START/END, START/DURATION or DURATION/END")
	case !isInterval && name == "":
		return errorf("a milestone needs a name: NAME=DATE")
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
		broken()
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
	ms := &Milestone{ID: name, Date: date, Rec: rec, Props: props, Source: src.Name, Line: line, Expr: expr}
	p.model.Milestones = append(p.model.Milestones, ms)
	p.names[strings.ToLower(name)] = ref{ms: ms}
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
	// the END is the last day; "<END" is the day after the last (DSL §5)
	lhs, rhs := strings.TrimSpace(sides[0]), strings.TrimSpace(sides[1])
	if strings.HasPrefix(lhs, "<") {
		return fail(errorf("the start is always included; < marks an exclusive end: START/<END"))
	}
	rhs, exclusive := strings.CutPrefix(rhs, "<")
	left, lIsDur, err := parseDuration(lhs)
	if err != nil {
		return fail(err)
	}
	right, rIsDur, err := parseDuration(rhs)
	if err != nil {
		return fail(err)
	}
	// end resolves the END to the exclusive end kept in Interval.End
	end := func() (time.Time, error) {
		t, err := p.date(rhs)
		if err != nil || exclusive {
			return t, err
		}
		return period.AddDays(t, 1), nil
	}
	iv := &Interval{ID: name, Props: props, Source: src.Name, Line: line, Expr: expr}
	switch {
	case lIsDur && rIsDur:
		return fail(errorf("only one side of an interval may be a duration"))
	case rIsDur && exclusive:
		return fail(errorf("a duration has no end to exclude; < goes before an end date"))
	case rIsDur: // START/DURATION
		if iv.Start, err = p.date(lhs); err != nil {
			return fail(err)
		}
		iv.End, iv.DurKind, iv.Duration = right.forward(iv.Start), StartDuration, right
	case lIsDur: // DURATION/END
		if iv.End, err = end(); err != nil {
			return fail(err)
		}
		iv.Start, iv.DurKind, iv.Duration = left.backward(iv.End), DurationEnd, left
	default: // START/END
		if iv.Start, err = p.date(lhs); err != nil {
			return fail(err)
		}
		if iv.End, err = end(); err != nil {
			return fail(err)
		}
		iv.DurKind = StartEnd
	}
	last := period.AddDays(iv.End, -1)
	if !inRange(iv.Start) || !inRange(last) {
		return fail(errorf("interval %s–%s is outside the years 0001–9999", iso(iv.Start), iso(last)))
	}
	if !iv.Start.Before(iv.End) {
		switch {
		case iv.DurKind != StartEnd:
			return fail(errorf("interval %s has no days: the duration must come to one day or more", expr))
		case exclusive && iv.End.Equal(iv.Start):
			return fail(errorf("interval %s has no days: an exclusive end must come after the start", expr))
		}
		return fail(errorf("interval ends on %s, before its start %s", iso(last), iso(iv.Start)))
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
		p.names[strings.ToLower(name)] = ref{iv: iv}
	}
	return true
}

// date resolves a date expression: an anchor (ISO date, today or @today,
// a reference, or nothing = today) and any number of signed offsets,
// applied left to right (DSL spec §14).
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
		var err error
		if t, err = p.reference(m[3], m[4], strings.ToLower(m[5])); err != nil {
			return time.Time{}, err
		}
	default: // "today", or an offset alone
		t = p.today
	}
	offsets, err := parseTerms(m[6])
	if err != nil {
		return time.Time{}, err
	}
	t = offsets.forward(t)
	if !inRange(t) {
		return time.Time{}, errorf("date expression %q is outside the years 0001–9999", expr)
	}
	return t, nil
}

// reference resolves @name, @name[index] and either of them followed by
// .start or .end (bound): a milestone's date, or an interval's first or
// last day, of the declaration or of one occurrence of its series
// (DSL spec §17, §18). Only names declared earlier are known.
func (p *parser) reference(name, index, bound string) (time.Time, error) {
	if strings.EqualFold(name, "today") { // @today is today: never a declaration
		if index != "" || bound != "" {
			return time.Time{}, errorf("@%s is a date, not a milestone or interval; leave out [N], .start and .end", name)
		}
		return p.today, nil
	}
	r, ok := p.names[strings.ToLower(name)]
	at := "@" + name
	if index != "" {
		at += "[" + index + "]"
	}
	switch {
	case !ok:
		return time.Time{}, errorf("unknown name @%s (only milestones and intervals declared earlier can be referenced)", name)
	case r.broken:
		return time.Time{}, ErrBroken
	case r.ms != nil && bound != "":
		return time.Time{}, errorf("@%s is a milestone; .start and .end apply to intervals", name)
	case r.iv != nil && bound == "":
		return time.Time{}, errorf("@%s is an interval; use %s.start or %s.end", name, at, at)
	}

	// the first occurrence, the recurrence and how a later one ends
	var (
		first   span
		rec     *Recurrence
		endFrom func(time.Time) time.Time
	)
	if r.ms != nil {
		first, rec, endFrom = span{r.ms.Date, r.ms.Date}, r.ms.Rec, sameDay
	} else {
		first, rec, endFrom = span{r.iv.Start, r.iv.End}, r.iv.Rec, r.iv.endFrom
	}
	sp := first
	switch {
	case index != "" && rec == nil:
		return time.Time{}, errorf("@%s is not recurring; leave out [%s]", name, index)
	case index == "" && rec != nil:
		return time.Time{}, errorf("@%s is recurring; pick one occurrence, e.g. @%s[1]%s or @%s[-1]%s",
			name, name, dotted(bound), name, dotted(bound))
	case index != "":
		var err error
		if sp, err = nthOccurrence(index, first, rec, endFrom); err != nil {
			return time.Time{}, errorf("%s: %v", at, err)
		}
	}
	if bound == "end" { // the last day, as an END is written
		return period.AddDays(sp.e, -1), nil
	}
	return sp.s, nil
}

func dotted(bound string) string {
	if bound == "" {
		return ""
	}
	return "." + bound
}

// nthOccurrence picks occurrence index of a series: 1 is the first, -1 the
// last, which needs a count or an until date. The page plays no part:
// a reference means the same date whatever the period.
func nthOccurrence(index string, first span, rec *Recurrence, endFrom func(time.Time) time.Time) (span, error) {
	n, err := strconv.Atoi(index)
	if err != nil || n == 0 || n > MaxOccurrences || n < -MaxOccurrences {
		return span{}, errorf("an occurrence number is 1 to %d, or -1 to -%d counting back from the last", MaxOccurrences, MaxOccurrences)
	}
	lastDay := period.Date(9999, 12, 31)
	if n > 0 {
		occ := occurrences(first, rec, endFrom, lastDay, n)
		if len(occ) < n {
			return span{}, errorf("the series has only %d occurrences", len(occ))
		}
		return occ[n-1], nil
	}
	if rec.Count == 0 && !rec.HasUntil {
		return span{}, errorf("counting back from the last occurrence needs a series limited by count or until")
	}
	occ := occurrences(first, rec, endFrom, lastDay, MaxOccurrences+1)
	switch {
	case len(occ) > MaxOccurrences:
		return span{}, errorf("the series has more than %d occurrences; count from the first instead", MaxOccurrences)
	case -n > len(occ):
		return span{}, errorf("the series has only %d occurrences", len(occ))
	}
	return occ[len(occ)+n], nil
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
