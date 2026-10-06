package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// semEngine builds an engine for the first PROGRAM in src, parsed as G.st,
// with the file's TYPEs, FBs, GVLs and FUNCTIONs registered.
func semEngine(t *testing.T, src string) *ScanCycleEngine {
	t.Helper()
	eng := gvlEngine(t, "G.st", src)
	res := parser.Parse("G.st", src)
	for _, d := range res.File.Declarations {
		if fd, ok := d.(*ast.FunctionDecl); ok {
			eng.interp.RegisterFunctionDecl(fd)
		}
	}
	return eng
}

// semRun runs one scan of src and returns the engine.
func semRun(t *testing.T, src string) *ScanCycleEngine {
	t.Helper()
	eng := semEngine(t, src)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	return eng
}

// semRunErr runs one scan of src and returns its RuntimeError.
func semRunErr(t *testing.T, src string) error {
	t.Helper()
	eng := semEngine(t, src)
	err := eng.Tick(10 * time.Millisecond)
	require.Error(t, err)
	var rt *RuntimeError
	require.ErrorAs(t, err, &rt)
	return err
}

const fxSrc = `
FUNCTION F_X : INT
VAR_INPUT a : INT; b : INT := 10; END_VAR
F_X := a + b;
END_FUNCTION
`

func TestCallFunction(t *testing.T) {
	t.Run("named positional mixed and defaults", func(t *testing.T) {
		eng := semRun(t, fxSrc+`
PROGRAM P
VAR n1, n2, n3, n4, n5, n6, n7, n8 : INT; END_VAR
n1 := F_X(a := 1, b := 2);
n2 := F_X(1, 2);
n3 := F_X(b := 2, a := 1);
n4 := F_X(1, b := 2);
n5 := F_X(a := 1, 2);
n6 := F_X(a := 1);
n7 := F_X(a := , b := 2);
n8 := F_X(A := 1, B := );
END_PROGRAM
`)
		for name, want := range map[string]int64{"n1": 3, "n2": 3, "n3": 3, "n4": 3, "n5": 3, "n6": 11, "n7": 2, "n8": 11} {
			assert.Equal(t, want, progVar(t, eng, name).Int, name)
		}
	})

	t.Run("outputs and in-outs are written after return", func(t *testing.T) {
		eng := semRun(t, `
TYPE ST_W : STRUCT w : WORD; END_STRUCT END_TYPE
FUNCTION F_Out : INT
VAR_INPUT a : INT; END_VAR
VAR_OUTPUT q : BOOL; END_VAR
VAR_IN_OUT io : INT; END_VAR
VAR t : INT := 100; END_VAR
q := a > 0;
io := io + a;
F_Out := a * 2 + t - 100;
END_FUNCTION
PROGRAM P
VAR n, m, k : INT; flag : BOOL; v : INT := 5; s : ST_W; END_VAR
n := F_Out(a := 1, q => flag, io := v);
m := F_Out(a := 2, q => s.w.2, io := v);
k := F_Out(3, v);
END_PROGRAM
`)
		assert.Equal(t, int64(2), progVar(t, eng, "n").Int)
		assert.Equal(t, int64(4), progVar(t, eng, "m").Int)
		assert.Equal(t, int64(6), progVar(t, eng, "k").Int)
		assert.True(t, progVar(t, eng, "flag").Bool)
		assert.Equal(t, int64(11), progVar(t, eng, "v").Int)
		assert.Equal(t, int64(4), progVar(t, eng, "s").Struct["W"].Int)
	})

	t.Run("statement call of a FUNCTION with named args", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION F_Inc : BOOL
VAR_IN_OUT io : INT; END_VAR
VAR_INPUT by : INT := 1; END_VAR
io := io + by;
F_Inc := TRUE;
END_FUNCTION
PROGRAM P
VAR v : INT; END_VAR
F_Inc(io := v);
F_Inc(io := v, by := 5);
END_PROGRAM
`)
		assert.Equal(t, int64(6), progVar(t, eng, "v").Int)
	})

	t.Run("RETURN ends the function", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION F_R : INT
VAR_INPUT a : INT; END_VAR
F_R := 1;
IF a > 0 THEN RETURN; END_IF
F_R := 2;
END_FUNCTION
PROGRAM P
VAR x, y : INT; END_VAR
x := F_R(a := 1);
y := F_R(0);
END_PROGRAM
`)
		assert.Equal(t, int64(1), progVar(t, eng, "x").Int)
		assert.Equal(t, int64(2), progVar(t, eng, "y").Int)
	})

	t.Run("recursion hits the call depth limit", func(t *testing.T) {
		err := semRunErr(t, `
FUNCTION F_Rec : DINT
VAR_INPUT n : DINT; END_VAR
F_Rec := F_Rec(n := n + 1);
END_FUNCTION
PROGRAM P
VAR r : DINT; END_VAR
r := F_Rec(0);
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "maximum call depth")
	})

	t.Run("body error propagates", func(t *testing.T) {
		err := semRunErr(t, `
FUNCTION F_Div : DINT
VAR_INPUT n : DINT; END_VAR
F_Div := 10 / n;
END_FUNCTION
PROGRAM P
VAR r : DINT; END_VAR
r := F_Div(0);
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "division by zero")
	})
}

