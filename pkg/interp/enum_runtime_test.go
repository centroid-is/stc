package interp

import (
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enumEngine parses src as file filename, registers its TYPEs (enums through
// RegisterEnumDecl), FBs and GVLs, and returns the engine for its PROGRAM.
func enumEngine(t *testing.T, filename, src string) *ScanCycleEngine {
	t.Helper()
	res := parser.Parse(filename, src)
	require.Empty(t, res.Diags, "parse diagnostics")
	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	typeDecls := map[string]ast.TypeSpec{}
	fbDecls := map[string]*ast.FunctionBlockDecl{}
	var enums []*ast.TypeDecl
	var funcDecls []*ast.FunctionDecl
	for _, d := range res.File.Declarations {
		switch d := d.(type) {
		case *ast.ProgramDecl:
			prog = d
		case *ast.GVLDecl:
			gvls = append(gvls, d)
		case *ast.TypeDecl:
			typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
			if _, ok := d.Type.(*ast.EnumType); ok {
				enums = append(enums, d)
			}
		case *ast.FunctionBlockDecl:
			fbDecls[strings.ToUpper(d.Name.Name)] = d
		case *ast.FunctionDecl:
			funcDecls = append(funcDecls, d)
		}
	}
	require.NotNil(t, prog, "no PROGRAM in source")
	eng := NewScanCycleEngine(prog)
	for _, d := range funcDecls {
		eng.interp.RegisterFunctionDecl(d)
	}
	eng.interp.TypeDecls = typeDecls
	eng.interp.FBDecls = fbDecls
	for _, d := range enums {
		eng.interp.RegisterEnumDecl(d.Name.Name, d.Type.(*ast.EnumType), d.Attributes)
	}
	eng.SetGlobals(gvls)
	return eng
}

func enumRun(t *testing.T, filename, src string) *ScanCycleEngine {
	t.Helper()
	eng := enumEngine(t, filename, src)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	return eng
}

func enumRunErr(t *testing.T, src string) error {
	t.Helper()
	eng := enumEngine(t, "P.st", src)
	err := eng.Tick(10 * time.Millisecond)
	require.Error(t, err)
	return err
}

const enumDecls = `
TYPE E : (tun := 0, rdy := 2, nst) UINT; END_TYPE
TYPE H : (a := 16#0006, b); END_TYPE
{attribute 'to_string'}
TYPE S : (idle, run := 5, stop); END_TYPE
`

func TestEnumRuntime(t *testing.T) {
	t.Run("previous plus one numbering and base type", func(t *testing.T) {
		eng := enumRun(t, "P.st", enumDecls+`
PROGRAM P
VAR n, r, ha, hb : DINT; vE, vH : DINT; END_VAR
n := E.nst; r := E.rdy; ha := H.a; hb := H.b;
END_PROGRAM
`)
		assert.Equal(t, int64(3), progVar(t, eng, "n").Int)
		assert.Equal(t, int64(2), progVar(t, eng, "r").Int)
		assert.Equal(t, int64(6), progVar(t, eng, "ha").Int)
		assert.Equal(t, int64(7), progVar(t, eng, "hb").Int)

		v, err := eng.interp.evalExpr(eng.env, &ast.MemberAccessExpr{Object: &ast.Ident{Name: "E"}, Member: &ast.Ident{Name: "rdy"}})
		require.NoError(t, err)
		assert.Equal(t, types.KindUINT, v.IECType)
		assert.Equal(t, "E", v.Enum)
		v, err = eng.interp.evalExpr(eng.env, &ast.Ident{Name: "b"})
		require.NoError(t, err)
		assert.Equal(t, types.KindINT, v.IECType, "default base type is INT")
		assert.Equal(t, int64(7), v.Int)
	})

	t.Run("assignment comparison and arithmetic", func(t *testing.T) {
		eng := enumRun(t, "P.st", enumDecls+`
PROGRAM P
VAR ev : E; ok : BOOL; x : UINT; END_VAR
ev := E.rdy;
IF ev = E.rdy THEN ok := TRUE; END_IF
x := E.rdy + 1;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "ok").Bool)
		assert.Equal(t, int64(3), progVar(t, eng, "x").Int)
		assert.Equal(t, "E", progVar(t, eng, "ev").Enum)
	})

	t.Run("CASE with qualified labels lists and ranges", func(t *testing.T) {
		src := enumDecls + `
PROGRAM P
VAR ev : E; a, b, c : INT; END_VAR
CASE ev OF
    E.tun: a := 1;
    E.rdy, E.nst: a := 2;
END_CASE
ev := E.nst;
CASE ev OF
    E.tun: b := 1;
    E.rdy, E.nst: b := 2;
END_CASE
ev := E.rdy;
CASE ev OF
    E.tun..E.rdy: c := 7;
ELSE
    c := -1;
END_CASE
END_PROGRAM
`
		eng := enumRun(t, "P.st", src)
		assert.Equal(t, int64(1), progVar(t, eng, "a").Int)
		assert.Equal(t, int64(2), progVar(t, eng, "b").Int)
		assert.Equal(t, int64(7), progVar(t, eng, "c").Int)
	})

	t.Run("variable named like the enum shadows it", func(t *testing.T) {
		eng := enumRun(t, "P.st", `
TYPE ST_X : STRUCT rdy : INT; END_STRUCT END_TYPE
TYPE E : (tun, rdy); END_TYPE
PROGRAM P
VAR E : ST_X; n : INT; END_VAR
E.rdy := 42;
n := E.rdy;
END_PROGRAM
`)
		assert.Equal(t, int64(42), progVar(t, eng, "n").Int)
	})

	t.Run("GVL named like the enum shadows it", func(t *testing.T) {
		eng := enumRun(t, "E.st", `
TYPE E : (tun, rdy); END_TYPE
VAR_GLOBAL rdy : INT := 42; END_VAR
PROGRAM P
VAR n : INT; END_VAR
n := E.rdy;
END_PROGRAM
`)
		assert.Equal(t, int64(42), progVar(t, eng, "n").Int)
	})

	t.Run("unknown qualified value is an error", func(t *testing.T) {
		err := enumRunErr(t, enumDecls+`
PROGRAM P
VAR n : INT; END_VAR
n := E.nope;
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "enum 'E' has no value 'nope'")
	})

	t.Run("TO_STRING", func(t *testing.T) {
		eng := enumRun(t, "P.st", enumDecls+`
PROGRAM P
VAR
    sv : S; ev : E; z : S;
    s1, s2, s3, s4, s5, s6, s7, s8, s9 : STRING;
    i : INT := 42; r : REAL := 1.5; t : TIME := T#1s;
END_VAR
sv := S.stop; ev := E.rdy;
s1 := TO_STRING(sv);
s2 := TO_STRING(ev);
s3 := TO_STRING(i);
s4 := TO_STRING(TRUE);
s5 := TO_STRING(FALSE);
s6 := TO_STRING(r);
s7 := TO_STRING('abc');
s8 := TO_STRING(z);
s9 := TO_STRING(t);
END_PROGRAM
`)
		assert.Equal(t, "stop", progVar(t, eng, "s1").Str)
		assert.Equal(t, "2", progVar(t, eng, "s2").Str)
		assert.Equal(t, "42", progVar(t, eng, "s3").Str)
		assert.Equal(t, "TRUE", progVar(t, eng, "s4").Str)
		assert.Equal(t, "FALSE", progVar(t, eng, "s5").Str)
		assert.Equal(t, formatReal(1.5), progVar(t, eng, "s6").Str)
		assert.Equal(t, "abc", progVar(t, eng, "s7").Str)
		assert.Equal(t, "idle", progVar(t, eng, "s8").Str, "zero value of a to_string enum is its first value")
		assert.Equal(t, TimeValue(time.Second).String(), progVar(t, eng, "s9").Str)
	})

	t.Run("TO_STRING of a to_string enum ordinal with no name", func(t *testing.T) {
		in := New()
		res := parser.Parse("x.st", "{attribute 'to_string'}\nTYPE S : (a, b); END_TYPE\n")
		require.Empty(t, res.Diags)
		td := res.File.Declarations[0].(*ast.TypeDecl)
		in.RegisterEnumDecl(td.Name.Name, td.Type.(*ast.EnumType), td.Attributes)
		s, ok := in.enumString(Value{Kind: ValInt, Int: 9, Enum: "S"})
		assert.False(t, ok)
		assert.Empty(t, s)
		_, ok = in.enumString(Value{Kind: ValInt, Int: 0, Enum: "OTHER"})
		assert.False(t, ok)
		s, ok = in.enumString(Value{Kind: ValInt, Int: 1, Enum: "S"})
		assert.True(t, ok)
		assert.Equal(t, "b", s)
	})

	t.Run("TO_STRING arity", func(t *testing.T) {
		_, err := StdlibFunctions["TO_STRING"](nil)
		require.Error(t, err)
	})

	t.Run("inline enum in a PROGRAM", func(t *testing.T) {
		eng := enumEngine(t, "P.st", `
PROGRAM P
VAR eStep : (E_IDLE, E_RUN) := E_RUN; wasRun, isIdle : BOOL; END_VAR
wasRun := eStep = E_RUN;
eStep := E_IDLE;
isIdle := eStep = E_IDLE;
END_PROGRAM
`)
		require.NoError(t, eng.Tick(10*time.Millisecond))
		assert.True(t, progVar(t, eng, "wasRun").Bool)
		assert.True(t, progVar(t, eng, "isIdle").Bool)
		assert.Equal(t, int64(0), progVar(t, eng, "eStep").Int)
		_, ok := eng.interp.EnumDefs["P.ESTEP"]
		assert.True(t, ok, "inline enum registered as <POU>.<var>")
	})

	t.Run("inline enum in an FB instance", func(t *testing.T) {
		eng := enumRun(t, "P.st", `
FUNCTION_BLOCK FB_Seq
VAR eStep : (S_IDLE, S_RUN := 4, S_DONE) := S_RUN; END_VAR
VAR_OUTPUT wasRun, done : BOOL; END_VAR
wasRun := eStep = S_RUN;
eStep := S_DONE;
done := eStep = 5;
END_FUNCTION_BLOCK
PROGRAM P
VAR fb : FB_Seq; a, b : BOOL; END_VAR
fb();
a := fb.wasRun; b := fb.done;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "a").Bool)
		assert.True(t, progVar(t, eng, "b").Bool)
	})

	t.Run("typed literal", func(t *testing.T) {
		eng := enumRun(t, "P.st", enumDecls+`
PROGRAM P
VAR ev : E; ok : BOOL; END_VAR
ev := E#rdy;
ok := ev = E.rdy;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "ok").Bool)
		assert.Equal(t, types.KindUINT, progVar(t, eng, "ev").IECType)
		err := enumRunErr(t, enumDecls+"PROGRAM P\nVAR ev : E; END_VAR\nev := E#zzz;\nEND_PROGRAM\n")
		assert.Contains(t, err.Error(), "unknown enum value")
	})

	t.Run("legacy RegisterEnumType values still resolve", func(t *testing.T) {
		in := New()
		in.RegisterEnumType("Color", map[string]int64{"RED": 0, "GREEN": 1})
		env := NewEnv(nil)
		v, err := in.evalExpr(env, &ast.Ident{Name: "green"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), v.Int)
		v, err = in.evalExpr(env, &ast.MemberAccessExpr{Object: &ast.Ident{Name: "Color"}, Member: &ast.Ident{Name: "Red"}})
		require.NoError(t, err)
		assert.Equal(t, int64(0), v.Int)
		v, err = in.parseLitTyped("Green", "Color")
		require.NoError(t, err)
		assert.Equal(t, int64(1), v.Int)
	})

	t.Run("RegisterEnumDecl ignores nil and uses unknown base as INT", func(t *testing.T) {
		in := New()
		in.RegisterEnumDecl("X", nil, nil)
		assert.Empty(t, in.EnumDefs)
		et := &ast.EnumType{
			BaseType: &ast.NamedType{Name: &ast.Ident{Name: "NOT_A_TYPE"}},
			Values:   []*ast.EnumValue{{Name: &ast.Ident{Name: "a"}}, {Name: &ast.Ident{Name: "b"}}, {Name: &ast.Ident{Name: "a2"}, Value: &ast.Literal{LitKind: ast.LitInt, Value: "1"}}},
		}
		in.RegisterEnumDecl("X", et, nil)
		def := in.EnumDefs["X"]
		require.NotNil(t, def)
		assert.Equal(t, types.KindINT, def.Base)
		assert.Equal(t, "b", def.Names[1], "first name wins for a duplicate ordinal")
		assert.Equal(t, int64(1), in.EnumTypes["X"]["A2"])
	})

	t.Run("enum zero value", func(t *testing.T) {
		et := &ast.EnumType{Values: []*ast.EnumValue{{Name: &ast.Ident{Name: "a"}, Value: &ast.Literal{LitKind: ast.LitInt, Value: "3"}}}}
		v := zeroFromTypeSpecWith(et, nil, 0)
		assert.Equal(t, int64(3), v.Int)
		assert.Equal(t, types.KindINT, v.IECType)
		v = zeroFromTypeSpecWith(&ast.EnumType{}, nil, 0)
		assert.Equal(t, int64(0), v.Int)
	})

	t.Run("helper edge cases", func(t *testing.T) {
		in := New()
		in.RegisterEnumType("Color", map[string]int64{"RED": 0})
		env := NewEnv(nil)
		// Unknown object name: not an enum, falls through to member access.
		_, handled, err := in.qualifiedEnum(env, &ast.MemberAccessExpr{Object: &ast.Ident{Name: "Nope"}, Member: &ast.Ident{Name: "x"}})
		assert.False(t, handled)
		assert.NoError(t, err)
		// Non-identifier object and missing member are not enum lookups.
		_, handled, _ = in.qualifiedEnum(env, &ast.MemberAccessExpr{Object: &ast.ParenExpr{Inner: &ast.Ident{Name: "Color"}}, Member: &ast.Ident{Name: "Red"}})
		assert.False(t, handled)
		_, handled, _ = in.qualifiedEnum(env, &ast.MemberAccessExpr{Object: &ast.Ident{Name: "Color"}})
		assert.False(t, handled)

		_, ok := constBitIndex(env, &ast.MemberAccessExpr{Object: &ast.Ident{Name: "w"}})
		assert.False(t, ok)

		in.RegisterInlineEnums("P", []*ast.VarBlock{nil})
		assert.Len(t, in.EnumDefs, 1)
	})
}

// TestEnumTagFollowsDestination covers review ME-01: a stored value takes
// the enum tag of its destination, so an INT that receives an enum value
// formats as a number and an enum variable that receives a number formats
// as a value name.
func TestEnumTagFollowsDestination(t *testing.T) {
	eng := enumRun(t, "P.st", enumDecls+`
TYPE ST : STRUCT n : INT; s : S; END_STRUCT END_TYPE
FUNCTION F_Str : STRING
VAR_INPUT v : INT; END_VAR
F_Str := TO_STRING(v);
END_FUNCTION
FUNCTION_BLOCK FB_In
VAR_INPUT v : INT; END_VAR
VAR_OUTPUT s : STRING; END_VAR
s := TO_STRING(v);
END_FUNCTION_BLOCK
PROGRAM P
VAR
	e : S; i : INT; arr : ARRAY[0..1] OF INT; st : ST; fb : FB_In;
	s1, s2, s3, s4, s5, s6, s7 : STRING;
END_VAR
e := S.stop;
i := e;
s1 := TO_STRING(i);
s2 := F_Str(v := e);
arr[0] := e;
s3 := TO_STRING(arr[0]);
st.n := e;
s4 := TO_STRING(st.n);
fb(v := e);
s5 := fb.s;
e := 5;
s6 := TO_STRING(e);
st.s := 6;
s7 := TO_STRING(st.s);
END_PROGRAM
`)
	assert.Equal(t, "6", progVar(t, eng, "s1").Str, "assignment to INT")
	assert.Equal(t, "6", progVar(t, eng, "s2").Str, "INT function input")
	assert.Equal(t, "6", progVar(t, eng, "s3").Str, "INT array element")
	assert.Equal(t, "6", progVar(t, eng, "s4").Str, "INT struct member")
	assert.Equal(t, "6", progVar(t, eng, "s5").Str, "INT FB input")
	assert.Equal(t, "run", progVar(t, eng, "s6").Str, "integer stored into an enum variable")
	assert.Equal(t, "stop", progVar(t, eng, "s7").Str, "integer stored into an enum struct member")
}
