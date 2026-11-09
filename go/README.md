# yearly-calendar: the Go version

A standalone binary for Windows and Linux. It needs no Python, no libraries and no
installation. It has the same options and draws the same page as the
[Python version](../py/README.md). The page and every option are described in the
[main README](../README.md).

## Download

Releases are on the repository's Releases page, one archive per platform:

| Platform | Archive |
|---|---|
| Windows, x64 | `yearly-calendar-<version>-windows-amd64.zip` |
| Linux, x64 | `yearly-calendar-<version>-linux-amd64.tar.gz` |
| Linux, ARM64 | `yearly-calendar-<version>-linux-arm64.tar.gz` |

Each archive holds the program (`yearly-calendar.exe` or `yearly-calendar`), this README,
and the licence files `OFL.txt` and `THIRD_PARTY_LICENSES.txt`.

### Verify the download

The binaries are not signed, so the SHA-256 checksum published next to each archive is the
integrity check. Compare it with:

```powershell
(Get-FileHash yearly-calendar-0.1.0-windows-amd64.zip -Algorithm SHA256).Hash.ToLower()
```

```sh
sha256sum -c yearly-calendar-0.1.0-linux-amd64.tar.gz.sha256
```

## Run

```powershell
.\yearly-calendar.exe --start 2027-01-01          # -> calendar_2027-01-01.pdf
.\yearly-calendar.exe --start 2026-11-07 --period 90d -o q4.pdf
```

```sh
./yearly-calendar --start 2027-01-01
./yearly-calendar --help
```

Put the program on your `PATH` to call it from anywhere.

### Built-in font

`--font pt-sans-narrow` uses PT Sans Narrow, which is embedded in the binary. It covers Latin
and Cyrillic, so a calendar in Ukrainian needs no font file:

```sh
yearly-calendar --start 2027-01-01 --font pt-sans-narrow \
    --day-letters "ПВСЧПСН" \
    --month-names "СІЧ,ЛЮТ,БЕР,КВІ,ТРА,ЧЕР,ЛИП,СЕР,ВЕР,ЖОВ,ЛИС,ГРУ" \
    --title "КАЛЕНДАР НА РІК"
```

`--font` also takes the path of any TrueType (`.ttf`) file. A file that is not a usable
TrueType font is rejected before anything is written. With a TrueType font, characters
outside the Basic Multilingual Plane, such as emoji, are replaced with U+FFFD. Most fonts
draw that as `�` or as an empty box.

### Version and licences

```sh
yearly-calendar --version     # yearly-calendar 0.1.0
yearly-calendar --licenses    # font and library licences
```

Every PDF the program writes carries the version in its metadata (Creator:
`yearly-calendar <version> (Go)`).

## Differences from the Python version

The options, defaults, validation rules, exit codes (2 for a usage error, 1 for anything
else) and the page itself are the same. The differences are:

- `--font pt-sans-narrow`, `--licenses` are Go-only.
- `--start` accepts only `YYYY-MM-DD`. Python also accepts the compact forms of ISO 8601.
- Some messages are worded differently. For example, an unknown option gives
  `unknown flag: --x` where Python says `unrecognized arguments: --x`. Options cannot be
  abbreviated: `--per 90d` does not work as `--period 90d`.
- An invalid `--weekend` value or colour gives an error message, where Python shows a
  traceback.

## Build from source

You need Go 1.24 or later. From the repository:

```powershell
.\go\build.ps1        # -> go/dist/: windows/amd64, linux/amd64, linux/arm64 + licence files
```

The script stamps the version from the repository's `VERSION` file into the binaries. A plain
`go build` or `go run` works too, but reports the version as `dev`:

```sh
go -C go build -o yearly-calendar ./cmd/yearly-calendar
go -C go build -ldflags "-X main.version=$(cat VERSION)" -o yearly-calendar ./cmd/yearly-calendar
```

Tests, the comparison against the Python version and the release procedure are in
[DEV.md](../DEV.md).
