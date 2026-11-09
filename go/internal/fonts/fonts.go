// Package fonts embeds PT Sans Narrow (ParaType, SIL Open Font License 1.1).
// The licence must ship with the binary: see OFL and the --licenses flag.
package fonts

import (
	_ "embed"
	"strings"
)

// PTSansNarrowKeyword selects the embedded font via --font.
const PTSansNarrowKeyword = "pt-sans-narrow"

// PTSansNarrow is PT_Sans-Narrow-Web-Regular.ttf from google/fonts.
//
//go:embed PT_Sans-Narrow-Web-Regular.ttf
var PTSansNarrow []byte

//go:embed OFL.txt
var oflRaw string

// OFL is the font's licence text, including the copyright notice (the
// upstream file's UTF-8 BOM is stripped).
var OFL = strings.TrimPrefix(oflRaw, "\ufeff")
