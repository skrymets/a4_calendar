// Copyright (c) 2026 Sergii Krymets
// SPDX-License-Identifier: MIT

// Command yearly-calendar renders a one-page calendar PDF; it is the Go
// port of yearly_calendar.py with the same options and output.
package main

import "os"

// version comes from the repository's VERSION file: build.ps1 and the
// workflows pass it via -ldflags "-X main.version=...". Plain `go run` and
// `go test` builds report "dev".
var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
