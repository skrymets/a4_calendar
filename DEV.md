# Development and release

## Layout

| Path | What |
|---|---|
| `VERSION` | the version, the only place it is written down |
| `LICENSE` | MIT; copied into every release archive |
| `go/` | the program: module `a4calendar`, binary `yearly-calendar` |
| `go/build.ps1` | cross-compiles windows/amd64, linux/amd64 and linux/arm64 into `go/dist/` |
| `Release-Notes.md` | what changed, one section per version |
| `.github/workflows/ci.yml` | every gate, on each push to and PR into `develop` and `main` |
| `.github/workflows/release.yml` | on a pushed `v*` tag: the CI gates, then packaging and publishing |
| `docs/superpowers/specs/` | design notes |

## Local checks

```powershell
go -C go test ./...                       # unit tests
gofmt -l go                               # must print nothing
go -C go vet ./...
.\go\build.ps1                            # binaries in go/dist/, stamped with VERSION
```

A layout change is checked by rendering a PDF and looking at it, for both a 1 January start
and a mid-year start, which exercise different code paths (12 vs 13 columns, partial months).

## Versioning

The version follows [Semantic Versioning 2.0.0](https://semver.org/) and is written in exactly
one place: the `VERSION` file. Everything else derives from it:

- `go/build.ps1` and the workflows pass it to the Go linker (`-X main.version=…`). A plain
  `go build` or `go run` reports `dev`.
- `--version`, and the Creator field of every PDF (`yearly-calendar <version>`).
- The release title, the asset file names, and the release-notes section that gets published.

The release workflow refuses a tag that disagrees with `VERSION`, and both workflows refuse a
binary that reports anything else.

What a version number promises for this tool, whose contract is its command line and the
printed page:

- **MAJOR**: something you relied on stops working as it did. An option is removed or
  renamed, a default changes, an option that was accepted is now refused, or the page layout
  changes enough to break a printed use (cell sizes, what appears where).
- **MINOR**: something is added and nothing that worked stops working. A new option whose
  default keeps the old behaviour, a new period unit, a new built-in font.
- **PATCH**: a fix that changes nothing you configure or rely on.

While the version is `0.x`, the same intent applies shifted one place: MINOR for what would be
MAJOR, PATCH for the rest. `Release-Notes.md` says so under **Changed** whenever it happens.

A pre-release carries a hyphenated identifier (`0.2.0-rc.1`) and is published as a GitHub
pre-release. Build metadata (`+…`) is not used: the tag already names the commit.

## Releasing

The tag is the release. Pushing `v<version>` is the only action that publishes anything, so
every version that shipped exists in git history under exactly that name.

### 1. Start from a green branch

The branch you release from should have passed CI. Tagging a red branch only moves the
failure later.

### 2. Bump the version

Edit `VERSION`, following the rules above:

```
0.2.0
```

### 3. Write the release notes

Add a section to `Release-Notes.md` headed by the bare version, newest first, with
**Added**, **Changed** and **Fixed** subsections as needed:

```markdown
## 0.2.0

### Added

- ...
```

That section, without its heading, becomes the GitHub release text. The release fails if it is
missing or empty.

### 4. Commit and push

```powershell
git add VERSION Release-Notes.md
git commit -m "Release 0.2.0"
git push
```

Wait for CI to pass on that commit. Everything up to here can be undone with another commit.
The tag is where that stops being true.

### 5. Tag and push the tag

```powershell
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

The leading `v` matters: the workflow starts on tags matching `v*`. File names drop it.

### 6. What the workflow does

1. Runs every CI gate on the tagged tree:
   - `VERSION` is valid SemVer;
   - `gofmt`, `go vet`, `go test`;
   - the binaries are built and report `VERSION`;
   - a smoke test that renders two calendars with the Linux binary.
2. Checks that the tag equals `v` + `VERSION`.
3. Checks that `Release-Notes.md` has a `## <version>` section.
4. Builds the binaries and checks their `--version` again.
5. Packages the assets, each with a `.sha256` in `sha256sum` format:
   - `yearly-calendar-<version>-windows-amd64.zip`;
   - `yearly-calendar-<version>-linux-amd64.tar.gz`;
   - `yearly-calendar-<version>-linux-arm64.tar.gz`.
   - Each archive holds `yearly-calendar[.exe]`, `README.md` (from `go/`), `LICENSE`,
     `OFL.txt` and `THIRD_PARTY_LICENSES.txt`.
6. Creates the release titled `yearly-calendar <version>`. Its text is install and verify
   instructions, then the release-notes section, then GitHub's generated list of changes since
   the previous tag. A tag with a hyphen becomes a pre-release.

Steps 1–4 all run before anything is published, so a failure leaves no release and no assets,
only a tag to delete.

### 7. Check the release

Download an archive and its `.sha256` from the release page, and verify them:

```sh
sha256sum -c yearly-calendar-0.2.0-linux-amd64.tar.gz.sha256
```

```powershell
(Get-FileHash yearly-calendar-0.2.0-windows-amd64.zip -Algorithm SHA256).Hash.ToLower()
```

## When a release fails

Nothing was published, so remove the tag, fix the problem, and tag again:

```powershell
git push origin :refs/tags/v0.2.0   # delete it on the remote first
git tag -d v0.2.0                   # then locally
# fix, commit, push, wait for CI
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

If a release was published and turns out to be wrong, do not reuse its version. Fix the
problem and release the next PATCH version.

## Pinned actions

The workflows use `actions/checkout@v4` and `actions/setup-go@v5`.
They are pinned to major-version tags rather than commit SHAs, so they pick up fixes without
edits, at the cost of trusting the publisher not to move the tag maliciously. Pinning to
full SHAs, for example with Dependabot keeping them current, closes that gap if it matters.
