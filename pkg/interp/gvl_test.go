package interp

import (
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gvlEngine parses src (as file filename, so the GVL takes its basename),
// registers its TYPEs and FUNCTION_BLOCKs on the engine's interpreter and its
// GVLs via SetGlobals, and returns the engine for the first PROGRAM.
func gvlEngine(t *testing.T, filename, src string) *ScanCycleEngine {
	t.Helper()
	res := parser.Parse(filename, src)
	require.Empty(t, res.Diags, "parse diagnostics")

	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	typeDecls := map[string]ast.TypeSpec{}
	fbDecls := map[string]*ast.FunctionBlockDecl{}
	for _, d := range res.File.Declarations {
		switch d := d.(type) {
		case *ast.ProgramDecl:
			if prog == nil {
				prog = d
			}
		case *ast.GVLDecl:
			gvls = append(gvls, d)
		case *ast.TypeDecl:
			typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
		case *ast.FunctionBlockDecl:
			fbDecls[strings.ToUpper(d.Name.Name)] = d
		}
	}
	require.NotNil(t, prog, "no PROGRAM in source")

	eng := NewScanCycleEngine(prog)
	eng.interp.TypeDecls = typeDecls
	eng.interp.FBDecls = fbDecls
	eng.SetGlobals(gvls)
	return eng
}

// gvlVar reads variable name from the registered GVL gvl.
func gvlVar(t *testing.T, eng *ScanCycleEngine, gvl, name string) Value {
	t.Helper()
	env := eng.interp.lookupGVL(gvl)
	require.NotNil(t, env, "GVL %s not registered", gvl)
	v, ok := env.Get(name)
	require.True(t, ok, "GVL %s has no variable %s", gvl, name)
	return v
}

func TestGVL(t *testing.T) {
	t.Run("qualified read write and nested struct persist across scans", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
TYPE ST_P : STRUCT a : INT; b : INT; END_STRUCT END_TYPE
VAR_GLOBAL
    x : INT := 3;
    s : ST_P;
END_VAR
PROGRAM P
VAR y : INT; END_VAR
y := G.x;
G.x := G.x + 1;
G.s.a := 7;
END_PROGRAM
`)
		require.NoError(t, eng.Tick(10*time.Millisecond))
		y, _ := eng.env.Get("y")
		assert.Equal(t, int64(3), y.Int)
		assert.Equal(t, int64(4), gvlVar(t, eng, "G", "x").Int)
		assert.Equal(t, int64(7), gvlVar(t, eng, "G", "s").Struct["A"].Int)

		require.NoError(t, eng.Tick(10*time.Millisecond))
		y, _ = eng.env.Get("y")
		assert.Equal(t, int64(4), y.Int)
		assert.Equal(t, int64(5), gvlVar(t, eng, "G", "x").Int)
	})

	t.Run("GVL name lookup is case-insensitive", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
VAR_GLOBAL x : INT := 2; END_VAR
PROGRAM P
VAR y : INT; END_VAR
y := g.X;
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		y, _ := eng.env.Get("y")
		assert.Equal(t, int64(2), y.Int)
	})

	t.Run("bare name resolves to a non-qualified_only GVL variable", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
VAR_GLOBAL x : INT := 10; END_VAR
PROGRAM P
x := x + 1;
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(12), gvlVar(t, eng, "G", "x").Int)
		_, local := eng.env.AllVars()["X"]
		assert.False(t, local, "bare write must update the GVL, not define a local")
	})

	t.Run("several unqualified GVLs chain in declaration order", func(t *testing.T) {
		g1 := parser.Parse("G1.st", "VAR_GLOBAL a : INT := 1; END_VAR")
		g2 := parser.Parse("G2.st", "VAR_GLOBAL b : INT := 2; END_VAR")
		p := parser.Parse("P.st", `
PROGRAM P
VAR y : INT; END_VAR
y := a + b;
a := 5;
b := 6;
END_PROGRAM
`)
		eng := NewScanCycleEngine(p.File.Declarations[0].(*ast.ProgramDecl))
		eng.SetGlobals([]*ast.GVLDecl{
			g1.File.Declarations[0].(*ast.GVLDecl),
			g2.File.Declarations[0].(*ast.GVLDecl),
		})
		require.NoError(t, eng.Tick(time.Millisecond))
		y, _ := eng.env.Get("y")
		assert.Equal(t, int64(3), y.Int)
		assert.Equal(t, int64(5), gvlVar(t, eng, "G1", "a").Int)
		assert.Equal(t, int64(6), gvlVar(t, eng, "G2", "b").Int)
	})

	t.Run("member lookup stays inside the named GVL", func(t *testing.T) {
		g1 := parser.Parse("G1.st", "VAR_GLOBAL a : INT := 1; END_VAR")
		g2 := parser.Parse("G2.st", "VAR_GLOBAL b : INT := 2; END_VAR")
		p := parser.Parse("P.st", `
PROGRAM P
VAR y : INT; END_VAR
y := G2.a;
END_PROGRAM
`)
		eng := NewScanCycleEngine(p.File.Declarations[0].(*ast.ProgramDecl))
		eng.SetGlobals([]*ast.GVLDecl{
			g1.File.Declarations[0].(*ast.GVLDecl),
			g2.File.Declarations[0].(*ast.GVLDecl),
		})
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "G2")
	})

	t.Run("qualified_only GVL is not an ancestor of the program env", func(t *testing.T) {
		eng := gvlEngine(t, "Q.st", `
{attribute 'qualified_only'}
VAR_GLOBAL x : INT := 4; END_VAR
PROGRAM P
VAR y : INT; z : INT; END_VAR
y := Q.x;
z := x;
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "undefined variable: x")
		y, _ := eng.env.Get("y")
		assert.Equal(t, int64(4), y.Int)
	})

	t.Run("FB instance inside a qualified_only GVL cannot read it bare", func(t *testing.T) {
		eng := gvlEngine(t, "Q.st", `
FUNCTION_BLOCK FB_Peek
VAR_OUTPUT seen : DINT; END_VAR
seen := secret;
END_FUNCTION_BLOCK
{attribute 'qualified_only'}
VAR_GLOBAL
    secret : DINT := 42;
    peek : FB_Peek;
END_VAR
PROGRAM P
VAR b : BOOL; END_VAR
Q.peek();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "undefined variable: secret")
	})

	t.Run("FB instance inside a qualified_only GVL reads it qualified", func(t *testing.T) {
		eng := gvlEngine(t, "Q.st", `
FUNCTION_BLOCK FB_Peek
VAR_OUTPUT seen : DINT; END_VAR
seen := Q.secret;
END_FUNCTION_BLOCK
{attribute 'qualified_only'}
VAR_GLOBAL
    secret : DINT := 42;
    peek : FB_Peek;
END_VAR
PROGRAM P
VAR b : BOOL; END_VAR
Q.peek();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(42), gvlVar(t, eng, "Q", "peek").FBRef.GetMember("seen").Int)
	})

	t.Run("FB instance inside a qualified_only GVL still sees plain GVLs", func(t *testing.T) {
		in := New()
		in.FBDecls = map[string]*ast.FunctionBlockDecl{"FB_PEEK": {
			Name: ident("FB_Peek"),
			VarBlocks: []*ast.VarBlock{{Section: ast.VarOutput, Declarations: []*ast.VarDecl{{
				Names: []*ast.Ident{ident("seen")}, Type: &ast.NamedType{Name: ident("INT")},
			}}}},
		}}
		plain := in.RegisterGVL(&ast.GVLDecl{Name: ident("G"), Blocks: []*ast.VarBlock{{
			Section:      ast.VarGlobal,
			Declarations: []*ast.VarDecl{{Names: []*ast.Ident{ident("shared")}, Type: &ast.NamedType{Name: ident("INT")}}},
		}}})
		q := in.RegisterGVL(&ast.GVLDecl{
			Name:       ident("Q"),
			Attributes: []*ast.Attribute{{Name: "qualified_only"}},
			Blocks: []*ast.VarBlock{{
				Section: ast.VarGlobal,
				Declarations: []*ast.VarDecl{
					{Names: []*ast.Ident{ident("secret")}, Type: &ast.NamedType{Name: ident("INT")}},
					{Names: []*ast.Ident{ident("peek")}, Type: &ast.NamedType{Name: ident("FB_Peek")}},
				},
			}},
		})
		v, ok := q.GetLocal("peek")
		require.True(t, ok)
		fbEnv := v.FBRef.Env
		_, sees := fbEnv.Get("secret")
		assert.False(t, sees, "qualified_only variables must not resolve bare")
		_, sees = fbEnv.Get("shared")
		assert.True(t, sees, "plain GVL variables still resolve bare")
		assert.Same(t, plain, in.GlobalParent())
	})

	t.Run("qualified_only on a VAR_GLOBAL block also counts", func(t *testing.T) {
		gvl := &ast.GVLDecl{
			Name: ident("Q"),
			Blocks: []*ast.VarBlock{nil, {
				Section:    ast.VarGlobal,
				Attributes: []*ast.Attribute{{Name: "qualified_only"}},
				Declarations: []*ast.VarDecl{{
					Names: []*ast.Ident{ident("x")},
					Type:  &ast.NamedType{Name: ident("INT")},
				}},
			}},
		}
		in := New()
		in.RegisterGVL(gvl)
		assert.Nil(t, in.GlobalParent())
		assert.NotNil(t, in.lookupGVL("q"))
	})

	t.Run("a local variable shadows the GVL name", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
TYPE ST_P : STRUCT a : INT; END_STRUCT END_TYPE
VAR_GLOBAL a : INT := 1; END_VAR
PROGRAM P
VAR G : ST_P; y : INT; END_VAR
G.a := 9;
y := G.a;
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		y, _ := eng.env.Get("y")
		assert.Equal(t, int64(9), y.Int)
		assert.Equal(t, int64(1), gvlVar(t, eng, "G", "a").Int)
	})

	t.Run("TON instance in a GVL runs and keeps state", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
VAR_GLOBAL t : TON; q : BOOL; END_VAR
PROGRAM P
VAR done : BOOL; END_VAR
G.t(IN := TRUE, PT := T#10MS, Q => G.q);
done := G.t.Q;
END_PROGRAM
`)
		require.NoError(t, eng.Tick(5*time.Millisecond))
		done, _ := eng.env.Get("done")
		assert.False(t, done.Bool)
		for i := 0; i < 3; i++ {
			require.NoError(t, eng.Tick(5*time.Millisecond))
		}
		done, _ = eng.env.Get("done")
		assert.True(t, done.Bool)
		assert.True(t, gvlVar(t, eng, "G", "q").Bool, "Q => G.q writes the GVL member")
	})

	t.Run("user FB instance in a GVL runs and its body sees the GVL", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
FUNCTION_BLOCK FB_Inc
VAR_INPUT step : INT; END_VAR
VAR_OUTPUT total : INT; END_VAR
total := total + step;
G.hits := G.hits + 1;
bare := bare + 2;
END_FUNCTION_BLOCK
VAR_GLOBAL f : FB_Inc; hits : INT; bare : INT; END_VAR
PROGRAM P
VAR local : FB_Inc; END_VAR
G.f(step := 3);
local(step := 1);
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		require.NoError(t, eng.Tick(time.Millisecond))
		f := gvlVar(t, eng, "G", "f")
		require.Equal(t, ValFBInstance, f.Kind)
		assert.Equal(t, int64(6), f.FBRef.GetOutput("total").Int)
		assert.Equal(t, int64(4), gvlVar(t, eng, "G", "hits").Int)
		assert.Equal(t, int64(8), gvlVar(t, eng, "G", "bare").Int)
	})

	t.Run("unknown GVL member is a RuntimeError", func(t *testing.T) {
		for name, body := range map[string]string{
			"read":  "y := G.missing;",
			"write": "G.missing := 1;",
		} {
			t.Run(name, func(t *testing.T) {
				eng := gvlEngine(t, "G.st", `
VAR_GLOBAL x : INT; END_VAR
PROGRAM P
VAR y : INT; END_VAR
`+body+`
END_PROGRAM
`)
				var err error
				require.NotPanics(t, func() { err = eng.Tick(time.Millisecond) })
				require.Error(t, err)
				_, isRT := err.(*RuntimeError)
				assert.True(t, isRT, "want *RuntimeError, got %T", err)
				assert.Contains(t, err.Error(), "missing")
			})
		}
	})

	t.Run("calls and output bindings on unknown GVL members are RuntimeErrors", func(t *testing.T) {
		for name, body := range map[string]string{
			"call":   "G.missing(IN := TRUE);",
			"output": "G.t(IN := TRUE, PT := T#1MS, Q => G.nope);",
		} {
			t.Run(name, func(t *testing.T) {
				eng := gvlEngine(t, "G.st", `
VAR_GLOBAL t : TON; END_VAR
PROGRAM P
`+body+`
END_PROGRAM
`)
				err := eng.Tick(time.Millisecond)
				require.Error(t, err)
				_, isRT := err.(*RuntimeError)
				assert.True(t, isRT, "want *RuntimeError, got %T", err)
			})
		}
	})

	t.Run("output binding to an undeclared name defines it", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
VAR_GLOBAL t : TON; END_VAR
PROGRAM P
G.t(IN := TRUE, PT := T#1MS, Q => fresh);
END_PROGRAM
`)
		require.NoError(t, eng.Tick(2*time.Millisecond))
		v, ok := eng.env.GetLocal("fresh")
		require.True(t, ok)
		assert.True(t, v.Bool)
	})

	t.Run("calling a non-FB GVL member is a RuntimeError", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
VAR_GLOBAL x : INT; END_VAR
PROGRAM P
G.x(IN := TRUE);
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a function block instance")
	})

	t.Run("program without GVLs behaves as before", func(t *testing.T) {
		res := parser.Parse("P.st", `
PROGRAM P
VAR y : INT; END_VAR
y := y + 1;
END_PROGRAM
`)
		prog := res.File.Declarations[0].(*ast.ProgramDecl)
		for _, set := range []bool{false, true} {
			eng := NewScanCycleEngine(prog)
			if set {
				eng.SetGlobals(nil)
			}
			require.NoError(t, eng.Tick(time.Millisecond))
			y, _ := eng.env.Get("y")
			assert.Equal(t, int64(1), y.Int)
			assert.Nil(t, eng.env.Parent())
			assert.Nil(t, eng.interp.lookupGVL("P"))
		}
	})

	t.Run("unknown root ident without GVL is undefined variable", func(t *testing.T) {
		res := parser.Parse("P.st", `
PROGRAM P
VAR y : INT; END_VAR
y := H.x;
END_PROGRAM
`)
		eng := NewScanCycleEngine(res.File.Declarations[0].(*ast.ProgramDecl))
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "undefined variable: H")
	})

	t.Run("nil and unnamed GVLs are ignored", func(t *testing.T) {
		in := New()
		assert.Nil(t, in.RegisterGVL(nil))
		assert.Nil(t, in.RegisterGVL(&ast.GVLDecl{}))
		assert.Nil(t, in.GlobalParent())
	})
}

