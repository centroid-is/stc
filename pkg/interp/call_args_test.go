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
VAR_INPUT step : INT := 1; END_VAR
io := io + step;
F_Inc := TRUE;
END_FUNCTION
PROGRAM P
VAR v : INT; END_VAR
F_Inc(io := v);
F_Inc(io := v, step := 5);
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

}

// TestCallEdgeCases covers the aggregate, write-back and error branches of
// the shared binder, METHOD and SUPER^ call paths.
func TestCallEdgeCases(t *testing.T) {
	const types = `
TYPE ST_W : STRUCT w : INT; END_STRUCT END_TYPE
VAR_GLOBAL defS : ST_W; idx : INT := 2; END_VAR
FUNCTION F_Next : INT
F_Next := idx;
idx := idx + 2;
END_FUNCTION
`
	t.Run("aggregate inputs, initialisers and outputs are copies", func(t *testing.T) {
		eng := semRun(t, types+`
FUNCTION F_Agg : INT
VAR_INPUT s : ST_W; END_VAR
VAR_OUTPUT o : ST_W; END_VAR
VAR loc : ST_W := defS; END_VAR
s.w := s.w + 1;
loc.w := 9;
o := s;
F_Agg := s.w + loc.w;
END_FUNCTION
FUNCTION_BLOCK FB_Agg
VAR_OUTPUT o : ST_W; END_VAR
o.w := 4;
END_FUNCTION_BLOCK
PROGRAM P
VAR a, b, c : ST_W; n : INT; fb : FB_Agg; END_VAR
a.w := 1;
n := F_Agg(s := a, o => b);
fb(o => c);
c.w := c.w + 1;
END_PROGRAM
`)
		assert.Equal(t, int64(1), progVar(t, eng, "a").Struct["W"].Int)
		assert.Equal(t, int64(2), progVar(t, eng, "b").Struct["W"].Int)
		assert.Equal(t, int64(11), progVar(t, eng, "n").Int)
		assert.Equal(t, int64(0), gvlVar(t, eng, "G", "defS").Struct["W"].Int)
		assert.Equal(t, int64(5), progVar(t, eng, "c").Struct["W"].Int)
		assert.Equal(t, int64(4), progVar(t, eng, "fb").FBRef.GetMember("o").Struct["W"].Int)
	})

	errs := map[string]string{
		"duplicate output binding": `
FUNCTION F_O : INT
VAR_OUTPUT q : INT; END_VAR
END_FUNCTION
PROGRAM P
VAR a, b : INT; END_VAR
a := F_O(q => a, q => b);
END_PROGRAM
`,
		"in-out write-back target re-evaluates out of range": `
FUNCTION F_IO : INT
VAR_IN_OUT io : INT; END_VAR
io := 50;
END_FUNCTION
PROGRAM P
VAR arr : ARRAY[0..3] OF INT; n : INT; END_VAR
n := F_IO(io := arr[F_Next()]);
END_PROGRAM
`,
		"method in-out write-back target re-evaluates out of range": `
FUNCTION_BLOCK FB_M
METHOD M : INT
VAR_IN_OUT io : INT; END_VAR
io := 50;
END_METHOD
END_FUNCTION_BLOCK
PROGRAM P
VAR arr : ARRAY[0..3] OF INT; n : INT; m : FB_M; END_VAR
n := m.M(io := arr[F_Next()]);
END_PROGRAM
`,
		"built-in argument error": `
PROGRAM P
VAR n : INT; END_VAR
n := ABS(nope);
END_PROGRAM
`,
		"SUPER outside an FB": `
PROGRAM P
VAR n : INT; END_VAR
n := SUPER^.M();
END_PROGRAM
`,
	}
	for name, src := range errs {
		t.Run(name, func(t *testing.T) {
			semRunErr(t, types+src)
		})
	}

	t.Run("RegisterFunctionDecl ignores a declaration without a name", func(t *testing.T) {
		in := New()
		in.RegisterFunctionDecl(nil)
		in.RegisterFunctionDecl(&ast.FunctionDecl{})
		assert.Empty(t, in.FuncDecls)
	})

	t.Run("SUPER^() past the call depth limit", func(t *testing.T) {
		in := New()
		base := &ast.FunctionBlockDecl{Name: &ast.Ident{Name: "FB_A"}}
		inst := NewUserFBInstance("FB_A", base, in, nil)
		in.callDepth = MaxCallDepth
		err := in.runBaseBody(inst, base, ast.Pos{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum call depth")
	})
}

// TestFBCallStmtEdges covers FB call statement branches that only hand-built
// ASTs or aggregate in-outs reach.
func TestFBCallStmtEdges(t *testing.T) {
	const fb = `
TYPE ST_W : STRUCT w : INT; END_STRUCT END_TYPE
FUNCTION_BLOCK FB_IO
VAR_INPUT a : INT; END_VAR
VAR_IN_OUT io : ST_W; END_VAR
io.w := io.w + a;
END_FUNCTION_BLOCK
`
	t.Run("aggregate in-out is copied back", func(t *testing.T) {
		eng := semRun(t, fb+`
PROGRAM P
VAR f : FB_IO; s : ST_W; END_VAR
f(a := 2, io := s);
END_PROGRAM
`)
		assert.Equal(t, int64(2), progVar(t, eng, "s").Struct["W"].Int)
	})

	t.Run("input argument error", func(t *testing.T) {
		semRunErr(t, fb+`
PROGRAM P
VAR f : FB_IO; s : ST_W; END_VAR
f(a := nope, io := s);
END_PROGRAM
`)
	})

	t.Run("in-out write-back error", func(t *testing.T) {
		semRunErr(t, fb+`
VAR_GLOBAL idx : INT := 2; END_VAR
FUNCTION F_Next : INT
F_Next := idx;
idx := idx + 2;
END_FUNCTION
PROGRAM P
VAR f : FB_IO; arr : ARRAY[0..3] OF ST_W; END_VAR
f(a := 1, io := arr[F_Next()]);
END_PROGRAM
`)
	})

	t.Run("positional entry in a hand-built FB call is ignored", func(t *testing.T) {
		in := New()
		env := NewEnv(nil)
		env.Define("t", MakeFBInstanceValue("TON", StdlibFBFactory["TON"]()))
		err := in.execCallStmt(env, &ast.CallStmt{
			Callee: &ast.Ident{Name: "t"},
			Args:   []*ast.CallArg{{Value: &ast.Literal{LitKind: ast.LitBool, Value: "TRUE"}}},
		})
		require.NoError(t, err)
	})

	t.Run("struct member holding an FB instance runs on s.fb()", func(t *testing.T) {
		in := New()
		env := NewEnv(nil)
		ton := MakeFBInstanceValue("TON", StdlibFBFactory["TON"]())
		env.Define("s", Value{Kind: ValStruct, Struct: map[string]Value{"FB": ton}})
		_, err := in.evalMethodCall(env, &ast.MemberAccessExpr{Object: &ast.Ident{Name: "s"}, Member: &ast.Ident{Name: "fb"}}, nil, nil)
		require.NoError(t, err)
		assert.True(t, ton.FBRef.hasRun)
	})

	t.Run("element of a member array is assigned in place", func(t *testing.T) {
		eng := semRun(t, `
TYPE ST_A : STRUCT a : ARRAY[0..2] OF INT; END_STRUCT END_TYPE
PROGRAM P
VAR s : ST_A; END_VAR
s.a[1] := 5;
END_PROGRAM
`)
		assert.Equal(t, int64(5), progVar(t, eng, "s").Struct["A"].Array[1].Int)
	})

	t.Run("LocalFunctions reject named arguments", func(t *testing.T) {
		in := New()
		in.RegisterFunction("MY_FN", func(args []Value, pos ast.Pos) (Value, error) { return IntValue(1), nil })
		_, err := in.evalCall(NewEnv(nil), &ast.CallExpr{
			Callee:    &ast.Ident{Name: "MY_FN"},
			NamedArgs: []*ast.CallArg{{Name: &ast.Ident{Name: "x"}, Value: &ast.Literal{LitKind: ast.LitInt, Value: "1"}}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "named arguments")
	})

	t.Run("DefineAction ignores a nil or unnamed action", func(t *testing.T) {
		env := NewEnv(nil)
		env.DefineAction(nil)
		env.DefineAction(&ast.ActionDecl{})
		assert.Nil(t, env.actions)
	})
}
