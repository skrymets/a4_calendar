package dsl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// examplesDir holds the design fixtures and their golden files (05 §1).
const examplesDir = "../../../docs/timeline/examples"

type golden struct {
	Fixture string `json:"fixture"`
	Options struct {
		Today string `json:"today"`
		End   string `json:"end"`
	} `json:"options"`
	Resolved struct {
		Milestones []struct {
			ID         string `json:"id"`
			Date       string `json:"date"`
			Occurrence string `json:"occurrence"`
		} `json:"milestones"`
		Intervals []struct {
			ID           *string `json:"id"`
			Key          string  `json:"key"`
			Start        string  `json:"start"`
			EndExclusive string  `json:"endExclusive"`
			LastDrawnDay string  `json:"lastDrawnDay"`
			Occurrence   string  `json:"occurrence"`
		} `json:"intervals"`
	} `json:"resolved"`
}

// TestGoldenResolved checks the dates of every golden file's "resolved"
// section (the rendering keys behind "hidden" are checked in package timeline).
func TestGoldenResolved(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(examplesDir, "*.expected.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files in %s: %v", examplesDir, err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			var g golden
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &g); err != nil {
				t.Fatal(err)
			}
			text, err := os.ReadFile(filepath.Join(examplesDir, g.Fixture))
			if err != nil {
				t.Fatal(err)
			}
			m, err := Parse([]Source{{Name: g.Fixture, Text: string(text)}}, d(g.Options.Today))
			if err != nil {
				t.Fatal(err)
			}
			r, err := Expand(m, d(g.Options.End))
			if err != nil {
				t.Fatal(err)
			}

			var gotMs, wantMs []string
			for _, o := range r.Milestones {
				gotMs = append(gotMs, fmt.Sprintf("%s %s %s", o.Decl.ID, iso(o.Date), occurrence(o.Decl.Rec != nil, o.Idx, o.N)))
			}
			for _, w := range g.Resolved.Milestones {
				wantMs = append(wantMs, fmt.Sprintf("%s %s %s", w.ID, w.Date, w.Occurrence))
			}
			compare(t, "milestones", gotMs, wantMs)

			var gotIv, wantIv []string
			for _, o := range r.Intervals {
				gotIv = append(gotIv, fmt.Sprintf("%s #%d %s %s %s %s", o.Decl.ID, o.Index+1, iso(o.Start), iso(o.End),
					iso(o.End.AddDate(0, 0, -1)), occurrence(o.Decl.Rec != nil, o.Idx, o.N)))
			}
			for _, w := range g.Resolved.Intervals {
				id := ""
				if w.ID != nil {
					id = *w.ID
				}
				wantIv = append(wantIv, fmt.Sprintf("%s #%s %s %s %s %s", id, w.Key, w.Start, w.EndExclusive, w.LastDrawnDay, w.Occurrence))
			}
			compare(t, "intervals", gotIv, wantIv)
		})
	}
}

func occurrence(series bool, idx, n int) string {
	if !series {
		return ""
	}
	return strconv.Itoa(idx) + "/" + strconv.Itoa(n)
}

func compare(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d, want %d", what, len(got), len(want))
	}
	for i := 0; i < min(len(got), len(want)); i++ {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %q, want %q", what, i, got[i], want[i])
		}
	}
}
