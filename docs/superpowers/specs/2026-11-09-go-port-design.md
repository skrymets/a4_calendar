# Design: Go standalone binary port of `yearly_calendar.py`

## Context

`yearly_calendar.py` needs a Python environment with ReportLab installed. The goal is a standalone, cross-platform binary, built first for Windows and Linux, with the **same CLI behaviour and the same visual output**. The Python script stays as the reference implementation and the parity oracle.

Decisions from brainstorming:
- **Language:** Go. Go 1.24.3 is installed, and nothing requires Rust.
- **Layout:** Go module in `go/`, next to the untouched `yearly_calendar.py`.
- **CLI parity:** flags, defaults, choices, validation, error meaning, exit codes and the `Wrote …` line are identical. `--help` lists the same options and descriptions in pflag's layout; it is not a byte-for-byte argparse clone.
- **PDF backend:** `github.com/go-pdf/fpdf`, pinned to its last tag. It is MIT-licensed and pure Go. Its repo was archived in 2025, so it is frozen; the risk is contained by putting it behind a small `Canvas` interface.
- **Embedded font:** PT Sans Narrow (SIL OFL 1.1), as a built-in alternative selected with `--font pt-sans-narrow`. The default stays built-in Helvetica, the same as Python.
- **Build:** a local cross-compile script. A GitHub Actions + GoReleaser release flow is the **planned next improvement** and out of scope here.

## 1. Layout and units

```
go/
  go.mod                         module a4calendar (Go 1.24); deps: go-pdf/fpdf, spf13/pflag
  cmd/yearly-calendar/main.go    flags → validation → render → "Wrote …"
  internal/period/               pure date logic (unit-tested)
  internal/render/               port of build(): layout + drawing via Canvas
  internal/canvas/               Canvas interface + fpdf implementation
  internal/fonts/                PT_Sans-Narrow-Web-Regular.ttf + OFL.txt (upstream file name)
  build.ps1                      cross-compile into go/dist/
tools/parity.py                  Python vs Go visual/text parity check (dev only)
```

**`internal/period`**: a 1:1 port of the logic functions in `yearly_calendar.py`. It uses `time.Time` at UTC midnight as a date (via a small `Date` helper) and depends only on the standard library.
- `ParsePeriod(s) (n int, unit byte, err)`: regex `^(\d+)([dwmy])$` after lowercase/trim, with `n >= 1`. The error text matches Python's.
- `AddMonths(d, n)`: rolls a missing day forward to the 1st of the next month.
- `PeriodEnd(start, n, unit)`. `MaxEnd(start) = AddMonths(start,12) - 1d`.
- `WeekNumber(day, start, mode, weekStart)` for relative, iso (`time.ISOWeek`) and rolling.
- `MonthsBetween(start, end)`.

**`internal/canvas`**: the `Canvas` interface uses **PDF-native coordinates (bottom-left origin, points)**, so the render code is a line-by-line port of the Python.

```go
type Canvas interface {
    SetFillColor(c RGB); SetStrokeColor(c RGB); SetLineWidth(w float64)
    SetDash(on, off float64); ClearDash(); SetRoundCap(bool)
    Rect(x, y, w, h float64)                 // filled, no stroke: the only rect style build() uses
    Line(x1, y1, x2, y2 float64)
    SetFont(f Font, size float64)
    DrawString(x, y float64, s string)        // like c.drawString
    DrawSpaced(x, y float64, s string, spacing float64) // Tc letter-spacing, reset to 0 after
    StringWidth(s string, f Font, size float64) float64
    SetTitle(s string)
    // Save lives on *canvas.FPDF; render never saves
}
```

The fpdf implementation (`fpdfCanvas`) works like this:
- **Page:** `fpdf.NewCustom` with unit `pt` and the ReportLab page sizes. A4 is 595.2756 × 841.8898; Letter is 612 × 792.
- **Coordinates:** flips y with `pageH - y`. `Rect` converts the bottom y to a top y.
- **Fonts:**
  - Default: core font `Helvetica`, non-embedded with WinAnsi encoding, the same as ReportLab. Strings go through `UnicodeTranslatorFromDescriptor("")`, so `–` and `·` encode as cp1252.
  - TTF: `AddUTF8Font` for a file path, and `AddUTF8FontFromBytes` for the embedded PT Sans Narrow.
- **Letter-spacing:** `RawWriteStr("<s> Tc")` before `Text`, then `0 Tc` after it. This matches `draw_spaced`. Fallback if the raw op misbehaves: draw glyph by glyph, advancing by `width(ch) + spacing`.
- **Dashes and caps:** set and reset explicitly with `SetDashPattern`/`SetLineCapStyle`, not with q/Q. fpdf caches graphics state, so a raw restore would desync it. This replaces Python's `saveState`/`restoreState` around the guides.

