# Release notes

Versions follow [Semantic Versioning](DEV.md#versioning). While the version is `0.x`, a
MINOR release may change behaviour you rely on, and says so under **Changed**.

Each release has a section headed by its bare version (`## 0.1.0`). The release workflow
publishes that section as the GitHub release text, and refuses a tag whose version has none.

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
