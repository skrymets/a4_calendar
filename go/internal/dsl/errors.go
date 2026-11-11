package dsl

import (
	"errors"
	"fmt"
	"strings"
)

// Error is one problem in one declaration: "source:line: message" for a
// file, "source: message" for an inline declaration (Line 0).
type Error struct {
	Source string
	Line   int
	Msg    string
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return e.Source + ": " + e.Msg
	}
	return fmt.Sprintf("%s:%d: %s", e.Source, e.Line, e.Msg)
}

// Errors is every problem found in one run, in source order.
type Errors []*Error

func (es Errors) Error() string {
	lines := make([]string, len(es))
	for i, e := range es {
		lines[i] = e.Error()
	}
	return strings.Join(lines, "\n")
}

// errBroken marks a declaration that fails only because it refers to a
// declaration that already failed; it is not reported again.
var errBroken = errors.New("refers to a declaration with errors")

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }
