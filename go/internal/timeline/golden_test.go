package timeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"pinplan/internal/dsl"
)

// a4Grid is the grid width of an A4 page with the default 10 mm margins.
const a4Grid = 190 * mm

var defaultWeekend = map[int]bool{5: true, 6: true}

// goldenOptions maps the reference renderer's options (golden.js) to ours.
func goldenOptions(t *testing.T, raw map[string]any) (Options, time.Time) {
	t.Helper()
	date := func(k string) time.Time {
		s, _ := raw[k].(string)
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatalf("option %s: %v", k, err)
		}
		return d
	}
	o := Options{Start: date("start"), End: date("end"), GridWidth: a4Grid, Weekend: defaultWeekend}
	if raw["layout"] == "flow" {
		o.Flow = true
	}
	if raw["compact"] == "weekly" {
		o.WeeklyCompact = true
	}
	if c, ok := raw["cap"].(float64); ok {
		o.LaneCap = int(c)
	}
	if raw["mergeGroups"] == "off" {
		o.Groups = GroupsSeparate
	}
	return o, date("today")
}

// build runs the whole pipeline on one fixture file.
func build(t *testing.T, fixture, text string, o Options, today time.Time) (*Decls, *dsl.Resolved, *Layout) {
	t.Helper()
	m, err := dsl.Parse([]dsl.Source{{Name: fixture, Text: text}}, today)
	if err != nil {
		t.Fatal(err)
	}
	r, err := dsl.Expand(m, o.End)
	if err != nil {
		t.Fatal(err)
	}
	d, _, err := ReadKeys(m)
	if err != nil {
		t.Fatal(err)
	}
	return d, r, Build(d, r, o)
}

func TestGoldenLayout(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(examplesDir, "*.expected.json"))
	if len(files) == 0 {
		t.Fatal("no golden files")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			fixture := want["fixture"].(string)
			o, today := goldenOptions(t, want["options"].(map[string]any))
			text, err := os.ReadFile(filepath.Join(examplesDir, fixture))
			if err != nil {
				t.Fatal(err)
			}
			d, r, l := build(t, fixture, string(text), o, today)
			out, err := DebugJSON(DebugInfo{Fixture: fixture}, d, r, l)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			for _, part := range []string{"resolved", "layout"} {
				if !reflect.DeepEqual(got[part], want[part]) {
					t.Errorf("%s differs:\n%s", part, diff(got[part], want[part], ""))
				}
			}
		})
	}
}

// diff describes where two decoded JSON values differ (first few places).
func diff(got, want any, path string) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %v, want %v\n", path, got, want)
		}
		s := ""
		for k := range w {
			if !reflect.DeepEqual(g[k], w[k]) {
				s += diff(g[k], w[k], path+"."+k)
			}
		}
		for k := range g {
			if _, ok := w[k]; !ok {
				s += fmt.Sprintf("%s.%s: unexpected %v\n", path, k, g[k])
			}
		}
		return s
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %v, want %v\n", path, got, want)
		}
		s := ""
		for i := 0; i < max(len(g), len(w)); i++ {
			if len(s) > 2000 {
				return s + "...\n"
			}
			switch {
			case i >= len(g):
				s += fmt.Sprintf("%s[%d]: missing %v\n", path, i, w[i])
			case i >= len(w):
				s += fmt.Sprintf("%s[%d]: unexpected %v\n", path, i, g[i])
			case !reflect.DeepEqual(g[i], w[i]):
				s += diff(g[i], w[i], fmt.Sprintf("%s[%d]", path, i))
			}
		}
		return s
	default:
		return fmt.Sprintf("%s: got %v, want %v\n", path, got, want)
	}
}
