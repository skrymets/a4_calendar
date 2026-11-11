# 05 · Acceptance (normative)

The feature is done when every check below passes in **both** implementations.

## 1. Golden layout tests (automated)

For each row, run the calendar with the fixture and options, export the **layout decisions** as JSON in the shape below, and compare with `examples/<expected>.expected.json`. Compare the `resolved` and `layout` objects field by field; key order and whitespace do not matter.

| Fixture | Options (all with `--today 2026-11-11`) | Expected file |
|---|---|---|
| exams.dates | `--start 2026-11-11 --period 120d` | exams |
| release.dates | `--start 2027-01-01 --period 5m` | release |
| release.dates | same + `--timeline-layout flow` | release.flow |
| release.dates | same + `--timeline-layout flow --timeline-compact weekly` | release.flow-weekly |
| year-plan.dates | `--start 2026-11-11 --period 1y` | year-plan |
| leave-2027.dates | `--start 2027-01-01` | leave-2027 |
| x01 … x23 | `--start 2027-01-01 --period 5m` unless listed | x… |
| x13-adaptive.dates | + `--timeline-lanes 8` | x13-adaptive |
| x20-groups.dates | `--start 2027-01-01` | x20-groups |
| x20-groups.dates | `--start 2027-01-01 --timeline-groups separate` | x20-groups.unmerged |
| x23-clipping.dates | `--start 2027-01-15 --period 5m` | x23-clipping |

Golden JSON shape (one file per row):

```json
{
 "resolved": {
  "milestones": [{"id": "tutor", "date": "2026-11-19", "occurrence": "1/15"}],
  "intervals":  [{"id": "labs", "key": "2", "start": "2026-11-16", "endExclusive": "2026-11-18",
                  "lastDrawnDay": "2026-11-17", "occurrence": "1/8"}]
 },
 "layout": {
  "geometry": "wide", "laneCap": 6, "groupsMerged": false, "layout": "packed",
  "columns": [{"month": "NOV '26", "railLanes": 2, "labelMode": "text",
               "labels": [{"day": 11, "milestone": "◆ Kick-off"}, {"day": 12, "items": ["THEORY · 35 D"]}]}]
 }
}
```

`railLanes` is `k` (lanes shown + overflow rail). `items` lists the top-row texts of that day in drawing order (labels, keys, `+N`). `milestone` is the glyphs plus the title shown, if any.

Implementations SHOULD expose this export behind a hidden flag (for example `--debug-timeline-json PATH`) so the same tests run against Go and Python.

## 2. Unit tests (automated)

| Area | Cases |
|---|---|
| Lane packing | priority order; a unit occupies only its spans; series split when occurrences overlap; cap and overflow; group units with gaps |
| Flow | daily slide; weekly compaction on `--week-start`; sticky overflow; no jog on day 1; per-day zone width |
| Labels | first free day skips milestone days; `(CONT.)`; series `#n`; `label=each/none`; tentative `?`; `length` days and workdays (with a custom `--weekend`) |
| Keys | declaration numbering incl. anonymous intervals; column-top keys only on wide pages and not for series; member keys in member colour |
| Milestones | several per day; ring + glyph combination; countdown clipped by period start; countdown ignored on series (warning) |
| Legend | in-period filtering; series `every/gap … ×N`; group lines; 4-column fill; REQ-LG-5 fallbacks |
| Validation | each REQ-KEY value error; unknown key ignored; `--timeline-compact` with `packed` warns |
| Columns | 1, 5, 7 (wide, cap 4), 8 (narrow), 12, 13 columns; Letter vs A4 widths change only the wide/narrow class |
| Dates | leap year (`2028-01-31` + `every=1m`), period starting mid-month, interval ending exactly on the period end |
| No timeline | output identical to the previous release (REQ-CLI-8) |

## 3. Visual checks (manual, per release)

Render the six full pages from 04 in Go and in Python and compare with `img/page-*.png` side by side:

1. Label block, header, weekend shading and fonts are pixel-identical to a page without a timeline.
2. No cell is tinted; only weekends are grey; hatch appears only where `style=hatch`.
3. Every rail stays inside its column's rail zone; no rail touches text.
4. Every drawn interval is traceable: label, or key + legend entry.
5. Caps are visible at every start and end inside the period; continuations are flush.
6. Flow: diagonals only at the top of a cell, never on day 1 of a column.
7. Printed in greyscale at 100 %, rails and titles remain distinguishable from the dotted note line.

## 4. Requirement coverage

| Requirement group | Covered by |
|---|---|
| REQ-CLI-* | unit tests, golden option rows |
| REQ-RES-* | golden `resolved` sections, DSL conformance tests |
| REQ-KEY-* | x01–x10, x14, x15, validation tests |
| REQ-MS-* | x01, x03, x15, exams |
| REQ-IV-* | x04, x05, x07–x10, x23 |
| REQ-PK-* | x11–x14, release, year-plan |
| REQ-FL-* | x21, x22, release.flow*, year-plan flow (visual) |
| REQ-GR-* | x20 (both), year-plan, leave-2027 |
| REQ-LB-* | x11, x12, x15, x16, all golden files |
| REQ-LG-* | full pages, legend unit tests |
| REQ-PG-*, REQ-ER-*, REQ-DT-* | visual checks, error tests, Go ↔ Python golden comparison |
