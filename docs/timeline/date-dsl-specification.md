# Date DSL Specification

## 1. Purpose

The Date DSL provides a compact, human-readable way to describe milestones, time intervals, and recurring schedules.

It supports:

- Absolute and relative dates.
    
- Named milestones that can be referenced by other definitions.
    
- Intervals defined by start/end dates or durations.
    
- Recurring milestones and intervals.
    
- Fixed-frequency and gap-based recurrence.
    
- Optional titles and arbitrary metadata.
    
- Direct command-line declarations and plain-text source files.
    

The language operates exclusively on **calendar dates**, without times of day.

---

# Phase 1 — Core DSL

## 2. Entity types

### 2.1. Milestone

A milestone represents a single calendar date.

Each milestone has:

- A required unique identifier.
    
- A date expression.
    
- An optional human-readable title.
    
- Optional key-value metadata.
    
- Optional recurrence configuration.
    

Examples:

```
release=2027-03-15
freeze=@release-2w
review=@release+1w;title="Release Review"
audit=2027-01-01;every=1m;count=6
```

A recurring milestone represents a sequence of dates rather than one scalar date.

### 2.2. Interval

An interval represents a period between two calendar dates.

Each interval has:

- An optional unique identifier.
    
- A start and an end, or a date and duration.
    
- An optional human-readable title.
    
- Optional key-value metadata.
    
- Optional recurrence configuration.
    

Examples:

```
2027-01-01/2027-02-01
qa=@freeze/@release
support=@release/1m
maintenance=@release/5d;every=2w;count=6
```

Intervals use **half-open boundaries**: `[start, end)`.

The start date is included; the end date is excluded.

---

## 3. Dates and relative expressions

### 3.1. Absolute dates

Dates use ISO calendar notation:

```
YYYY-MM-DD
```

Example:

```
2027-03-15
```

### 3.2. Relative dates

Dates may be calculated relative to today, another absolute date, or a named milestone.

|Expression|Meaning|
|---|---|
|`today`|Current calendar date|
|`@today`|The same as `today`|
|`+2w`|Two weeks from today|
|`-3d`|Three days before today|
|`today+1m`|One month from today|
|`2027-03-15-2w`|Two weeks before March 15|
|`@release`|Date of milestone `release`|
|`@release-2w`|Two weeks before release|
|`@freeze+3d`|Three days after freeze|
|`@qa.start`|First day of interval `qa`|
|`@qa.end+1w`|One week after the end of `qa`|
|`@review[3]`|Third occurrence of recurring milestone `review`|

The `@` prefix denotes a reference to a previously defined milestone or interval (§3.4). `@today` is the one exception: it is another spelling of `today` (so `@today-1w` is `today-1w`), since `today` is reserved and never names a declaration. `@today[N]`, `@today.start` and `@today.end` are errors.

A date expression supports at most one arithmetic offset in Phase 1.

### 3.3. Duration units

|Unit|Meaning|
|---|---|
|`d`|Calendar days|
|`w`|Weeks (7 days)|
|`m`|Calendar months|
|`y`|Calendar years|

Units are **case-insensitive**.

Examples:

```
2d  2D
3w  3W
1m  1M
2y  2Y
```

All pairs are equivalent.

The same duration notation applies to intervals, relative date offsets, and recurrences. Context determines its meaning.

Examples:

```
@release-2m     # Two months before release
@release/2m     # Interval lasting two months
2m/@release     # Interval ending on release
```

A duration must be a strictly positive integer followed by exactly one unit.

Composite durations are reserved for Phase 2.

### 3.4. References

A reference names a declaration made earlier (in an earlier line or an earlier source) and stands for one of its dates. It may be followed by one offset, like any other anchor.

|Reference|Refers to|Valid for|
|---|---|---|
|`@name`|The milestone's date|Non-recurring milestones|
|`@name.start`|The interval's first day|Non-recurring intervals|
|`@name.end`|The interval's end as written: the first day after it (ends are exclusive, §5)|Non-recurring intervals|
|`@name[N]`|Occurrence N of the series, counted from 1|Recurring milestones|
|`@name[-N]`|Occurrence N counted back from the last; `-1` is the last|Recurring milestones with `count` or `until`|
|`@name[N].start`, `@name[N].end`, `@name[-N].start`, `@name[-N].end`|The start or end of that occurrence|Recurring intervals (negative N: with `count` or `until`)|

