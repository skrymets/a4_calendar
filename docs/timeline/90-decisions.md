# 90 · Decisions and rejected options (informative)

## Chosen

| Decision | Why |
|---|---|
| Rails on the right edge, milestones on the left | Keeps the writing space in one piece and the existing label block untouched. |
| Per-column lanes | A busy March no longer narrows a quiet January. Lanes are recomputed per month, so a long interval may change lane at a month boundary; the flat continuation ends make that readable. |
| Adaptive rail width + lane cap + overflow rail | Bounded rail zone on every page size; nothing disappears (keys and legend). |
| Keys when `k > 3` or on narrow pages | Text labels need ~60 % of a wide column; at 4+ lanes there is no room. |
| `layer=back` as a margin line | Long, low-attention spans (OKR periods, quarters) need no lane. A tint was tried and rejected (below). |
| Group merge on narrow pages only | On 5-column pages members are better seen separately; on 13-column pages merging is what makes September fit. |
| Flow layout as an option | Gives back writing space on quiet days and a compact outline; harder to follow for intervals that move several times, so not the default. |
| `length=` instead of `count=` | `count` is a reserved recurrence key in the DSL. |
| Series in one lane, `#n` labels | A series reads as one stripe; the title is not repeated every week. |

## Rejected

| Option | Why not |
|---|---|
| Global lanes (one lane count for the whole page) | Every column loses the busiest day's width; at 13 columns rails cover the day numbers. |
| Tinting cells (`style=band`, tinted back layer) | Mixes with weekend shading; the page stops looking like the original calendar. |
| Hairline rails (1 px at 3 px pitch) for everything | Densest, but colours blur and dashed styles vanish; kept only as the last step of adaptive width. |
| Horizontal tracks (ticks stacked in the cell) | Fixed width, but reads as a ladder; continuity is weaker than a rail. |
| Density rail (darkness = count) | Smallest, but no identity without the legend. |
| Two hatch directions for background layers | Muddy with weekends; replaced by margin lines. |

The design canvas (Design artifact "Milestones & Intervals") shows each rejected option on the same data.

## Implementation decisions (Go, not stated elsewhere)

