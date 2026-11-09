#!/usr/bin/env python3
"""
Yearly calendar PDF generator (one A4 page, months as vertical columns).

Covers --period starting at --start: N days (d), weeks (w) or months (m), or
one year (1y, the default and the maximum: start + 1 year - 1 day, i.e. 365
or 366 days). Every month the period touches gets a column, e.g. 12 columns
for a full year starting on the 1st, 13 otherwise. Each in-range day shows:

    001/01              <- tiny:  day of period (001..365/366) / week number
    7W                  <- small: day of month + weekday letter (coloured)
                           right part of the cell is left free for notes

Weekends are shaded. Days of a touched month that fall outside the period
are drawn faint, without the small numbers.

Requires: pip install -r requirements.txt   (reportlab)

Examples:
    python yearly_calendar.py --start 2026-01-01
    python yearly_calendar.py --start 2026-11-07 -o cal.pdf --week-mode iso
    python yearly_calendar.py --start 2026-11-07 --period 90d   # also 12w, 6m
    python yearly_calendar.py --start 2027-01-01 --font DejaVuSans.ttf \\
        --day-letters "ПВСЧПСН" \\
        --month-names "СІЧ,ЛЮТ,БЕР,КВІ,ТРА,ЧЕР,ЛИП,СЕР,ВЕР,ЖОВ,ЛИС,ГРУ" \\
        --title "КАЛЕНДАР НА РІК"
"""

import argparse
import calendar
import datetime as dt
import os
import re
import sys

from reportlab.lib.colors import HexColor
from reportlab.lib.pagesizes import A4, LETTER
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas

# ---------------------------------------------------------------- styling
INK = HexColor("#1F1F1F")
INK_SOFT = HexColor("#7A7A7A")
INK_FAINT = HexColor("#C8C8C8")
HEADER_BG = HexColor("#E6E6E6")
WEEKEND_BG = HexColor("#E6E6E6")
RULE = HexColor("#8C8C8C")
GUIDE = HexColor("#B4B4B4")
DIVIDER = HexColor("#D0D0D0")

MM = 72 / 25.4
TITLE_SIZE = 20
HEADER_H = 24
MAIN_SIZE = 7.0        # "7W"
SMALL_SIZE = 3.9       # "001/01"
CELL_PAD = 1.8         # left padding inside a cell, pt

DEFAULT_LETTERS = "MTWTFSS"  # Monday..Sunday (date.weekday() order)
DEFAULT_MONTHS = "JAN,FEB,MAR,APR,MAY,JUN,JUL,AUG,SEP,OCT,NOV,DEC"


def read_version() -> str:
    """The version from the VERSION file: beside this script (release zip) or
    one directory up (repository checkout); "dev" when neither exists."""
    here = os.path.dirname(os.path.abspath(__file__))
    for path in (os.path.join(here, "VERSION"), os.path.join(here, os.pardir, "VERSION")):
        try:
            with open(path, encoding="utf-8") as f:
                return f.read().strip() or "dev"
        except OSError:
            continue
    return "dev"


VERSION = read_version()


# ---------------------------------------------------------------- logic
PERIOD_UNITS = {"d": "DAY", "w": "WEEK", "m": "MONTH", "y": "YEAR"}


def parse_period(text: str):
    """'90d' / '12w' / '6m' / '1y' -> (n, unit)."""
    m = re.fullmatch(r"(\d+)([dwmy])", text.strip().lower())
    if not m or int(m.group(1)) < 1:
        raise argparse.ArgumentTypeError(
            f"invalid period '{text}': use <N>d, <N>w, <N>m or 1y (N >= 1)")
    return int(m.group(1)), m.group(2)


def add_months(d: dt.date, n: int) -> dt.date:
    """Same day n months later; a missing day rolls to the 1st of the next month."""
    y, m = divmod(d.month - 1 + n, 12)
    y, m = d.year + y, m + 1
    if d.day <= calendar.monthrange(y, m)[1]:
        return dt.date(y, m, d.day)
    return dt.date(y, m + 1, 1)   # m < 12 here: December has 31 days


def period_end(start: dt.date, n: int, unit: str) -> dt.date:
    """Inclusive last day of a period of n units starting at start."""
    if unit == "d":
        return start + dt.timedelta(days=n - 1)
    if unit == "w":
        return start + dt.timedelta(days=7 * n - 1)
    return add_months(start, n if unit == "m" else 12 * n) - dt.timedelta(days=1)


