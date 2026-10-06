package interp

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseRT parses src as file name and fails on parse diagnostics.
func parseRT(t *testing.T, name, src string) *ast.SourceFile {
	t.Helper()
	res := parser.Parse(name, src)
	require.Empty(t, res.Diags, "parse diagnostics")
	return res.File
}

const rtLib = `
VAR_GLOBAL CONSTANT
	N : INT := 3;
END_VAR
`

const rtTypes = `
TYPE E_State : (Idle, Run := 5, Stop); END_TYPE
TYPE ST_Pt :
STRUCT
	x : INT := 7;
	y : REAL;
	State : E_State;
END_STRUCT
END_TYPE
FUNCTION Twice : INT
VAR_INPUT a : INT; END_VAR
Twice := a * 2;
END_FUNCTION
FUNCTION_BLOCK FB_Count
VAR_INPUT enable : BOOL; END_VAR
VAR_OUTPUT n : INT; END_VAR
VAR pt : ST_Pt; END_VAR
IF enable THEN n := n + 1; END_IF
END_FUNCTION_BLOCK
`

const rtGVL = `
VAR_GLOBAL
	shared : INT;
	arr : ARRAY[1..Lib.N] OF ST_Pt := [(x := 1), (x := 2), (x := 3)];
	fb : FB_Count;
	w : WORD := 16#9;
END_VAR
`

const rtProgs = `
PROGRAM PA
VAR
	k : INT := 2;
	limit : INT := 10;
END_VAR
VAR CONSTANT
	C : INT := 4;
END_VAR
GVL.shared := Twice(k);
GVL.fb(enable := TRUE);
END_PROGRAM
PROGRAM PB
VAR
	seen : INT;
	t : TON;
	p : POINTER TO INT;
	r : REFERENCE TO INT;
	s : STRING := 'hi';
	d : TIME := T#5s;
END_VAR
seen := GVL.shared;
r REF= seen;
t(IN := TRUE, PT := T#20ms);
END_PROGRAM
`

// newRT builds the standard runtime fixture: a library file with a constant
// GVL, a types file, a GVL file and a programs file.
func newRT(t *testing.T) *Runtime {
	t.Helper()
	lib := parseRT(t, "Lib.st", rtLib)
	types := parseRT(t, "types.st", rtTypes)
	gvl := parseRT(t, "GVL.st", rtGVL)
	progs := parseRT(t, "progs.st", rtProgs)
	rt, err := NewRuntime([]*ast.SourceFile{types, gvl, progs}, RuntimeOpts{LibraryFiles: []*ast.SourceFile{lib}})
	require.NoError(t, err)
	return rt
}

func TestRegisterFiles(t *testing.T) {
	interp := New()
	types := parseRT(t, "types.st", rtTypes)
	lib := parseRT(t, "Lib.st", rtLib)
	gvl := parseRT(t, "GVL.st", rtGVL)
	require.NoError(t, interp.RegisterFiles([]*ast.SourceFile{lib, types, gvl, nil}))
	assert.Contains(t, interp.FuncDecls, "TWICE")
	assert.Contains(t, interp.FBDecls, "FB_COUNT")
	assert.Contains(t, interp.TypeDecls, "ST_PT")
	assert.Contains(t, interp.EnumDefs, "E_STATE")
	g := interp.lookupGVL("GVL")
	require.NotNil(t, g)
	arr, ok := g.GetLocal("ARR")
	require.True(t, ok)
	assert.Equal(t, 1, arr.ArrayLow)
	assert.Len(t, arr.Array, 4)
	assert.Equal(t, int64(3), arr.Array[3].Struct["X"].Int)

	t.Run("type default", func(t *testing.T) {
		in := New()
		f := parseRT(t, "T.st", "TYPE T_Speed : INT := 50; END_TYPE\nVAR_GLOBAL v : T_Speed; END_VAR\n")
		require.NoError(t, in.RegisterFiles([]*ast.SourceFile{f}))
		v, _ := in.lookupGVL("T").GetLocal("V")
		assert.Equal(t, int64(50), v.Int)
	})
	t.Run("init error", func(t *testing.T) {
		in := New()
		f := parseRT(t, "G.st", "VAR_GLOBAL a : ARRAY[1..Nope.X] OF INT; END_VAR\n")
		assert.Error(t, in.RegisterFiles([]*ast.SourceFile{f}))
	})
}

