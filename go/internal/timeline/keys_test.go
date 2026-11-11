package timeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"a4calendar/internal/dsl"
)

var today = time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

const examplesDir = "../../../docs/timeline/examples"

func readKeys(t *testing.T, text string) (*Decls, []Warning, error) {
	t.Helper()
	m, err := dsl.Parse([]dsl.Source{{Name: "k.dates", Text: text}}, today)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return ReadKeys(m)
}

func mustReadKeys(t *testing.T, text string) *Decls {
	t.Helper()
	d, _, err := readKeys(t, text)
	if err != nil {
		t.Fatalf("ReadKeys: %v", err)
	}
	return d
}

func TestDefaults(t *testing.T) {
	d := mustReadKeys(t, "m=2027-01-01\n2027-01-01/2d")
	m, iv := d.Milestones[0], d.Intervals[0]
	if m.Title != "m" || m.Color != "#1F1F1F" || m.Mark != Diamond || m.Tentative || m.Countdown != 0 || m.Hidden || m.Label != LabelFirst {
		t.Errorf("milestone defaults = %+v", m)
	}
	if iv.Title != "" || iv.Key != "1" || iv.Color != "#1F1F1F" || iv.Style != Solid || iv.Tentative ||
		iv.Length != NoLength || iv.Layer != Front || iv.Priority != Normal || iv.Group != "" || iv.Hidden || iv.Label != LabelFirst {
		t.Errorf("interval defaults = %+v", iv)
	}
}

func TestValues(t *testing.T) {
	d := mustReadKeys(t, `m=2027-01-01;title="Kick-off";color=TEAL;mark=Star;status=tentative;countdown=7;hide=YES;label=each
q=2027-01-01;title="";mark=ring;color=#a04a14
a=2027-01-01/2d;title="A";color=Gray;style=hatch;status=tentative;length=workdays;layer=back;priority=high;group=People;label=none
b=2027-01-01/2d;status=tentative
c=2027-01-01/2d;style=dashed;status=confirmed;length=Days;priority=low`)
	m, q := d.Milestones[0], d.Milestones[1]
	if m.Title != "Kick-off" || m.Color != "#2E6E62" || m.Mark != Star || !m.Tentative || m.Countdown != 7 || !m.Hidden || m.Label != LabelEach {
		t.Errorf("m = %+v", m)
	}
	if q.Title != "q" || q.Mark != Ring || q.Color != "#A04A14" {
		t.Errorf("q (empty title falls back to the id; hex upper-cased) = %+v", q)
	}
	a, b, c := d.Intervals[0], d.Intervals[1], d.Intervals[2]
	if a.Color != "#7A7A7A" || a.Style != Hatch || !a.Tentative || a.Length != Workdays || a.Layer != Back ||
		a.Priority != High || a.Group != "people" || a.Label != LabelNone {
		t.Errorf("a = %+v", a)
	}
	if b.Style != Dashed || !b.Tentative || b.Title != "b" {
		t.Errorf("b (tentative solid becomes dashed) = %+v", b)
	}
	if c.Style != Dashed || c.Tentative || c.Length != Days || c.Priority != Low {
		t.Errorf("c = %+v", c)
	}
}

func TestPaletteNames(t *testing.T) {
	for name, hex := range Palette {
		d := mustReadKeys(t, "m=2027-01-01;color="+strings.ToUpper(name))
		if d.Milestones[0].Color != hex {
			t.Errorf("color=%s -> %s; want %s", name, d.Milestones[0].Color, hex)
		}
	}
}

func TestIntervalKeysCountEveryInterval(t *testing.T) {
	d := mustReadKeys(t, "a=2027-01-01/2d\n2027-01-05/2d\nh=2027-01-09/2d;hide=yes\nz=2027-01-12/1d;every=1w")
	var keys []string
	for _, iv := range d.Intervals {
		keys = append(keys, iv.Key)
	}
	if got := strings.Join(keys, ","); got != "1,2,3,4" {
		t.Errorf("keys = %s; want 1,2,3,4 (anonymous and hidden included)", got)
	}
}

func TestUnknownKeysAreData(t *testing.T) {
	d, warn, err := readKeys(t, "m=2027-01-01;owner=alice;Team=backend\n2027-01-01/2d;ticket=ABC-1")
	if err != nil || len(warn) != 0 {
		t.Fatalf("err = %v, warnings = %v", err, warn)
	}
	if v, _ := d.Milestones[0].Props.Get("team"); v != "backend" {
		t.Errorf("team = %q", v)
	}
}

