# Design: configurable period length (`--period`)

## Context

`yearly_calendar.py` always renders exactly one year from `--start`
(`add_one_year(start) - 1 day`). The user wants that to remain the default
and the hard maximum, while also allowing shorter sheets: the next N days,
weeks or months.

Decisions made during brainstorming:
- One option with a unit suffix: `--period 90d | 12w | 6m | 1y` (default `1y`).
- Going over one year is an **error**, not a clamp.
- Units are **exact lengths from start**, not aligned to calendar weeks or months.
- Fewer months mean fewer columns, which **stretch to the page width** (current behaviour, no layout change).
- The default title **names the period** (`90-DAY CALENDAR`). `--title` still overrides it, and an over-long title is **truncated with `...`**.
- The default filename **appends the period** when it is not `1y`.

Only `yearly_calendar.py` changes (plus the local, uncommitted `CLAUDE.md` notes).

## 1. Period parsing and end date

**`parse_period(text) -> (n, unit)`**: used as the argparse `type` for `--period`.
- Regex `^(\d+)([dwmy])$`, case-insensitive, surrounding whitespace stripped. Requires `n >= 1`.
- Anything else raises `argparse.ArgumentTypeError`, e.g. `invalid period '90': use <N>d, <N>w, <N>m or 1y`.

**`add_months(d, n)`**: replaces `add_one_year`.
- Returns the same day-of-month `n` months later.
- When that day doesn't exist, it rolls forward to the 1st of the next month. This is the same rule `add_one_year` uses today for Feb 29, so `add_one_year(d) == add_months(d, 12)` and `add_one_year` can be removed.

**`period_end(start, n, unit) -> end`** (inclusive):
- `d`: `start + (n-1) days`
- `w`: `start + (7n-1) days`
- `m`: `add_months(start, n) - 1 day`
- `y`: `add_months(start, 12n) - 1 day`

Edge case: start Jan 31 with `1m` gives Mar 1 − 1 = **Feb 28** (the whole of February). The brainstorming preview showed Feb 27 under a clamp rule. I picked roll-forward so that `12m` and `1y` agree, and so that a month-end start covers the next month in full.

**Max check**: `parse_args` runs it after parsing, because it needs `--start`. If `end > add_months(start, 12) - 1 day`, it calls `p.error(...)`:
`--period 53w (371 days) exceeds the 1-year maximum (365 days from 2026-11-07)`.
This covers `2y`, `13m`, `53w` and `400d`. A `366d` period is accepted only when the year from start actually has 366 days.

## 2. Effects on the rest of `build()`

- `end = period_end(start, *args.period)` replaces line 112. `total_days`, `months_between` (column count), `multi_year`, week numbers, footer and the in-range shading all already derive from `start`/`end`, so they need no change.
- **Period label** (line 144): it currently shows `"2026"` whenever start is Jan 1. The new condition is start == Jan 1 **and** end == Dec 31 of the same year; otherwise it shows the date range. Without this, a Jan-1 `3m` sheet would be labelled `2026`.
- Column width and header font: unchanged. `hdr_size` already picks the larger size for `n_cols <= 12`.

## 3. Title

- `--title` default becomes `None`. When it isn't given, the title is resolved as follows:
  - `1y` → `YEARLY CALENDAR`
  - `Nd`/`Nw`/`Nm` → `f"{n}-{DAY|WEEK|MONTH} CALENDAR"`
  - Only a literal `1y` counts as "yearly"; `12m` gives `12-MONTH CALENDAR`.
- **Truncation**: factor the width formula out of `draw_spaced` into `spaced_width(text, font, size, spacing)`. Compute the label width first. Available title width = `grid_w - label_w - 12pt` gap. If the title is wider, drop characters from the end until `title.rstrip() + "..."` fits. Plain `...` is used because a custom TTF may not have `…`.
- The PDF metadata title (`c.setTitle`) keeps the full, untruncated title.

## 4. Output name, help, docs

- Default `-o`: `calendar_<start>.pdf` for `1y`, otherwise `calendar_<start>_<n><unit>.pdf` (normalised lowercase, e.g. `_90d`).
- `--period` help text: `length of the period: <N>d, <N>w, <N>m or 1y (default 1y, max 1 year)`.
- Update the module docstring (first paragraph plus a `--period` example) and the `CLAUDE.md` "Period model" bullet. `CLAUDE.md` is never committed.

## Process steps after approval

1. Write this design to `docs/superpowers/specs/2026-11-09-period-length-design.md`.
2. Implement the changes in `yearly_calendar.py`.
3. Verify (below).
4. Offer a commit message. Per `CLAUDE.md`: no auto-push, no co-author trailer, never stage `CLAUDE.md`/`.claude/`.

## Verification

Run with `.\.venv\Scripts\python.exe`, outputs to the scratchpad dir:
- Regression: `--start 2026-01-01` and `--start 2026-11-07` must produce the same layout as before (label `2026` / range, 12 / 13 cols, title `YEARLY CALENDAR`, same filenames).
- New periods: `--start 2026-11-07 --period 90d` (ends 2027-02-04, 4 cols, `90-DAY CALENDAR`), `--period 2w` (ends 2026-11-20, 1 wide col), `--period 6m`, `--period 1d`, `--start 2026-01-01 --period 3m` (label shows a range, not `2026`). Check the printed `Wrote …: start → end (N days)` line, then inspect the PDFs visually.
- Edge cases via `python -c` on `period_end`: Jan 31 + 1m → Feb 28; Feb 29 2028 + 1y → Feb 28 2029; 366d from 2026-03-01 is an error; 366d from 2027-03-01 or 2027-11-07 (both span Feb 29 2028) is OK.
- Errors: `--period 53w`, `2y`, `13m`, `0d`, `90`, `abc` each exit with a clear argparse message.
- Truncation: `--title "A VERY LONG CUSTOM TITLE THAT WILL NOT FIT NEXT TO THE DATE RANGE LABEL"` ends in `...` and does not overlap the label. Check this with both `--header compact` and `--header full`.