Examples:

```
qa       = 2027-03-01/2027-03-15
qa_done  = @qa.end; title="QA done"           # 2027-03-15
followup = @qa.end/1w                          # 2027-03-15 to 2027-03-22, right after qa
review   = 2027-01-04; every=1w; count=8
last     = @review[-1]                         # 2027-02-22
labs     = 2027-01-11/2d; gap=1w; count=6
lab3     = @labs[3].start                      # 2027-01-29
```

Rules:

1. `.start` and `.end` are case-insensitive and apply only to intervals; an interval cannot be referenced without one.
    
2. `[N]` applies only to recurring declarations, and a recurring declaration cannot be referenced without one.
    
3. N is between 1 and 100 000 (or −1 and −100 000). An N beyond the last occurrence is an error that states how many there are.
    
4. Occurrences are counted from the declaration alone (its `every`/`gap`, `count`, `until` and the year 9999), never from what a consumer displays: `@review[24]` means the same date whatever period a calendar shows.
    
5. Anonymous intervals cannot be referenced.
    
6. Only names declared earlier can be referenced: forward references remain unsupported (§20).
    
7. Chained offsets remain Phase 2 (§14): `@qa.end+1d-2d` is invalid.
    

---

## 4. Milestone declarations

### Syntax

```
NAME=DATE_EXPR[;PROPERTY=VALUE...]
```

Examples:

```
release=2027-03-15
freeze=@release-2w
review=@release+2w;title="Post-release Review"
deadline=today+3m;owner=alice;priority=high
```

### Rules

1. A milestone requires a unique identifier.
    
2. Identifiers are case-insensitive.
    
3. A milestone may reference previously defined milestones and intervals (§3.4).
    
4. An interval is referenced through its start or end (`@qa.start`, `@qa.end`), never as `@qa`.
    
5. A recurring milestone or interval is referenced through one occurrence (`@review[3]`, `@labs[-1].end`).
    
6. Circular and undefined references are invalid.
    

Example dependency chain:

```
release=2027-03-15
freeze=@release-2w
testing=@freeze-1w
```

---

## 5. Interval declarations

### Supported forms

```
[NAME=]START/END
[NAME=]START/DURATION
[NAME=]DURATION/END
```

Every form supports optional metadata.

Examples:

```
# Explicit dates
2027-01-01/2027-02-01

# Named interval
qa=@freeze/@release

# Start and duration
support=@release/1m

# Duration and end
preparation=2w/@freeze

# Relative start and end
audit=@release+1w/@release+3w

# Metadata
qa=@freeze/@release;title="Final QA";team=backend
```

### Rules

- Both interval boundaries resolve to calendar dates.
    
- The start must be strictly earlier than the end.
    
- The end is exclusive.
    
- Anonymous intervals are permitted.
    
- Named intervals have unique identifiers.
    
- A duration may appear on either side, but not both.
    
- A duration on the right is added to the start.
    
- A duration on the left is subtracted from the end.
    

---

## 6. Recurrences

Recurrence extends an existing milestone or interval declaration through reserved properties.

### 6.1. Recurrence properties

|Property|Meaning|Applies to|
|---|---|---|
|`every`|Start-to-start recurrence frequency|Milestones and intervals|
|`gap`|End-to-start spacing between occurrences|Intervals only|
|`count`|Number of occurrences, including the first|Both|
|`until`|Last permitted occurrence date, inclusive|Both|

### 6.2. Recurring milestones

Syntax:

```
NAME=DATE_EXPR;every=PERIOD[;count=N]
NAME=DATE_EXPR;every=PERIOD[;until=DATE_EXPR]
```

Examples:

```
# Every 3 days, 10 occurrences
backup=today;every=3d;count=10

# Every 2 weeks, starting after release
review=@release+1w;every=2w;count=5

# Monthly until a fixed date
audit=2027-01-01;every=1m;until=2027-12-31

# Unlimited weekly recurrence
meeting=today;every=1w
```

