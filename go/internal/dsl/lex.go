package dsl

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"a4calendar/internal/period"
)

var (
	identRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	periodRE = regexp.MustCompile(`^(?i)([1-9][0-9]*)([dwmy])$`)
	// anchor (ISO date | today | @name with an optional occurrence [n] and
	// boundary .start|.end), then any number of signed offsets.
	dateRE = regexp.MustCompile(`^(?i)(?:([0-9]{4}-[0-9]{2}-[0-9]{2})|(today)|@([A-Za-z_][A-Za-z0-9_]*)(?:\[(-?[0-9]+)\])?(?:\.(start|end))?)?((?:[+-][1-9][0-9]*[dwmy])*)$`)
	// a duration: terms, the first unsigned (1m+13d, 1m-1d, 1m2w)
	durationRE = regexp.MustCompile(`^(?i)[1-9][0-9]*[dwmy](?:[+-]?[1-9][0-9]*[dwmy])*$`)
	termRE     = regexp.MustCompile(`(?i)([+-]?)([1-9][0-9]*)([dwmy])`)
	countRE    = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// errUnquotedSpace marks a property value with whitespace but no quotes;
// for inline declarations the shell may have removed them.
var errUnquotedSpace = errors.New("unquoted value contains whitespace")

// maxAmount bounds a period's number: more days than years 1..9999 hold, so
// any larger number is out of range and arithmetic cannot overflow.
const maxAmount = 4_000_000

// splitTop splits s at sep outside double-quoted text. Inside quotes a
// backslash escapes the next character.
func splitTop(s string, sep byte) ([]string, error) {
	var out []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quoted:
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			} else if c == '"' {
				quoted = false
			}
		case c == '"':
			quoted = true
			cur.WriteByte(c)
		case c == sep:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quoted {
		return nil, errorf("unterminated quoted value")
	}
	return append(out, cur.String()), nil
}

// unquote returns a property value: a double-quoted string with \" and \\
// unescaped, or an unquoted value without whitespace.
func unquote(v string) (string, error) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, `"`) {
		if len(v) < 2 || !strings.HasSuffix(v, `"`) {
			return "", errorf("bad quoted value %s", v)
		}
		body := v[1 : len(v)-1]
		var b strings.Builder
		for i := 0; i < len(body); i++ {
			if body[i] == '\\' && i+1 < len(body) && (body[i+1] == '"' || body[i+1] == '\\') {
				i++
			}
			b.WriteByte(body[i])
		}
		return b.String(), nil
	}
	if v == "" {
		return "", errorf("empty property value")
	}
	if strings.ContainsAny(v, " \t;") {
		return "", errorf("%w: %s (use double quotes)", errUnquotedSpace, v)
	}
	return v, nil
}

// parseProps reads the ";key=value" parts after a declaration's expression.
func parseProps(parts []string) (Props, error) {
	var props Props
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue // a trailing or doubled ';'
		}
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, errorf("property without value: %q", strings.TrimSpace(p))
		}
		k = strings.TrimSpace(k)
		if !identRE.MatchString(k) {
			return nil, errorf("invalid property key %q", k)
		}
		k = strings.ToLower(k)
		if _, dup := props.Get(k); dup {
			return nil, errorf("duplicate property %q", k)
		}
		val, err := unquote(v)
		if err != nil {
			return nil, err
		}
		props = append(props, Prop{Key: k, Value: val})
	}
	return props, nil
}

// parsePeriod reads "2w" (case-insensitive); ok is false if s is not a
// period at all.
func parsePeriod(s string) (p period.Period, ok bool, err error) {
	m := periodRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return period.Period{}, false, nil
	}
	n, err := amount(m[1])
	if err != nil {
		return period.Period{}, true, err
	}
	return period.Period{N: n, Unit: strings.ToLower(m[2])[0]}, true, nil
}

// parseDuration reads a duration of one or more terms ("2w", "1m+13d",
// "1m2w"); ok is false if s is not a duration at all.
func parseDuration(s string) (d Duration, ok bool, err error) {
	s = strings.TrimSpace(s)
	if !durationRE.MatchString(s) {
		return nil, false, nil
	}
	d, err = parseTerms(s)
	return d, true, err
}

// parseTerms reads the signed terms of a duration or of a date's offsets
// (already checked by durationRE or dateRE); an unsigned term counts as +.
func parseTerms(s string) (Duration, error) {
	var d Duration
	for _, m := range termRE.FindAllStringSubmatch(s, -1) {
		n, err := amount(m[2])
		if err != nil {
			return nil, err
		}
		if m[1] == "-" {
			n = -n
		}
		d = append(d, Term{N: n, Unit: strings.ToLower(m[3])[0]})
	}
	return d, nil
}

func amount(digits string) (int, error) {
	n, err := strconv.Atoi(digits)
	if err != nil || n > maxAmount {
		return 0, errorf("number %s is too large", digits)
	}
	return n, nil
}

// parseCount reads a recurrence count (a positive integer).
func parseCount(s string) (int, error) {
	if !countRE.MatchString(s) {
		return 0, errorf("count must be a positive integer, got %q", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		n = math.MaxInt // only "too many digits" is possible: no practical limit
	}
	return n, nil
}

// shift moves t by n (signed) units of a period, with calendar arithmetic
// for months and years (clamped to the month end).
func shift(t time.Time, n int, unit byte) time.Time {
	switch unit {
	case 'd':
		return period.AddDays(t, n)
	case 'w':
		return period.AddDays(t, 7*n)
	case 'm':
		return period.AddMonthsClamped(t, n)
	default: // 'y'
		return period.AddMonthsClamped(t, 12*n)
	}
}

// inRange reports whether t is a date with a 4-digit year (0001-9999).
func inRange(t time.Time) bool { return t.Year() >= 1 && t.Year() <= 9999 }

func iso(t time.Time) string { return t.Format("2006-01-02") }
