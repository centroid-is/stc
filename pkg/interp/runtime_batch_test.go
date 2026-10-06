package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeCheckSet(t *testing.T) {
	tf := parseRT(t, "types.st", "TYPE ST_P : STRUCT a : INT; END_STRUCT END_TYPE\n")
	gf := parseRT(t, "GVL.st", `VAR_GLOBAL
	x : INT;
	w : WORD;
	t : TON;
	p : POINTER TO INT;
	r : REFERENCE TO INT;
	s : ST_P;
END_VAR
VAR_GLOBAL CONSTANT
	K : INT := 1;
END_VAR
`)
	rt, err := NewRuntime([]*ast.SourceFile{tf, gf})
	require.NoError(t, err)
	assert.NoError(t, rt.CheckSet("GVL.x", 5))
	assert.ErrorContains(t, rt.CheckSet("GVL.x", 70000), "out of range")
	assert.ErrorContains(t, rt.CheckSet("GVL.x", true), "cannot use")
	assert.ErrorContains(t, rt.CheckSet("GVL.K", 2), "is a constant")
	assert.Error(t, rt.CheckSet("GVL.nope", 1))
	assert.Error(t, rt.CheckSet("GVL.s.nope", 1))
	assert.Error(t, rt.CheckSet("GVL.r", 1), "unbound reference")
	assert.ErrorContains(t, rt.CheckSet("GVL.p", 1), "pointer not writable")
	assert.ErrorContains(t, rt.CheckSet("GVL.t.Q", true), "read-only output")
	assert.NoError(t, rt.CheckSet("GVL.t.IN", true))
	assert.NoError(t, rt.CheckSet("GVL.w.3", true))
	assert.ErrorContains(t, rt.CheckSet("GVL.w.3", "x"), "GVL.w.3")
	assert.NoError(t, rt.CheckSet("GVL.s", map[string]any{"a": 1}))
	// Nothing was written.
	v, err := rt.Get("GVL.x")
	require.NoError(t, err)
	assert.Equal(t, int64(0), v.Int)
	v, err = rt.Get("GVL.s.a")
	require.NoError(t, err)
	assert.Equal(t, int64(0), v.Int)
}

func TestRuntimeGetMany(t *testing.T) {
	gf := parseRT(t, "GVL.st", `VAR_GLOBAL
	a : ARRAY[1..2] OF INT := [1, 2];
	b : BOOL := TRUE;
	r : REFERENCE TO INT;
END_VAR
`)
	rt, err := NewRuntime([]*ast.SourceFile{gf})
	require.NoError(t, err)
	vs, err := rt.GetMany([]string{"GVL.a", "GVL.b"})
	require.NoError(t, err)
	require.Len(t, vs, 2)
	assert.True(t, vs[1].Bool)
	vs[0].Array[vs[0].ArrayLow].Int = 99 // a copy: the runtime is unchanged
	v, err := rt.Get("GVL.a[1]")
	require.NoError(t, err)
	assert.Equal(t, int64(1), v.Int)

	_, err = rt.GetMany([]string{"GVL.b", "GVL.nope"})
	assert.Error(t, err)
	_, err = rt.GetMany([]string{"GVL.r"})
	assert.Error(t, err)
	vs, err = rt.GetMany(nil)
	require.NoError(t, err)
	assert.Empty(t, vs)
}
