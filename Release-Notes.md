# Release notes

Versions follow [Semantic Versioning](DEV.md#versioning). While the version is `0.x`, a
MINOR release may change behaviour you rely on, and says so under **Changed**.

Each release has a section headed by its bare version (`## 0.1.0`). The release workflow
publishes that section as the GitHub release text, and refuses a tag whose version has none.

## 1.0.0

The first stable release, and the first under the name **PinPlan**: *Plan it. Print it.
Pin it.*

### Added

- **Milestones and intervals** on the calendar, from `.dates` files (`--file`) or inline
  (`--milestone`, `--interval`), written in a small date language: fixed dates, `today`,
  durations, repeats (`every`, `gap`, `count`, `until`), and references with offsets to
  earlier milestones (`@exam-2w`), to an interval's first or last day (`@qa.end+1d`) and to
  one occurrence of a repeat (`@review[3]`, `@review[-1]`). Offsets chain
  (`@release-1m+3d`) and durations may have several parts (`2027-08-01/1m+13d`).
  `--today` sets the evaluation date and is the default `--start`.
  - An interval includes both dates written (`2027-08-13/2027-09-05` ends on 5 September);
    `<` before the end leaves it out (`2w/<@exam` ends the day before the exam). An
    interval may be a single day (`2027-11-17/2027-11-17`), drawn one cell long.
  - Milestones: a mark (diamond, star, dot, or a ring around the day number), a title on
    the note line, and an optional countdown.
  - Intervals: colored rails at the right edge of the cells, solid, dashed or hatched,
    labeled or numbered; margin lines for background spans; merged groups on crowded
    pages; an overflow rail with `+N` when a column is full.
  - A legend under the grid, in date order and read column by column; interval keys are
    numbered in that order. The rows give up the room it needs. The footer names the
    timeline sources, and the PDF Subject lists them.
  - Options: `--timeline-layout packed|flow`, `--timeline-compact daily|weekly`,
    `--timeline-lanes`, `--timeline-groups auto|merge|separate`, `--timeline-legend full|off`.
  - Errors in declarations stop the run with one `file:line: message` per problem (exit 2).
- Without milestones or intervals the page is exactly as before.
- **`--end` and `--until`** give the period by its last day, or the day after it, instead of
  `--period`. The title and file name follow the shortest matching length
  (`--end 2027-06-30` from 1 January: `6-MONTH CALENDAR`, `calendar_2027-01-01_6m.pdf`).
- **Date expressions in `--start`, `--end` and `--until`**: `today+1w`, `@kickoff`,
  `@exam+1w`, `@qa.end`; the page follows the plan. `@today` is another spelling of `today`,
  in options and declarations alike.
- **All options in one file**: a line starting with `--` in a `.dates` file sets an option
  as on the command line (`--start = @kickoff`, `--paper letter`), and `--file` in a file
  includes another at that line. Paths are relative to the file; the command line wins;
  an option set twice in files is an error. `pinplan plan.dates` reads a file
  without `--file`.
- **An icon**: `pinplan.exe` shows the PinPlan calendar icon in Explorer and the taskbar.

### Changed

- **The program is now called PinPlan.** The command is `pinplan` (`pinplan.exe` on
  Windows), and the release archives are `pinplan-<version>-<platform>`. `--version` and
  the PDF Creator metadata say `PinPlan <version>` (they said
  `yearly-calendar <version> (Go)`). The options and the default PDF name
  (`calendar_<start>.pdf`) are unchanged.

### Removed

- **The Python version** (`py/yearly_calendar.py`) and its release archive
  (`yearly-calendar-py-<version>.zip`). The Go binary prints the same page with the same
  options. The Python/Go comparison tool (`tools/parity.py`) is gone with it.

## 0.1.0

First release.

### Added

- **A one-page calendar PDF**, A4 or US Letter (`--paper`).
  - A column per month and a row per day, with weekends shaded.
  - Each day shows the day number with a coloured weekday letter and a dotted note guide.
  - Above them in small type: the day of the period and the week number.
- **Any period up to a year.** `--period` takes `<N>d`, `<N>w`, `<N>m` or `1y` (the default)
  from `--start` (default today). Longer periods are refused.
- Days outside the period, in a month the period touches, are drawn faint or hidden
  (`--hide-outside`).
- **Week numbering** relative to the start date, ISO-8601 or in rolling 7-day blocks
  (`--week-mode`, `--week-start`, `--week-labels`).
- A compact or full header (`--header`).
- A title that follows the period (`YEARLY CALENDAR`, `90-DAY CALENDAR`). A custom
  `--title` too long to fit is cut with `...`.
- Settings for:
  - note lines: `--note-lines`, `--no-note-guide`;
  - weekend days: `--weekend`;
  - colours: `--workday-color`, `--weekend-color`;
  - margin: `--margin`;
  - footer: `--no-footer`.
- **Other languages**: weekday letters (`--day-letters`), month names (`--month-names`) and a
  TrueType font (`--font`).
- **Two implementations** with the same options and output:
  - the **Go binary** for Windows x64, Linux x64 and Linux ARM64, which needs nothing installed;
  - the **Python script**, which needs ReportLab.
- The Go binary has PT Sans Narrow built in (`--font pt-sans-narrow`), which covers Cyrillic
  without a font file. `--licenses` prints the licences of the embedded font and libraries.
- `--version` in both implementations, and the version in every PDF's metadata (Creator:
  `yearly-calendar 0.1.0 (Go)` or `(Python)`).
- Release archives for each platform and for the Python script, each with a SHA-256
  checksum.
- Open source under the MIT License (`LICENSE`, included in every release archive).
