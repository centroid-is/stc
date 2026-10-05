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

	t.Run("qualified_only on a VAR_GLOBAL block also counts", func(t *testing.T) {
		gvl := &ast.GVLDecl{
			Name: ident("Q"),
			Blocks: []*ast.VarBlock{{
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
