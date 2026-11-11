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
| `--start` defaults to the evaluation date (`--today`, else the local date), captured once | One notion of "today" per run. |
| Where `reference/engine.js` hard-codes a default that the requirements make configurable, the requirement wins: workdays follow `--weekend`, weekly flow compaction follows `--week-start` | The golden files use the defaults, so they agree. |
| Inline declarations are reported as `--milestone[N]` / `--interval[N]` (1-based per option) in errors | Each argument is its own one-line source. |
| DSL errors are collected (all lines reported); a declaration that fails is still registered by name so references to it do not cascade | REQ-ER-1 asks for one line per error. |

## Open items (not blocking v1)

1. `--debug-timeline-json` naming and whether to ship it publicly.
2. Whether the Subject metadata should also list inline declarations.
3. Phase 2 DSL features (interval boundary references, indexed occurrences) will need label and legend wording; no layout change is expected.
