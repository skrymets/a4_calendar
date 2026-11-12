# yearly-calendar for coding agents: options, `.dates` files and the date language

This is a self-contained reference for an agent that turns a person's request ("a calendar
for my exam preparation, the exam is on 1 March, revise for two weeks before it") into a
printable PDF. It covers:

- the command-line contract;
- every option;
- the `.dates` file format;
- the date language;
- how things look on the page;
- the errors and how to fix them;
- a cookbook that maps phrasing to syntax;
- worked examples;
- a suggested tool interface (for example MCP).

The full language specification is in `docs/timeline/date-dsl-specification.md`, and the
user guide is `README.md`. This file is written to be enough on its own.

---

## 1. The tool in one paragraph

`yearly-calendar` is a single binary (`yearly-calendar.exe` on Windows) with no
dependencies. It writes **one PDF page**:
- one **column per month**, and one **row per day** (31 rows);
- room to write beside each date;
- optionally a **timeline** drawn on the cells: **milestones** (single dates with a mark and a
  title) and **intervals** (spans of days, drawn as coloured vertical rails), with a legend
  under the grid.

The page covers **one day up to one year**, on A4 or US Letter. Everything can be given as
options, or written into one `.dates` text file that holds both the options and the dates.

```sh
yearly-calendar plan.dates          # the whole job described by one file
```

---

## 2. Recommended agent workflow

1. **Fix "today".** Relative dates (`today`, `today+2w`) and the default start depend on it.
   Always pass `--today YYYY-MM-DD`, using the current date or the one the user means, so
   the result is reproducible.
2. **Choose the period** the page shows:
   - Choose `--start`, then either `--end` (last day, included), `--until` (the day after),
     or `--period` (`90d`, `12w`, `6m`, `1y`).
   - The maximum is one year. A longer plan needs several PDFs, one per year or per part.
   - Often the best choice is to tie the page to the plan: `--start = @kickoff` and
     `--until = @exam+1w`.
3. **Classify every item in the request:**
   - a single date that is an *event* (deadline, exam, birthday, call) → **milestone**;
   - a span of days (course, holiday, sprint, trip) → **interval**;
   - a single day that is a *state* (closure day, bank holiday, day off) → **one-day interval**
     (`D/D`);
   - a series that repeats (weekly call, every second Monday, 8 lab sessions) → `every=` or
     `gap=`, with `count=` or `until=`.