The initial date is occurrence #1.

Subsequent occurrences are spaced according to `every`.

### 6.3. Recurring intervals — fixed frequency

The `every` property measures the time between consecutive interval starts.

Syntax:

```
INTERVAL;every=PERIOD[;count=N]
INTERVAL;every=PERIOD[;until=DATE_EXPR]
```

Example:

```
sprint=2027-01-01/2w;every=3w;count=3
```

Resolved occurrences:

|#|Start|End (exclusive)|
|---|---|---|
|1|Jan 1|Jan 15|
|2|Jan 22|Feb 5|
|3|Feb 12|Feb 26|

The interval lasts two weeks, while consecutive starts are separated by three weeks.

Overlapping occurrences are allowed when the recurrence frequency is shorter than the interval duration.

### 6.4. Recurring intervals — gap-based

The `gap` property measures the time between the end of one interval and the start of the next.

Syntax:

```
INTERVAL;gap=PERIOD[;count=N]
INTERVAL;gap=PERIOD[;until=DATE_EXPR]
```

Example:

```
work=2027-01-04/5d;gap=2d;count=3
```

Resolved occurrences:

|#|Start|End (exclusive)|
|---|---|---|
|1|Jan 4|Jan 9|
|2|Jan 11|Jan 16|
|3|Jan 18|Jan 23|

Each interval lasts five calendar days, followed by a two-calendar-day gap.

### 6.5. Recurrence limits

**Count-limited:**

```
review=today;every=1w;count=5
```

Produces exactly five occurrences, including the initial one.

**Date-limited:**

```
review=today;every=1w;until=2027-03-31
```

Produces occurrences whose dates are on or before the limit.

For intervals, `until` applies to the **start date**, not the end.

**Unbounded:**

```
backup=today;every=1d
```

Produces a conceptually unlimited sequence, evaluated lazily within a requested window.

### 6.6. Recurrence validation

Rules:

1. `every` and `gap` are mutually exclusive.
    
2. `gap` is valid only for intervals.
    
3. `count` and `until` are mutually exclusive.
    
4. `count` and `until` require `every` or `gap`.
    
5. `count` must be a positive integer.
    
6. `every` and `gap` must contain a positive period.
    
7. `until` must resolve to a date on or after the first occurrence start.
    
8. A recurring milestone or interval is referenced through one occurrence (§3.4), never as a whole.
    

### 6.7. Calendar-month recurrence

Monthly and yearly recurrence uses calendar arithmetic.

Dates that do not exist in the target month are clamped to that month's last valid day.

Example:

```
report=2027-01-31;every=1m;count=4
```

Results:

```
2027-01-31
2027-02-28
2027-03-31
2027-04-30
```

**Fixed-frequency occurrences are calculated from the original start date**, not by repeatedly adding a period to the previous occurrence.

This prevents unwanted calendar drift.

For gap-based recurrence, each subsequent start is instead calculated from the preceding occurrence's actual end.

For recurring intervals:

- `START/DURATION` preserves the original calendar duration for every occurrence.
    
- `START/END` preserves the original fixed duration measured in calendar days.
    
- `DURATION/END` resolves the first interval by subtracting the period from the end. Subsequent intervals preserve that period, applied forward from each new start. Around month ends, this may produce different actual day counts.
    

For predictable recurring calendar-month intervals, `START/DURATION` is the preferred form.

---

## 7. Metadata

Metadata consists of semicolon-separated key-value pairs.

Syntax:

```
EXPRESSION;key=value;key=value
```

Examples:

```
release=2027-03-15;title="Public Release"
qa=@freeze/@release;team=backend;priority=high
review=today;every=1m;count=6;title="Monthly Review";owner=alice
```

Rules:

- Metadata is optional.
    
- `title` is a reserved display-title property.
    
- `every`, `gap`, `count`, and `until` are reserved recurrence properties.
    
- Other property keys are arbitrary identifiers.
    
- Keys are case-insensitive.
    
- Values preserve their original case.
    
- Unquoted values cannot contain whitespace or semicolons.
    
- Double-quoted values may contain whitespace and semicolons.
    