func TestNewRuntime(t *testing.T) {
	rt := newRT(t)
	require.NotNil(t, rt.Interpreter())
	// Programs are initialised eagerly, before any Tick.
	e := rt.Engine("pa")
	require.NotNil(t, e)
	v, ok := e.env.GetLocal("LIMIT")
	require.True(t, ok)
	assert.Equal(t, int64(10), v.Int)
	assert.Same(t, e, rt.Engine("PA"))
	assert.Nil(t, rt.Engine("nope"))
	assert.NotNil(t, rt.Engine("PB"))
	assert.True(t, rt.consts["PA.C"])
	assert.True(t, rt.consts["LIB.N"])
	assert.False(t, rt.consts["PA.K"])

	t.Run("library first", func(t *testing.T) {
		// Without the library file the bound Lib.N is unknown.
		types := parseRT(t, "types.st", rtTypes)
		gvl := parseRT(t, "GVL.st", rtGVL)
		_, err := NewRuntime([]*ast.SourceFile{types, gvl})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Lib.N")
	})
	t.Run("program init error", func(t *testing.T) {
		f := parseRT(t, "p.st", "PROGRAM P\nVAR a : ARRAY[0..Q.Z] OF INT; END_VAR\nEND_PROGRAM\n")
		_, err := NewRuntime([]*ast.SourceFile{f})
		require.Error(t, err)
	})
	t.Run("empty", func(t *testing.T) {
		rt, err := NewRuntime(nil)
		require.NoError(t, err)
		require.NoError(t, rt.Tick(time.Millisecond))
	})
}

func TestRuntimeTick(t *testing.T) {
	rt := newRT(t)
	require.NoError(t, rt.Tick(10*time.Millisecond))
	// PA writes GVL.shared, PB reads it in the same Tick.
	v, _ := rt.Engine("PB").env.GetLocal("SEEN")
	assert.Equal(t, int64(4), v.Int)
	// The interpreter clock advances once per Tick, not once per program.
	assert.Equal(t, 10*time.Millisecond, rt.Interpreter().Clock())
	require.NoError(t, rt.Tick(10*time.Millisecond))
	assert.Equal(t, 20*time.Millisecond, rt.Interpreter().Clock())
	tv, _ := rt.Engine("PB").env.GetLocal("T")
	assert.True(t, tv.FBRef.GetMember("Q").Bool, "TON elapsed after two ticks of 10ms")

	t.Run("error", func(t *testing.T) {
		f := parseRT(t, "p.st", "PROGRAM P\nVAR a : ARRAY[0..1] OF INT; i : INT := 5; END_VAR\na[i] := 1;\nEND_PROGRAM\n")
		rt, err := NewRuntime([]*ast.SourceFile{f})
		require.NoError(t, err)
		err = rt.Tick(time.Millisecond)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "P"))
	})
}

func mustGet(t *testing.T, rt *Runtime, path string) Value {
	t.Helper()
	v, err := rt.Get(path)
	require.NoError(t, err, path)
	return v
}