func TestValueErrors(t *testing.T) {
	cases := []struct{ decl, want string }{
		{"m=2027-01-01;color=pink", "color=pink: expected one of ink, red, teal, violet, orange, steel, ochre, magenta, grey, gray or #RRGGBB"},
		{"m=2027-01-01;color=#12345", "color=#12345: expected one of"},
		{"m=2027-01-01;color=#GG0000", "color=#GG0000: expected one of"},
		{"m=2027-01-01;mark=square", "mark=square: expected one of diamond, star, dot, ring"},
		{"m=2027-01-01;countdown=0", "countdown=0: expected an integer from 1 to 99"},
		{"m=2027-01-01;countdown=100", "countdown=100: expected an integer from 1 to 99"},
		{"m=2027-01-01;countdown=+5", "countdown=+5: expected an integer from 1 to 99"},
		{"m=2027-01-01;countdown=week", "countdown=week: expected an integer from 1 to 99"},
		{"m=2027-01-01;status=maybe", "status=maybe: expected one of confirmed, tentative"},
		{"m=2027-01-01;hide=true", "hide=true: expected one of yes, no"},
		{"m=2027-01-01;label=all", "label=all: expected one of first, each, none"},
		{"2027-01-01/2d;style=dotted", "style=dotted: expected one of solid, dashed, hatch"},
		{"2027-01-01/2d;status=planned", "status=planned: expected one of confirmed, tentative"},
		{"2027-01-01/2d;length=weeks", "length=weeks: expected one of days, workdays"},
		{"2027-01-01/2d;layer=top", "layer=top: expected one of front, back"},
		{"2027-01-01/2d;priority=urgent", "priority=urgent: expected one of high, normal, low"},
		{"2027-01-01/2d;group=1st", "group=1st: expected an identifier"},
		{"2027-01-01/2d;hide=1", "hide=1: expected one of yes, no"},
		{"2027-01-01/2d;label=last", "label=last: expected one of first, each, none"},
	}
	for _, c := range cases {
		_, _, err := readKeys(t, c.decl)
		if err == nil || !strings.HasPrefix(err.Error(), "k.dates:1: "+c.want) {
			t.Errorf("%s: error = %v; want k.dates:1: %s", c.decl, err, c.want)
		}
	}
}

func TestValueErrorsAreCollected(t *testing.T) {
	_, _, err := readKeys(t, "a=2027-01-01;mark=x;color=y\nb=2027-01-01\n2027-01-01/2d;style=z")
	want := "k.dates:1: color=y: expected one of ink, red, teal, violet, orange, steel, ochre, magenta, grey, gray or #RRGGBB\n" +
		"k.dates:1: mark=x: expected one of diamond, star, dot, ring\n" +
		"k.dates:3: style=z: expected one of solid, dashed, hatch"
	if err == nil || err.Error() != want {
		t.Errorf("errors:\n%v\nwant:\n%s", err, want)
	}
}

func TestWarnings(t *testing.T) {
	d, warn, err := readKeys(t, "r=2027-01-01;every=1w;countdown=3\nm=2027-01-01;style=dashed;group=g\n2027-01-01/2d;mark=star;countdown=2")
	if err != nil {
		t.Fatal(err)
	}
	if d.Milestones[0].Countdown != 0 {
		t.Errorf("countdown on a series must be dropped")
	}
	var got []string
	for _, w := range warn {
		got = append(got, w.String())
	}
	want := []string{
		"k.dates:1: warning: countdown is ignored on a recurring milestone",
		"k.dates:2: warning: style has no effect on a milestone",
		"k.dates:2: warning: group has no effect on a milestone",
		"k.dates:3: warning: mark has no effect on an interval",
		"k.dates:3: warning: countdown has no effect on an interval",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("warnings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestGoldenHidden checks the "hidden" flags in every golden file's resolved
// milestones (the dates themselves are checked in package dsl).
func TestGoldenHidden(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(examplesDir, "*.expected.json"))
	if len(files) == 0 {
		t.Fatal("no golden files")
	}
	for _, path := range files {
		var g struct {
			Fixture  string
			Resolved struct {
				Milestones []struct {
					ID     string
					Hidden bool
				}
			}
		}
		raw, _ := os.ReadFile(path)
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatal(err)
		}
		text, _ := os.ReadFile(filepath.Join(examplesDir, g.Fixture))
		d := mustReadKeys(t, string(text))
		hidden := map[string]bool{}
		for _, m := range d.Milestones {
			hidden[m.ID] = m.Hidden
		}
		for _, w := range g.Resolved.Milestones {
			if hidden[w.ID] != w.Hidden {
				t.Errorf("%s: %s hidden = %v; want %v", filepath.Base(path), w.ID, hidden[w.ID], w.Hidden)
			}
		}
	}
}