- Inside quoted values, `\"` and `\\` are supported escapes.
    
- Duplicate property keys within one declaration are errors.
    
- Properties may appear in any order.
    

---

## 8. Identifiers and case sensitivity

Identifiers follow:

```
[A-Za-z_][A-Za-z0-9_]*
```

Examples:

```
release
code_freeze
review2
_phase_1
```

Invalid identifiers include:

```
2release
code-freeze
release date
```

Identifiers, keywords, metadata keys, and duration units are case-insensitive.

For example:

```
RELEASE=2027-03-15
freeze=@Release-2W
QA=@FREEZE/@release;TITLE="Final QA"
```

Metadata values and titles retain their original case.

Identifiers must be unique across named milestones and named intervals. `today` is reserved and cannot be used as an identifier.

---

## 9. Input methods

### 9.1. Command-line declarations

The application supports three repeatable arguments:

```
--milestone DECLARATION
--interval DECLARATION
--file PATH
```

Example:

```
myapp \
  --milestone 'release=2027-03-15;title="Release"' \
  --milestone 'freeze=@release-2w' \
  --milestone 'review=@release+1w;every=2w;count=5' \
  --interval 'qa=@freeze/@release;title="Final QA"' \
  --interval 'support=@release/1m;gap=1w;count=3'
```

The DSL declarations are passed as single shell arguments.

### 9.2. Plain-text source files

Suggested extension: `.dates`.

The format uses one declaration per line.

Rules:

1. Files are UTF-8 encoded.
    
2. Each nonempty line contains one complete declaration.
    
3. Blank lines are ignored.
    
4. Lines beginning with `#` are comments.
    
5. Whitespace around separators is optional.
    
6. Milestones and intervals may be declared in any order. (Earlier drafts required all milestones before the first interval; that rule was lifted when interval boundaries became referable, so that a milestone can follow the interval it refers to.)
    
7. Referenced milestones and intervals must already be defined.
    
8. Recurring declarations are referenced through one occurrence (§3.4).
    
9. A declaration is classified as an interval if its expression contains `/` outside quoted text; otherwise it is a milestone.
    
10. A line whose first non-blank characters are `--` is not a declaration: it sets an option of the application, spelled as on its command line (`--start = 2027-01-01`, `--start 2027-01-01`, a bare `--hide-outside`; a value in double quotes may contain `#` and `;`). An identifier never starts with `-`, so the two cannot be confused. Which options exist, and which a file may set, is up to the application (for yearly-calendar: the README, "All options in one file").
    
11. `--file PATH` in a file reads PATH at that line, PATH taken relative to the including file: its declarations come between the lines around it, so later lines may refer to them (rule 7). A file read twice, by a cycle or otherwise, is an error.
    

Example `roadmap.dates`:

```
# ==========================================
# Milestones
# ==========================================

release = 2027-03-15; title="Public Release"
freeze  = @release-2w; title="Code Freeze"
review  = @release+1w; title="Post-release Review"

# Recurring milestones
audit   = @release+1m; every=3m; count=4
backup  = today; every=1w; title="Weekly Backup"

# ==========================================
# Intervals
# ==========================================

# Ordinary intervals
qa      = @freeze/@release; title="Final QA"
support = @release/1m; title="Hypercare"

# Recurring intervals
sprint  = @release/2w; every=3w; count=4
maint   = @release+1m/3d; gap=2w; count=6

# Date-limited recurrence
checkup = @release/1w; every=1m; until=@release+6m

# Anonymous interval
@review/2w; title="Follow-up period"
```

Usage:

```
myapp --file roadmap.dates
```

### 9.3. Combining source files and CLI input

Files and CLI declarations may be combined:

```
myapp \
  --file roadmap.dates \
  --milestone 'extra_review=@release+3w' \
  --interval 'extra_qa=@release/@extra_review'
```

Processing order:

1. Load source files in their specified order, evaluating declarations top-to-bottom.
    
2. Process inline milestone declarations in their supplied order.
    
3. Process inline interval declarations in their supplied order.
    

Definitions in later sources may reference earlier milestones and intervals. Inline milestones come before inline intervals, so an inline milestone cannot refer to an inline interval; put the interval in a file instead.

