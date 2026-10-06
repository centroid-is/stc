package interp

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
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
FUNCTION_BLOCK FB_Cmd
VAR_INPUT p_cmd_Start : BOOL; END_VAR
VAR pt : ST_Pt; inner : FB_Count; END_VAR
END_FUNCTION_BLOCK
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
	x : FB_Cmd;
	i : INT; ui : UINT; li : LINT; ul : ULINT; bt : BYTE; b : BOOL;
	re : REAL; lr : LREAL; st : E_State; tm : TIME; str : STRING;
	ia : ARRAY[1..3] OF INT; da : DATE; tod0 : TOD; dtt : DT;
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
		"":                                 "empty path",
		"Nope.x":                           "unknown root",
		"GVL":                              "names a root",
		"GVL.missing":                      "unknown variable",
		"GVL.arr[0]":                       "out of range",
		"GVL.arr[4]":                       "out of range",
		"GVL.arr[1,2]":                     "multi-dimensional",
		"GVL.arr[1].nope":                  "no member",
		"GVL.shared[1]":                    "not an array",
		"GVL.shared.x":                     "no member",
		"GVL.w.16":                         "bit index 16 out of range",
		"GVL.w.1.2":                        "bit access must be last",
		"PB.s.0":                           "bit access",
		"PB.t.Nope":                        "no member",
		"PB.t.Q.x":                         "no member",
		"PB.p.x":                           "pointer",
		"GVL.fb.zz":                        "no member",
		"3abc":                             "identifier",
		"GVL..x":                           "expected identifier",
		"GVL.arr[x]":                       "invalid array index",
		"GVL.arr[1":                        "expected ']'",
		"GVL.arr$":                         "unexpected character",
		"GVL.w.99999999999999999999999":    "invalid bit",
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

