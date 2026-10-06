package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const refTypes = `
TYPE ST_B : STRUCT x : INT; END_STRUCT END_TYPE
TYPE ST_Set : STRUCT m : INT; p_stat_Batches : ARRAY[0..3] OF ST_B; END_STRUCT END_TYPE
`

func TestRefAssign(t *testing.T) {
	t.Run("ptr.st: REF= to a variable reads and writes through", func(t *testing.T) {
		eng := semRun(t, `
PROGRAM MAIN
VAR n, m : INT; p : POINTER TO INT; r : REFERENCE TO INT; END_VAR
p := ADR(n);
p^ := 5;
r REF= n;
r := 7;
m := n;
n := 9;
n := r + 1;
END_PROGRAM
`)
		assert.Equal(t, int64(7), progVar(t, eng, "m").Int)
		assert.Equal(t, int64(10), progVar(t, eng, "n").Int)
		assert.Equal(t, ValReference, progVar(t, eng, "r").Kind)
	})

	t.Run("REF= to a struct member and an array element", func(t *testing.T) {
		eng := semRun(t, refTypes+`
PROGRAM P
VAR s : ST_Set; arr : ARRAY[0..3] OF INT; i : INT := 1; r, q : REFERENCE TO INT; k : INT; END_VAR
r REF= s.m;
r := 3;
q REF= arr[i];
i := i + 1;
q := 5;
arr[1] := arr[1] + 1;
k := q;
END_PROGRAM
`)
		assert.Equal(t, int64(3), progVar(t, eng, "s").Struct["M"].Int)
		arr := progVar(t, eng, "arr").Array
		assert.Equal(t, int64(6), arr[1].Int)
		assert.Equal(t, int64(0), arr[2].Int)
		assert.Equal(t, int64(6), progVar(t, eng, "k").Int)
	})

	t.Run("index and member writes through a reference keep the reference", func(t *testing.T) {
		eng := semRun(t, refTypes+`
PROGRAM P
VAR
  settings : ST_Set; other : ST_B;
  batches : REFERENCE TO ARRAY[0..3] OF ST_B;
  rs : REFERENCE TO ST_B;
  one : ST_B;
  n, n2, k : INT;
END_VAR
batches REF= settings.p_stat_Batches;
batches[2].x := 1;
n := batches[2].x;
one.x := 7;
batches[1] := one;
settings.p_stat_Batches[3].x := 4;
n2 := batches[3].x;
rs REF= other;
rs.x := 5;
other.x := other.x + 1;
k := rs.x;
END_PROGRAM
`)
		set := progVar(t, eng, "settings").Struct["P_STAT_BATCHES"].Array
		assert.Equal(t, int64(1), set[2].Struct["X"].Int)
		assert.Equal(t, int64(7), set[1].Struct["X"].Int)
		assert.Equal(t, int64(1), progVar(t, eng, "n").Int)
		assert.Equal(t, int64(4), progVar(t, eng, "n2").Int)
		assert.Equal(t, int64(6), progVar(t, eng, "k").Int)
		assert.Equal(t, ValReference, progVar(t, eng, "batches").Kind)
		assert.Equal(t, ValReference, progVar(t, eng, "rs").Kind)
	})

	t.Run("REF= to a GVL member path", func(t *testing.T) {
		eng := semRun(t, `
TYPE ST_Cfg : STRUCT limit : INT; END_STRUCT END_TYPE
VAR_GLOBAL cfg : ST_Cfg; g : INT; END_VAR
PROGRAM P
VAR r, rg : REFERENCE TO INT; k : INT; END_VAR
r REF= G.cfg.limit;
r := 50;
k := r;
rg REF= G.g;
rg := 2;
END_PROGRAM
`)
		assert.Equal(t, int64(50), gvlVar(t, eng, "G", "cfg").Struct["LIMIT"].Int)
		assert.Equal(t, int64(2), gvlVar(t, eng, "G", "g").Int)
		assert.Equal(t, int64(50), progVar(t, eng, "k").Int)
	})

	t.Run("REF= to another reference binds to its target", func(t *testing.T) {
		eng := semRun(t, `
PROGRAM P
VAR n, m : INT; r, r2 : REFERENCE TO INT; END_VAR
r REF= n;
r2 REF= r;
r REF= m;
r2 := 4;
r := 8;
END_PROGRAM
`)
		assert.Equal(t, int64(4), progVar(t, eng, "n").Int)
		assert.Equal(t, int64(8), progVar(t, eng, "m").Int)
	})

	t.Run("REF() function form, plain and path", func(t *testing.T) {
		eng := semRun(t, refTypes+`
PROGRAM P
VAR n : INT; s : ST_Set; r, q : REFERENCE TO INT; END_VAR
r := REF(n);
r := 8;
q := REF(s.p_stat_Batches[1].x);
q := 2;
END_PROGRAM
`)
		assert.Equal(t, int64(8), progVar(t, eng, "n").Int)
		assert.Equal(t, int64(2), progVar(t, eng, "s").Struct["P_STAT_BATCHES"].Array[1].Struct["X"].Int)
	})

	t.Run("REF= inside an FB to instance members and THIS^", func(t *testing.T) {
		eng := semRun(t, `
FUNCTION_BLOCK FB_In
VAR_OUTPUT v : INT; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_R
VAR_OUTPUT n : INT; END_VAR
VAR inner : FB_In; r, q : REFERENCE TO INT; END_VAR
r REF= THIS^.n;
r := 3;
q REF= inner.v;
q := 4;
END_FUNCTION_BLOCK
TYPE ST_H : STRUCT f : FB_In; END_STRUCT END_TYPE
PROGRAM P
VAR x : FB_R; h : ST_H; r : REFERENCE TO INT; END_VAR
x();
r REF= x.n;
r := r + 10;
END_PROGRAM
`)
		x := progVar(t, eng, "x").FBRef
		assert.Equal(t, int64(13), x.GetMember("n").Int)
		assert.Equal(t, int64(4), x.Env.vars["INNER"].FBRef.GetMember("v").Int)
	})

	t.Run("struct member holding a reference", func(t *testing.T) {
		eng := semRun(t, `
TYPE ST_P : STRUCT r : REFERENCE TO INT; END_STRUCT END_TYPE
PROGRAM P
VAR n : INT; s : ST_P; END_VAR
s.r REF= n;
END_PROGRAM
`)
		assert.Equal(t, ValReference, progVar(t, eng, "s").Struct["R"].Kind)
	})

	bad := map[string]string{
		"non-lvalue":            "r REF= n + 1;",
		"bit access":            "r REF= w.3;",
		"undefined target":      "r REF= nope;",
		"undefined ref var":     "nope REF= n;",
		"missing member":        "r REF= s.nope;",
		"index out of range":    "r REF= arr[9];",
		"non-integer index":     "r REF= arr[TRUE];",
		"index of scalar":       "r REF= n[0];",
		"deref of non-pointer":  "r REF= n^;",
		"index error":           "r REF= arr[nope];",
		"base error":            "r REF= nope.x;",
		"GVL member missing":    "r REF= G.nope;",
		"stdlib FB member":      "r REF= t.Q;",
		"missing FB member":     "r REF= fb.nope;",
		"REF() of a non-lvalue": "r := REF(1);",
	}
	for name, stmt := range bad {
		t.Run(name, func(t *testing.T) {
			semRunErr(t, refTypes+`
FUNCTION_BLOCK FB_E
VAR_OUTPUT v : INT; END_VAR
END_FUNCTION_BLOCK
VAR_GLOBAL gg : INT; END_VAR
PROGRAM P
VAR n : INT; w : WORD; s : ST_Set; arr : ARRAY[0..3] OF INT; r : REFERENCE TO INT; t : TON; fb : FB_E; END_VAR
`+stmt+`
END_PROGRAM
`)
		})
	}

	t.Run("REF= through a pointer dereference", func(t *testing.T) {
		eng := semRun(t, `
PROGRAM P
VAR n : INT; p : POINTER TO INT; r : REFERENCE TO INT; END_VAR
p := ADR(n);
r REF= p^;
r := 6;
END_PROGRAM
`)
		assert.Equal(t, int64(6), progVar(t, eng, "n").Int)
	})
}