def months_between(start: dt.date, end: dt.date):
    y, m = start.year, start.month
    while (y, m) <= (end.year, end.month):
        yield y, m
        y, m = (y + 1, 1) if m == 12 else (y, m + 1)


def week_number(day: dt.date, start: dt.date, mode: str, week_start: int) -> int:
    if mode == "iso":
        return day.isocalendar()[1]
    if mode == "rolling":                       # every 7 days from start
        return (day - start).days // 7 + 1
    # "relative": week 01 = calendar week containing start; increments on week_start day
    def anchor(d):
        return d - dt.timedelta(days=(d.weekday() - week_start) % 7)
    return (anchor(day) - anchor(start)).days // 7 + 1


# ---------------------------------------------------------------- drawing
def register_font(path):
    if not path:
        return "Helvetica", "Helvetica"
    name = os.path.splitext(os.path.basename(path))[0]
    pdfmetrics.registerFont(TTFont(name, path))
    return name, name


def spaced_width(text, font, size, spacing):
    return pdfmetrics.stringWidth(text, font, size) + spacing * (len(text) - 1)


def fit_text(text, font, size, spacing, max_w):
    """Truncate text with '...' so that it fits into max_w."""
    if spaced_width(text, font, size, spacing) <= max_w:
        return text
    while text:
        text = text[:-1]
        cut = text.rstrip() + "..."
        if spaced_width(cut, font, size, spacing) <= max_w:
            return cut
    return ""


def draw_spaced(c, text, x, y, font, size, spacing, align="left"):
    """Draw text with letter-spacing (tracking)."""
    width = spaced_width(text, font, size, spacing)
    if align == "right":
        x -= width
    elif align == "center":
        x -= width / 2
    t = c.beginText(x, y)
    t.setFont(font, size)
    t.setCharSpace(spacing)
    t.textOut(text)
    t.setCharSpace(0)          # Tc persists in the PDF text state; reset it
    c.drawText(t)


