<#
.SYNOPSIS
  Cross-compile yearly-calendar into go/dist (windows/amd64, linux/amd64, linux/arm64).
#>
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $version = git describe --tags --always --dirty 2>$null
    if (-not $version) { $version = 'dev' }
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
    Copy-Item internal/fonts/OFL.txt dist/OFL.txt
}
finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
