package interp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvDefineAndGet(t *testing.T) {
	env := NewEnv(nil)
	val := Value{Kind: ValInt, Int: 42}
	env.Define("X", val)

	got, ok := env.Get("X")
	assert.True(t, ok)
	assert.Equal(t, int64(42), got.Int)
}

func TestEnvGetMissing(t *testing.T) {
	env := NewEnv(nil)
	_, ok := env.Get("X")
	assert.False(t, ok)
}

func TestEnvParentChain(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("X", Value{Kind: ValInt, Int: 10})

	child := NewEnv(parent)
	got, ok := child.Get("X")
	assert.True(t, ok)
	assert.Equal(t, int64(10), got.Int)
}

func TestEnvSetInDeclaringScope(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("X", Value{Kind: ValInt, Int: 10})

	child := NewEnv(parent)
	ok := child.Set("X", Value{Kind: ValInt, Int: 99})
	assert.True(t, ok)

	// Parent should have been updated
	got, _ := parent.Get("X")
	assert.Equal(t, int64(99), got.Int)
}

func TestEnvSetMissing(t *testing.T) {
	env := NewEnv(nil)
	ok := env.Set("X", Value{Kind: ValInt, Int: 1})
	assert.False(t, ok)
}

func TestEnvCaseInsensitive(t *testing.T) {
	env := NewEnv(nil)
	env.Define("myVar", Value{Kind: ValInt, Int: 42})

	got, ok := env.Get("MYVAR")
	assert.True(t, ok)
	assert.Equal(t, int64(42), got.Int)

	got, ok = env.Get("myvar")
	assert.True(t, ok)
	assert.Equal(t, int64(42), got.Int)
}

func TestEnvAllVars(t *testing.T) {
	env := NewEnv(nil)
	env.Define("A", Value{Kind: ValInt, Int: 1})
	env.Define("B", Value{Kind: ValInt, Int: 2})

	vars := env.AllVars()
	assert.Len(t, vars, 2)
}

func TestEnvChildShadowsParent(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("X", Value{Kind: ValInt, Int: 10})

	child := NewEnv(parent)
	child.Define("X", Value{Kind: ValInt, Int: 20})

	got, ok := child.Get("X")
	assert.True(t, ok)
	assert.Equal(t, int64(20), got.Int)

	// Parent unchanged
	got, ok = parent.Get("X")
	assert.True(t, ok)
	assert.Equal(t, int64(10), got.Int)
}
