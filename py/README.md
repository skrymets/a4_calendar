# yearly_calendar.py: the Python version

The reference implementation of the calendar. It is a single script built on
[ReportLab](https://www.reportlab.com/). The [Go binary](../go/README.md) is a port of it and
is checked against it. What the page shows and every option are described in the
[main README](../README.md).

## Requirements

- Python 3 (developed and tested with 3.12)
- ReportLab, from `requirements.txt`

## Set up

From a release, unzip `yearly-calendar-py-<version>.zip`. From the repository, work in `py/`.
Then create a virtual environment and install the dependency:

```powershell
# Windows
python -m venv .venv
.\.venv\Scripts\python.exe -m pip install -r requirements.txt
```

```sh
# Linux / macOS
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
```

## Run

```sh
python yearly_calendar.py --start 2027-01-01          # -> calendar_2027-01-01.pdf
python yearly_calendar.py --start 2026-11-07 --period 90d -o q4.pdf
python yearly_calendar.py --help
```

Use the virtual environment's interpreter (`.venv\Scripts\python.exe` or `.venv/bin/python`)
in place of `python`.

On a Windows console or under Git Bash, the closing `Wrote … → …` line can fail with
`UnicodeEncodeError` when the console uses a legacy code page. The PDF has already been
written by then. Set `PYTHONIOENCODING=utf-8` to avoid the error.

## Version

```sh
python yearly_calendar.py --version        # yearly_calendar.py 0.1.0
```

The script reads its version from the `VERSION` file. In a release zip that file sits next
to the script. In the repository it is one directory up. Without it, the script reports `dev`.
Every PDF it writes is stamped with the version in its metadata (Creator:
`yearly-calendar <version> (Python)`).

## Fonts

The default font is the PDF built-in Helvetica, which covers Western European languages.
For other scripts, pass a TrueType file with `--font path/to/font.ttf`. Unlike the Go
binary, the script has no built-in alternative font.