func TestRuntimeGet(t *testing.T) {
	rt := newRT(t)
	// Cycle 0: initial values are visible before any Tick.
	assert.Equal(t, int64(10), mustGet(t, rt, "pa.LIMIT").Int)
	assert.Equal(t, int64(2), mustGet(t, rt, "GVL.arr[2].x").Int)
	assert.Equal(t, int64(3), mustGet(t, rt, "gvl.ARR[3].X").Int)
	assert.Equal(t, "hi", mustGet(t, rt, "PB.s").Str)
	_, err := rt.Get("PB.r")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unbound reference")

	require.NoError(t, rt.Tick(10*time.Millisecond))
	assert.Equal(t, int64(4), mustGet(t, rt, "GVL.shared").Int)
	assert.Equal(t, int64(1), mustGet(t, rt, "GVL.fb.n").Int, "FB hop")
	assert.Equal(t, int64(7), mustGet(t, rt, "GVL.fb.pt.x").Int, "struct inside FB")
	assert.Equal(t, int64(4), mustGet(t, rt, "PB.r").Int, "reference reads through")
	assert.Equal(t, 20*time.Millisecond, mustGet(t, rt, "PB.t.PT").Time, "stdlib input")
	assert.Equal(t, 10*time.Millisecond, mustGet(t, rt, "PB.t.ET").Time, "stdlib output")
	assert.False(t, mustGet(t, rt, "PB.t.Q").Bool)
	// w = 16#9: bits 0 and 3 set.
	assert.True(t, mustGet(t, rt, "GVL.w.3").Bool)
	assert.False(t, mustGet(t, rt, "GVL.w.1").Bool)
	assert.Equal(t, types.KindBOOL, mustGet(t, rt, "GVL.w.0").IECType)

	errCases := map[string]string{
		"":                        "empty path",
		"Nope.x":                  "unknown root",
		"GVL":                     "names a root",
		"GVL.missing":             "unknown variable",
		"GVL.arr[0]":              "out of range",
		"GVL.arr[4]":              "out of range",
		"GVL.arr[1,2]":            "multi-dimensional",
		"GVL.arr[1].nope":         "no member",
		"GVL.shared[1]":           "not an array",
		"GVL.shared.x":            "no member",
		"GVL.w.16":                "bit 16 out of range",
		"GVL.w.1.2":               "bit access must be last",
		"PB.s.0":                  "bit access",
		"PB.t.Nope":               "no member",
		"PB.t.Q.x":                "no member",
		"PB.p.x":                  "pointer",
		"GVL.fb.zz":               "no member",
		"3abc":                    "identifier",
		"GVL..x":                  "expected identifier",
		"GVL.arr[x]":              "invalid array index",
		"GVL.arr[1":               "expected ']'",
		"GVL.arr$":                "unexpected character",
		"GVL.w.99999999999999999": "invalid bit",
		"GVL." + strings.Repeat("a", 1100): "longer than",
	}
	for path, want := range errCases {
		_, err := rt.Get(path)
		if assert.Error(t, err, path) {
			assert.Contains(t, err.Error(), want, path)
		}
	}
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func TestToJSON(t *testing.T) {
	rt := newRT(t)
	require.NoError(t, rt.Tick(10*time.Millisecond))
	j := func(path string) string { return marshal(t, rt.ToJSON(mustGet(t, rt, path))) }
	assert.Equal(t, `{"x":2,"y":0,"State":"Idle"}`, j("GVL.arr[2]"))
	assert.Equal(t, `[{"x":1,"y":0,"State":"Idle"},{"x":2,"y":0,"State":"Idle"},{"x":3,"y":0,"State":"Idle"}]`, j("GVL.arr"))
	assert.Equal(t, `{"enable":true,"n":1,"pt":{"x":7,"y":0,"State":"Idle"}}`, j("GVL.fb"))
	assert.Equal(t, `{"IN":true,"PT":{"ms":20,"iso":"PT0.02S"},"Q":false,"ET":{"ms":10,"iso":"PT0.01S"}}`, j("PB.t"))
	assert.Equal(t, `{"ms":5000,"iso":"PT5S"}`, j("PB.d"))
	assert.Equal(t, `"hi"`, j("PB.s"))
	assert.Equal(t, `true`, j("GVL.w.3"))
	assert.Equal(t, `9`, j("GVL.w"))

	cases := []struct {
		v    Value
		want string
	}{
		{Value{Kind: ValInt, Int: -1, IECType: types.KindULINT}, `18446744073709551615`},
		{Value{Kind: ValInt, Int: 99, IECType: types.KindINT, Enum: "E_STATE"}, `99`},
		{Value{Kind: ValInt, Int: 5, IECType: types.KindINT, Enum: "E_STATE"}, `"Run"`},
		{RealValue(math.NaN()), `"NaN"`},
		{RealValue(math.Inf(1)), `"+Inf"`},
		{RealValue(math.Inf(-1)), `"-Inf"`},
		{RealValue(1.5), `1.5`},
		{TimeValue(0), `{"ms":0,"iso":"PT0S"}`},
		{TimeValue(-(90*time.Minute + 1500*time.Millisecond)), `{"ms":-5401500,"iso":"-PT1H30M1.5S"}`},
		{TimeValue(26 * time.Hour), `{"ms":93600000,"iso":"PT26H"}`},
		{Value{Kind: ValDate, Time: 24 * time.Hour}, `"D#1970-01-02"`},
		{Value{Kind: ValDateTime, Time: 24*time.Hour + 3*time.Second}, `"DT#1970-01-02-00:00:03"`},
		{Value{Kind: ValTod, Time: 12*time.Hour + 5*time.Millisecond}, `"TOD#12:00:00.005"`},
		{Value{Kind: ValStruct, Struct: map[string]Value{"B": IntValue(2), "A": IntValue(1)}}, `{"A":1,"B":2}`},
		{Value{Kind: ValArray}, `[]`},
		{Value{Kind: ValPointer, PtrVar: "X"}, `"PTR(X)"`},
		{Value{Kind: ValFBInstance}, `null`},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, marshal(t, rt.ToJSON(c.v)), c.v.String())
	}

	t.Run("deterministic", func(t *testing.T) {
		v := mustGet(t, rt, "GVL.arr")
		first := marshal(t, rt.ToJSON(v))
		for i := 0; i < 20; i++ {
			assert.Equal(t, first, marshal(t, rt.ToJSON(v)))
		}
	})
	t.Run("snapshot", func(t *testing.T) {
		s := marshal(t, rt.Snapshot())
		assert.True(t, strings.HasPrefix(s, `{"Lib":{"N":3},"GVL":{"shared":4,`), s)
		assert.Contains(t, s, `"PA":{"k":2,"limit":10,"C":4}`)
		assert.Equal(t, s, marshal(t, rt.Snapshot()))
	})
}
