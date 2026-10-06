package format

import (
	"strings"
	"testing"
)

func TestDoubleQuotedAttributeRoundTrip(t *testing.T) {
	src := "{attribute \"qualified_only\"}\nVAR_GLOBAL\n\tx : INT;\nEND_VAR\n" +
		"{attribute 'strict'}\nTYPE E : (a, b); END_TYPE\n"
	out := formatClean(t, src)
	if !strings.Contains(out, `{attribute "qualified_only"}`) {
		t.Fatalf("double quotes lost:\n%s", out)
	}
	if !strings.Contains(out, `{attribute 'strict'}`) {
		t.Fatalf("single quotes lost:\n%s", out)
	}
	assertIdempotent(t, out)
}
