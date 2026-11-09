<#
.SYNOPSIS
  Cross-compile yearly-calendar into go/dist (windows/amd64, linux/amd64, linux/arm64).
.DESCRIPTION
  The version stamped into the binaries (--version, PDF Creator) is read from the
  repository's VERSION file, the single source of the version (see DEV.md).
#>
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $version = (Get-Content -Raw ../VERSION).Trim()
    if ($version -notmatch '^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$') {
        throw "VERSION holds '$version', which is not MAJOR.MINOR.PATCH[-prerelease]"
    }
    $targets = @(
        @{ GOOS = 'windows'; GOARCH = 'amd64'; Ext = '.exe' },
        @{ GOOS = 'linux';   GOARCH = 'amd64'; Ext = '' },
        @{ GOOS = 'linux';   GOARCH = 'arm64'; Ext = '' }
    )
    New-Item -ItemType Directory -Force dist | Out-Null
    $env:CGO_ENABLED = '0'
    foreach ($t in $targets) {
        $env:GOOS = $t.GOOS
        $env:GOARCH = $t.GOARCH
        $out = "dist/yearly-calendar-$($t.GOOS)-$($t.GOARCH)$($t.Ext)"
        go build -trimpath -ldflags "-s -w -X main.version=$version" -o $out ./cmd/yearly-calendar
        if ($LASTEXITCODE -ne 0) { throw "build failed for $($t.GOOS)/$($t.GOARCH)" }
        Write-Host "built $out"
    }
    Copy-Item ../LICENSE dist/LICENSE
    Copy-Item internal/fonts/OFL.txt dist/OFL.txt
    Copy-Item cmd/yearly-calendar/THIRD_PARTY_LICENSES.txt dist/THIRD_PARTY_LICENSES.txt
}
finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