def build(args):
    start = args.start
    end = period_end(start, *args.period)
    total_days = (end - start).days + 1
    letters = args.day_letters
    month_names = [s.strip() for s in args.month_names.split(",")]
    if len(letters) != 7:
        sys.exit("--day-letters must have exactly 7 characters (Mon..Sun)")
    if len(month_names) != 12:
        sys.exit("--month-names must have 12 comma-separated names")
    weekend = {int(x) for x in args.weekend.split(",")}

    font, font_title = register_font(args.font)
    workday_color = HexColor(args.workday_color)
    weekend_color = HexColor(args.weekend_color)
    MARGIN_X = MARGIN_BOTTOM = args.margin * MM
    MARGIN_TOP = args.margin * MM
    compact = args.header == "compact"
    title_size = 10 if compact else TITLE_SIZE
    page_w, page_h = A4 if args.paper == "a4" else LETTER
    c = canvas.Canvas(args.output, pagesize=(page_w, page_h))
    c.setTitle(f"{args.title} {start:%Y-%m-%d} – {end:%Y-%m-%d}")
    c.setCreator(f"yearly-calendar {VERSION} (Python)")

    months = list(months_between(start, end))
    multi_year = months[0][0] != months[-1][0]
    n_cols = len(months)
    grid_w = page_w - 2 * MARGIN_X
    col_w = grid_w / n_cols

    # --- title + period label
    title_y = page_h - MARGIN_TOP - title_size * 0.75
    c.setFillColor(INK)
    if (start.month, start.day) == (1, 1) and end == dt.date(start.year, 12, 31):
        label = f"{start.year}"
    else:
        label = f"{start.day:02d} {month_names[start.month-1]} {start.year}  –  " \
                f"{end.day:02d} {month_names[end.month-1]} {end.year}"
    label_size = 8 if compact else 9
    draw_spaced(c, label, page_w - MARGIN_X, title_y, font, label_size, 0.8, "right")
    if args.title:
        title_max_w = grid_w - spaced_width(label, font, label_size, 0.8) - 12
        draw_spaced(c, fit_text(args.title, font_title, title_size, 1.2, title_max_w),
                    MARGIN_X, title_y, font_title, title_size, 1.2)

    # --- header
    header_top = title_y - (6 if compact else 14)
    hdr_h = 16 if compact else HEADER_H
    header_bot = header_top - hdr_h
    c.setStrokeColor(RULE)
    c.setLineWidth(0.8)
    c.line(MARGIN_X, header_top, page_w - MARGIN_X, header_top)
    c.setFillColor(HEADER_BG)
    c.rect(MARGIN_X, header_bot, grid_w, hdr_h, stroke=0, fill=1)
    c.setLineWidth(0.4)
    c.line(MARGIN_X, header_bot, page_w - MARGIN_X, header_bot)

    hdr_size = (7.5 if compact else 9) if n_cols <= 12 else (7 if compact else 8)
    for i, (y, m) in enumerate(months):
        cx = MARGIN_X + col_w * (i + 0.5)
        name = month_names[m - 1]
        c.setFillColor(INK)
        if multi_year and (i == 0 or m == 1 or i == n_cols - 1):
            if compact:   # "NOV '26" on one line
                draw_spaced(c, f"{name} '{y % 100:02d}", cx, header_bot + hdr_h / 2 - hdr_size * 0.35,
                            font, hdr_size, 0.6, "center")
                continue
            draw_spaced(c, name, cx, header_bot + hdr_h * 0.52, font, hdr_size, 0.8, "center")
            c.setFillColor(INK_SOFT)
            draw_spaced(c, str(y), cx, header_bot + 3.5, font, 5, 0.4, "center")
        else:
            draw_spaced(c, name, cx, header_bot + hdr_h / 2 - hdr_size * 0.35,
                        font, hdr_size, 0.8, "center")

    # --- day grid
    grid_top = header_bot - (1.5 if compact else 3)
    grid_bot = MARGIN_BOTTOM + (8 if args.footer else 2)
    row_h = (grid_top - grid_bot) / 31
    pad = CELL_PAD
    # note guides start right of the widest possible label ("28W"), same x in every column
    label_w = max(pdfmetrics.stringWidth(f"28{ch}", font, MAIN_SIZE) for ch in letters) + 0.4
    small_w = pdfmetrics.stringWidth("000/00", font, SMALL_SIZE)
    guide_x0_off = pad + max(label_w, small_w) + 1.5

    for i, (y, m) in enumerate(months):
        x0 = MARGIN_X + col_w * i
        cx = x0 + col_w / 2
        for d in range(1, calendar.monthrange(y, m)[1] + 1):
            day = dt.date(y, m, d)
            yt = grid_top - row_h * (d - 1)          # cell top
            yb = yt - row_h
            in_range = start <= day <= end
            wd = day.weekday()

            if wd in weekend and in_range:
                c.setFillColor(WEEKEND_BG)
                c.rect(x0, yb, col_w, row_h, stroke=0, fill=1)

            if not in_range and not args.show_outside:
                continue

            # main "7W": number in ink, weekday letter in work/weekend colour
            main_y = yb + row_h * 0.22
            num = str(d)
            c.setFont(font, MAIN_SIZE)
            c.setFillColor(INK if in_range else INK_FAINT)
            c.drawString(x0 + pad, main_y, num)
            c.setFillColor((weekend_color if wd in weekend else workday_color)
                           if in_range else INK_FAINT)
            c.drawString(x0 + pad + pdfmetrics.stringWidth(num, font, MAIN_SIZE) + 0.4,
                         main_y, letters[wd])

            if not in_range:
                continue

            # hairline dotted note guide(s) in the free right part of the cell
            if args.note_guide and args.note_lines > 0:
                gx0, gx1 = x0 + guide_x0_off, x0 + col_w - pad
                c.saveState()
                c.setStrokeColor(GUIDE)
                c.setLineWidth(0.25)
                c.setDash(0.4, 1.2)
                c.setLineCap(1)
                step = (yt - main_y) / args.note_lines   # evenly upward from the label baseline
                for k in range(args.note_lines):
                    gy = main_y + k * step
                    c.line(gx0, gy, gx1, gy)
                c.restoreState()

            # tiny "ddd/ww" above it
            idx = (day - start).days + 1
            wk = week_number(day, start, args.week_mode, args.week_start)
            show_week = args.week_labels == "all" or day == start or \
                (args.week_mode == "rolling" and (idx - 1) % 7 == 0) or \
                (args.week_mode != "rolling" and wd == args.week_start) or d == 1
            c.setFillColor(INK_SOFT)
            c.setFont(font, SMALL_SIZE)
            c.drawString(x0 + pad, main_y + MAIN_SIZE * 0.85 + 1.2,
                         f"{idx:03d}/{wk:02d}" if show_week else f"{idx:03d}")

    # column dividers + bottom rule
    c.setStrokeColor(DIVIDER)
    c.setLineWidth(0.35)
    for i in range(1, n_cols):
        x = MARGIN_X + col_w * i
        c.line(x, header_bot + 4, x, header_top - 4)
        c.line(x, grid_bot, x, grid_top)
    c.setStrokeColor(RULE)
    c.setLineWidth(0.8)
    c.line(MARGIN_X, grid_bot - 2, page_w - MARGIN_X, grid_bot - 2)

    if args.footer:
        c.setFillColor(INK_SOFT)
        c.setFont(font, 5.5)
        weeks_total = week_number(end, start, args.week_mode, args.week_start)
        c.drawString(MARGIN_X, grid_bot - 8,
                     f"{total_days} days · week numbering: {args.week_mode}"
                     + ("" if args.week_mode == "iso" else f" ({weeks_total} weeks)"))

    c.showPage()
    c.save()
    return start, end, total_days


