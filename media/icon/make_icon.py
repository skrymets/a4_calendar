"""Builds the Windows icon of pinplan.exe from the drawings pinplan_<N>.svg in this folder.

Writes two files:
  media/icon/pinplan.ico                    the icon, one image per drawing size
  go/cmd/pinplan/rsrc_windows_amd64.syso    the same images as a Windows resource

`go build` links a *_windows_amd64.syso in the main package into Windows/amd64 builds
only, so building needs no extra tool; run this script only when the drawings change,
and commit both files.

Each SVG is rasterized at its own size (N x N, transparent background) by the installed
Microsoft Edge through Playwright, so no browser download is needed. Sizes below 256 are
stored as 32-bit BMP images (readable by every Windows version and resource tool), 256 as
PNG, as Windows itself does.

Usage (from anywhere):  python media/icon/make_icon.py
"""
import base64
import io
import pathlib
import re
import struct

from PIL import Image
from playwright.sync_api import sync_playwright

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent.parent
ICO = HERE / "pinplan.ico"
SYSO = ROOT / "go" / "cmd" / "pinplan" / "rsrc_windows_amd64.syso"

RT_ICON, RT_GROUP_ICON = 3, 14
ICON_ID = 1  # the application icon: the lowest group id is the one Explorer shows
LANG = 0x0409  # en-US


# ---------------------------------------------------------------- images

def rasterize(page, svg: pathlib.Path, n: int) -> Image.Image:
    data = base64.b64encode(svg.read_bytes()).decode("ascii")
    page.set_viewport_size({"width": n, "height": n})
    page.set_content(
        '<html><body style="margin:0;background:transparent">'
        f'<img src="data:image/svg+xml;base64,{data}" width="{n}" height="{n}" style="display:block">'
        "</body></html>"
    )
    page.wait_for_load_state("load")
    png = page.screenshot(omit_background=True, clip={"x": 0, "y": 0, "width": n, "height": n})
    return Image.open(io.BytesIO(png)).convert("RGBA")