func TestRefDangling(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("s", Value{Kind: ValStruct, Struct: map[string]Value{"M": IntValue(1)}})
	env.Define("a", Value{Kind: ValArray, Array: []Value{IntValue(1)}})
	env.Define("n", IntValue(1))

	paths := map[string]*RefPath{
		"missing member":          {Env: env, Var: "S", Steps: []RefStep{{Member: "GONE"}}},
		"index past end":          {Env: env, Var: "A", Steps: []RefStep{{IsIndex: true, Index: 5}}},
		"member of int":           {Env: env, Var: "N", Steps: []RefStep{{Member: "X"}}},
		"index of int":            {Env: env, Var: "N", Steps: []RefStep{{IsIndex: true, Index: 0}, {Member: "X"}}},
		"missing root":            {Env: env, Var: "GONE"},
		"missing root with steps": {Env: env, Var: "GONE", Steps: []RefStep{{Member: "X"}}},
	}
	for name, p := range paths {
		t.Run(name, func(t *testing.T) {
			env.Define("r", Value{Kind: ValReference, Ref: p})
			_, err := interp.evalIdent(env, &ast.Ident{Name: "r"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "dangling reference")
			err = interp.assignToTarget(env, &ast.Ident{Name: "r"}, IntValue(2))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "dangling reference")
		})
	}

	t.Run("String names the path", func(t *testing.T) {
		v := Value{Kind: ValReference, Ref: &RefPath{Env: env, Var: "S", Steps: []RefStep{{Member: "M"}, {IsIndex: true, Index: 2}}}}
		assert.Equal(t, "REF(S.M[2])", v.String())
	})
}