def parse_args(argv=None):
    p = argparse.ArgumentParser(description="Generate a one-page yearly calendar PDF.")
    p.add_argument("--start", type=dt.date.fromisoformat, default=dt.date.today(),
                   help="first day of the period, YYYY-MM-DD (default: today)")
    p.add_argument("--period", type=parse_period, default=(1, "y"),
                   help="length of the period: <N>d, <N>w, <N>m or 1y "
                        "(default 1y, max 1 year)")
    p.add_argument("-o", "--output", default=None, help="output PDF path")
    p.add_argument("--week-mode", choices=["relative", "iso", "rolling"], default="relative",
                   help="relative: w01 = calendar week containing start (default); "
                        "iso: ISO-8601 week of year; rolling: blocks of 7 days from start")
    p.add_argument("--week-start", type=int, default=0,
                   help="first weekday for 'relative' mode, 0=Mon..6=Sun (default 0)")
    p.add_argument("--week-labels", choices=["all", "first"], default="first",
                   help="show week number on every day, or only on the week's first day")
    p.add_argument("--weekend", default="5,6", help="weekday indexes to shade (default 5,6 = Sat,Sun)")
    p.add_argument("--title", default=None,
                   help="page title; truncated with '...' if too long "
                        "(default: YEARLY CALENDAR for 1y, else e.g. 90-DAY CALENDAR)")
    p.add_argument("--day-letters", default=DEFAULT_LETTERS, help="7 letters, Monday first")
    p.add_argument("--month-names", default=DEFAULT_MONTHS, help="12 comma-separated names")
    p.add_argument("--font", default=None, help="path to a .ttf (needed for Cyrillic etc.)")
    p.add_argument("--paper", choices=["a4", "letter"], default="a4")
    p.add_argument("--no-footer", dest="footer", action="store_false")
    p.add_argument("--no-note-guide", dest="note_guide", action="store_false",
                   help="disable the dotted hairline note guide (enabled by default)")
    p.add_argument("--note-lines", type=int, default=1,
                   help="number of dotted guide lines per cell (default 1)")
    p.add_argument("--workday-color", default="#1A3A7A", help="weekday-letter colour, workdays")
    p.add_argument("--weekend-color", default="#8E1B1B", help="weekday-letter colour, weekends")
    p.add_argument("--margin", type=float, default=10, help="page margin in mm (default 10)")
    p.add_argument("--header", choices=["compact", "full"], default="compact",
                   help="compact: small one-line title, slim month row (default)")
    p.add_argument("--hide-outside", dest="show_outside", action="store_false",
                   help="leave days outside the period blank instead of faint")
    p.add_argument("--version", action="version", version=f"%(prog)s {VERSION}")
    a = p.parse_args(argv)
    n, unit = a.period
    full_year = (n, unit) == (1, "y")
    year_end = add_months(a.start, 12) - dt.timedelta(days=1)
    try:
        end = period_end(a.start, n, unit)
    except (ValueError, OverflowError):        # absurdly large N
        end = dt.date.max
    if end > year_end:
        length = "" if end == dt.date.max else f" ({(end - a.start).days + 1} days)"
        p.error(f"--period {n}{unit}{length} exceeds the 1-year maximum "
                f"({(year_end - a.start).days + 1} days from {a.start})")
    if a.title is None:
        a.title = "YEARLY CALENDAR" if full_year else f"{n}-{PERIOD_UNITS[unit]} CALENDAR"
    if a.output is None:
        suffix = "" if full_year else f"_{n}{unit}"
        a.output = f"calendar_{a.start:%Y-%m-%d}{suffix}.pdf"
    return a


if __name__ == "__main__":
    args = parse_args()
    s, e, n = build(args)
    print(f"Wrote {args.output}: {s} → {e} ({n} days)")
