"""Builds docs/timeline/04-scenarios.md from the fixtures, so every snippet shown is the exact input of its picture."""
import pathlib
ROOT = pathlib.Path(__file__).resolve().parent
DOCS = ROOT.parent
EX = DOCS / "examples"

H1 = "`--start 2027-01-01 --period 5m` (5 columns, wide)"
YEAR = "`--start 2027-01-01` (12 columns, narrow)"
S = [
  ("Milestones", None),
  ("x01-marks", "Marks", "x01-marks.dates", H1, "March 1–6. Each mark sits after the weekday letter; `ring` circles the number instead. Two milestones on 5 Mar: both glyphs, one title + ` +1`.", "REQ-KEY-3, REQ-MS-1, -2, -4"),
  ("x02-colors", "Named colours", "x02-colors.dates", H1, "Every palette name plus a `#RRGGBB`, as milestone titles on white and weekend grey.", "REQ-KEY-2, 02 §4"),
  ("x03-countdown", "Countdown", "x03-countdown.dates", H1, "`countdown=5` writes T−5 … T−1 top-left in the five days before the exam.", "REQ-KEY-4, REQ-MS-5"),
  ("x04-anchor", "Hidden anchor and back-to-back intervals", "x04-anchor.dates", H1, "`term` resolves but is not drawn (no mark on 8 Mar). `before` ends the day `after` starts; half-open ranges never share a cell, so they reuse one lane with a visible gap between the caps.", "REQ-KEY-11, REQ-IV-3"),
  ("Intervals", None),
  ("x05-interval", "One interval", "x05-interval.dates", H1, "`2027-03-03/2027-03-10` is drawn 3–9 Mar. Rounded caps 3 pt inside the first and last cell. Label on the first day: title + ` · 7 d` from `length=days`.", "REQ-RES-3, REQ-IV-1, -2, REQ-KEY-7"),
  ("x07-continuation", "Across a month boundary", "x07-continuation.dates", H1, "Flat ends at the column break; February shows `PROJECT · 31 D`, March `PROJECT (CONT.)` on its first free day.", "REQ-IV-4, REQ-LB-3"),
  ("x08-dashed", "Dashed and tentative", "x08-dashed.dates", H1, "`style=dashed` breaks the rail. `status=tentative` does the same and appends ` ?` to the label and legend.", "REQ-KEY-5, -6, REQ-IV-6"),
  ("x09-hatch", "Hatch", "x09-hatch.dates", H1, "Holidays hatch the covered cells, over weekend grey too; no lane is used, so `Work` keeps lane 0. Hatch is the only cell-wide pattern.", "REQ-IV-7, -8"),
  ("x10-back", "Background layer", "x10-back.dates", H1, "`layer=back` intervals become margin lines between the date and the note line: the first solid, the second dotted. No lane and no fill; `Feature` alone uses the rail zone.", "REQ-KEY-8, REQ-IV-10"),
  ("Packing", None),
  ("x11-overlap", "Overlap and first free day", "x11-overlap.dates", H1, "Three overlapping intervals take lanes 0–2. `Test` starts on 8 Mar, which has a milestone, so its label moves to 9 Mar. Three lanes or fewer: text labels.", "REQ-PK-1, -2, REQ-LB-1, -2"),
  ("x12-keys", "Keys instead of labels", "x12-keys.dates", H1, "A fourth lane switches the column to keys: the declaration number on each start day; the legend explains them.", "REQ-LB-1, -4, REQ-LG-2"),
  ("x13-adaptive", "Adaptive rail width", "x13-adaptive.dates", H1 + " and `--timeline-lanes 8`", "January has 2 lanes (2.25 pt rails), February 5 (1.5 pt), March 8 (1.1 pt). Each column decides alone, so January keeps a wide note area.", "REQ-PK-5, 02 §3"),
  ("x14-overflow", "Lane cap, overflow, priority", "x14-overflow.dates", H1, "Eight intervals, cap 6. `Security` has `priority=high`, so it is packed first and keeps lane 0; the two latest normal ones (Ops, Docs) share the dotted overflow rail. `+1`, `+2`, `+1` mark where the overflow count changes; their keys still mark their start days.", "REQ-KEY-9, REQ-PK-3, -4"),
  ("Recurrence", None),
  ("x15-recurring-milestone", "Recurring milestones", "x15-recurring-milestone.dates", H1, "Weekly `Team sync`: a dot every Monday, title only on the first one in each month (`label=first`). Monthly `Payroll` with `label=each`: title every time. On 25 Jan both fall on one day: two glyphs, the title that is due (Payroll) + ` +1`.", "REQ-KEY-12, REQ-MS-4, REQ-LB-3"),
  ("x16-recurring-interval", "Recurring interval keeps one lane", "x16-recurring-interval.dates", H1, "`every=2w; count=6`: six 2-day labs in one lane next to the course. `LAB #1` on the first occurrence in January, `#2` after; February starts again with `LAB #3`.", "REQ-PK-1 (series unit), REQ-LB-3"),
  ("x17-gap", "Gap-based recurrence", "x17-gap.dates", H1, "`/3d; gap=2d`: 3 days on, 2 off, independent of weekdays.", "DSL §6.4"),
  ("x18-monthly", "Monthly recurrence clamps", "x18-monthly.dates", H1, "From 31 Jan every month: 28 Feb, 31 Mar, 30 Apr, 31 May — always computed from the original date, so no drift.", "DSL §6.7"),
  ("x19-overlapping-series", "Series whose occurrences overlap", "x19-overlapping-series.dates", H1, "10-day reviews every week overlap; the series splits into sub-units on two adjacent lanes that alternate.", "03 §3"),
  ("Narrow pages and groups", None),
  ("x20-groups-merged", "Groups merged (narrow default)", "x20-groups.dates", YEAR, "People out (`P`) is one rail: dashed while one person is away, thicker where two overlap, ticks where a member starts or ends inside the run. Security (`S`) breaks between audit and pen test. Member keys appear on their start days.", "REQ-GR-1 … -4, REQ-LB-5"),
  ("x20-groups-unmerged", "Same, `--timeline-groups separate`", "x20-groups.dates", YEAR, "Without merging March needs 4 units with a cap of 3: an overflow rail and `+1` appear.", "REQ-GR-5, REQ-PK-3"),
  ("Flow layout", None),
  ("x21-packed", "Packed (default) — for comparison", "release.dates", H1 + ", 15–28 Mar", "Lanes never move; when an inner interval ends its lane stays empty until reused.", "REQ-PK-1"),
  ("x21-flow", "Flow, daily", "release.dates", H1 + " and `--timeline-layout flow`", "Rails outside an ended one slide inward the next day with a short diagonal; the note line of each day ends at that day's outermost rail.", "REQ-FL-1 … -4"),
  ("x22-flow-weekly", "Flow, weekly compaction", "release.dates", H1 + " and `--timeline-layout flow --timeline-compact weekly`", "Gaps stay until Monday (15 and 22 Mar), then everything closes up at once: fewer diagonals.", "REQ-FL-2"),
  ("Period edges", None),
  ("x23-clipping", "Clipped by the period", "x23-clipping.dates", "`--start 2027-01-15 --period 5m` (6 columns)", "An interval that began before the period starts flush on 15 Jan; one that ends after it stops flush on 14 Jun. Lengths and legend use the full intervals. Out-of-period days stay faint and empty.", "REQ-IV-5, REQ-PG-2"),
]

