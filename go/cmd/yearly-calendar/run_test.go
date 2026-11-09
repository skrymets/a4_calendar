package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if code, stdout, _ := call("--licenses"); code != 0 || !strings.Contains(stdout, "SIL OPEN FONT LICENSE") {
		t.Errorf("--licenses: exit %d", code)
	}
}
