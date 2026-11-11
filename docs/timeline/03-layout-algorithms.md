# 03 · Layout algorithms (normative)

Pseudocode is language-neutral. Dates are civil dates (day numbers); "last" means the last drawn day (`end − 1`). Sorting MUST be stable and use exactly the keys given. The reference renderer (`reference/engine.js`) implements every step and is the tie-breaker for anything not stated here.

## 1. Pipeline

```
sources  ──parse (DSL spec)──▶ declarations
         ──expand recurrences up to period end──▶ occurrences          (REQ-RES-2)
         ──read rendering keys, validate──▶ drawable items              (REQ-KEY-*)
         ──split into layers──▶ hatch | back | front                    (REQ-IV-7, -10)
         ──merge groups (if on)──▶ units                                (REQ-GR-1)
         ──per column: pack (packed) or read flow lanes (flow)──▶ lanes, overflow
         ──per day: rails, marks, labels, keys──▶ draw list
         ──legend──▶ entries                                            (REQ-LG-*)
```

Occurrence fields used below: `key`, `s` (first day), `e` (last drawn day), `series` (bool), `idx`/`n`, `priority`, `style`, `group`, `label`.

## 2. Units

```
front := occurrences with layer = front and style ≠ hatch, not hidden
units := []
for o in front (declaration order, then occurrence order):
    if groupsMerged and o.group:
        G := unit for o.group (create on first member: colour = o.colour, priority = normal)
        G.members += o ; G.s := min ; G.e := max ; if o.priority = high: G.priority := high
    else:
        units += o
covers(u, day) := u has members ? any member m with m.s ≤ day ≤ m.e : u.s ≤ day ≤ u.e
```

## 3. Packed layout — one column

```
spans(u)  := for each m in (u.members or [u]): [max(m.s, colStart), min(m.e, colEnd)] if non-empty
pack unit := one occurrence | all occurrences of one series in this column | one merged group
             (key: series → the declaration, else the occurrence or group)
P.spans   := union of spans of its members/occurrences

# a series whose occurrences overlap each other is split, greedily in start order,
# into sub-units whose spans do not overlap (they land on adjacent lanes)

order pack units by:
    1. priority  high < normal < low
    2. first span start in this column
    3. declared start (o.s, may be before the column)
    4. longer first (later e)

lanes := []                       # lanes[i] = list of occupied spans
for P in order:
    i := lowest i such that no span of lanes[i] overlaps a span of P   (a new lane if none)
    if i ≥ cap: mark all occurrences of P overflowed; continue
    lanes[i] += P.spans ; lane(occurrence) := i for all of P

shown := len(lanes) ; k := shown + (any overflowed ? 1 : 0)
(width, step) := table 02 §3 for k and page class
zone := k > 0 ? pad + k·step + pad : pad + 0.75 pt
labelMode := (wide page and k ≤ 3) ? text : keys
```

`cap` = `--timeline-lanes` or the default from 02 §3.

### Drawing a rail on day `t` (packed and flow)

```
top := (t = u.s) or not covers(u, t − 1)
bot := (t = u.e) or not covers(u, t + 1)
x   := columnRight − pad − lane·step − width
y0  := top ? cap : (jog ? jogHeight : 0) ; y1 := bot ? cellHeight − cap : cellHeight
colour/style := one member active ? that member's : u's
width        := group with ≥ 2 members active ? width + 0.75 pt : width
if group: for each member m starting/ending at t while the group continues: draw tick
```

## 4. Flow layout — whole period

```
slots := []         # slot i = unit id or EMPTY; slot 0 is lane 0
over  := {}         # units on the overflow rail (sticky until they end)
for day in period start … period end:
    active := units covering day
    replace slots whose unit is not active by EMPTY ; remove inactive units from over
    if compact = daily or day is the first day of the week:
        remove EMPTY slots (everything outside slides inward)
    newcomers := active units not in slots and not in over,
                 ordered by priority, then s, then longer first
    for u in newcomers:
        if count(non-empty slots) ≥ cap: over += u ; continue
        put u into the innermost EMPTY slot, else append a new outermost slot
    drop trailing EMPTY slots
    lane(u, day) := index of u in slots ; used(day) := len(slots) ; overCount(day) := |over|

per column: k := max over the column's days of used(day) + (overCount(day) > 0 ? 1 : 0)
(width, step) from k ; labelMode as in packed
per day:    zone(day) := (used + (overCount>0)) > 0 ? pad + (used + (overCount>0))·step + pad : pad + 0.75 pt
overflow rail lane on day = used(day)
jog: if day ≠ day 1 of column and lane(u, day) ≠ lane(u, day − 1) and u was active yesterday:
     draw a parallelogram of the rail's width from (x(lane(day−1)), top) to (x(lane(day)), top + jogHeight),
     then the rail from top + jogHeight
```

## 5. Labels and keys

```
for each column, for day in column (in-period days only):
    candidates := back intervals covering day (lane −1), hatch intervals covering day (lane −1),
                  then front units covering day (their lane; overflowed ones lane +∞)
    if labelMode = text:
        for c in candidates ordered by lane:
            if c already labelled in this column run, or day has a milestone, or day already has a label: skip
            if c.label = none: mark labelled; skip
            text := series ? (label = each or first occurrence of the series in this column ? TITLE #idx : #idx)
                   : first column of c ? TITLE [?] [· N d | · N wd] : TITLE (CONT.)
            place text right-aligned in the top row, clip before the zone; mark labelled
    else (keys):
        for c in candidates (back, hatch, then front; each in declaration order):
            if day = c.s: add c.key
            else if wide page and day = column's first day and c started before it and c is not a series: add c.key
            for member m of a merged group: if day = m.s and day ≠ c.s: add m.key (member colour)
    if overflow count changed since yesterday in this column (or first day): add "+N" (grey)
```

A "column run" is the set of consecutive days of one occurrence inside one column.

## 6. Milestones per day

```
list := in-period, non-hidden milestones on day, declaration order
for m in list: due(m) := m.label = each or (m.label = first and (not m.series or first occurrence of m's series in this column))
                record "series seen in this column" for every series milestone in list
glyphs := marks of all m (ring → no glyph, draw ring around the day number in that colour)
title  := first m with due(m) ; text = title + (len(list) > 1 ? " +" + (len(list) − 1) : "")
countdown: for non-series m with countdown = N: days m.t − N … m.t − 1 in period → "T−i" (wide pages only)
```

## 7. Legend

```
entries := []
for each declaration (milestones first, then intervals; declaration order):
    occ := its non-hidden occurrences overlapping the period ; if empty: skip
    key := interval ? declaration key : glyph of mark (ring → ◯)
    title := title [+ " ?"] [+ " · back"] [+ " · " + group key if merged]
    when := milestone: d Mon [ "–" last d Mon " · every P · ×" len(occ) ]
            interval:  first.s "–" (series ? last occurrence's e : e) " · " (series ? "every|gap P · ×" len(occ) : N " d" | N " wd")
entries sorted by first in-period date (stable)
if merged: + one entry per group: key, name, "= " member keys joined by " + " (unique, in declaration order)
layout: 4 columns, filled row by row; fallbacks per REQ-LG-5
```

## 8. Page height budget

```
legendHeight := entries ? 6 pt + rows × 7.5 pt : 0
rowHeight    := (pageHeight − 2·margin − titleLine − headerBand − legendHeight − footer) / 31
if rowHeight < 6.0 mm: apply REQ-LG-5 fallbacks and recompute
```
