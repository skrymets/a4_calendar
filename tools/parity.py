#!/usr/bin/env python3
"""Parity check: render the same cases with yearly_calendar.py and the Go
binary, rasterise both with pdftoppm and compare pixels (and text via
pdftotext when available). Dev tool only.

    .venv/Scripts/python.exe tools/parity.py [--go PATH] [--dpi 100]
                                             [--keep DIR] [--case NAME ...]
"""
import argparse
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PY_SCRIPT = os.path.join(ROOT, "yearly_calendar.py")
ARIAL = r"C:\Windows\Fonts\arial.ttf"

CHANNEL_TOL = 48      # per-channel difference that counts as a differing pixel
MAX_DIFF_RATIO = 0.0025  # tolerated share of differing pixels
# worst observed 2026-11-09: 0.1187% (cyrillic-ttf); the diffs are sub-pixel glyph
# positioning noise (fpdf writes %.2f coordinates), no visible layout differences.

S = ["--start", "2026-11-07"]
CASES = {
    "jan1-year": ["--start", "2026-01-01"],
    "mid-year": S,
    "90d": S + ["--period", "90d"],
    "2w": S + ["--period", "2w"],
    "1d": S + ["--period", "1d"],
    "jan1-3m": ["--start", "2026-01-01", "--period", "3m"],
    "iso": S + ["--week-mode", "iso"],
    "rolling": S + ["--week-mode", "rolling"],
    "week-start-sun": S + ["--week-start", "6"],
    "labels-all": S + ["--week-labels", "all"],
    "full-header": S + ["--header", "full"],
    "letter": ["--start", "2026-01-01", "--paper", "letter"],
    "hide-outside": S + ["--period", "3m", "--hide-outside"],
    "note-lines-3": S + ["--note-lines", "3"],
    "no-footer": S + ["--no-footer"],
    "no-note-guide": S + ["--no-note-guide"],
    "colors": S + ["--workday-color", "#006600", "--weekend-color", "0xCC6600", "--weekend", "4,5,6"],
    "long-title": S + ["--period", "90d", "--header", "full", "--title",
                       "A VERY LONG CUSTOM TITLE THAT WILL NOT FIT NEXT TO THE DATE RANGE LABEL"],
    "margin-5": S + ["--margin", "5"],
    "cyrillic-ttf": ["--start", "2027-01-01", "--font", ARIAL, "--day-letters", "ПВСЧПСН",
                     "--month-names", "СІЧ,ЛЮТ,БЕР,КВІ,ТРА,ЧЕР,ЛИП,СЕР,ВЕР,ЖОВ,ЛИС,ГРУ",
                     "--title", "КАЛЕНДАР НА РІК"],
}


def read_ppm(path):
    """Return (width, height, rgb bytes) of a binary P6 PPM."""
    with open(path, "rb") as f:
        data = f.read()
    fields, i = [], 0
    while len(fields) < 4:
        while data[i:i + 1].isspace():
            i += 1
        if data[i:i + 1] == b"#":
            while data[i:i + 1] != b"\n":
                i += 1
            continue
        j = i
        while not data[j:j + 1].isspace():
            j += 1
        fields.append(data[i:j])
        i = j
    i += 1  # single whitespace after maxval
    if fields[0] != b"P6":
        raise ValueError(f"{path}: not a P6 PPM")
    w, h = int(fields[1]), int(fields[2])
    return w, h, data[i:i + w * h * 3]


def write_ppm(path, w, h, rgb):
    with open(path, "wb") as f:
        f.write(b"P6\n%d %d\n255\n" % (w, h))
        f.write(rgb)


