// Package fonts embeds PT Sans Narrow (ParaType, SIL Open Font License 1.1).
// The licence must ship with the binary: see OFL and the --licenses flag.
package fonts

import _ "embed"

// PTSansNarrowKeyword selects the embedded font via --font.
const PTSansNarrowKeyword = "pt-sans-narrow"

// PTSansNarrow is PT_Sans-Narrow-Web-Regular.ttf from google/fonts.
//
//go:embed PT_Sans-Narrow-Web-Regular.ttf
var PTSansNarrow []byte

// OFL is the font's licence text, including the copyright notice.
//
//go:embed OFL.txt
var OFL string
