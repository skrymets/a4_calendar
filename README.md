# A4 annual calendar

A one-page calendar to print: a column for every month, a row for every day, and room
for a note beside each date. A4 or US Letter, a full year by default, or any shorter
period from one day up.

![A full-year calendar starting on 7 November 2026](docs/images/calendar-preview.png)

It is a single binary for Windows or Linux that needs nothing installed. Download, build and
run instructions are in [go/README.md](go/README.md).

## What is on the page

![Detail: header, month columns and day cells](docs/images/calendar-detail.png)

- **Title** at the top left, and the period at the top right. A period from 1 January to
  31 December shows just the year.
- **Month columns.** Every month the period touches gets a column: 12 for a year that
  starts on the 1st, 13 otherwise. A column that starts a new year says so (`JAN '27`).
- **Day cells.** The day number and weekday letter (`7S`), with the weekday letter blue
  on workdays and red at weekends. Above them in small type is the day of the period
  and the week number (`001/01`). A dotted guide line fills the rest of the cell for notes.
- **Weekends** are shaded.
- **Days outside the period** in a month the period touches are drawn faint, or left out
  with `--hide-outside`.
- **Footer**: the number of days and how weeks are counted.

## Quick start

```sh
yearly-calendar                                # one year from today
yearly-calendar --start 2027-01-01             # calendar year 2027 -> calendar_2027-01-01.pdf
yearly-calendar --start 2026-11-07 --period 90d -o q4.pdf
```

The file is written to the current directory as `calendar_<start>.pdf`, or
`calendar_<start>_<period>.pdf` for a period shorter than a year, unless `-o` names it.

## Period

`--period` sets the length, counted from `--start`:

| Value | Covers | Example from 2026-11-07 |
|---|---|---|
| `<N>d` | N days | `90d` → to 2027-02-04 |
| `<N>w` | N × 7 days | `2w` → to 2026-11-20 |
| `<N>m` | to the same day N months later, minus a day | `3m` → to 2027-02-06 |
| `1y` | one year (the default) | → to 2027-11-06 |

One year is the maximum: `53w`, `13m` or `400d` are refused. When the target day does not
exist (31 January + 1 month), the period runs to the end of the shorter month.

The default title follows the period: `YEARLY CALENDAR` for `1y`, otherwise for example
`90-DAY CALENDAR`. A title too long to fit beside the period label is cut with `...`.

## Options

| Option | Default | Meaning |
|---|---|---|
| `--start YYYY-MM-DD` | today | first day of the period |
| `--period P` | `1y` | `<N>d`, `<N>w`, `<N>m` or `1y` |
| `-o`, `--output PATH` | `calendar_<start>[_<period>].pdf` | output file |
| `--title TEXT` | from the period | page title; `--title ""` for none |
| `--paper a4\|letter` | `a4` | paper size |
| `--header compact\|full` | `compact` | `full`: a large title and a taller month row |
| `--week-mode relative\|iso\|rolling` | `relative` | how weeks are numbered (below) |
| `--week-start 0..6` | `0` (Monday) | first weekday for `relative` mode |
| `--week-labels first\|all` | `first` | show the week number on its first day, or on every day |
| `--weekend LIST` | `5,6` | weekday indexes to shade, `0`=Monday … `6`=Sunday |
| `--note-lines N` | `1` | dotted guide lines per cell |
| `--no-note-guide` | | no guide lines |
| `--hide-outside` | | leave days outside the period blank instead of faint |
| `--no-footer` | | no footer line |
| `--workday-color #RRGGBB` | `#1A3A7A` | weekday letter on workdays |
| `--weekend-color #RRGGBB` | `#8E1B1B` | weekday letter at weekends |
| `--margin MM` | `10` | page margin in millimetres |
| `--day-letters LLLLLLL` | `MTWTFSS` | weekday letters, Monday first |
| `--month-names A,B,…` | `JAN,…,DEC` | 12 comma-separated month names |
| `--font PATH.ttf` | Helvetica | a TrueType font, needed for non-Latin scripts |
| `--version` | | print the version |

`--help` lists them too.

### Week numbering

- **`relative`**: week 01 is the calendar week that contains the start date. Weeks begin on
  `--week-start`.
- **`iso`**: ISO-8601 week of the year.
- **`rolling`**: blocks of seven days counted from the start date.

## Other languages

The built-in Helvetica covers Western European languages only. For Cyrillic or other scripts,
give a TrueType font and the weekday and month names:

```sh
yearly-calendar --start 2027-01-01 --font DejaVuSans.ttf \
    --day-letters "ПВСЧПСН" \
    --month-names "СІЧ,ЛЮТ,БЕР,КВІ,ТРА,ЧЕР,ЛИП,СЕР,ВЕР,ЖОВ,ЛИС,ГРУ" \
    --title "КАЛЕНДАР НА РІК"
```

The binary has PT Sans Narrow built in, which covers Cyrillic: use `--font pt-sans-narrow`
and no font file is needed.

## Versions

Releases follow [Semantic Versioning](DEV.md#versioning). The version is in the
[`VERSION`](VERSION) file. The program reports it with `--version` and writes it into
every PDF it produces (Document Properties → Creator, e.g. `yearly-calendar 0.2.0`).
What changed in each release is in [Release-Notes.md](Release-Notes.md). Downloads are on
the repository's Releases page. How releases are made is in [DEV.md](DEV.md).

## Licences

This project is released under the [MIT License](LICENSE).

The binary also contains third-party work under its own licences. It embeds PT Sans
Narrow (ParaType, SIL Open Font License 1.1). It links
[go-pdf/fpdf](https://github.com/go-pdf/fpdf) (MIT), [spf13/pflag](https://github.com/spf13/pflag)
(BSD-3-Clause) and the Go standard library (BSD-3-Clause).
`yearly-calendar --licenses` prints these licences. Every release archive includes `LICENSE`,
`OFL.txt` and `THIRD_PARTY_LICENSES.txt`.
