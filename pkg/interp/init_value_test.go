package interp

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initSource is one parsed file for initEngine; GVLs take the file basename.
type initSource struct{ name, src string }

// initEngine parses every source, registers TYPEs (with their defaults) and
// FUNCTION_BLOCKs, registers all GVLs in source order and initialises the
// first PROGRAM (if any) without running a scan.
func initEngine(t *testing.T, srcs ...initSource) *ScanCycleEngine {
	t.Helper()
	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	in := New()
	in.TypeDecls = map[string]ast.TypeSpec{}
	in.TypeInits = map[string]ast.Expr{}
	in.FBDecls = map[string]*ast.FunctionBlockDecl{}
	for _, s := range srcs {
		res := parser.Parse(s.name, s.src)
		require.Empty(t, res.Diags, "parse diagnostics in %s", s.name)
		for _, d := range res.File.Declarations {
			switch d := d.(type) {
			case *ast.ProgramDecl:
				if prog == nil {
					prog = d
				}
			case *ast.GVLDecl:
				gvls = append(gvls, d)
			case *ast.TypeDecl:
				name := strings.ToUpper(d.Name.Name)
				in.TypeDecls[name] = d.Type
				if d.InitValue != nil {
					in.TypeInits[name] = d.InitValue
				}
				if et, ok := d.Type.(*ast.EnumType); ok {
					in.RegisterEnumDecl(d.Name.Name, et, d.Attributes)
				}
			case *ast.FunctionBlockDecl:
				in.FBDecls[strings.ToUpper(d.Name.Name)] = d
			}
		}
	}
	if prog == nil {
		prog = &ast.ProgramDecl{Name: &ast.Ident{Name: "P"}}
	}
	eng := NewScanCycleEngine(prog)
	eng.interp = in
	eng.SetGlobals(gvls)
	eng.Initialize()
	return eng
}

// progVar reads a program variable after initialisation.
func initVar(t *testing.T, eng *ScanCycleEngine, name string) Value {
	t.Helper()
	v, ok := eng.env.Get(name)
	require.True(t, ok, "no variable %s", name)
	return v
}

func initErrText(eng *ScanCycleEngine) string {
	var b strings.Builder
	for _, e := range eng.interp.InitErrors() {
		b.WriteString(e.Error())
		b.WriteString("\n")
	}
	return b.String()
}

