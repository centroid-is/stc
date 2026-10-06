package interp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnqualifiedMethod(t *testing.T) {
	t.Run("FB body and action call a METHOD of the same instance", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_Counter
VAR_INPUT big : BOOL; END_VAR
VAR_OUTPUT n : INT; END_VAR
IF big THEN Inc(step := 5); ELSE Inc(); END_IF
METHOD Inc : BOOL
VAR_INPUT step : INT := 1; END_VAR
n := n + step;
Inc := TRUE;
END_METHOD
ACTION A_Bump
Inc();
END_ACTION
END_FUNCTION_BLOCK
PROGRAM P
VAR c, d, e : FB_Counter; END_VAR
c();
c();
d(big := TRUE);
e.A_Bump();
END_PROGRAM
`)
		assert.Equal(t, int64(2), fbOutput(t, eng.env, "c", "n").Int)
		assert.Equal(t, int64(5), fbOutput(t, eng.env, "d", "n").Int)
		assert.Equal(t, int64(1), fbOutput(t, eng.env, "e", "n").Int)
	})

	t.Run("member method calls bind named and mixed arguments", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_M
VAR_OUTPUT n : INT; q : BOOL; END_VAR
METHOD Sub : INT
VAR_INPUT a : INT; b : INT := 1; END_VAR
VAR_OUTPUT neg : BOOL; END_VAR
Sub := a - b;
neg := a < b;
END_METHOD
METHOD Set : BOOL
VAR_INPUT v : INT; END_VAR
n := v;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR m : FB_M; r1, r2, r3 : INT; neg : BOOL; END_VAR
r1 := m.Sub(b := 2, a := 7);
r2 := m.Sub(7);
r3 := m.Sub(1, b := 3, neg => neg);
m.Set(v := 9);
END_PROGRAM
`)
		assert.Equal(t, int64(5), progVar(t, eng, "r1").Int)
		assert.Equal(t, int64(6), progVar(t, eng, "r2").Int)
		assert.Equal(t, int64(-2), progVar(t, eng, "r3").Int)
		assert.True(t, progVar(t, eng, "neg").Bool)
		assert.Equal(t, int64(9), fbOutput(t, eng.env, "m", "n").Int)
	})

	t.Run("method argument errors are RuntimeErrors", func(t *testing.T) {
		err := semRunErr(t, `
FUNCTION_BLOCK FB_M
METHOD Sub : INT
VAR_INPUT a : INT; END_VAR
Sub := a;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR m : FB_M; r : INT; END_VAR
r := m.Sub(z := 1);
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "no input parameter 'z'")
	})

	t.Run("FB body does not see an action of the enclosing PROGRAM", func(t *testing.T) {
		err := semRunErr(t, `
FUNCTION_BLOCK FB_X
PAct();
END_FUNCTION_BLOCK
PROGRAM P
VAR x : FB_X; n : INT; END_VAR
x();
ACTION PAct
n := n + 1;
END_ACTION
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "undefined function: PACT")
	})

	t.Run("self-recursive method hits the call depth limit", func(t *testing.T) {
		err := semRunErr(t, `
FUNCTION_BLOCK FB_R
METHOD R : INT
R := R() + 1;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR x : FB_R; n : INT; END_VAR
n := x.R();
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "maximum call depth")
	})
}

func TestThisSuper(t *testing.T) {
	t.Run("THIS^ reads, writes and calls on the current instance", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_T
VAR_OUTPUT n : INT; seen : INT; viaAct : INT; END_VAR
THIS^.n := 7;
seen := THIS^.Get();
ACTION A
viaAct := THIS^.Get() + THIS^.n;
END_ACTION
METHOD Get : INT
Get := THIS^.n * 2;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR t : FB_T; END_VAR
t();
t.A();
END_PROGRAM
`)
		assert.Equal(t, int64(7), fbOutput(t, eng.env, "t", "n").Int)
		assert.Equal(t, int64(14), fbOutput(t, eng.env, "t", "seen").Int)
		assert.Equal(t, int64(21), fbOutput(t, eng.env, "t", "viaAct").Int)
	})

	const baseDerived = `
FUNCTION_BLOCK FB_Base
VAR_OUTPUT n : INT; END_VAR
n := n + 100;
METHOD M : INT
M := 1;
END_METHOD
METHOD Add : BOOL
VAR_INPUT k : INT; END_VAR
n := n + k;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
VAR_OUTPUT got : INT; END_VAR
SUPER^();
Add(k := 1);
got := THIS^.M();
METHOD M : INT
M := SUPER^.M() + 10;
END_METHOD
END_FUNCTION_BLOCK
`

	t.Run("SUPER^ runs the base body and base methods on the same instance", func(t *testing.T) {
		eng := semRun(t, baseDerived+`
PROGRAM P
VAR d : FB_Derived; r : INT; END_VAR
d();
r := d.M();
END_PROGRAM
`)
		assert.Equal(t, int64(101), fbOutput(t, eng.env, "d", "n").Int)
		assert.Equal(t, int64(11), fbOutput(t, eng.env, "d", "got").Int)
		assert.Equal(t, int64(11), progVar(t, eng, "r").Int)
	})

	t.Run("SUPER^ is relative to the declaring FB in a three-level chain", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_A
METHOD M : INT
M := 1;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
METHOD M : INT
M := SUPER^.M() + 10;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_C EXTENDS FB_B
METHOD M : INT
M := SUPER^.M() + 100;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR c : FB_C; b : FB_B; rc, rb : INT; END_VAR
rc := c.M();
rb := b.M();
END_PROGRAM
`)
		assert.Equal(t, int64(111), progVar(t, eng, "rc").Int)
		assert.Equal(t, int64(11), progVar(t, eng, "rb").Int)
	})

	t.Run("SUPER^.M as a call statement with named arguments", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_A
VAR_OUTPUT n : INT; END_VAR
METHOD Put : BOOL
VAR_INPUT v : INT; END_VAR
n := v;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
METHOD Put : BOOL
VAR_INPUT v : INT; END_VAR
SUPER^.Put(v := v * 2);
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR b : FB_B; END_VAR
b.Put(v := 4);
END_PROGRAM
`)
		assert.Equal(t, int64(8), fbOutput(t, eng.env, "b", "n").Int)
	})

	errCases := map[string]struct{ src, msg string }{
		"THIS outside an FB": {`
PROGRAM P
VAR n : INT; END_VAR
n := THIS^.n;
END_PROGRAM
`, "THIS"},
		"SUPER without EXTENDS": {`
FUNCTION_BLOCK FB_A
VAR n : INT; END_VAR
n := SUPER^.M();
END_FUNCTION_BLOCK
PROGRAM P
VAR a : FB_A; END_VAR
a();
END_PROGRAM
`, "SUPER"},
		"SUPER^() without EXTENDS": {`
FUNCTION_BLOCK FB_A
SUPER^();
END_FUNCTION_BLOCK
PROGRAM P
VAR a : FB_A; END_VAR
a();
END_PROGRAM
`, "SUPER"},
		"SUPER^() with arguments": {`
FUNCTION_BLOCK FB_A
VAR_INPUT x : INT; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
SUPER^(1);
END_FUNCTION_BLOCK
PROGRAM P
VAR b : FB_B; END_VAR
b();
END_PROGRAM
`, "takes no arguments"},
		"base method missing": {`
FUNCTION_BLOCK FB_A
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
VAR n : INT; END_VAR
n := SUPER^.Nope();
END_FUNCTION_BLOCK
PROGRAM P
VAR b : FB_B; END_VAR
b();
END_PROGRAM
`, "Nope"},
		"SUPER and THIS recursion is bounded": {`
FUNCTION_BLOCK FB_A
METHOD M : INT
M := THIS^.M();
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
METHOD M : INT
M := SUPER^.M();
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR b : FB_B; n : INT; END_VAR
n := b.M();
END_PROGRAM
`, "maximum call depth"},
		"SUPER^() recursion is bounded": {`
FUNCTION_BLOCK FB_A
THIS^();
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
SUPER^();
END_FUNCTION_BLOCK
PROGRAM P
VAR b : FB_B; END_VAR
b();
END_PROGRAM
`, "maximum call depth"},
		"calling a non-FB expression": {`
PROGRAM P
VAR n : INT; p : POINTER TO INT; END_VAR
p := ADR(n);
p^();
END_PROGRAM
`, "unsupported call target"},
	}
	for name, c := range errCases {
		t.Run(name, func(t *testing.T) {
			err := semRunErr(t, c.src)
			assert.Contains(t, err.Error(), c.msg)
		})
	}

	t.Run("THIS evaluates to a pointer to the instance", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_T
VAR_OUTPUT n : INT; END_VAR
n := n + 1;
END_FUNCTION_BLOCK
PROGRAM P
VAR t : FB_T; END_VAR
t();
END_PROGRAM
`)
		inst := progVar(t, eng, "t").FBRef
		require.NotNil(t, inst)
		assert.Same(t, inst, inst.Env.CurrentFB())
	})
}