Forward references are not supported in Phase 1.

Duplicate identifiers across sources are errors.

---

## 10. Formal grammar (EBNF)

The following grammar defines the core structure. Lexical handling of whitespace, quoted strings, case-insensitive tokens, comments, and semantic validation follows the rules above.

```
document          = { blank_line
                    | comment_line
                    | option_line          (* §9.2 rules 10–11, defined by the application *)
                    | declaration_line } ;

declaration_line  = declaration, newline ;

declaration      = milestone
                 | interval ;

milestone         = identifier, "=", date_expr, properties ;

interval          = [ identifier, "=" ], range, properties ;

range             = date_expr, "/", date_expr
                  | date_expr, "/", period
                  | period, "/", date_expr ;

date_expr         = anchor, [ sign, period ]
                  | sign, period ;

anchor            = iso_date
                  | "today" | "@today"
                  | reference ;

reference         = "@", identifier,
                    [ "[", [ "-" ], positive_integer, "]" ],
                    [ ".", ( "start" | "end" ) ] ;

sign              = "+" | "-" ;

period            = positive_integer, unit ;

unit              = "d" | "w" | "m" | "y" ;

properties        = { ";", property } ;

property          = title_property
                  | recurrence_property
                  | custom_property ;

title_property    = "title", "=", value ;

custom_property   = identifier, "=", value ;

recurrence_property
                  = frequency
                  | recurrence_limit ;

frequency         = "every", "=", period
                  | "gap", "=", period ;

recurrence_limit  = "count", "=", positive_integer
                  | "until", "=", date_expr ;

value             = unquoted_value
                  | quoted_string ;

identifier        = identifier_start,
                    { identifier_char } ;

identifier_start  = letter | "_" ;

identifier_char   = letter | digit | "_" ;

positive_integer  = nonzero_digit, { digit } ;

iso_date          = digit, digit, digit, digit,
                    "-", digit, digit,
                    "-", digit, digit ;
```

Additional semantic constraints:

- An interval expression must contain one `/`.
    
- A milestone expression cannot contain `/`.
    
- Exactly one recurrence frequency is permitted.
    
- A recurrence may have at most one limit.
    
- A recurrence limit cannot exist without a frequency.
    
- `gap` requires an interval.
    
- Reserved property names cannot be used as custom metadata keys.
    
- Calendar dates must be valid Gregorian dates.
    
- Resolved interval starts must precede their ends.
    
- Date expressions may reference only previously defined milestones and named intervals, in the forms of §3.4.
    

---

## 11. Evaluation semantics

### 11.1. Calendar model

- Gregorian calendar.
    
- Calendar dates only.
    
- No time-of-day representation.
    
- No time-zone information stored with dates.
    
- The configured local calendar determines `today`.
    

### 11.2. Date evaluation

`today` is captured once per evaluation.

Relative dates are resolved against that fixed evaluation date or a reference (§3.4).

Months and years use calendar arithmetic with day clamping.

### 11.3. Interval evaluation

All intervals are half-open:

```
[start, end)
```

End-date subtraction and start-date addition follow calendar arithmetic.

Adding and subtracting calendar months are not necessarily inverse operations.

### 11.4. Recurrence evaluation

For `every`:

- First occurrence uses the original date or interval.
    
- Subsequent occurrence starts are calculated from the original start plus N recurrence periods.
    
- The period is not repeatedly added to the previous occurrence.
    

For `gap`:

- First occurrence uses the original interval.
    
- Each subsequent start is the previous end plus the gap.
    
- The interval's original duration semantics are preserved as specified in Section 6.7.
    

For `count`:

- The initial occurrence counts as #1.
    

For `until`:

- The date is inclusive.
    
- The limit applies to occurrence starts.
    

For unbounded recurrences:

- Occurrences are generated lazily.
    
- Consumers must request a finite evaluation window or maximum occurrence count.
    

### 11.5. Errors

The parser or evaluator must reject:

- Invalid syntax or dates.
    
- Unknown references.
    
- References in a form §3.4 does not allow: a recurring declaration without an occurrence, an interval without `.start` or `.end`, `.start`/`.end` on a milestone, `[N]` on a non-recurring declaration.
    