**`internal/render`**: `Build(cfg Config, c Canvas) (start, end, totalDays)` is a direct port of `build()`, including:
- the styling constants (`INK` … `DIVIDER`, `MM`, `TITLE_SIZE`, `HEADER_H`, `MAIN_SIZE`, `SMALL_SIZE`, `CELL_PAD`);
- `fit_text` and the title truncation;
- `guide_x0_off`, computed from the widest label;
- the compact/full header, multi-year markers, faint outside days, the `show_week` condition, and the footer.

`Config` is plain data filled by `main`.

**`internal/fonts`**: holds `PTSansNarrow-Regular.ttf` and `OFL.txt`, both from `github.com/google/fonts/tree/main/ofl/ptsansnarrow`, embedded with `go:embed`.

## 2. CLI (`cmd/yearly-calendar`)

The binary is named `yearly-calendar` (`.exe` on Windows). It uses `spf13/pflag` for GNU-style `--long` options and the `-o` shorthand.

All Python options keep the same names, defaults, choices and help text: `--start`, `--period`, `-o/--output`, `--week-mode`, `--week-start`, `--week-labels`, `--weekend`, `--title`, `--day-letters`, `--month-names`, `--font`, `--paper`, `--no-footer`, `--no-note-guide`, `--note-lines`, `--workday-color`, `--weekend-color`, `--margin`, `--header`, `--hide-outside`.

Behaviour matches Python as follows:
- **`--start`:** `YYYY-MM-DD`, default today in local time. Python 3.12's `fromisoformat` also accepts compact forms; Go accepts only `YYYY-MM-DD`, which is documented.
- **Usage errors:** bad choice, bad period, period over one year, bad date or bad number print `Error: …` to stderr and exit **2**. The messages match Python's wording.
- **Validation errors:** `--day-letters` not 7 characters, or `--month-names` not 12 names, print the same messages as Python and exit **1**, like Python's `sys.exit(msg)`.
- **`--weekend` and colours:** a bad `--weekend` gives a clean error, where Python shows a traceback. Colours accept `#RRGGBB` or `0xRRGGBB`, like ReportLab's `HexColor`.
- **Defaults:** default title, default filename (`calendar_<start>[_<period>].pdf`) and the max check behave as in Python.
- **`--font`:** a TTF path, or the keyword `pt-sans-narrow` for the embedded font. The keyword is new; Python only accepts paths.
- **New flags:** `--version`, stamped via `-ldflags`, and `--licenses`, which prints the PT Sans Narrow copyright notice and OFL text. The OFL requires that notice to ship with the binary.
- **Success:** prints `Wrote <path>: <start> → <end> (<n> days)` and exits 0.

## 3. Build

`go/build.ps1` runs `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<git describe>"` for:
- windows/amd64
- linux/amd64
- linux/arm64

It writes `go/dist/yearly-calendar-<os>-<arch>[.exe]` and copies `OFL.txt` into `dist/`. `go/dist/` is added to `.gitignore`.

## 4. Testing and parity

- **Go unit tests:**
  - `internal/period`: table-driven, reusing every case already verified in Python (Jan 31 + 1m → Feb 28; Feb 29 2028 + 1y → Feb 28 2029; 366d from 2026-03-01 is an error and from 2027-03-01 is OK; 53w/13m/2y errors; ISO/relative/rolling week numbers at year edges).
  - `render`: `FitText`, and the default title and filename.
  - `cmd`: flag parsing and exit codes, via a `run(args, stdout, stderr) int` function so `main` stays thin.
- **Parity check, `tools/parity.py`:**
  - Runs both implementations over the same case matrix with `--start` always given: Jan-1 year, mid-year, `90d`, `2w`, `--period 1d`, iso, rolling, `--week-labels all`, `--header full`, `--paper letter`, `--hide-outside`, `--note-lines 3`, `--no-footer`, `--no-note-guide`, custom colours, a long `--title`, and a Cyrillic case with `--font C:\Windows\Fonts\arial.ttf`.
  - Rasterises both outputs with `pdftoppm -r 150`, which is installed via MiKTeX.
  - Compares the PPM pixels in pure Python, so no Pillow is needed.
  - Reports the share of differing pixels (channel delta > 48). It fails above a small tolerance for anti-aliasing from coordinate rounding; the threshold is tuned once and recorded. It writes diff images to a temp dir.
  - If `pdftotext` is available, it also compares the extracted text exactly.
- **Manual check:** look at the Go PDFs with the Read tool, including `--font pt-sans-narrow` with Cyrillic day letters and month names, which has no Python counterpart.

## 5. Out of scope / next improvements

- GitHub Actions + GoReleaser release workflow (tag → binaries), once the repo has a remote.
- macOS builds; adding them is trivial later.
- `--font pt-sans-narrow` in the Python script.

## Process after approval

1. Write this design to `docs/superpowers/specs/2026-11-09-go-port-design.md` and offer a commit message (no auto-commit/push, no co-author trailer, `CLAUDE.md` excluded).
2. The user reviews the spec.
3. Invoke `superpowers:writing-plans` for the step-by-step implementation plan.
4. Implement, run the unit tests and `tools/parity.py`, update the local `CLAUDE.md` with the Go commands.
