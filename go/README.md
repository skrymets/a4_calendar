# PinPlan: download, run, build

A standalone binary for Windows and Linux. It needs no libraries and no installation. The
page, every option and the date language of `.dates` files are described in the
[main README](https://github.com/skrymets/pinplan#readme).

## Download

Releases are on the repository's Releases page, one archive per platform:

| Platform | Archive |
|---|---|
| Windows, x64 | `pinplan-<version>-windows-amd64.zip` |
| Linux, x64 | `pinplan-<version>-linux-amd64.tar.gz` |
| Linux, ARM64 | `pinplan-<version>-linux-arm64.tar.gz` |

Each archive holds the program (`pinplan.exe` or `pinplan`), this README,
the project's `LICENSE` (MIT), and the third-party licence files `OFL.txt` and
`THIRD_PARTY_LICENSES.txt`.

### Verify the download

The binaries are not signed, so the SHA-256 checksum published next to each archive is the
integrity check. Compare it with:

```powershell
(Get-FileHash pinplan-1.0.0-windows-amd64.zip -Algorithm SHA256).Hash.ToLower()
```

```sh
sha256sum -c pinplan-1.0.0-linux-amd64.tar.gz.sha256
```

## Run

```powershell
.\pinplan.exe --start 2027-01-01          # -> calendar_2027-01-01.pdf
.\pinplan.exe --start 2026-11-07 --period 90d -o q4.pdf
.\pinplan.exe plan.dates                  # milestones and intervals from a .dates file
```

```sh
./pinplan --start 2027-01-01
./pinplan --help
```

Put the program on your `PATH` to call it from anywhere.

### Built-in font

`--font pt-sans-narrow` uses PT Sans Narrow, which is embedded in the binary. It covers Latin
and Cyrillic, so a calendar in Ukrainian needs no font file:

```sh
pinplan --start 2027-01-01 --font pt-sans-narrow \
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
pinplan --version     # PinPlan 1.0.0
pinplan --licenses    # font and library licences
```

Every PDF the program writes carries the version in its metadata (Creator:
`PinPlan <version>`).

### Exit codes

`0` on success, `2` for a usage error (unknown option, invalid value) or an error in a
milestone or interval declaration or in an option line of a `.dates` file, `1` for anything
else (for example an unreadable `--file`). Errors in files are printed one per line as
`file:line: message`, or `--milestone[2]: message` for the second `--milestone`; a bad
value set in a file names its line (`plan.dates:3: argument --paper: …`). Nothing is
written.
Options cannot be abbreviated: `--per 90d` does not work as `--period 90d`.

## Build from source

You need Go 1.24 or later. From the repository:

```powershell
.\go\build.ps1        # -> go/dist/: windows/amd64, linux/amd64, linux/arm64 + licence files
```

The script stamps the version from the repository's `VERSION` file into the binaries. A plain
`go build` or `go run` works too, but reports the version as `dev`:

```sh
go -C go build -o pinplan ./cmd/pinplan
go -C go build -ldflags "-X main.version=$(cat VERSION)" -o pinplan ./cmd/pinplan
```

Tests and the release procedure are in
[DEV.md](https://github.com/skrymets/pinplan/blob/develop/DEV.md).
