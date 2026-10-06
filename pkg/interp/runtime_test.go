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
