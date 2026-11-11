# Implementation plan (for the coding agent)

Read [README](README.md), then [01](01-requirements.md), [02](02-visual-guidelines.md), [03](03-layout-algorithms.md). Pictures in [04](04-scenarios.md) show the expected result of each rule. Done means [05](05-acceptance.md) passes in both implementations.

Keep the domain model independent of PDF drawing: **parse → resolve → layout (pure, testable) → draw**. Only the last step touches ReportLab / fpdf.

| Step | Deliverable | Requirements | Proof |
|---|---|---|---|
| 1 | Date DSL parser + evaluator, incl. recurrence and errors with `source:line` | DSL spec, REQ-RES-*, REQ-ER-1 | DSL conformance tests; `resolved` part of every golden file |
| 2 | Rendering-key reader and validation | REQ-KEY-* | validation unit tests |
| 3 | Layout engine: units, packed lanes, overflow, flow lanes, labels, keys, milestones, legend entries | REQ-PK-*, REQ-FL-*, REQ-GR-*, REQ-LB-*, REQ-MS-*, REQ-LG-2…4 | `layout` part of every golden file via `--debug-timeline-json` |
| 4 | CLI options and help | REQ-CLI-* | unit tests; REQ-CLI-8 regression (no timeline → identical PDF) |
| 5 | Drawing: rails, caps, dashes, overflow rail, margin lines, hatch, ticks, jogs, marks, ring, texts | REQ-IV-*, REQ-MS-*, 02 §3–5 | visual checks against `img/page-*.png` |
| 6 | Page budget: legend block, row-height shrink, fallbacks, footer, Subject | REQ-LG-1, -5, -6, REQ-PG-* | full-page visual checks |

Notes:

- Use civil dates (no time zones). Month arithmetic clamps to month end; recurrences with `every` are always computed from the original start.
- All orderings are listed in 03; use stable sorts.
- Sizes are in pt in 02. The reference renderer works in CSS px (1 px = 0.75 pt); where 02 and the renderer differ by rounding, 02 wins.
- `reference/engine.js` is a readable, runnable model of every rule. When a rule is ambiguous, match its behaviour and file a note.