out = ["# 04 · Scenarios (informative)", "",
       "Every picture is rendered from the `.dates` file shown with it (`examples/`). Crops are zoomed 2–2.5×. "
       "Expected layout decisions for each are in `examples/<name>.expected.json` (see 05).", ""]
toc = []
for row in S:
    if row[1] is None:
        out += ["", "## " + row[0], ""]; toc.append(row[0]); continue
    pid, title, fx, opts, notes, reqs = row
    text = (EX / fx).read_text().rstrip()
    if fx == "release.dates":
        text = "# see examples/release.dates (full file, 3 milestones and 17 intervals)"
    out += ["### " + title, "", "*Options:* " + opts + " · *Requirements:* " + reqs, "", "```text", text, "```", "",
            "![" + title + "](img/" + pid + ".png)", "", notes, ""]
out += ["", "## Full pages", "",
        "| Picture | Input | Options |", "|---|---|---|",
        "| ![](img/page-exams.png) | `exams.dates` | `--start 2026-11-11 --period 120d --title \"Prepare for Exams\" --today 2026-11-11` |",
        "| ![](img/page-release-packed.png) | `release.dates` | `--start 2027-01-01 --period 5m` |",
        "| ![](img/page-release-flow.png) | `release.dates` | same + `--timeline-layout flow` |",
        "| ![](img/page-year-packed.png) | `year-plan.dates` | `--start 2026-11-11 --period 1y` (13 columns; groups merged by default) |",
        "| ![](img/page-year-flow.png) | `year-plan.dates` | same + `--timeline-layout flow` |",
        "| ![](img/page-leave.png) | `leave-2027.dates` | `--start 2027-01-01` (12 columns) |", ""]
(DOCS / "04-scenarios.md").write_text("\n".join(out))
print("written", len(S))
