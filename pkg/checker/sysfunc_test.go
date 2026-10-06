package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/stretchr/testify/assert"
)

func TestSystemBuiltins(t *testing.T) {
	t.Run("SIZEOF, ADR, shifts and conversions check clean", func(t *testing.T) {
		ds := runAction(t, `
TYPE ST_Regs :
STRUCT
	a : ARRAY[0..3] OF WORD;
END_STRUCT
END_TYPE
FUNCTION_BLOCK FB_X
VAR_OUTPUT q : INT; END_VAR
END_FUNCTION_BLOCK
VAR_GLOBAL
	regs : ST_Regs;
END_VAR
PROGRAM MAIN
VAR
	n : UDINT;
	p : POINTER TO BYTE;
	dw : DWORD;
	w : WORD;
	u : UINT := 7;
	b : BOOL;
END_VAR
n := SIZEOF(regs) + SIZEOF(ST_Regs) + SIZEOF(FB_X) + SIZEOF(regs.a);
p := ADR(regs);
p := ADR(regs.a[1]);
dw := SHL(TO_DWORD(u), 16) OR SHR(dw, 2);
dw := ROL(dw, 1);
w := ROR(UINT_TO_WORD(u), 3);
u := BOOL_TO_UINT(b);
b := UINT_TO_BOOL(u);
END_PROGRAM
`)
		for _, d := range ds {
			assert.NotEqual(t, diag.Error, d.Severity, d.String())
		}
	})

	t.Run("SIZEOF of an undeclared name is still reported", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nVAR n : UDINT; END_VAR\nn := SIZEOF(nope);\nEND_PROGRAM\n")
		var errs int
		for _, d := range ds {
			if d.Severity == diag.Error {
				errs++
			}
		}
		assert.Equal(t, 1, errs)
	})

	t.Run("SHL requires a bit-string", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nVAR r : REAL; END_VAR\nr := SHL(r, 1);\nEND_PROGRAM\n")
		assert.Contains(t, codesOf(ds), CodeWrongArgType)
	})
}
