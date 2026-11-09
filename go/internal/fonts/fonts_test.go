package fonts

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmbedded(t *testing.T) {
	if !bytes.HasPrefix(PTSansNarrow, []byte{0, 1, 0, 0}) {
		t.Errorf("PTSansNarrow is not a TrueType font (len %d)", len(PTSansNarrow))
	}
	if !strings.Contains(OFL, "SIL OPEN FONT LICENSE") {
		t.Error("OFL text missing")
	}
}