def bmp_image(im: Image.Image) -> bytes:
    """A 32-bit DIB as stored in an icon: header with double height, BGRA rows bottom-up, AND mask."""
    n = im.width
    header = struct.pack("<IiiHHIIiiII", 40, n, 2 * n, 1, 32, 0, 0, 0, 0, 0, 0)
    px = im.load()
    rows = [
        b"".join(struct.pack("BBBB", b, g, r, a) for r, g, b, a in (px[x, y] for x in range(n)))
        for y in range(n - 1, -1, -1)
    ]
    stride = ((n + 31) // 32) * 4
    mask = []
    for y in range(n - 1, -1, -1):
        bits = bytearray(stride)
        for x in range(n):
            if px[x, y][3] == 0:  # transparent pixel
                bits[x // 8] |= 0x80 >> (x % 8)
        mask.append(bytes(bits))
    return header + b"".join(rows) + b"".join(mask)


def png_image(im: Image.Image) -> bytes:
    buf = io.BytesIO()
    im.save(buf, format="PNG", optimize=True)
    return buf.getvalue()


def entry_head(n: int, size: int) -> bytes:
    """The fields an .ico entry and a group-icon entry share (0 means 256)."""
    side = 0 if n >= 256 else n
    return struct.pack("<BBBBHHI", side, side, 0, 0, 1, 32, size)


# ---------------------------------------------------------------- .ico

def ico_file(images: list[tuple[int, bytes]]) -> bytes:
    out = bytearray(struct.pack("<HHH", 0, 1, len(images)))
    offset = 6 + 16 * len(images)
    for n, blob in images:
        out += entry_head(n, len(blob)) + struct.pack("<I", offset)
        offset += len(blob)
    for _, blob in images:
        out += blob
    return bytes(out)


# ---------------------------------------------------------------- .syso (COFF object, one .rsrc section)

def align(b: bytearray, k: int = 8) -> None:
    b += b"\0" * (-len(b) % k)


def directory(entries: int) -> bytes:
    return struct.pack("<IIHHHH", 0, 0, 0, 0, 0, entries)


def rsrc_section(images: list[tuple[int, bytes]]) -> tuple[bytes, list[int]]:
    """The resource tree: type -> id -> language -> data. Returns the section and the
    offsets of the data entries' RVA fields, which the linker relocates."""
    group = bytearray(struct.pack("<HHH", 0, 1, len(images)))
    for i, (n, blob) in enumerate(images, 1):
        group += entry_head(n, len(blob)) + struct.pack("<H", i)
    # leaves in id order: RT_ICON 1..k, then RT_GROUP_ICON ICON_ID
    leaves = [(RT_ICON, i, blob) for i, (_, blob) in enumerate(images, 1)] + [(RT_GROUP_ICON, ICON_ID, bytes(group))]
    types = [RT_ICON, RT_GROUP_ICON]

    # sizes of the tree, laid out level by level, then data entries, then data
    root = 16 + 8 * len(types)
    level2 = sum(16 + 8 * sum(1 for t, _, _ in leaves if t == ty) for ty in types)
    level3 = len(leaves) * (16 + 8)
    entries_at = root + level2 + level3
    data_at = entries_at + 16 * len(leaves)

    sec = bytearray()
    sec += directory(len(types))
    off = root
    type_dirs = {}
    for ty in types:
        sec += struct.pack("<II", ty, 0x80000000 | off)
        type_dirs[ty] = off
        off += 16 + 8 * sum(1 for t, _, _ in leaves if t == ty)
    lang_dir = root + level2
    for ty in types:
        mine = [(i, k) for k, (t, i, _) in enumerate(leaves) if t == ty]
        sec += directory(len(mine))
        for i, k in mine:
            sec += struct.pack("<II", i, 0x80000000 | (lang_dir + 24 * k))
    for k in range(len(leaves)):
        sec += directory(1) + struct.pack("<II", LANG, entries_at + 16 * k)

    relocs = []
    data = bytearray()
    for _, _, blob in leaves:
        relocs.append(len(sec))
        sec += struct.pack("<IIII", data_at + len(data), len(blob), 0, 0)
        data += blob
        align(data)
    sec += data
    return bytes(sec), relocs


def syso_file(images: list[tuple[int, bytes]]) -> bytes:
    sec, relocs = rsrc_section(images)
    raw_at = 20 + 40
    reloc_at = raw_at + len(sec)
    sym_at = reloc_at + 10 * len(relocs)
    IMAGE_FILE_MACHINE_AMD64, IMAGE_REL_AMD64_ADDR32NB = 0x8664, 3
    out = bytearray()
    out += struct.pack("<HHIIIHH", IMAGE_FILE_MACHINE_AMD64, 1, 0, sym_at, 1, 0, 0)
    out += struct.pack("<8sIIIIIIHHI", b".rsrc", 0, 0, len(sec), raw_at, reloc_at, 0, len(relocs), 0,
                       0x40000040)  # initialized data, readable
    out += sec
    for r in relocs:
        out += struct.pack("<IIH", r, 0, IMAGE_REL_AMD64_ADDR32NB)  # relative to symbol 0, the section
    out += struct.pack("<8sIhHBB", b".rsrc", 0, 1, 0, 3, 0)  # section symbol, static
    out += struct.pack("<I", 4)  # empty string table
    return bytes(out)


# ---------------------------------------------------------------- main

def main() -> None:
    svgs = sorted(
        (int(m.group(1)), p) for p in HERE.glob("pinplan_*.svg")
        if (m := re.fullmatch(r"pinplan_(\d+)\.svg", p.name))
    )
    if not svgs:
        raise SystemExit("no pinplan_<N>.svg files next to this script")
    with sync_playwright() as p:
        browser = p.chromium.launch(channel="msedge")
        page = browser.new_page(device_scale_factor=1)
        rendered = [(n, rasterize(page, svg, n)) for n, svg in svgs]
        browser.close()
    images = [(n, png_image(im) if n >= 256 else bmp_image(im)) for n, im in rendered]

    ICO.write_bytes(ico_file(images))
    SYSO.write_bytes(syso_file(images))
    sizes = ", ".join(str(n) for n, _ in images)
    print(f"wrote {ICO.relative_to(ROOT)} and {SYSO.relative_to(ROOT)}: sizes {sizes}")


if __name__ == "__main__":
    main()
