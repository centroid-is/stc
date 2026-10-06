package checker

import (
	"strings"
	"testing"
)

func TestADRToTypedPointer(t *testing.T) {
	src := `TYPE S : STRUCT a : INT; END_STRUCT END_TYPE
FUNCTION_BLOCK FB
VAR_INPUT p : POINTER TO ARRAY[0..3] OF S; END_VAR
END_FUNCTION_BLOCK
PROGRAM MAIN
VAR buf : ARRAY[0..3] OF S; q : POINTER TO ARRAY[0..3] OF S; r : POINTER TO INT; n : INT; f : FB; END_VAR
q := ADR(buf);
f(p := ADR(buf));
r := q;
n := ADR(buf);
END_PROGRAM
`
	diags := runAction(t, src)
	var msgs []string
	for _, d := range diags {
		msgs = append(msgs, d.Message)
	}
	joined := strings.Join(msgs, "\n")
	if strings.Contains(joined, "POINTER TO BYTE to POINTER") || strings.Contains(joined, "cannot pass POINTER TO BYTE") {
		t.Errorf("ADR result rejected for a typed pointer:\n%s", joined)
	}
	if !strings.Contains(joined, "cannot assign POINTER TO ARRAY OF S to POINTER TO INT") {
		t.Errorf("typed pointer mismatch not reported:\n%s", joined)
	}
	if !strings.Contains(joined, "cannot assign POINTER TO BYTE to INT") {
		t.Errorf("ADR into a non-pointer not reported:\n%s", joined)
	}
}

func TestADRToTypedPointerFunctionArg(t *testing.T) {
	src := `TYPE S : STRUCT a : INT; END_STRUCT END_TYPE
FUNCTION F : INT
VAR_INPUT p : POINTER TO S; END_VAR
END_FUNCTION
PROGRAM MAIN
VAR s1 : S; n : INT; END_VAR
n := F(ADR(s1));
n := F(p := ADR(s1));
END_PROGRAM
`
	for _, d := range runAction(t, src) {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
}