func TestRegisterGVLsCrossConst(t *testing.T) {
	diag := parser.Parse("ECT_Diag.st", `
VAR_GLOBAL
    Device_1_SlaveInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo :=
        [(p_stat_sName := 'A1'), (p_stat_sName := 'A2'), (p_stat_sName := 'A3')];
    eLevel : E_Level := E_Level.High;
END_VAR`)
	param := parser.Parse("EcDiagParam.st", `
VAR_GLOBAL CONSTANT
    MAX_EC_SLAVES : INT := 4;
    HIGH_LEVEL : INT := 7;
END_VAR`)
	types := parser.Parse("T.st", `
TYPE ST_EcSlaveInfo : STRUCT p_stat_sName : STRING; nAddr : UINT := 1001; END_STRUCT END_TYPE
TYPE E_Level : (Low := 0, High := EcDiagParam.HIGH_LEVEL) END_TYPE`)
	for _, r := range []parser.ParseResult{diag, param, types} {
		require.Empty(t, r.Diags)
	}
	in := New()
	in.TypeDecls = map[string]ast.TypeSpec{}
	for _, d := range types.File.Declarations {
		td := d.(*ast.TypeDecl)
		in.TypeDecls[strings.ToUpper(td.Name.Name)] = td.Type
		if et, ok := td.Type.(*ast.EnumType); ok {
			in.RegisterEnumDecl(td.Name.Name, et, nil)
		}
	}
	in.RegisterGVLs([]*ast.GVLDecl{
		diag.File.Declarations[0].(*ast.GVLDecl),
		param.File.Declarations[0].(*ast.GVLDecl),
	})
	require.Empty(t, in.InitErrors())

	info, ok := in.lookupGVL("ECT_Diag").GetLocal("Device_1_SlaveInfo")
	require.True(t, ok)
	require.Len(t, info.Array, 5, "index 4 must be valid")
	assert.Equal(t, 1, info.ArrayLow)
	assert.Equal(t, "A3", info.Array[3].Struct["P_STAT_SNAME"].Str)
	assert.Equal(t, int64(1001), info.Array[4].Struct["NADDR"].Int)
	lvl, _ := in.lookupGVL("ECT_Diag").GetLocal("eLevel")
	assert.Equal(t, int64(7), lvl.Int)

	t.Run("single GVL keeps RegisterGVL behaviour", func(t *testing.T) {
		in := New()
		env := in.RegisterGVL(param.File.Declarations[0].(*ast.GVLDecl))
		require.NotNil(t, env)
		v, _ := env.GetLocal("MAX_EC_SLAVES")
		assert.Equal(t, int64(4), v.Int)
		assert.Nil(t, in.RegisterGVL(nil))
	})
}
