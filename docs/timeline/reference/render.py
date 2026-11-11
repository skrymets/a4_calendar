"""Renders every design-guideline example from its .dates fixture into docs/timeline/img/*.png."""
import json, pathlib, sys
from playwright.sync_api import sync_playwright

ROOT = pathlib.Path(__file__).resolve().parent
DOCS = ROOT.parent
EX, IMG = DOCS / "examples", DOCS / "img"
TODAY = "2026-11-11"
H1 = dict(start="2027-01-01", end="2027-05-31")        # 5 columns, 143.6 px each (wide geometry)
YEAR = dict(start="2027-01-01", end="2027-12-31")      # 12 columns, 59.8 px each (narrow geometry)

EXAMPLES = [
    # ---- full pages
    dict(id="page-exams", kind="page", file="exams.dates", start="2026-11-11", end="2027-03-10", title="Prepare for Exams"),
    dict(id="page-release-packed", kind="page", file="release.dates", title="Release Plan", **H1),
    dict(id="page-release-flow", kind="page", file="release.dates", title="Release Plan", layout="flow", **H1),
    dict(id="page-year-packed", kind="page", file="year-plan.dates", start="2026-11-11", end="2027-11-10", title="Team Year Plan"),
    dict(id="page-year-flow", kind="page", file="year-plan.dates", start="2026-11-11", end="2027-11-10", title="Team Year Plan", layout="flow"),
    dict(id="page-leave", kind="page", file="leave-2027.dates", title="Vacation Request", **YEAR),
    # ---- anatomy
    dict(id="anatomy", kind="crop", file="exams.dates", start="2026-11-11", end="2027-03-10", cols=[3], frm=13, to=28, zoom=3,
         callouts=[dict(n=1, row=13, x=24, dy=5), dict(n=2, row=13, x=34, dy=19), dict(n=3, row=13, x=70, dy=23),
                   dict(n=4, row=14, x=116, dy=14), dict(n=5, row=15, x=60, dy=6), dict(n=6, row=17, x=112, dy=22),
                   dict(n=7, row=18, x=44, dy=19), dict(n=8, row=19, x=34, dy=9), dict(n=9, row=19, x=92, dy=20),
                   dict(n=10, row=22, x=62, dy=6), dict(n=11, row=22, x=100, dy=6)]),
    # ---- milestones
    dict(id="x01-marks", kind="crop", file="x01-marks.dates", cols=[2], frm=1, to=6, **H1),
    dict(id="x02-colors", kind="crop", file="x02-colors.dates", cols=[2], frm=8, to=17, **H1),
    dict(id="x03-countdown", kind="crop", file="x03-countdown.dates", cols=[2], frm=5, to=13, **H1),
    dict(id="x04-anchor", kind="crop", file="x04-anchor.dates", cols=[2], frm=1, to=16, **H1),
    # ---- intervals
    dict(id="x05-interval", kind="crop", file="x05-interval.dates", cols=[2], frm=1, to=12, **H1),
    dict(id="x07-continuation", kind="crop", file="x07-continuation.dates", cols=[1, 2], frm=1, to=31, zoom=2, **H1),
    dict(id="x08-dashed", kind="crop", file="x08-dashed.dates", cols=[2], frm=1, to=16, **H1),
    dict(id="x09-hatch", kind="crop", file="x09-hatch.dates", cols=[2], frm=1, to=22, **H1),
    dict(id="x10-back", kind="crop", file="x10-back.dates", cols=[2], frm=1, to=18, **H1),
    # ---- packing
    dict(id="x11-overlap", kind="crop", file="x11-overlap.dates", cols=[2], frm=1, to=24, **H1),
    dict(id="x12-keys", kind="crop", file="x12-keys.dates", cols=[2], frm=1, to=24, legend=True, **H1),
    dict(id="x13-adaptive", kind="crop", file="x13-adaptive.dates", cols=[0, 1, 2], frm=1, to=21, cap=8, zoom=2, **H1),
    dict(id="x14-overflow", kind="crop", file="x14-overflow.dates", cols=[2], frm=1, to=19, legend=True, **H1),
    # ---- recurrence
    dict(id="x15-recurring-milestone", kind="crop", file="x15-recurring-milestone.dates", cols=[0, 1], frm=1, to=31, zoom=2, **H1),
    dict(id="x16-recurring-interval", kind="crop", file="x16-recurring-interval.dates", cols=[0, 1], frm=1, to=31, zoom=2, **H1),
    dict(id="x17-gap", kind="crop", file="x17-gap.dates", cols=[0], frm=1, to=31, zoom=2, **H1),
    dict(id="x18-monthly", kind="crop", file="x18-monthly.dates", cols=[0, 1, 2, 3, 4], frm=24, to=31, zoom=2, **H1),
    dict(id="x19-overlapping-series", kind="crop", file="x19-overlapping-series.dates", cols=[2], frm=1, to=31, zoom=2, **H1),
    # ---- narrow pages
    dict(id="x20-groups-merged", kind="crop", file="x20-groups.dates", cols=[2, 3], frm=1, to=31, zoom=2.5, legend=True, **YEAR),
    dict(id="x20-groups-unmerged", kind="crop", file="x20-groups.dates", cols=[2, 3], frm=1, to=31, zoom=2.5, legend=True, mergeGroups="off", **YEAR),
    # ---- flow
    dict(id="x21-packed", kind="crop", file="release.dates", cols=[2], frm=15, to=28, **H1),
    dict(id="x21-flow", kind="crop", file="release.dates", cols=[2], frm=15, to=28, layout="flow", **H1),
    dict(id="x22-flow-weekly", kind="crop", file="release.dates", cols=[2], frm=15, to=28, layout="flow", compact="weekly", **H1),
    # ---- period edges
    dict(id="x23-clipping", kind="crop", file="x23-clipping.dates", start="2027-01-15", end="2027-06-14", cols=[0, 5], frm=1, to=31, zoom=2),
]


def main(only=None):
    with sync_playwright() as p:
        b = p.chromium.launch()
        pg = b.new_page(device_scale_factor=2, viewport=dict(width=2400, height=2400))
        pg.goto((ROOT / "harness.html").as_uri())
        for ex in EXAMPLES:
            if only and ex["id"] not in only:
                continue
            spec = dict(ex)
            spec["dates"] = (EX / ex["file"]).read_text()
            spec["today"] = TODAY
            spec.setdefault("layout", "packed")
            spec["from"] = spec.pop("frm", 1)
            spec["showLegend"] = spec.pop("legend", False)
            zoom = spec.pop("zoom", 2.5 if ex["kind"] == "crop" else 1)
            err = pg.evaluate("""([spec, zoom]) => {
                try {
                  const html = spec.kind === 'page' ? TL.renderPage(spec) : TL.renderCrop(spec);
                  const st = document.getElementById('stage'); st.style.zoom = zoom; st.innerHTML = html; return null;
                } catch (e) { return String(e && e.stack || e); }
            }""", [spec, zoom])
            if err:
                print("ERROR", ex["id"], err); continue
            pg.locator("#stage").screenshot(path=str(IMG / (ex["id"] + ".png")))
            print("ok", ex["id"])
        b.close()


if __name__ == "__main__":
    main(set(sys.argv[1:]) or None)