- Occurrence numbers beyond the series, or counting back on a series without `count` or `until`.
    
- Duplicate identifiers.
    
- Invalid or non-positive durations.
    
- Non-positive interval lengths.
    
- Invalid recurrence configurations.
    
- Unsupported forward references.
    
- Invalid metadata and reserved-property usage.
    

Errors should identify the source file or argument, declaration, and reason.

---

# Phase 2 — Optional Extensions

## 12. Composite periods

Allow multiple duration components:

```
1m2w
2w3d
1y2m5d
```

Define explicit component ordering for calendar arithmetic.

## 13. ISO 8601 period compatibility

Accept ISO-style periods as aliases:

```
P2W
P1M
P1Y2M
```

The short syntax remains canonical.

## 14. Chained date arithmetic

Support multiple offsets within one expression:

```
@release-1m+3d
today+2w-1d
```

## 15. Advanced recurrence patterns

Add calendar-oriented frequency expressions, such as:

- Every Monday.
    
- Every weekday.
    
- The first Monday of each month.
    
- The last day of each month.
    
- Every second Tuesday.
    

These may use an optional RFC 5545-inspired recurrence form rather than overloading the basic `every` unit syntax.

## 16. Recurrence exceptions

Support:

- Skipping individual occurrences.
    
- Adding extra occurrences.
    
- Excluding holidays.
    
- Overriding the date, duration, or metadata of a specific occurrence.
    

## 17. Indexed occurrence references

**Implemented** (with §18); the rules are in §3.4.

Allow references to individual occurrences of recurring milestones:

```
@review[1]
@review[3]
@review[-1]
```

Indexing is one-based. Negative indexing requires a finite recurrence.

## 18. Interval boundary references

**Implemented** (with §17); the rules are in §3.4.

Allow referencing named interval boundaries:

```
@qa.start
@qa.end
@support.end+1w
```

## 19. Derived recurring schedules

Allow recurring intervals to be attached to recurring milestones, with one interval generated per matching milestone occurrence.

This should require explicit occurrence-alignment rules.

## 20. Forward references

Permit declarations to reference names defined later.

Resolve dependencies through a graph with cycle detection.

## 21. Source-file imports

Support directives such as:

```
include "common.dates"
```

Add import-path resolution and circular-import detection.

## 22. Validation and inspection commands

Provide operations to:

- Validate source files.
    
- Explain resolved dates and dependencies.
    
- List individual recurrence occurrences.
    
- Detect interval overlaps.
    
- Evaluate schedules for a specified reference date.
    

## 23. Export and visualization

Support exports to:

- JSON.
    
- CSV.
    
- Timeline or Gantt formats.
    
- Calendar-compatible formats where semantics permit.
    

Preserve both original expressions and resolved dates when useful.

---

## Implementation strategy

**Go implementation**

- Implement a small lexer, parser, and evaluator.
    
- Keep parsing independent from the domain model.
    
- Use a date-only representation.
    
- Implement explicit calendar arithmetic with day clamping.
    
- Support lazy recurrence generation.
    

Language-conformance tests should cover dates, references, recurrences, metadata, and month-end behaviour.

**Compatibility guarantee:** Phase 2 extensions must preserve the interpretation of all valid Phase 1 declarations.

---

## Summary

The core DSL uses a small set of concepts:

|Concept|Syntax|
|---|---|
|Milestone|`name=date`|
|Relative milestone|`name=@other+2w`|
|Interval|`name=start/end`|
|Duration-based interval|`name=start/2w`|
|Recurring milestone|`name=date;every=2w`|
|Recurring interval|`name=start/1w;every=2w`|
|Gap-based recurrence|`name=start/1w;gap=3d`|
|Count-limited recurrence|`...;count=5`|
|Date-limited recurrence|`...;until=2027-12-31`|
|Title|`...;title="Description"`|
|Metadata|`...;owner=alice`|

The guiding principles are **compactness, predictability, backward compatibility, and minimal syntax**. The initial implementation deliberately avoids advanced recurrence rules, complex arithmetic, and dependency graphs until real use cases justify them.