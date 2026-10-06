package interp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSuperInInheritedCode covers SUPER^ inside code declared by a middle FB
// of an EXTENDS chain and run on a most-derived instance: an ACTION called
// qualified and unqualified, and both property accessors. SUPER^ is relative
// to the declaring FB (FB_B), so SUPER^.M() runs FB_A.M.
func TestSuperInInheritedCode(t *testing.T) {
	eng := semRun(t, `
FUNCTION_BLOCK FB_A
VAR_OUTPUT trace : INT; END_VAR
METHOD M : INT
M := 1;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
VAR_OUTPUT viaProp : INT; END_VAR
METHOD M : INT
M := 2;
END_METHOD
ACTION Act
trace := SUPER^.M();
END_ACTION
PROPERTY P : INT
GET
P := SUPER^.M();
END_GET
SET
viaProp := P + SUPER^.M() * 10;
END_SET
END_PROPERTY
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_C EXTENDS FB_B
VAR_OUTPUT inner : INT; END_VAR
METHOD M : INT
M := 3;
END_METHOD
ACTION Run
Act();
inner := trace;
END_ACTION
END_FUNCTION_BLOCK
PROGRAM Prog
VAR c, d : FB_C; got : INT; END_VAR
c.Act();
d.Run();
got := c.P;
c.P := 5;
END_PROGRAM
`)
	assert.Equal(t, int64(1), fbOutput(t, eng.env, "c", "trace").Int, "qualified inherited action")
	assert.Equal(t, int64(1), fbOutput(t, eng.env, "d", "inner").Int, "unqualified inherited action")
	assert.Equal(t, int64(1), progVar(t, eng, "got").Int, "property getter")
	assert.Equal(t, int64(15), fbOutput(t, eng.env, "c", "viaProp").Int, "property setter")
}