| Decision | Why |
|---|---|
| Go only; the Python version was retired in 0.2.0 | One implementation to keep correct; the golden files remain the contract. |
| Bold text: Helvetica → core Helvetica-Bold; a TrueType font (`--font PATH`, `pt-sans-narrow`) → synthetic bold (text render mode 2, fill + thin stroke in the same colour) | TrueType fonts are loaded as a single face; no extra option needed, works for Cyrillic. |
| Glyphs ◆ ★ ● and the legend's ◯ are always drawn as vector shapes | Identical on every font; WinAnsi Helvetica has none of them (REQ-PG-5 fallback). Debug/golden JSON still uses the Unicode characters. |
| Countdown drawn as `T–7` (en dash) | U+2212 is not in WinAnsi. |
| Hatch lines are #DBDBDB on white and #C6C6C6 on weekend grey | The reference pictures draw ink at 16 % opacity; a flat #DBDBDB vanishes on #E6E6E6. |
| Dashed and dotted rails and margin lines are drawn as dashes counted from the page top; a running rail overlaps the next cell by 0.15 pt | The pattern runs on unbroken from cell to cell, and no hairline seam shows between cells. |
| A milestone title starts at `guideStart`, or 0.5 pt after the glyphs when a wide day label (`28W ◆`) pushes them past it | The title never covers its own mark; the label block stays unchanged. |
| `--start` defaults to the evaluation date (`--today`, else the local date), captured once | One notion of "today" per run. |
| Where `reference/engine.js` hard-codes a default that the requirements make configurable, the requirement wins: workdays follow `--weekend`, weekly flow compaction follows `--week-start`, and the legend sorts an interval that began before the period by its first in-period day (REQ-LG-2) | The golden files use the defaults, so they agree. |
| Inline declarations are reported as `--milestone[N]` / `--interval[N]` (1-based per option) in errors | Each argument is its own one-line source. |
| DSL errors are collected (all lines reported); a declaration that fails is still registered by name so references to it do not cascade | REQ-ER-1 asks for one line per error. |
| Rendering-key errors are reported in the same run as DSL errors, for the declarations that parsed, all in source order; the lines carry no `Error:` prefix | One run shows every problem; the lines stay `source:line: message` as REQ-ER-1 asks. |
| A `.dates` file is named in messages by the path as given on the command line; an unreadable file is an ordinary error (exit 1), like an unreadable `--font` | The user can find the file; exit 2 stays for usage and DSL errors. |
| The REQ-CLI-4 warning appears only when `--timeline-compact` is given explicitly with the packed layout | The default value is not something the user asked for. |
| `--debug-timeline-json PATH` is hidden from `--help`, writes the export even without timeline input, and the PDF is still written | Usable by tests on any run. |
| `status=tentative` on a milestone appends ` ?` to its title and legend entry (REQ-KEY-6 "both"; the reference renderer ignored it) | Follows the requirement; no fixture uses it. |
| A rendering key of the other kind (`mark` on an interval, `style` on a milestone) is accepted with a warning | It is most likely a mistake, but not an invalid value. |
| An anonymous interval without a title gets no text label, only its rail and key | REQ-KEY-1; the reference renderer would print ` (CONT.)` or ` #n` on its own. |
| The legend marks members with ` · G` only when groups are merged | REQ-LG-2 and the x20 pictures; REQ-GR-5's wording suggested it also when separate. |
| The legend lists a merged group only if a member is in the period | An entry with nothing to point to would be noise. |
| Legend dates use `--month-names` with only the first letter upper case (`1 Jan`, `1 Січ`) | Localised like the header, readable in the small legend type. |
| Footer and Subject name the sources by file name, then `command line` for inline declarations (`timeline: exams.dates, command line`); a flow page says `layout: flow (weekly)` with weekly compaction; a footer too long for the grid is cut with `…` | Short, and the reader can tell where the dates came from. |
| Legend: rows filled left to right; `+N more` takes the last cell; when even one row of 5 columns would push the grid rows below 6 mm, one row is kept anyway | The legend never disappears silently; the stderr list (`Warning: the legend does not fit the page; all entries:`) is complete. |
| Limits: dates in the years 0001–9999; a period number above 4 000 000 is an error; one recurrence may produce at most 100 000 occurrences up to the period end (an error names the declaration) | Keeps arithmetic exact and a run bounded; a daily series started decades ago still fits. |
| References to interval boundaries and to one occurrence of a series (DSL §17, §18) are supported: `@qa.start`, `@qa.end`, `@review[3]`, `@review[-1]`, `@labs[2].end`; rules in DSL §3.4 | Plans are written relative to phases (`@registration.start+15d`); a hidden helper milestone was a workaround. |
| `@qa.end` is the interval's last day, the END as written (`2027-03-01/2027-03-14` → 14 March); `next = @qa.end+1d/1w` follows straight on | The value is the one the user wrote, and it agrees with the inclusive END and with `--end`. (Before the inclusive END, `.end` was the exclusive end.) |
| A `.dates` file may declare milestones and intervals in any order (DSL §9.2 rule 6 lifted) | Otherwise a milestone could never refer to an interval of the same file. No valid file changes meaning; interval keys still follow interval order. |
| Occurrence references count from the declaration alone (its count/until and the year 9999), not from the page; `[-N]` needs `count` or `until`; N is at most 100 000 | A date must not move when the period changes. |
| `@x[N]` on a non-recurring declaration and a recurring one without `[N]` are errors, as are `.start`/`.end` on a milestone and an interval without them | Each mistake gets a message that shows the right form. |
| Narrow pages draw the milestone title too (REQ-MS-3 changed): on the note line, 1 pt after the glyph, 4.9 pt bold, clipped with `…` | With 8–9 columns a short title such as "Kick-off" fits whole; without it the reader had to look up every mark in the legend. The narrow reference pictures (`page-year-*`, `page-leave`, `x20-*`) predate this and show the glyph alone. |
| A milestone title that would keep fewer than three characters before the `…` is left out, on wide and narrow pages | "Ki…" tells nothing; the mark and the legend still show the milestone. |
| `--start`, `--end` and `--until` take DSL date expressions, resolved after every declaration is parsed (an option line at the top of a file may name a later declaration); `@today` is `today` (DSL §3.2) | The page can follow the plan (`--until @exam+1w`). Declarations never depend on the page (decision 61), so parsing first is safe. |
| Option lines (`--name = value`) and `--file` includes in `.dates` files (DSL §9.2 rules 10–11, README "All options in one file"): paths relative to the file, the command line wins, an option set twice in files is an error, `--period`/`--end`/`--until` count as one | One file can hold the whole command; no hidden order between files. |
| An interval's END is included as written; `<` directly before it excludes it (`2027-08-13/<2027-09-05`, `2w/<@exam`); `<` anywhere else is an error. Internally intervals stay half-open `[start, end)`, so layout, legend and golden files are unchanged; the bundled examples were rewritten to keep their meaning (every expected JSON file stayed byte-identical, in Go and in `reference/engine.js`) | Plans name the last day (`2026-12-24/2027-01-03`), not the day after; the old form needed `+1d` on every END. `<` reads as "up to, before" and costs nothing when the exclusive form is wanted. |
| A date takes several offsets (`@release-1m+3d`) and an interval duration several terms (`1m+13d`, `1m-1d`, `1m2w`; DSL §12, §14), applied left to right with month clamping at each step; `DURATION/END` applies the terms negated in reverse order; `every=`/`gap=` keep one term | `2027-08-01/1m+13d` and `1m+13d/2027-09-13` describe the same interval. |
| A one-day interval is an ordinary interval (`2027-11-17/2027-11-17`, `/1d`, `1d/…`, `/<next day`), drawn as a rail one cell long with both caps; the legend names its day once (`17 Nov · 1 d`); an interval with no day (`d/<d`, a duration of zero or less) or ending before it starts is an error | A day that is a span, not an event (a closure day), keeps the interval's look and keys instead of a milestone mark. |
| The timeline exists only when something is declared: a file of options alone, or an empty file, leaves the page as the same flags would (no legend, footer suffix or Subject); footer and Subject name only files that declare something, in reading order | REQ-CLI-8 holds for option files; a file that only sets `--paper` is not a timeline source. |
| Interval keys number the intervals the legend lists, 1…N by first in-period day (ties in declaration order); hidden and out-of-period intervals get none. Keys of one day and a group's member list ascend | The legend is sorted by date, so numbers in file order read `2, 28, 1 …`; in date order they read 1, 2, 3. Keys belong to the page and can change with the period, like the legend itself |
| The legend is filled column by column (was row by row) | A date-sorted list reads down a column, like any list |

## Open items (not blocking v1)

1. ~~`--debug-timeline-json` naming and whether to ship it publicly.~~ Shipped hidden (not in `--help`), under this name.
2. ~~Whether the Subject metadata should also list inline declarations.~~ They are named `command line`.
3. ~~Phase 2 DSL features (interval boundary references, indexed occurrences) will need label and legend wording; no layout change is expected.~~ Implemented; references resolve to plain dates, so labels, legend and layout are unchanged.
