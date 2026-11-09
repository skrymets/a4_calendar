// Command yearly-calendar renders a one-page calendar PDF; it is the Go
// port of yearly_calendar.py with the same options and output.
package main

import "os"

var version = "dev" // set by build.ps1 via -ldflags "-X main.version=..."

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
