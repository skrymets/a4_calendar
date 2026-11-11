package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func call(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

func TestRunWritesPDF(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.pdf")
	code, stdout, stderr := call("--start", "2026-11-07", "--period", "90d", "-o", out)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "Wrote " + out + ": 2026-11-07 → 2027-02-04 (90 days)\n"; stdout != want {
		t.Errorf("stdout = %q; want %q", stdout, want)
	}
	data, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Errorf("no PDF written: %v", err)
	}
	if !bytes.Contains(data, []byte("/Creator (yearly-calendar dev)")) {
		t.Error("PDF metadata lacks Creator 'yearly-calendar dev'")
	}
}

func TestRunDefaultOutputName(t *testing.T) {
	t.Chdir(t.TempDir())
	for args, name := range map[string]string{
		"--start 2026-11-07":              "calendar_2026-11-07.pdf",
		"--start 2026-11-07 --period 90d": "calendar_2026-11-07_90d.pdf",
	} {
		if code, _, stderr := call(strings.Fields(args)...); code != 0 {
			t.Fatalf("%s: exit %d, %s", args, code, stderr)
		}
		if _, err := os.Stat(name); err != nil {
			t.Errorf("%s: %s not created", args, name)
		}
	}
}

func TestRunUsageErrors(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--start", "2026-11-07", "--period", "53w"},
			"--period 53w (371 days) exceeds the 1-year maximum (365 days from 2026-11-07)"},
		{[]string{"--start", "2026-11-07", "--period", "999999m"},
			"--period 999999m exceeds the 1-year maximum (365 days from 2026-11-07)"},
		{[]string{"--start", "2026-03-01", "--period", "366d"},
			"--period 366d (366 days) exceeds the 1-year maximum (365 days from 2026-03-01)"},
		{[]string{"--period", "90"},
			"argument --period: invalid period '90': use <N>d, <N>w, <N>m or 1y (N >= 1)"},
		{[]string{"--period", "0d"}, "argument --period: invalid period '0d'"},
		{[]string{"--week-mode", "x"},
			"argument --week-mode: invalid choice: 'x' (choose from 'relative', 'iso', 'rolling')"},
		{[]string{"--paper", "a3"}, "argument --paper: invalid choice: 'a3' (choose from 'a4', 'letter')"},
		{[]string{"--week-start", "x"}, "argument --week-start: invalid int value: 'x'"},
		{[]string{"--note-lines", "1.5"}, "argument --note-lines: invalid int value: '1.5'"},
		{[]string{"--margin", "x"}, "argument --margin: invalid float value: 'x'"},
		{[]string{"--start", "2026-13-01"}, "argument --start: invalid fromisoformat value: '2026-13-01'"},
		{[]string{"--start", "0000-01-01"}, "argument --start: invalid fromisoformat value: '0000-01-01'"},
		{[]string{"--bogus"}, "unknown flag: --bogus"},
		{[]string{"extra"}, "unrecognized arguments: extra"},
	}
	for _, c := range cases {
		code, _, stderr := call(c.args...)
		if code != 2 || !strings.HasPrefix(stderr, "Error: ") || !strings.Contains(stderr, c.want) {
			t.Errorf("%v: exit %d, stderr %q; want exit 2 with %q", c.args, code, stderr, c.want)
		}
	}
}

