package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValueStoreBool(t *testing.T) {
	v := Value{Kind: ValBool, Bool: true}
	assert.Equal(t, ValBool, v.Kind)
	assert.True(t, v.Bool)
}

func TestValueStoreInt(t *testing.T) {
	v := Value{Kind: ValInt, Int: 42, IECType: types.KindDINT}
	assert.Equal(t, ValInt, v.Kind)
	assert.Equal(t, int64(42), v.Int)
	assert.Equal(t, types.KindDINT, v.IECType)
}

func TestValueStoreReal(t *testing.T) {
	v := Value{Kind: ValReal, Real: 3.14, IECType: types.KindLREAL}
	assert.Equal(t, ValReal, v.Kind)
	assert.InDelta(t, 3.14, v.Real, 0.001)
	assert.Equal(t, types.KindLREAL, v.IECType)
}

func TestValueStoreString(t *testing.T) {
	v := Value{Kind: ValString, Str: "hello"}
	assert.Equal(t, ValString, v.Kind)
	assert.Equal(t, "hello", v.Str)
}

func TestValueStoreTime(t *testing.T) {
	v := Value{Kind: ValTime, Time: 5 * time.Second}
	assert.Equal(t, ValTime, v.Kind)
	assert.Equal(t, 5*time.Second, v.Time)
}

func TestZeroBool(t *testing.T) {
	v := Zero(types.KindBOOL)
	assert.Equal(t, ValBool, v.Kind)
	assert.False(t, v.Bool)
}

func TestZeroInt(t *testing.T) {
	v := Zero(types.KindINT)
	assert.Equal(t, ValInt, v.Kind)
	assert.Equal(t, int64(0), v.Int)
}

func TestZeroReal(t *testing.T) {
	v := Zero(types.KindREAL)
	assert.Equal(t, ValReal, v.Kind)
	assert.Equal(t, 0.0, v.Real)
}

func TestZeroString(t *testing.T) {
	v := Zero(types.KindSTRING)
	assert.Equal(t, ValString, v.Kind)
	assert.Equal(t, "", v.Str)
}

func TestZeroTime(t *testing.T) {
	v := Zero(types.KindTIME)
	assert.Equal(t, ValTime, v.Kind)
	assert.Equal(t, time.Duration(0), v.Time)
}

func TestZeroSignedInts(t *testing.T) {
	for _, kind := range []types.TypeKind{types.KindSINT, types.KindINT, types.KindDINT, types.KindLINT} {
		v := Zero(kind)
		assert.Equal(t, ValInt, v.Kind, "kind=%v", kind)
		assert.Equal(t, int64(0), v.Int, "kind=%v", kind)
		assert.Equal(t, kind, v.IECType, "kind=%v", kind)
	}
}

func TestZeroUnsignedInts(t *testing.T) {
	for _, kind := range []types.TypeKind{types.KindUSINT, types.KindUINT, types.KindUDINT, types.KindULINT} {
		v := Zero(kind)
		assert.Equal(t, ValInt, v.Kind, "kind=%v", kind)
		assert.Equal(t, int64(0), v.Int, "kind=%v", kind)
		assert.Equal(t, kind, v.IECType, "kind=%v", kind)
	}
}

func TestZeroReals(t *testing.T) {
	for _, kind := range []types.TypeKind{types.KindREAL, types.KindLREAL} {
		v := Zero(kind)
		assert.Equal(t, ValReal, v.Kind, "kind=%v", kind)
		assert.Equal(t, 0.0, v.Real, "kind=%v", kind)
		assert.Equal(t, kind, v.IECType, "kind=%v", kind)
	}
}

func TestValueString(t *testing.T) {
	v := Value{Kind: ValInt, Int: 42}
	s := v.String()
	assert.Contains(t, s, "42")
}