func TestRuntimeSet(t *testing.T) {
	rt := newRT(t)
	set := func(path string, v any) {
		t.Helper()
		require.NoError(t, rt.Set(path, v), "%s := %v", path, v)
	}
	setErr := func(path string, v any, want string) {
		t.Helper()
		err := rt.Set(path, v)
		if assert.Error(t, err, "%s := %v", path, v) {
			assert.Contains(t, err.Error(), want, "%s := %v", path, v)
		}
	}

	// BOOL
	set("GVL.x.p_cmd_Start", true)
	assert.Equal(t, BoolValue(true), mustGet(t, rt, "GVL.x.p_cmd_Start"))
	set("GVL.b", 1)
	assert.True(t, mustGet(t, rt, "GVL.b").Bool)
	set("GVL.b", "false")
	assert.False(t, mustGet(t, rt, "GVL.b").Bool)
	set("GVL.b", json.Number("1"))
	assert.True(t, mustGet(t, rt, "GVL.b").Bool)
	set("GVL.b", "0")
	set("GVL.b", "TRUE")
	setErr("GVL.b", 2, "BOOL")
	setErr("GVL.b", "yes", "BOOL")
	setErr("GVL.b", 1.5, "BOOL")

	// Integers
	for _, in := range []any{json.Number("42"), float64(42), int8(42), "INT#42", "16#2A", "4_2", int64(42), uint16(42), float32(42), int(42), int16(42), int32(42), uint(42), uint8(42), uint32(42), uint64(42), "+42", "2#101010", "8#52"} {
		set("GVL.i", 0)
		set("GVL.i", in)
		v := mustGet(t, rt, "GVL.i")
		assert.Equal(t, int64(42), v.Int, "%T %v", in, in)
		assert.Equal(t, types.KindINT, v.IECType)
	}
	set("GVL.i", -32768)
	setErr("GVL.i", 40000, "out of range for INT")
	setErr("GVL.i", float64(1.5), "not an integer")
	setErr("GVL.i", json.Number("1.5"), "not an integer")
	setErr("GVL.i", json.Number("1e3x"), "not a number")
	setErr("GVL.i", "abc", "invalid integer")
	setErr("GVL.i", "16#", "invalid integer")
	setErr("GVL.i", "99#1", "invalid integer")
	setErr("GVL.i", true, "cannot use bool")
	setErr("GVL.i", math.Inf(1), "not an integer")
	setErr("GVL.ui", -1, "out of range for UINT")
	set("GVL.ul", "18446744073709551615")
	assert.Equal(t, "18446744073709551615", marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.ul"))))
	setErr("GVL.ul", "18446744073709551616", "out of range for ULINT")
	set("GVL.li", json.Number("-9223372036854775808"))
	set("GVL.bt", "BYTE#16#FF")
	assert.Equal(t, int64(255), mustGet(t, rt, "GVL.bt").Int)
	set("GVL.li", json.Number("1e3"))
	assert.Equal(t, int64(1000), mustGet(t, rt, "GVL.li").Int)

	// REAL
	for _, in := range []any{1.5, float32(1.5), json.Number("1.5"), "1.5", "REAL#1.5", "LREAL#1_.5"} {
		set("GVL.re", 0)
		set("GVL.re", in)
		v := mustGet(t, rt, "GVL.re")
		assert.Equal(t, 1.5, v.Real, "%T %v", in, in)
		assert.Equal(t, types.KindREAL, v.IECType)
	}
	set("GVL.re", 3)
	assert.Equal(t, 3.0, mustGet(t, rt, "GVL.re").Real)
	set("GVL.lr", 1e300)
	setErr("GVL.re", 1e39, "out of range for REAL")
	setErr("GVL.re", math.NaN(), "NaN")
	setErr("GVL.lr", math.Inf(-1), "infinite")
	setErr("GVL.re", "x1", "invalid real")
	setErr("GVL.re", true, "cannot use bool")
	setErr("GVL.re", json.Number("zz"), "invalid real")

	// Enums
	for in, want := range map[any]int64{"Run": 5, "e_state.stop": 6, "E_State#Idle": 0, "RUN": 5, 6: 6, json.Number("5"): 5} {
		set("GVL.st", in)
		v := mustGet(t, rt, "GVL.st")
		assert.Equal(t, want, v.Int, "%v", in)
		assert.Equal(t, "E_STATE", v.Enum)
	}
	setErr("GVL.st", "Walk", "unknown value")
	setErr("GVL.st", 3, "unknown value")
	setErr("GVL.st", 1.5, "not an integer")
	set("GVL.x.pt.State", "Run")
	assert.Equal(t, `"Run"`, marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.x.pt.State"))))

	// TIME, STRING, DATE/TOD/DT
	set("GVL.tm", "T#5s")
	assert.Equal(t, 5*time.Second, mustGet(t, rt, "GVL.tm").Time)
	set("GVL.tm", 250)
	assert.Equal(t, 250*time.Millisecond, mustGet(t, rt, "GVL.tm").Time)
	set("GVL.tm", json.Number("1.5"))
	assert.Equal(t, 1500*time.Microsecond, mustGet(t, rt, "GVL.tm").Time)
	setErr("GVL.tm", "soon", "invalid time")
	setErr("GVL.tm", true, "cannot use bool")
	setErr("GVL.tm", json.Number("x"), "not a number")
	set("GVL.str", "'it''s $24 $'ok$' $N$L$R$T$P$$'")
	assert.Equal(t, "it's $ 'ok' \n\n\r\t\f$", mustGet(t, rt, "GVL.str").Str)
	set("GVL.str", "plain")
	assert.Equal(t, "plain", mustGet(t, rt, "GVL.str").Str)
	setErr("GVL.str", "'bad $Z'", "invalid escape")
	setErr("GVL.str", "'bad $4'", "invalid escape")
	setErr("GVL.str", 5, "cannot use int")
	set("GVL.da", "D#2026-01-02")
	assert.Equal(t, `"D#2026-01-02"`, marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.da"))))
	set("GVL.da", "DATE#1970-01-03")
	set("GVL.tod0", "TOD#12:30:01.250")
	assert.Equal(t, `"TOD#12:30:01.250"`, marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.tod0"))))
	set("GVL.tod0", "TIME_OF_DAY#01:00:00")
	set("GVL.dtt", "DT#2026-01-02-03:04:05")
	assert.Equal(t, `"DT#2026-01-02-03:04:05"`, marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.dtt"))))
	set("GVL.dtt", "DATE_AND_TIME#2026-01-02-03:04:05.5")
	setErr("GVL.da", "D#2026-13-01", "invalid DATE")
	setErr("GVL.da", 5, "cannot use int")

	// Arrays, structs and FB maps
	set("GVL.ia", []any{1, json.Number("2")})
	assert.Equal(t, `[1,2,0]`, marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.ia"))))
	setErr("GVL.ia", []any{1, 2, 3, 4}, "4 elements")
	setErr("GVL.ia", []any{1, "x"}, "invalid integer")
	setErr("GVL.ia", 5, "cannot use int")
	set("GVL.arr[2]", map[string]any{"X": 9, "state": "Stop"})
	assert.Equal(t, `{"x":9,"y":0,"State":"Stop"}`, marshal(t, rt.ToJSON(mustGet(t, rt, "GVL.arr[2]"))))
	set("GVL.arr", []any{map[string]any{"y": 2.5}})
	assert.Equal(t, 2.5, mustGet(t, rt, "GVL.arr[1].y").Real)
	assert.Equal(t, int64(9), mustGet(t, rt, "GVL.arr[2].x").Int)
	setErr("GVL.arr[2]", map[string]any{"nope": 1}, "no member")
	setErr("GVL.arr[2]", map[string]any{"x": "q"}, "invalid integer")
	setErr("GVL.arr[2]", 1, "cannot use int")
	set("GVL.x", map[string]any{"p_cmd_start": false, "pt": map[string]any{"x": 3}, "inner": map[string]any{"enable": true}})
	assert.False(t, mustGet(t, rt, "GVL.x.p_cmd_Start").Bool)
	assert.Equal(t, int64(3), mustGet(t, rt, "GVL.x.pt.x").Int)
	assert.True(t, mustGet(t, rt, "GVL.x.inner.enable").Bool)
	// All-or-nothing: a bad key leaves the FB untouched.
	setErr("GVL.x", map[string]any{"inner": map[string]any{"enable": false}, "zz": 1}, "no member")
	assert.True(t, mustGet(t, rt, "GVL.x.inner.enable").Bool)
	setErr("GVL.x", map[string]any{"inner": map[string]any{"enable": "maybe"}}, "BOOL")
	setErr("GVL.x", 1, "cannot use int")
	set("PB.t", map[string]any{"PT": "T#1s", "in": true})
	assert.Equal(t, time.Second, mustGet(t, rt, "PB.t.PT").Time)
	setErr("PB.t", map[string]any{"Q": true}, "read-only output")
	setErr("PB.t", map[string]any{"zz": true}, "no member")
	setErr("PB.t", map[string]any{"PT": true}, "cannot use bool")
	set("PB.t.IN", false)
	assert.False(t, mustGet(t, rt, "PB.t.IN").Bool)
	setErr("PB.t.IN", "x", "BOOL")

	// Bits
	set("GVL.w.3", false)
	set("GVL.w.15", true)
	assert.Equal(t, int64(0x8001), mustGet(t, rt, "GVL.w").Int)
	setErr("GVL.w.2", "x", "BOOL")

	// Access rules
	setErr("GVL.nope", 1, "unknown variable")
	setErr("PA.C", 1, "constant")
	setErr("Lib.N", 1, "constant")
	setErr("PB.p", 1, "pointer not writable by path")
	setErr("PB.t.Q", true, "read-only output")
	setErr("PB.t.ET", 5, "read-only output")
	setErr("PB.r", 1, "unbound reference")
	setErr("GVL.i", nil, "nil")
	require.NoError(t, rt.Tick(time.Millisecond))
	set("PB.r", 77)
	assert.Equal(t, int64(77), mustGet(t, rt, "PB.seen").Int, "write through reference")

	// Values set take effect at the next Tick.
	set("PA.k", 10)
	require.NoError(t, rt.Tick(time.Millisecond))
	assert.Equal(t, int64(20), mustGet(t, rt, "GVL.shared").Int)
}

func TestRuntimeConcurrent(t *testing.T) {
	rt := newRT(t)
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				switch g {
				case 0:
					assert.NoError(t, rt.Tick(time.Millisecond))
				case 1:
					assert.NoError(t, rt.Set("PA.k", i%100))
				case 2:
					_, err := rt.Get("GVL.shared")
					assert.NoError(t, err)
				default:
					_ = rt.Snapshot()
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestArrayOfFB(t *testing.T) {
	src := `FUNCTION_BLOCK FB_C
VAR_INPUT step : INT := 1; END_VAR
VAR n : INT; END_VAR
n := n + step;
END_FUNCTION_BLOCK
TYPE ST_Holder :
STRUCT
	c : FB_C;
END_STRUCT
END_TYPE
PROGRAM P
VAR
	fbs : ARRAY[1..3] OF FB_C;
	tons : ARRAY[0..1] OF TON;
	h : ST_Holder;
	ints : ARRAY[0..1] OF INT;
	i : INT := 3;
END_VAR
fbs[1]();
fbs[i](step := 10);
tons[1](IN := TRUE, PT := T#1ms);
h.c();
END_PROGRAM
`
	rt, err := NewRuntime([]*ast.SourceFile{parseRT(t, "p.st", src)})
	require.NoError(t, err)
	require.NoError(t, rt.Tick(time.Millisecond))
	require.NoError(t, rt.Tick(time.Millisecond))
	assert.Equal(t, int64(2), mustGet(t, rt, "P.fbs[1].n").Int)
	assert.Equal(t, int64(0), mustGet(t, rt, "P.fbs[2].n").Int, "elements are separate instances")
	assert.Equal(t, int64(20), mustGet(t, rt, "P.fbs[3].n").Int)
	assert.True(t, mustGet(t, rt, "P.tons[1].Q").Bool)
	assert.False(t, mustGet(t, rt, "P.tons[0].Q").Bool)
	assert.Equal(t, int64(2), mustGet(t, rt, "P.h.c.n").Int)

	bad := parseRT(t, "b.st", "PROGRAM B\nVAR ints : ARRAY[0..1] OF INT; END_VAR\nints[0](x := 1);\nEND_PROGRAM\n")
	rt, err = NewRuntime([]*ast.SourceFile{bad})
	require.NoError(t, err)
	err = rt.Tick(time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a function block instance")
	bad = parseRT(t, "b.st", "PROGRAM B\nVAR ints : ARRAY[0..1] OF INT; END_VAR\nints[5](x := 1);\nEND_PROGRAM\n")
	rt, err = NewRuntime([]*ast.SourceFile{bad})
	require.NoError(t, err)
	assert.Error(t, rt.Tick(time.Millisecond))

	// A self-referencing ARRAY OF FB stops at the nesting limit.
	rec := parseRT(t, "r.st", "FUNCTION_BLOCK FB_R\nVAR kids : ARRAY[0..0] OF FB_R; END_VAR\nEND_FUNCTION_BLOCK\nVAR_GLOBAL r : FB_R; END_VAR\n")
	_, err = NewRuntime([]*ast.SourceFile{rec})
	require.NoError(t, err)
}