func TestRunValidationErrors(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.pdf")
	notTTF := filepath.Join(t.TempDir(), "tiny.ttf")
	if err := os.WriteFile(notTTF, []byte("tiny"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--day-letters", "ABC"}, "--day-letters must have exactly 7 characters (Mon..Sun)"},
		{[]string{"--month-names", "A,B"}, "--month-names must have 12 comma-separated names"},
		{[]string{"--weekend", "a,b"}, "--weekend must be comma-separated weekday indexes"},
		{[]string{"--workday-color", "nope"}, "invalid colour 'nope'"},
		{[]string{"--font", "missing.ttf"}, "cannot read --font"},
		{[]string{"--font", notTTF}, "--font: '" + notTTF + "' is not a TrueType (.ttf) font"},
	}
	for _, c := range cases {
		code, stdout, stderr := call(append(c.args, "--start", "2026-11-07", "-o", out)...)
		if code != 1 || !strings.Contains(stderr, c.want) {
			t.Errorf("%v: exit %d, stderr %q; want exit 1 with %q", c.args, code, stderr, c.want)
		}
		if stdout != "" || strings.Contains(stderr, "get metrics") {
			t.Errorf("%v: stdout %q, stderr %q; want no stdout and no library noise", c.args, stdout, stderr)
		}
	}
}

func TestRunFontFromFile(t *testing.T) {
	font, err := filepath.Abs("../../internal/fonts/PT_Sans-Narrow-Web-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "f.pdf")
	code, _, stderr := call("--start", "2027-01-01", "--font", font,
		"--day-letters", "ПВСЧПСН", "-o", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("no PDF written: %v", err)
	}
}

func TestRunEmbeddedFontCyrillic(t *testing.T) {
	out := filepath.Join(t.TempDir(), "c.pdf")
	code, _, stderr := call("--start", "2027-01-01", "--font", "pt-sans-narrow",
		"--day-letters", "ПВСЧПСН",
		"--month-names", "СІЧ,ЛЮТ,БЕР,КВІ,ТРА,ЧЕР,ЛИП,СЕР,ВЕР,ЖОВ,ЛИС,ГРУ",
		"--title", "КАЛЕНДАР НА РІК", "-o", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestRunHelpVersionLicenses(t *testing.T) {
	code, stdout, _ := call("--help")
	for _, want := range []string{"--period PERIOD", "first day of the period", "-o OUTPUT, --output OUTPUT", "pt-sans-narrow"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Errorf("--help: exit %d, missing %q", code, want)
		}
	}
	if code, stdout, _ := call("--version"); code != 0 || !strings.HasPrefix(stdout, "yearly-calendar ") {
		t.Errorf("--version: exit %d, %q", code, stdout)
	}
	code, stdout, _ = call("--licenses")
	for _, want := range []string{"Released under the MIT License",
		"SIL OPEN FONT LICENSE", "Copyright (c) 2010, ParaType",
		"go-pdf/fpdf v0.9.0", "spf13/pflag v1.1.0", "The Go project (Go 1.24.3)"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Errorf("--licenses: exit %d, missing %q", code, want)
		}
	}
}

func TestRunNonBMPTextWithTTF(t *testing.T) {
	out := filepath.Join(t.TempDir(), "e.pdf")
	code, _, stderr := call("--start", "2027-01-01", "--font", "pt-sans-narrow",
		"--title", "😀 TEST", "--month-names", "😀,F,M,A,M,J,J,A,S,O,N,D", "-o", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if data, err := os.ReadFile(out); err != nil || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Errorf("no PDF written: %v", err)
	}
}

func checkBadFont(t *testing.T, content []byte) {
	t.Helper()
	dir := t.TempDir()
	font := filepath.Join(dir, "bad.ttf")
	out := filepath.Join(dir, "keep.pdf")
	if err := os.WriteFile(font, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("KEEP"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := call("--start", "2027-01-01", "--font", font, "-o", out)
	if code != 1 || !strings.HasPrefix(stderr, "Error: ") || !strings.Contains(stderr, "not a usable TrueType") ||
		strings.Contains(stderr, "goroutine") || strings.Contains(stderr, "panic") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if got, _ := os.ReadFile(out); string(got) != "KEEP" {
		t.Errorf("existing output modified: %q", got)
	}
}

func TestRunCorruptFonts(t *testing.T) {
	checkBadFont(t, []byte{0, 1, 0, 0})
	full, err := os.ReadFile("../../internal/fonts/PT_Sans-Narrow-Web-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	checkBadFont(t, full[:2000])
}

const examplesDir = "../../../docs/timeline/examples"

// TestRunDebugTimelineJSON runs every golden fixture through the command
// line and compares the export with the golden file (docs/timeline/05 §1).
func TestRunDebugTimelineJSON(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(examplesDir, "*.expected.json"))
	if len(files) == 0 {
		t.Fatal("no golden files")
	}
	dir := t.TempDir()
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			var want struct {
				Fixture  string         `json:"fixture"`
				Options  map[string]any `json:"options"`
				Resolved any            `json:"resolved"`
				Layout   any            `json:"layout"`
			}
			raw, err := os.ReadFile(path)
			if err == nil {
				err = json.Unmarshal(raw, &want)
			}
			if err != nil {
				t.Fatal(err)
			}
			opt := func(k string) string { s, _ := want.Options[k].(string); return s }
			start, _ := time.Parse(time.DateOnly, opt("start"))
			end, _ := time.Parse(time.DateOnly, opt("end"))
			out := filepath.Join(dir, "debug.json")
			args := []string{"--start", opt("start"), "--today", opt("today"),
				"--period", fmt.Sprintf("%dd", int(end.Sub(start).Hours()/24)+1),
				"--file", filepath.Join(examplesDir, want.Fixture),
				"--debug-timeline-json", out, "-o", filepath.Join(dir, "x.pdf")}
			if opt("layout") == "flow" {
				args = append(args, "--timeline-layout", "flow")
			}
			if c := opt("compact"); c != "" {
				args = append(args, "--timeline-compact", c)
			}
			if c, ok := want.Options["cap"].(float64); ok {
				args = append(args, "--timeline-lanes", strconv.Itoa(int(c)))
			}
			if opt("mergeGroups") == "off" {
				args = append(args, "--timeline-groups", "separate")
			}
			if code, _, stderr := call(args...); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			var got map[string]any
			raw, err = os.ReadFile(out)
			if err == nil {
				err = json.Unmarshal(raw, &got)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got["resolved"], want.Resolved) {
				t.Error("resolved differs from the golden file")
			}
			if !reflect.DeepEqual(got["layout"], want.Layout) {
				t.Error("layout differs from the golden file")
			}
		})
	}
}

func TestRunTimelineErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.dates")
	if err := os.WriteFile(bad, []byte("a = 2027-01-01\nb = @nope\nc = 2027-02-01; color=pink\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "keep.pdf")
	if err := os.WriteFile(out, []byte("KEEP"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		want []string // stderr lines, in order
	}{
		{[]string{"--file", bad}, []string{
			bad + ":2: unknown milestone @nope (only milestones declared earlier can be referenced)",
			bad + ":3: color=pink: expected one of ink, red,"}},
		{[]string{"--milestone", "a=2027-01-01", "--milestone", "b=@a/1w", "--interval", "@a"}, []string{
			"--milestone[2]: a milestone cannot contain '/'; declare intervals with --interval",
			"--interval[1]: "}},
	}
	for _, c := range cases {
		code, stdout, stderr := call(append(c.args, "--start", "2027-01-01", "-o", out)...)
		lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
		if code != 2 || stdout != "" || len(lines) != len(c.want) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 2 and %d lines", c.args, code, stdout, stderr, len(c.want))
			continue
		}
		for i, w := range c.want {
			if !strings.HasPrefix(lines[i], w) {
				t.Errorf("%v: line %d = %q; want prefix %q", c.args, i+1, lines[i], w)
			}
		}
		if got, _ := os.ReadFile(out); string(got) != "KEEP" {
			t.Errorf("%v: output written despite errors", c.args)
		}
	}
}

func TestRunTimelineOptions(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.pdf")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--timeline-layout", "grid"},
			"argument --timeline-layout: invalid choice: 'grid' (choose from 'packed', 'flow')"},
		{[]string{"--timeline-compact", "hourly"}, "argument --timeline-compact: invalid choice: 'hourly'"},
		{[]string{"--timeline-groups", "x"}, "argument --timeline-groups: invalid choice: 'x'"},
		{[]string{"--timeline-legend", "on"}, "argument --timeline-legend: invalid choice: 'on'"},
		{[]string{"--timeline-lanes", "13"}, "argument --timeline-lanes: invalid value: '13' (use auto or 1-12)"},
		{[]string{"--timeline-lanes", "0"}, "invalid value: '0'"},
		{[]string{"--today", "2027-02-30"}, "argument --today: invalid fromisoformat value: '2027-02-30'"},
	} {
		code, _, stderr := call(append(c.args, "-o", out)...)
		if code != 2 || !strings.Contains(stderr, c.want) {
			t.Errorf("%v: exit %d, stderr %q; want exit 2 with %q", c.args, code, stderr, c.want)
		}
	}

	// --timeline-compact with packed is ignored with a warning (REQ-CLI-4)
	code, _, stderr := call("--start", "2027-01-01", "--timeline-compact", "weekly", "--timeline-lanes", "12", "-o", out)
	if code != 0 || stderr != "Warning: --timeline-compact is ignored with --timeline-layout packed\n" {
		t.Errorf("compact with packed: exit %d, stderr %q", code, stderr)
	}

	// key warnings go to stderr and do not stop the run
	code, _, stderr = call("--start", "2027-01-01", "--milestone", "m=2027-02-01;every=1w;countdown=3", "-o", out)
	if code != 0 || stderr != "--milestone[1]: warning: countdown is ignored on a recurring milestone\n" {
		t.Errorf("warning: exit %d, stderr %q", code, stderr)
	}
}

func TestRunStartDefaultsToToday(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.pdf")
	code, stdout, stderr := call("--today", "2027-05-04", "--period", "10d", "--milestone", "m=today+3d", "-o", out)
	if code != 0 || !strings.Contains(stdout, ": 2027-05-04 → 2027-05-13 (10 days)") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// REQ-LG-5: a legend that does not fit ends in "+N more" and the full list
// goes to stderr; the run still succeeds.
func TestRunLegendOverflow(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 300; i++ {
		fmt.Fprintf(&b, "2027-%02d-%02d/3d; title=\"Task %d\"\n", i%12+1, i%27+1, i)
	}
	file := filepath.Join(dir, "many.dates")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := call("--start", "2027-01-01", "--paper", "letter", "--file", file, "-o", filepath.Join(dir, "x.pdf"))
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if code != 0 || lines[0] != "Warning: the legend does not fit the page; all entries:" || len(lines) != 301 {
		t.Errorf("exit %d, %d stderr lines, first %q", code, len(lines), lines[0])
	}
}