4. **Express dates relative to each other** when the user does ("two weeks before the
   exam" → `2w/<@exam`), so that moving one date moves the rest.
5. **Write one `.dates` file:** option lines at the top, then the declarations. Give every
   declaration a short `title`, and a `color` per category.
6. **Run it** and check the **exit code**:
   - `0`: the PDF was written. stdout reads `Wrote <path>: <start> → <end> (<n> days)`.
   - `2`: an error in the input, nothing written. Every problem is printed on stderr as
     `file:line: message`. Fix them all and run again.
   - `1`: another failure, such as an unreadable file or font, or an unwritable output.
7. **Read stderr even on success.** Lines with `warning:` mean that a key was ignored.
8. **Report the PDF path** to the user, with the resolved period from stdout.

---

## 3. Command line

```
yearly-calendar [options] [FILE.dates ...]
```

### Inputs and results

- **Inputs:**
  - A path given without a flag is read like `--file PATH`.
  - Options can also come from option lines inside `.dates` files (§5.2).
- **Output:**
  - The PDF goes to `-o PATH`.
  - Without `-o`, it goes to `calendar_<start>.pdf` in the **current directory**, or to
    `calendar_<start>_<period>.pdf` for a period shorter than a year
    (`calendar_2027-01-01_6m.pdf`).
  - An existing file is overwritten.
- **stdout:**
  - On success it prints `Wrote <path>: 2027-01-01 → 2027-06-30 (181 days)`.
  - `--help`, `--version` and `--licenses` print their own text.
- **stderr:**
  - errors;
  - `warning:` lines;
  - `Warning: --timeline-compact is ignored with --timeline-layout packed`;
  - the full legend list when the legend does not fit (§7.4).

### Exit codes

| Code | Meaning | stderr format |
|---|---|---|
| 0 | PDF written | possibly `warning:` lines |
| 2 | a usage error (unknown option, invalid value, period too long, conflicting options) | `Error: <message>` |
| 2 | an error in a declaration or an option line | one `source:line: message` per problem, without `Error:` |
| 1 | anything else (unreadable `--file` or `--font`, output not writable) | `Error: <message>` |

The PDF metadata: Creator is `yearly-calendar <version>`, and Subject names the `.dates`
files when there is a timeline.

### Quoting

- **Unix shells:** put a whole declaration in single quotes:
  `--interval 'qa=@exam-2w/<@exam;title="Final QA"'`.
- **Windows PowerShell 5.1** loses the double quotes inside arguments. Either put `--%`
  before the arguments and write `\"`, or, better, **use a `.dates` file**.
- **`@` in PowerShell:** quote any value that starts with `@` (`--start '@kickoff'`).
  Unquoted, PowerShell reads it as a variable.
- **Agents:** generate a `.dates` file instead of long command lines. It avoids every
  quoting problem.

---

## 4. Options

Each option can also be written in a `.dates` file as `--name = value` (§5.2).

### Period and output

| Option | Default | Meaning |
|---|---|---|
| `--start DATE` | `--today` | first day of the page; any date expression (§6.1), e.g. `2027-01-01`, `today+1w`, `@kickoff` |
| `--period P` | `1y` | length: `<N>d`, `<N>w`, `<N>m` or `1y` (case-insensitive); at most one year |
| `--end DATE` | | last day of the page (included); a date expression |
| `--until DATE` | | the day after the page (excluded); a date expression |
| `-o`, `--output PATH` | `calendar_<start>[_<period>].pdf` | output file |
| `--today YYYY-MM-DD` | local date | the date `today`/`@today` stand for; plain date only |

**Rules for the period:**
- Give only one of `--period`, `--end` and `--until`; two of them is a usage error.
- The page must not end before it starts, and must not be longer than one year. One year
  from 2027-01-01 ends on 2027-12-31; from 2027-03-01 it ends on 2028-02-29.
- `--end` or `--until` is turned into the shortest equal period. That period names the
  page and the file:
  - `--end 2027-06-30` from 2027-01-01 is `6m`: `6-MONTH CALENDAR`, `_6m.pdf`;
  - `--until 2028-01-01` is `1y`: `YEARLY CALENDAR`, no suffix;
  - `--end 2027-02-14` is `45d`.
- `--period Nm` runs to the same day N months later, minus one day. From 31 January, `1m`
  ends on 28 (or 29) February.
- **How many month columns.** Every month the period touches gets a column. A year from
  1 January has 12 columns; a year from any other day has 13. 90 days from mid-November has
  4 columns (NOV, DEC, JAN '27, FEB).

### Page appearance

| Option | Default | Meaning |
|---|---|---|
| `--title TEXT` | `YEARLY CALENDAR` for `1y`, else e.g. `90-DAY CALENDAR` | page title; `--title ""` for none; cut with `...` if too long |
| `--paper a4\|letter` | `a4` | paper size (portrait) |
| `--header compact\|full` | `compact` | `full`: larger title, taller month row |
| `--margin MM` | `10` | page margin in millimetres |
| `--week-mode relative\|iso\|rolling` | `relative` | week numbers: `relative` = week 01 contains the start; `iso` = ISO-8601; `rolling` = 7-day blocks from the start |
| `--week-start 0..6` | `0` | first weekday for `relative` mode and for weekly flow compaction (0 = Monday … 6 = Sunday) |
| `--week-labels first\|all` | `first` | week number on the week's first day only, or on every day |
| `--weekend LIST` | `5,6` | weekday indexes shaded as weekend (0 = Monday); also used by `length=workdays` |
| `--note-lines N` | `1` | dotted guide lines per cell |
| `--no-note-guide` | | no guide lines |
| `--hide-outside` | | days outside the period (in months it touches) left blank instead of faint |
| `--no-footer` | | no footer line |
| `--workday-color #RRGGBB` | `#1A3A7A` | weekday letter colour on workdays |
| `--weekend-color #RRGGBB` | `#8E1B1B` | weekday letter colour at weekends |
| `--day-letters LLLLLLL` | `MTWTFSS` | 7 weekday letters, Monday first |
| `--month-names A,B,…` | `JAN,…,DEC` | 12 comma-separated month names |
| `--font PATH.ttf\|pt-sans-narrow` | Helvetica | a TrueType font; `pt-sans-narrow` is built in and covers Cyrillic |

The built-in Helvetica covers **Western European** text only (cp1252: English, German,
French, Spanish and so on). For Cyrillic, Greek or other scripts, use
`--font pt-sans-narrow` (built in, Latin and Cyrillic) or `--font path/to/Font.ttf`. With a
non-English page, also set `--day-letters` and `--month-names`.

### Timeline input and layout

| Option | Default | Meaning |
|---|---|---|
| `--file PATH` | | read declarations and option lines from a `.dates` file; repeatable |
| `--milestone DECL` | | one inline milestone declaration; repeatable |
| `--interval DECL` | | one inline interval declaration; repeatable |
| `--timeline-layout packed\|flow` | `packed` | `packed`: rails arranged per month; `flow`: rails keep their position across months and close up when intervals end |
| `--timeline-compact daily\|weekly` | `daily` | `flow` only: close up every day, or only on `--week-start` |
| `--timeline-lanes auto\|1–12` | `auto` | rails per column before the overflow rail |
| `--timeline-groups auto\|merge\|separate` | `auto` | members of a `group=` share one rail; `auto` merges on narrow pages only |
| `--timeline-legend full\|off` | `full` | the legend under the grid |
| `--version`, `--licenses`, `-h/--help` | | print and exit |

**Loading order:**
1. files, in the order given (positional files after the `--file` values);
2. inline milestones;
3. inline intervals.

A declaration can refer only to names declared **before** it in this order. Date options
(`--start`, `--end`, `--until`) can refer to **any** declaration.

---

## 5. The `.dates` file

### 5.1 Lines

- The file is UTF-8 text (a BOM is allowed). There is **one item per line**; CRLF and LF
  both work.
- **Blank lines** are ignored.
- A line whose first non-blank character is `#` is a **comment**. There are **no trailing
  comments**: `x = 2027-01-10 # note` is an error. Put notes on their own lines, or in a
  property (`note="…"`).
- A line starting with `--` is an **option line** (§5.2).
- Every other line is a **declaration** (§6): a milestone or an interval.

### 5.2 Option lines

```
--title    = "Prepare for Exams"
--start    = @kickoff
--until    = @exam+1w
--paper    = letter
--hide-outside
--file     = holidays.dates
```

- **Spelling:**
  - `--name = value`, `--name=value` and `--name value` are all accepted.
  - A switch such as `--hide-outside` needs no value. `--hide-outside = false` turns it off.
  - The option name is exactly as on the command line, in lower case.
- **Values:**
  - An unquoted value is the rest of the line, trimmed, and may contain spaces:
    `--title = Plan for 2027`.
  - A value in double quotes may contain `#` and `;`. `\"` is a quote and `\\` a backslash.
    Nothing may follow the closing quote.
- **Not allowed in a file:** `--help`, `--version`, `--licenses`, `--milestone` and
  `--interval`. For the last two, write the declaration itself.
- **Paths are relative to the file** that contains them: `--file`, `--output`, `--font` and
  `--debug-timeline-json`. When you generate a file in a temporary directory, use absolute
  paths, or give `-o` on the command line.
- **`--file` includes another file at that line.** Its declarations come between the lines
  around it, so later lines can refer to them. A file read twice (a cycle, or a duplicate)
  is an error.
- **Precedence:**
  - The **command line wins** over files.
  - The **same option set in two places in files is an error**.
  - `--period`, `--end` and `--until` count as **one** setting for both rules.
- **No declarations.** A file with only option lines draws no timeline, legend or footer
  note. The page is exactly what the same flags would give.

---

## 6. The date language

### 6.1 Date expressions

A date expression is an **anchor**, followed by any number of **offsets**.

| Anchor | Meaning |
|---|---|
| `2027-03-15` | an ISO date (years 0001–9999) |
| `today` or `@today` | the `--today` date (else the local date) |
| `@name` | a non-recurring milestone's date |
| `@name.start` / `@name.end` | a non-recurring interval's **first** / **last** day |
| `@name[N]` / `@name[-N]` | occurrence N of a recurring milestone (from 1; `-1` is the last) |
| `@name[N].start` / `@name[N].end` | first / last day of occurrence N of a recurring interval |
| *(nothing)* | today, so `+2w` means `today+2w` |

- **Offsets** are `+N unit` or `-N unit`, with units `d` (days), `w` (weeks), `m` (calendar
  months) and `y` (years). Units are case-insensitive.
- **Several offsets** apply left to right: `@exam-1m+3d`, `today+2w-1d`.
- **Months and years clamp to the end of the month**, at each step:
  - `2027-01-31+1m` is `2027-02-28`;
  - `2027-01-31+1m+1d` is `2027-03-01`;
  - `2028-02-29+1y` is `2029-02-28`.
- **No calendar words.** There are no weekday names, no "next Monday", no "end of month",
  and no times of day. Compute such dates yourself and write them as ISO dates or offsets.
- **Rules for references:**
  - `@name` must have been **declared earlier** (§4, loading order). There are no forward
    references, except from `--start`, `--end` and `--until`.
  - An interval is always referenced through `.start` or `.end`, never as `@name`.
  - A recurring declaration is always referenced through one occurrence, `[N]`.
  - `[-N]` needs a series limited by `count` or `until`.
  - N is between 1 and 100 000. An N beyond the series is an error that says how many
    occurrences there are.
  - Occurrences are counted from the declaration alone, never from the page, so a reference
    does not move when the period changes.
  - Anonymous intervals cannot be referenced.

### 6.2 Durations

- A duration is one or more terms, the first without a sign: `2w`, `90d`, `1m`, `1y`,
  `1m+13d`, `1m-1d`, `1m2w`. In `1m2w`, a term without a sign counts as `+`.
- It appears as one side of an interval (§6.4).
- In `every=` and `gap=` it has exactly **one** term: `every=2w`, not `every=1m2w`.

### 6.3 Milestones

```
NAME = DATE_EXPR [; key=value ...]
```

- A milestone needs a name.
- It marks one day: a symbol after the weekday letter, and its title on the note line.

### 6.4 Intervals

```
[NAME =] START/END        [; key=value ...]
[NAME =] START/<END       # the end date excluded
[NAME =] START/DURATION
[NAME =] DURATION/END
[NAME =] DURATION/<END
```

- **The end date is included.** `2027-08-13/2027-09-05` covers 13 August to **5 September**.
- **`<` before the end excludes it.** `2027-08-13/<2027-09-05` ends on 4 September. `<` is
  allowed only there: not on the start, and not before a duration.
- **A duration is a length.** `2027-08-01/1m` is 1–31 August, and `2027-01-04/2w` is
  4–17 January (14 days).
- **`DURATION/END` counts back** from the end. `2w/<@exam` is the 14 days ending the day
  before the exam. `2w/@exam` ends *on* the exam day.
- **Several terms:** `2027-08-01/1m+13d` and `1m+13d/2027-09-13` are the same interval,
  1 August to 13 September.
- **One-day interval:** `2027-11-17/2027-11-17`, which is the same as `2027-11-17/1d` or
  `1d/2027-11-17`. It is drawn as a rail one cell long.
- **Errors:**
  - an end before the start: `interval ends on …, before its start …`;
  - no day at all, such as `D/<D` or a duration of zero or less: `interval … has no days`.
- **Anonymous intervals.** The name is optional for intervals, but an anonymous one cannot
  be referenced. An anonymous one without a `title` is drawn without a text label (rail and
  legend key only).
- **The page clips.** An interval may run past the page. The page shows only the days
  inside the period.

### 6.5 Repeats (recurrence)

Repeats are properties on a milestone or interval:

| Property | Meaning |
|---|---|
| `every=PERIOD` | each start is the first start + k × PERIOD (no drift: monthly from 31 Jan gives 28 Feb, 31 Mar, 30 Apr …) |
| `gap=PERIOD` | intervals only: each start is the day after the previous last day + PERIOD |
| `count=N` | N occurrences in total (the first counts as 1) |
| `until=DATE_EXPR` | no occurrence **starts** after this date (inclusive) |

- **Valid combinations:**
  - `every` and `gap` exclude each other;
  - `count` and `until` exclude each other;
  - `count` and `until` need `every` or `gap`.
- **Without a limit**, the series runs to the end of the page. At most 100 000 occurrences
  are generated up to the page end.
- **The length of later occurrences** depends on the form:
  - `START/END` keeps its number of days;
  - `START/DURATION` adds the duration to each start;
  - `DURATION/END` applies the duration forward from each start.
- **Example:** weekly on Mondays from 4 January 2027, eight times:
  `call = 2027-01-04; every=1w; count=8`. 2027-01-04 is a Monday; you must pick the first
  date yourself.

### 6.6 Names, properties and values

- **Names:** letters, digits and `_`, not starting with a digit. They are
  **case-insensitive**, unique across milestones and intervals, and `today` is reserved.
- **Properties:** `; key=value`, separated by `;`. Keys are case-insensitive, and the same
  key may not appear twice on one declaration.
- **Values:**
  - An unquoted value may not contain whitespace or `;`.
  - Anything else must be in double quotes: `title="Final QA"`, with `\"` and `\\`.
  - Values keep their case.
- **Unknown keys** (`owner=alice`, `note="…"`) are accepted, kept and ignored, so they are
  safe for your own metadata.

---

## 7. How it is drawn

### 7.1 Rendering keys

| Key | Values (default first) | On | Effect |
|---|---|---|---|
| `title` | text | both | label and legend text (default: the name). Interval labels are drawn in capitals |
| `color` | `ink`, `red`, `teal`, `violet`, `orange`, `steel`, `ochre`, `magenta`, `grey`/`gray`, `#RRGGBB` | both | colour of the mark, rail and title |
| `mark` | `diamond`, `star`, `dot`, `ring` | milestone | symbol; `ring` circles the day number |
| `countdown` | `1`–`99` | milestone | `T–7` … `T–1` on the days before (pages with 7 or fewer month columns only; ignored on a recurring milestone) |
| `style` | `solid`, `dashed`, `hatch` | interval | `hatch`: the cells are hatched instead of drawing a rail (good for holidays and closures) |
| `status` | `confirmed`, `tentative` | both | `tentative`: dashed, and ` ?` after the title |
| `length` | *(none)*, `days`, `workdays` | interval | adds ` · 14 d` or ` · 10 wd` to the label (workdays skip `--weekend` days) |
| `layer` | `front`, `back` | interval | `back`: a thin line beside the day label, no rail (long background spans: quarters, OKR cycles) |
| `priority` | `normal`, `high`, `low` | interval | which intervals get a rail first when a column is full |
| `group` | a name | interval | members can share one rail (`--timeline-groups`) |
| `hide` | `no`, `yes` | both | `yes`: never drawn, but usable as `@name` (helper dates) |
| `label` | `first`, `each`, `none` | both | which occurrences of a repeat get a text label |

- **Palette hex values:**

  | Name | Hex |
  |---|---|
  | `ink` | #1F1F1F |
  | `red` | #8E1B1B |
  | `teal` | #2E6E62 |
  | `violet` | #5E4A8F |
  | `orange` | #A04A14 |
  | `steel` | #3D6A99 |
  | `ochre` | #8A6D1F |
  | `magenta` | #9C3D6B |
  | `grey` | #7A7A7A |

  The palette is muted on purpose, so it prints well. `#RRGGBB` must have exactly six hex
  digits.
- **An invalid value is an error** that lists the allowed values. For example `color=pink`
  gives `color=pink: expected one of ink, red, …`.
- **A key of the wrong kind is a warning**, and the key is ignored: `mark` on an interval,
  or `style` on a milestone.

### 7.2 Wide and narrow pages

The look depends on how wide the month columns are:

- **Wide** (with the default margins: **1–7 month columns**, A4 or Letter):
  - milestone titles in full size;
  - text labels for intervals;
  - countdowns.
- **Narrow** (**8–13 columns**, for example a full year):
  - the mark and a smaller title share the note line, and a long title is cut with `…`;
  - a title that would keep fewer than three characters is left out (the legend still
    names it);
  - intervals are labelled with numbered keys;
  - groups merge into one rail by default;
  - no countdown.

For a dense plan, a period of seven months or less gives the most readable page.

### 7.3 Crowding

- **Room per column.** A column holds 6 rails on wide pages, and 4 or 3 on narrower ones.
- **Overflow.** Further intervals share a dotted grey **overflow rail**, and a grey `+N`
  counts them. They are still named by their keys and in the legend.
- **What helps on a crowded page:**
  - `priority=high` for the intervals that matter most;
  - `layer=back` for long background spans;
  - `group=` for parallel items of the same kind (people's leave);
  - `style=hatch` for closures;
  - a shorter period.

### 7.4 Legend, footer and title

- **Legend.** It sits under the grid and lists everything drawn, in time order:
  - an interval reads `1 Aug–13 Sep · 44 d`, a one-day interval `17 Nov · 1 d`, and a
    series `4 Jan–29 Mar · every 2w · ×7`;
  - the legend takes its room from the rows;
  - if it does not fit, it drops the dates, then ends with `+N more`, and the full list is
    printed to stderr.
  - `--timeline-legend off` removes it.
- **Footer.** It shows the number of days, the week mode, and, with a timeline, the files
  and counts. `--no-footer` removes it.
- **Title.** It goes top left; the period goes top right. A title that is too long is cut
  with `...`.

---

## 8. Cookbook: from words to syntax

| The user says | Write |
|---|---|
| "from 13 August to 5 September" | `2027-08-13/2027-09-05` |
| "until 5 September" (meaning: not on the 5th) | `2027-08-13/<2027-09-05` |
| "for two weeks from 4 January" | `2027-01-04/2w` |
| "a month and 13 days from 1 August" | `2027-08-01/1m+13d` |
| "the two weeks before the exam" | `2w/<@exam` |
| "the last week up to and including the exam" | `1w/@exam` |
| "three days after kick-off" | `@kickoff+3d` |
| "a month before the release, then 3 days later" | `@release-1m+3d` |
| "right after the course" | `@course.end+1d/…` |
| "the day the course ends" | `@course.end` |
| "the closure day on 17 November" | `closed = 2027-11-17/2027-11-17; style=hatch` |
| "the exam on 1 March" | `exam = 2027-03-01; title="Exam"; mark=star; color=red` |
| "count down the last week" | `…; countdown=7` (milestone) |
| "every Monday from 4 January" | `2027-01-04; every=1w` (pick a Monday yourself) |
| "every other week, 6 times" | `every=2w; count=6` |
| "weekly until the exam" | `every=1w; until=@exam` |
| "a 2-day lab, then a week off, 8 times" | `2027-01-11/2d; gap=1w; count=8` |
| "the last lab session" | `@labs[-1].start` |
| "maybe" / "not confirmed" | `status=tentative` |
| "most important" | `priority=high` (and `mark=star`, `color=red` for milestones) |
| "public holidays / school closed" | `style=hatch` |
| "the quarter / the OKR cycle" (a long background span) | `layer=back` |
| "show the number of days" | `length=days` (or `workdays`) |
| "a helper date, not shown" | `hide=yes` |
| "the calendar should run to a week after the exam" | `--until = @exam+1w` |
| "the school year" | `--start = 2026-08-01` and `--end = 2027-07-31` |
| "print on Letter" | `--paper = letter` |
| "in Ukrainian" | `--font pt-sans-narrow`, `--day-letters`, `--month-names`, `--title` |
| "the week starts on Sunday" | `--week-start = 6` (and `--weekend = 5,6` stays Sat/Sun) |

---

## 9. Worked examples

All examples assume today is 2026-11-12, and pass `--today` to fix it.

### 9.1 A plain year

```sh
yearly-calendar --today 2026-11-12 --start 2027-01-01 -o calendar-2027.pdf
```

### 9.2 Exam preparation (one file holds everything)

*"Make me a calendar to prepare for my final exam on 1 March. Kick-off today. Theory for
five weeks, then three weeks of practice. A tutor call every week until the exam. Labs
every two weeks from 16 November, eight of them. Revise the two weeks before the exam.
Christmas break 24 December to 6 January."*

```dates
--title    = "Prepare for the Final Exam"
--start    = @kickoff
--until    = @exam+1w
--today    = 2026-11-12

kickoff  = today;              title="Kick-off"; mark=ring; color=teal
exam     = 2027-03-01;         title="Final exam"; mark=star; color=red; countdown=7
tutor    = 2026-11-19;         title="Tutor call"; every=1w; until=@exam; mark=dot; color=steel
theory   = @kickoff/5w;        title="Theory"; color=teal; length=days
practice = @theory.end+1d/3w;  title="Practice"; color=violet; length=days
labs     = 2026-11-16/2d;      title="Lab"; every=2w; count=8; color=steel
holidays = 2026-12-24/2027-01-06; title="Christmas break"; style=hatch
revision = 2w/<@exam;          title="Revision"; color=orange; priority=high
```

```sh
yearly-calendar exams.dates -o exams.pdf
```

The page runs from 12 November 2026 to 7 March 2027.

### 9.3 Product release (five months)

```dates
--title  = "Release 3.0"
--start  = 2027-01-01
--period = 5m
--today  = 2026-11-12

release  = 2027-04-15;          title="Release 3.0"; mark=star; color=red; countdown=10
freeze   = @release-3w;         title="Code freeze"; mark=diamond; color=orange
beta     = @release-6w;         title="Beta"; mark=dot; color=steel
build    = 2027-01-11/<@freeze; title="Build"; color=teal; length=workdays
qa       = @freeze/<@release;   title="Final QA"; color=orange; length=days; priority=high
docs     = @beta/<@release;     title="Docs"; color=violet; status=tentative
support  = @release/1m;         title="Hypercare"; color=grey
sprint   = 2027-01-11/2w;       title="Sprint"; every=2w; until=@freeze; color=steel; label=each
okr      = 2027-01-01/2027-03-31; title="Q1 OKRs"; layer=back; color=grey
```

### 9.4 School year with holidays and closure days (German titles)

```dates
--title = "Schuljahr 2026/27"
--start = 2026-08-01
--end   = 2027-07-31
--today = 2026-11-12

herbst    = 2026-10-19/2026-10-31; title="Herbstferien"; style=hatch
weihnacht = 2026-12-23/2027-01-02; title="Weihnachtsferien"; style=hatch
winter    = 2027-02-01/2027-02-06; title="Winterferien"; style=hatch
ostern    = 2027-03-22/2027-04-02; title="Osterferien"; style=hatch
bruecke   = 2027-05-07/2027-05-07; title="Brückentag"; color=orange; length=days
pfingsten = 2027-05-18/2027-05-18; title="Pfingstferien"; style=hatch
sommer    = 2027-07-08/1m+12d;     title="Sommerferien"; style=hatch
elternabend = 2026-09-15;          title="Elternabend"; mark=dot; color=steel
zeugnis   = 2027-01-29;            title="Halbjahreszeugnis"; mark=star; color=red
```

The page has 12 columns (narrow): titles are short, and the summer holiday is clipped at
31 July.

### 9.5 Team leave planner (groups, dashed spans)

```dates
--title = "Team leave 2027"
--start = 2027-01-01
--today = 2026-11-12

carol = 2027-03-10/2027-03-20; title="Carol"; color=grey; style=dashed; group=people
alice = 2027-03-18/2027-03-26; title="Alice"; color=teal; style=dashed; group=people
bob   = 2027-03-22/2027-04-02; title="Bob"; color=violet; style=dashed; group=people
dana  = 2027-07-05/3w;         title="Dana"; color=orange; style=dashed; group=people
audit = 2027-03-01/2027-03-19; title="Security audit"; color=red; priority=high
offsite = 2027-06-10/2027-06-11; title="Offsite"; color=steel; length=days
```

On a full-year page (narrow), `group=people` merges into one rail. Use
`--timeline-groups separate` to show each person.

### 9.6 Personal: a wedding countdown (three months)

```dates
--title  = "Wedding"
--start  = 2027-04-01
--end    = 2027-06-30
--today  = 2026-11-12

wedding  = 2027-06-12;          title="Wedding"; mark=ring; color=magenta; countdown=14
dress    = @wedding-6w;         title="Dress fitting"; mark=dot
invites  = 2027-04-05;          title="Send invitations"; mark=diamond; color=steel
rsvp     = @invites+4w;         title="RSVP deadline"; mark=star; color=red
honeymoon = @wedding+1d/2w;     title="Honeymoon"; color=teal; length=days
```

### 9.7 A Ukrainian page

```sh
yearly-calendar --today 2026-11-12 --start 2027-01-01 --font pt-sans-narrow \
  --day-letters "ПВСЧПСН" \
  --month-names "СІЧ,ЛЮТ,БЕР,КВІ,ТРА,ЧЕР,ЛИП,СЕР,ВЕР,ЖОВ,ЛИС,ГРУ" \
  --title "КАЛЕНДАР НА 2027 РІК" -o ua-2027.pdf
```

### 9.8 Inline declarations, without a file

```sh
yearly-calendar --today 2026-11-12 --start 2027-01-01 --period 3m \
  --milestone 'launch=2027-02-15;title="Launch";mark=star;color=red' \
  --interval  'prep=2w/<@launch;title="Preparation";color=teal' \
  -o launch.pdf
```

---

## 10. Errors and how to fix them

Errors come as `source:line: message`. The source is the file path, or `--milestone[N]` or
`--interval[N]` for inline declarations. All problems in a run are listed together.

| Message (start) | Cause | Fix |
|---|---|---|
| `invalid date expression "…"` | a typo, a word date, a trailing comment, or `<` outside an interval end | use ISO dates and offsets; put comments on their own lines |
| `invalid date 2027-02-30` | the day does not exist | correct the date |
| `unknown name @x (only milestones and intervals declared earlier…)` | a misspelt or forward reference | declare it earlier (or include its file earlier) |
| `@x is an interval; use @x.start or @x.end` | an interval referenced without a bound | add `.start` or `.end` |
| `@x is recurring; pick one occurrence, e.g. @x[1] …` | a series referenced without `[N]` | add `[N]` or `[-1]` |
| `counting back from the last occurrence needs a series limited by count or until` | `[-N]` on an unbounded series | add `count` or `until` |
| `the series has only N occurrences` | `[N]` is too large | use a smaller N |
| `interval ends on …, before its start …` | the end is before the start | swap them, or fix the dates |
| `interval … has no days` | `D/<D`, or a duration of zero or less | use `D/D` for one day |
| `the start is always included; < marks an exclusive end` | `<` on the start | move `<` to the end |
| `a duration has no end to exclude` | `START/<2w` | drop `<` |
| `only one side of an interval may be a duration` | `2w/3d` | make one side a date |
| `a milestone needs a name: NAME=DATE` | a bare date line | add `name =` |
| `duplicate identifier "x"` | a name used twice (case-insensitive) | rename |
| `"today" is reserved` | `today = …` | rename |
| `unquoted value contains whitespace: … (use double quotes)` | `title=Two words` | `title="Two words"` |
| `property without value` / `duplicate property` | `;foo` or the same key twice | fix the property list |
| `color=pink: expected one of …` (and the same for other keys) | a value outside the allowed list | use an allowed value |
| `every and gap are mutually exclusive`, `count and until are mutually exclusive`, `count and until need every or gap`, `gap is only valid for intervals` | a wrong recurrence combination | fix the combination |
| `every/gap needs a positive period such as 2w` | `every=1m2w` or `every=0d` | one term only |
| `the recurrence has more than 100000 occurrences` | a daily series started long ago, or a huge page | start it later, or add a limit |
| `Error: --period, --end and --until are mutually exclusive` | two length options | keep one |
| `Error: --end … exceeds the 1-year maximum` | the page is longer than one year | split into several PDFs |
| `Error: argument --end: … is before the start …` | the end is before the start | fix the dates |
| `file:line: --paper is already set in other.dates:3` | the same option in two files | keep it in one file, or pass it on the command line |
| `file:line: unknown option --colour` | a misspelt option line | correct the option name |
| `Error: cannot read --file: …` (exit 1) | a wrong path | check the path (paths in files are relative to the file) |

**Warnings** do not stop the run:
- `warning: mark has no effect on an interval`
- `warning: countdown is ignored on a recurring milestone`

---

## 11. Pitfalls checklist

- [ ] Pass `--today`, so that `today` and the default start are what you intend.
- [ ] Interval ends are **included**. For "up to but not including", use `<`.
- [ ] Choose `2w/<@exam` (ends the day before) or `2w/@exam` (ends on the exam day) on
      purpose.
- [ ] The page is at most **one year**.
- [ ] Declare a name before you refer to it. Only `--start`, `--end` and `--until` may
      refer forward.
- [ ] Recurring items are referenced as `@x[N]`, and intervals as `@x.start` or `@x.end`.
- [ ] There are no weekday words: compute the first Monday and other such dates yourself.
- [ ] Quote titles that contain spaces or `;`. There are no trailing `#` comments.
- [ ] `every=` and `gap=` take a single term (`2w`, not `1m2w`).
- [ ] Unknown keys are ignored silently, so check the spelling of rendering keys (§7.1).
- [ ] Non-Latin text needs `--font pt-sans-narrow` or a TTF font.
- [ ] Paths inside a `.dates` file are relative to that file.
- [ ] Check the exit code and stderr. Exit 2 means nothing was written.

---

## 12. Verifying the result

- **stdout** confirms the period: `Wrote x.pdf: 2027-01-01 → 2027-06-30 (181 days)`.
- **Resolved dates.** The hidden option `--debug-timeline-json PATH` writes the resolved
  dates and the layout decisions as JSON, and the PDF is still written. Check each
  interval's `start`, its `lastDrawnDay` and its occurrences before you show the page.
  This is a debugging aid, and its format may change.

  ```json
  {"resolved": {"milestones": [{"id": "kick", "date": "2027-01-04"}],
                "intervals": [{"id": "sprint", "key": "1", "start": "2027-01-04",
                               "endExclusive": "2027-01-18", "lastDrawnDay": "2027-01-17",
                               "occurrence": "1/3"}]}}
  ```
- **Looking at the page.** A PDF-to-image tool (for example `pdftoppm -r 100 -png`) lets
  a multimodal agent look at the page.

---

## 13. A suggested tool interface (MCP or similar)

The tool takes a single `.dates` text; options are lines in it. This keeps the interface
small and avoids shell quoting.

```json
{
  "name": "render_calendar",
  "description": "Render a one-page printable calendar PDF (one column per month, one row per day; up to one year) with optional milestones and intervals. Input is the text of a .dates file: option lines such as '--start = 2027-01-01' and '--end = 2027-06-30', then declarations such as 'exam = 2027-03-01; title=\"Exam\"; mark=star' or 'revision = 2w/<@exam; title=\"Revision\"'. Interval end dates are included; '<' before the end excludes it.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "dates": {
        "type": "string",
        "description": "Contents of a .dates file: option lines (--name = value) and declarations, one per line; # starts a whole-line comment."
      },
      "today": {
        "type": "string",
        "pattern": "^\\d{4}-\\d{2}-\\d{2}$",
        "description": "Evaluation date for 'today'; defaults to the server's local date."
      },
      "output": {
        "type": "string",
        "description": "Absolute path of the PDF to write; defaults to a temporary file."
      }
    },
    "required": ["dates"]
  }
}
```

A sketch of an implementation:

1. Write `dates` to `<tmpdir>/input.dates`.
2. Run `yearly-calendar --today <today> <tmpdir>/input.dates -o <output>`. Options on the
   command line override the file, so `-o` always wins.
3. On exit 0, return the PDF path, the stdout line and any warnings.
4. On exit 2, return stderr unchanged. Its `input.dates:LINE: message` lines point the agent
   straight to the line to fix.
5. On exit 1, return the error. It is usually a path or permission problem.

Possible additional tools, all built on the same binary:

| Tool | What it does |
|---|---|
| `validate_dates` | runs with `--debug-timeline-json` into a temp file and returns the resolved dates without keeping the PDF |
| `list_options` | returns `yearly-calendar --help` |
| `version` | returns `yearly-calendar --version` |

---

## 14. Grammar summary

```
file          = { blank | comment | option_line | declaration } ;
comment       = "#", { any } ;                         (* whole line only *)
option_line   = "--", name, [ ( "=" | ws ), value ] ;  (* value: rest of line, or "quoted" *)
declaration   = milestone | interval ;
milestone     = ident, "=", date_expr, props ;
interval      = [ ident, "=" ], range, props ;
range         = date_expr, "/", end
              | date_expr, "/", duration
              | duration,  "/", end ;
end           = [ "<" ], date_expr ;                   (* "<": end date excluded *)
date_expr     = anchor, { sign, term } | sign, term, { sign, term } ;
anchor        = iso_date | "today" | "@today"
              | "@", ident, [ "[", [ "-" ], int, "]" ], [ ".", ( "start" | "end" ) ] ;
duration      = term, { [ sign ], term } ;
term          = int, ( "d" | "w" | "m" | "y" ) ;
props         = { ";", key, "=", value } ;             (* value: bare (no spaces, no ;) or "quoted" *)
ident         = ( letter | "_" ), { letter | digit | "_" } ;   (* case-insensitive *)
```