def diff_pixels(a, b):
    """Count differing pixels; return (count, diff image: red = differs)."""
    if a == b:
        return 0, None
    out = bytearray(len(a))
    bad = 0
    for p in range(0, len(a), 3):
        if (abs(a[p] - b[p]) > CHANNEL_TOL or abs(a[p + 1] - b[p + 1]) > CHANNEL_TOL
                or abs(a[p + 2] - b[p + 2]) > CHANNEL_TOL):
            bad += 1
            out[p:p + 3] = b"\xff\x00\x00"
        else:
            g = (a[p] + 3 * 255) // 4  # faded reference image
            out[p] = out[p + 1] = out[p + 2] = g
    return bad, bytes(out)


def rasterise(pdf, prefix, dpi):
    subprocess.run(["pdftoppm", "-q", "-r", str(dpi), "-singlefile", pdf, prefix], check=True)
    return read_ppm(prefix + ".ppm")


def pdf_text(pdf):
    # -raw (content-stream order) rather than -layout: -layout column spacing shifts
    # with sub-point coordinate rounding and gives false DIFFs.
    r = subprocess.run(["pdftotext", "-q", "-enc", "UTF-8", "-raw", pdf, "-"],
                       capture_output=True, check=True)
    return r.stdout.decode("utf-8")


def build_go(tmp):
    exe = os.path.join(tmp, "yearly-calendar" + (".exe" if os.name == "nt" else ""))
    subprocess.run(["go", "-C", os.path.join(ROOT, "go"), "build", "-o", exe, "./cmd/yearly-calendar"],
                   check=True)
    return exe


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--go", help="Go binary to test (default: build from go/)")
    ap.add_argument("--dpi", type=int, default=100)
    ap.add_argument("--keep", help="directory for PDFs and diff images (default: temp, deleted)")
    ap.add_argument("--case", action="append", choices=sorted(CASES), help="run only these cases")
    a = ap.parse_args()
    if not shutil.which("pdftoppm"):
        sys.exit("pdftoppm not found on PATH (install poppler, e.g. via MiKTeX or MSYS2)")
    have_text = bool(shutil.which("pdftotext"))

    tmp = a.keep or tempfile.mkdtemp(prefix="parity-")
    os.makedirs(tmp, exist_ok=True)
    go_exe = a.go or build_go(tmp)
    env = dict(os.environ, PYTHONIOENCODING="utf-8")

    failed = 0
    print(f"{'case':16} {'pixels':>10} {'text':>6}  result")
    for name in a.case or CASES:
        args = CASES[name]
        if ARIAL in args and not os.path.exists(ARIAL):
            print(f"{name:16} {'':>10} {'':>6}  SKIP (no {ARIAL})")
            continue
        py_pdf, go_pdf = (os.path.join(tmp, f"{name}.{k}.pdf") for k in ("py", "go"))
        subprocess.run([sys.executable, PY_SCRIPT, *args, "-o", py_pdf], check=True,
                       env=env, stdout=subprocess.DEVNULL)
        subprocess.run([go_exe, *args, "-o", go_pdf], check=True, stdout=subprocess.DEVNULL)

        wa, ha, ra = rasterise(py_pdf, os.path.join(tmp, f"{name}.py"), a.dpi)
        wb, hb, rb = rasterise(go_pdf, os.path.join(tmp, f"{name}.go"), a.dpi)
        if (wa, ha) != (wb, hb):
            print(f"{name:16} size {wa}x{ha} vs {wb}x{hb}  FAIL")
            failed += 1
            continue
        bad, img = diff_pixels(ra, rb)
        ratio = bad / (wa * ha)
        if img:
            write_ppm(os.path.join(tmp, f"{name}.diff.ppm"), wa, ha, img)
        text_ok = pdf_text(py_pdf) == pdf_text(go_pdf) if have_text else None
        ok = ratio <= MAX_DIFF_RATIO and text_ok is not False
        failed += not ok
        text_col = "-" if text_ok is None else ("same" if text_ok else "DIFF")
        print(f"{name:16} {ratio:>9.4%} {text_col:>6}  {'PASS' if ok else 'FAIL'}")

    print(f"\nartefacts: {tmp}" if a.keep else "")
    if not a.keep:
        shutil.rmtree(tmp, ignore_errors=True)
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