func TestConstBounds(t *testing.T) {
	t.Run("program constant N", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
PROGRAM P
VAR CONSTANT N : INT := 5; END_VAR
VAR arr : ARRAY[1..N] OF INT; END_VAR
END_PROGRAM`})
		arr := initVar(t, eng, "arr")
		assert.Len(t, arr.Array, 6)
		assert.Equal(t, 1, arr.ArrayLow)
		assert.Empty(t, eng.interp.InitErrors())
	})
	t.Run("GVL constant expressions", func(t *testing.T) {
		eng := initEngine(t,
			initSource{"G.st", "VAR_GLOBAL CONSTANT C : INT := 4; END_VAR"},
			initSource{"P.st", `
PROGRAM P
VAR
    a : ARRAY[0..G.C*2] OF INT;
    b : ARRAY[1..G.C - 1] OF INT;
    c : ARRAY[1..(G.C + 1)] OF BOOL;
END_VAR
END_PROGRAM`})
		assert.Len(t, initVar(t, eng, "a").Array, 9)
		assert.Equal(t, 0, initVar(t, eng, "a").ArrayLow)
		assert.Len(t, initVar(t, eng, "b").Array, 4)
		assert.Len(t, initVar(t, eng, "c").Array, 6)
		assert.Empty(t, eng.interp.InitErrors())
	})
	t.Run("unknown bound is reported", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
PROGRAM P
VAR arr : ARRAY[1..UNKNOWN] OF INT; END_VAR
END_PROGRAM`})
		arr := initVar(t, eng, "arr")
		assert.Equal(t, ValArray, arr.Kind)
		assert.Contains(t, initErrText(eng), "UNKNOWN")
	})
	t.Run("negative lower bound is reported", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
PROGRAM P
VAR arr : ARRAY[-2..3] OF INT; END_VAR
END_PROGRAM`})
		assert.Equal(t, ValArray, initVar(t, eng, "arr").Kind)
		assert.Contains(t, initErrText(eng), "negative array lower bound not supported")
	})
	t.Run("oversized array is capped and reported", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
PROGRAM P
VAR arr : ARRAY[0..20000] OF BYTE; END_VAR
END_PROGRAM`})
		assert.Len(t, initVar(t, eng, "arr").Array, maxArraySlots)
		assert.Contains(t, initErrText(eng), "exceeds")
	})
	t.Run("upper bound below lower bound is reported", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
PROGRAM P
VAR arr : ARRAY[5..2] OF BYTE; END_VAR
END_PROGRAM`})
		assert.Equal(t, ValArray, initVar(t, eng, "arr").Kind)
		assert.Contains(t, initErrText(eng), "upper bound")
	})
	t.Run("literal bounds without an interpreter", func(t *testing.T) {
		v := ZeroFromTypeSpec(&ast.ArrayType{
			Ranges: []*ast.SubrangeSpec{{
				Low:  &ast.Literal{LitKind: ast.LitInt, Value: "1"},
				High: &ast.Literal{LitKind: ast.LitInt, Value: "3"},
			}},
			ElementType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
		})
		assert.Len(t, v.Array, 4)
		assert.Equal(t, 1, v.ArrayLow)
	})
}

func TestStructMemberDefaults(t *testing.T) {
	eng := initEngine(t, initSource{"P.st", `
TYPE ST_A : STRUCT a : INT := 5; s : STRING := 'x'; b : BOOL; END_STRUCT END_TYPE
TYPE T_Speed : INT := 50; END_TYPE
TYPE E_State : (Idle, Run, Stop) := Run; END_TYPE
PROGRAM P
VAR
    sa : ST_A;
    v : T_Speed;
    e : E_State;
END_VAR
END_PROGRAM`})
	sa := initVar(t, eng, "sa")
	assert.Equal(t, []string{"a", "s", "b"}, sa.Fields)
	assert.Equal(t, int64(5), sa.Struct["A"].Int)
	assert.Equal(t, types.KindINT, sa.Struct["A"].IECType)
	assert.Equal(t, "x", sa.Struct["S"].Str)
	v := initVar(t, eng, "v")
	assert.Equal(t, int64(50), v.Int)
	assert.Equal(t, types.KindINT, v.IECType)
	e := initVar(t, eng, "e")
	assert.Equal(t, int64(1), e.Int)
	assert.Equal(t, "E_STATE", e.Enum)
	assert.Empty(t, eng.interp.InitErrors())

	t.Run("bad member default is reported", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
TYPE ST_B : STRUCT a : INT := nope; END_STRUCT END_TYPE
PROGRAM P
VAR sb : ST_B; END_VAR
END_PROGRAM`})
		assert.Equal(t, int64(0), initVar(t, eng, "sb").Struct["A"].Int)
		assert.Contains(t, initErrText(eng), "undefined variable: nope")
	})
	t.Run("member defaults without an interpreter", func(t *testing.T) {
		res := parser.Parse("T.st", `TYPE ST_C : STRUCT a : INT := 7; END_STRUCT END_TYPE`)
		td := res.File.Declarations[0].(*ast.TypeDecl)
		v := ZeroFromTypeSpec(td.Type)
		assert.Equal(t, int64(7), v.Struct["A"].Int)
		assert.Equal(t, []string{"a"}, v.Fields)
	})
	t.Run("self-recursive struct terminates", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
TYPE ST_R : STRUCT r : ST_R; END_STRUCT END_TYPE
PROGRAM P
VAR x : ST_R; END_VAR
END_PROGRAM`})
		assert.Equal(t, ValStruct, initVar(t, eng, "x").Kind)
	})
}

func TestCloneMetadata(t *testing.T) {
	arr := Value{Kind: ValArray, Array: []Value{{Kind: ValInt}, {Kind: ValInt}}, ArrayLow: 1}
	c := arr.Clone()
	assert.Equal(t, 1, c.ArrayLow)
	st := Value{Kind: ValStruct, Struct: map[string]Value{"A": {Kind: ValInt}}, Fields: []string{"a"}}
	cs := st.Clone()
	assert.Equal(t, []string{"a"}, cs.Fields)
	cs.Struct["A"] = Value{Kind: ValInt, Int: 3}
	assert.Equal(t, int64(0), st.Struct["A"].Int)
}

func TestInitArrays(t *testing.T) {
	eng := initEngine(t, initSource{"P.st", `
TYPE ST_A : STRUCT a : INT; s : STRING := 'x'; END_STRUCT END_TYPE
PROGRAM P
VAR
    arr : ARRAY[1..3] OF INT := [1, 2, 3];
    rep : ARRAY[1..4] OF INT := [3(0), 7];
    tail : ARRAY[0..3] OF INT := [9];
    nested : ARRAY[1..2] OF ARRAY[1..2] OF INT := [[1, 2], [3, 4]];
    sa : ARRAY[1..2] OF ST_A := [(a := 1, s := 'y'), (a := 2)];
    empty : ARRAY[1..2] OF INT := [2()];
END_VAR
END_PROGRAM`})
	require.Empty(t, eng.interp.InitErrors())
	arr := initVar(t, eng, "arr")
	assert.Equal(t, int64(2), arr.Array[2].Int)
	assert.Equal(t, types.KindINT, arr.Array[2].IECType)
	rep := initVar(t, eng, "rep")
	assert.Equal(t, []int64{0, 0, 0, 7}, []int64{rep.Array[1].Int, rep.Array[2].Int, rep.Array[3].Int, rep.Array[4].Int})
	tail := initVar(t, eng, "tail")
	assert.Equal(t, []int64{9, 0, 0, 0}, []int64{tail.Array[0].Int, tail.Array[1].Int, tail.Array[2].Int, tail.Array[3].Int})
	nested := initVar(t, eng, "nested")
	assert.Equal(t, int64(3), nested.Array[2].Array[1].Int)
	assert.Equal(t, int64(2), nested.Array[1].Array[2].Int)
	sa := initVar(t, eng, "sa")
	assert.Equal(t, "y", sa.Array[1].Struct["S"].Str)
	assert.Equal(t, int64(2), sa.Array[2].Struct["A"].Int)
	assert.Equal(t, "x", sa.Array[2].Struct["S"].Str)
	assert.Equal(t, []string{"a", "s"}, sa.Array[2].Fields)
}

func TestInitErrors(t *testing.T) {
	cases := map[string]struct{ decl, want string }{
		"too many elements":   {"a : ARRAY[1..2] OF INT := [1, 2, 3];", "more elements"},
		"repetition overflow": {"a : ARRAY[1..2] OF INT := [5(1)];", "more elements"},
		"bad repetition":      {"a : ARRAY[1..2] OF INT := [K(1)];", "repetition count"},
		"unknown field":       {"s : ST_A := (zz := 1);", "no member 'zz'"},
		"array init on INT":   {"i : INT := [1, 2];", "array initialiser"},
		"struct init on INT":  {"i : INT := (a := 1);", "structure initialiser"},
		"bad element":         {"a : ARRAY[1..2] OF INT := [nope];", "undefined variable: nope"},
		"bad scalar":          {"i : INT := nope;", "undefined variable: nope"},
		"bad field value":     {"s : ST_A := (a := nope);", "undefined variable: nope"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			eng := initEngine(t, initSource{"P.st", `
TYPE ST_A : STRUCT a : INT; END_STRUCT END_TYPE
PROGRAM P
VAR ` + c.decl + ` END_VAR
END_PROGRAM`})
			assert.Contains(t, initErrText(eng), c.want)
		})
	}
}

func TestInitScalarsKeepType(t *testing.T) {
	eng := initEngine(t, initSource{"P.st", `
TYPE E_State : (Idle, Run) END_TYPE
TYPE T_Speed : INT := 50; END_TYPE
PROGRAM P
VAR
    w : WORD := 16#9;
    e : E_State := E_State.Run;
    v : T_Speed;
    v2 : T_Speed := 7;
END_VAR
END_PROGRAM`})
	require.Empty(t, eng.interp.InitErrors())
	assert.Equal(t, types.KindWORD, initVar(t, eng, "w").IECType)
	e := initVar(t, eng, "e")
	assert.Equal(t, int64(1), e.Int)
	assert.Equal(t, "E_STATE", e.Enum)
	assert.Equal(t, int64(50), initVar(t, eng, "v").Int)
	assert.Equal(t, int64(7), initVar(t, eng, "v2").Int)
}

func TestInitFBInstances(t *testing.T) {
	eng := initEngine(t, initSource{"P.st", `
TYPE ST_HMI : STRUCT p_cfg_ManualFreq : REAL; on : BOOL; END_STRUCT END_TYPE
FUNCTION_BLOCK FB_Base
VAR base_x : INT := 3; tbl : ARRAY[1..2] OF INT := [4, 5]; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Drive EXTENDS FB_Base
VAR_INPUT speed : INT; END_VAR
VAR HMI : ST_HMI := (p_cfg_ManualFreq := 20.0); END_VAR
END_FUNCTION_BLOCK
PROGRAM P
VAR
    d1, d2 : FB_Drive;
    d3 : FB_Drive := (speed := 9);
END_VAR
END_PROGRAM`})
	require.Empty(t, eng.interp.InitErrors())
	d1 := initVar(t, eng, "d1").FBRef.Env
	d2 := initVar(t, eng, "d2").FBRef.Env
	for _, env := range []*Env{d1, d2} {
		x, _ := env.Get("base_x")
		assert.Equal(t, int64(3), x.Int)
		tbl, _ := env.Get("tbl")
		assert.Equal(t, int64(5), tbl.Array[2].Int)
		hmi, _ := env.Get("HMI")
		assert.InDelta(t, 20.0, hmi.Struct["P_CFG_MANUALFREQ"].Real, 1e-9)
	}
	hmi, _ := d1.Get("HMI")
	hmi.Struct["P_CFG_MANUALFREQ"] = Value{Kind: ValReal, Real: 1}
	other, _ := d2.Get("HMI")
	assert.InDelta(t, 20.0, other.Struct["P_CFG_MANUALFREQ"].Real, 1e-9)
	speed, _ := initVar(t, eng, "d3").FBRef.Env.Get("speed")
	assert.Equal(t, int64(9), speed.Int)
	assert.Equal(t, []string{"SPEED"}, initVar(t, eng, "d1").FBRef.inputNames)

	t.Run("FB initialiser errors are reported", func(t *testing.T) {
		eng := initEngine(t, initSource{"P.st", `
FUNCTION_BLOCK FB_A
VAR_INPUT a : INT; END_VAR
END_FUNCTION_BLOCK
PROGRAM P
VAR
    f1 : FB_A := (zz := 1);
    f2 : FB_A := (a := nope);
    f3 : FB_A := 5;
    t1 : TON := (PT := T#1S);
END_VAR
END_PROGRAM`})
		txt := initErrText(eng)
		assert.Contains(t, txt, "no member 'zz'")
		assert.Contains(t, txt, "undefined variable: nope")
		assert.Contains(t, txt, "structure initialiser")
		ton := initVar(t, eng, "t1").FBRef.FB.GetInput("PT")
		assert.Equal(t, int64(1000), ton.Time.Milliseconds())
	})
}

func TestInitFunctionLocals(t *testing.T) {
	res := parser.Parse("F.st", `
FUNCTION F : INT
VAR tbl : ARRAY[1..3] OF INT := [10, 20, 30]; END_VAR
F := tbl[2];
END_FUNCTION
PROGRAM P
VAR r : INT; END_VAR
r := F();
END_PROGRAM`)
	require.Empty(t, res.Diags)
	var prog *ast.ProgramDecl
	eng := (*ScanCycleEngine)(nil)
	for _, d := range res.File.Declarations {
		if p, ok := d.(*ast.ProgramDecl); ok {
			prog = p
		}
	}
	eng = NewScanCycleEngine(prog)
	for _, d := range res.File.Declarations {
		if f, ok := d.(*ast.FunctionDecl); ok {
			eng.interp.RegisterFunctionDecl(f)
		}
	}
	require.NoError(t, eng.Tick(0))
	assert.Equal(t, int64(20), initVar(t, eng, "r").Int)
}