func TestNamedArgs(t *testing.T) {
	bad := map[string]string{
		"unknown name":           "n := F_X(c := 1);",
		"too many positional":    "n := F_X(1, 2, 3);",
		"duplicate binding":      "n := F_X(a := 1, a := 2);",
		"output on an input":     "n := F_X(a => n);",
		"unknown output":         "n := F_X(a := 1, z => n);",
		"bad argument value":     "n := F_X(a := undefinedVar);",
		"positional after named": "n := F_X(a := 1, 2, 3);",
	}
	for name, stmt := range bad {
		t.Run(name, func(t *testing.T) {
			semRunErr(t, fxSrc+`
PROGRAM P
VAR n : INT; END_VAR
`+stmt+`
END_PROGRAM
`)
		})
	}

	t.Run("output binding error propagates", func(t *testing.T) {
		semRunErr(t, `
FUNCTION F_O : INT
VAR_OUTPUT q : INT; END_VAR
q := 1;
END_FUNCTION
PROGRAM P
VAR n : INT; arr : ARRAY[0..1] OF INT; END_VAR
n := F_O(q => arr[5]);
END_PROGRAM
`)
	})

	t.Run("FB call statement writes outputs through member, index and bit targets", func(t *testing.T) {
		eng := semRun(t, `
TYPE ST_S : STRUCT m : BOOL; w : WORD; END_STRUCT END_TYPE
FUNCTION_BLOCK FB_Q
VAR_OUTPUT q : BOOL; END_VAR
q := TRUE;
END_FUNCTION_BLOCK
VAR_GLOBAL s : ST_S; END_VAR
PROGRAM P
VAR fb : FB_Q; arr : ARRAY[0..2] OF ST_S; b : BOOL; END_VAR
fb(q => G.s.m);
fb(q => arr[1].w.0);
fb(q => b);
END_PROGRAM
`)
		assert.True(t, gvlVar(t, eng, "G", "s").Struct["M"].Bool)
		assert.Equal(t, int64(1), progVar(t, eng, "arr").Array[1].Struct["W"].Int)
		assert.True(t, progVar(t, eng, "b").Bool)
	})

	t.Run("built-in function rejects named arguments", func(t *testing.T) {
		err := semRunErr(t, `
PROGRAM P
VAR n : INT; END_VAR
n := LIMIT(MN := 0, IN := 5, MX := 10);
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "named arguments")
	})

	t.Run("built-in function accepts trailing positional entries", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_Q
VAR_INPUT a : INT; END_VAR
END_FUNCTION_BLOCK
PROGRAM P
VAR n : INT; END_VAR
n := ABS(-4);
END_PROGRAM
`)
		assert.Equal(t, int64(4), progVar(t, eng, "n").Int)
	})
}
