package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanWiden_SignedInts(t *testing.T) {
	assert.True(t, CanWiden(KindSINT, KindINT))
	assert.True(t, CanWiden(KindINT, KindDINT))
	assert.True(t, CanWiden(KindDINT, KindLINT))
	assert.False(t, CanWiden(KindLINT, KindSINT))
	assert.False(t, CanWiden(KindINT, KindSINT))

	// Transitive: SINT -> DINT, SINT -> LINT
	assert.True(t, CanWiden(KindSINT, KindDINT))
	assert.True(t, CanWiden(KindSINT, KindLINT))
}

func TestCanWiden_UnsignedInts(t *testing.T) {
	assert.True(t, CanWiden(KindUSINT, KindUINT))
	assert.True(t, CanWiden(KindUINT, KindUDINT))
	assert.True(t, CanWiden(KindUDINT, KindULINT))
	assert.False(t, CanWiden(KindULINT, KindUSINT))
}

func TestCanWiden_Reals(t *testing.T) {
	assert.True(t, CanWiden(KindREAL, KindLREAL))
	assert.False(t, CanWiden(KindLREAL, KindREAL))
}

func TestCanWiden_CrossCategory(t *testing.T) {
	// INT->REAL: precision-preserving (16-bit int fits in 32-bit float)
	assert.True(t, CanWiden(KindINT, KindREAL))
	assert.True(t, CanWiden(KindSINT, KindREAL))

	// DINT->LREAL: 32-bit int fits in 64-bit double
	assert.True(t, CanWiden(KindDINT, KindLREAL))

	// LINT->LREAL: false (64-bit int doesn't fit precisely in 64-bit double)
	assert.False(t, CanWiden(KindLINT, KindLREAL))
}

func TestCanWiden_BitTypes(t *testing.T) {
	assert.True(t, CanWiden(KindBYTE, KindWORD))
	assert.True(t, CanWiden(KindWORD, KindDWORD))
	assert.True(t, CanWiden(KindDWORD, KindLWORD))
	assert.False(t, CanWiden(KindLWORD, KindBYTE))
}

func TestCanWiden_Rejected(t *testing.T) {
	// Signed to unsigned rejected
	assert.False(t, CanWiden(KindSINT, KindUSINT))
	assert.False(t, CanWiden(KindINT, KindUINT))

	// Unsigned to signed rejected
	assert.False(t, CanWiden(KindUSINT, KindSINT))

	// BYTE -> INT rejected (ANY_BIT to ANY_INT)
	assert.False(t, CanWiden(KindBYTE, KindINT))

	// BOOL -> INT rejected
	assert.False(t, CanWiden(KindBOOL, KindINT))
	assert.False(t, CanWiden(KindBOOL, KindBYTE))

	// STRING -> INT rejected
	assert.False(t, CanWiden(KindSTRING, KindINT))

	// TIME -> INT rejected
	assert.False(t, CanWiden(KindTIME, KindINT))
}

func TestCanWiden_SameType(t *testing.T) {
	// Widening from a type to itself is NOT widening (it's identity)
	assert.False(t, CanWiden(KindINT, KindINT))
}

func TestCommonType_Same(t *testing.T) {
	ct, ok := CommonType(KindINT, KindINT)
	assert.True(t, ok)
	assert.Equal(t, KindINT, ct)
}

func TestCommonType_Widening(t *testing.T) {
	ct, ok := CommonType(KindINT, KindDINT)
	assert.True(t, ok)
	assert.Equal(t, KindDINT, ct)

	ct, ok = CommonType(KindDINT, KindINT)
	assert.True(t, ok)
	assert.Equal(t, KindDINT, ct)

	ct, ok = CommonType(KindSINT, KindLINT)
	assert.True(t, ok)
	assert.Equal(t, KindLINT, ct)
}

func TestCommonType_CrossCategory(t *testing.T) {
	// INT and REAL -> REAL
	ct, ok := CommonType(KindINT, KindREAL)
	assert.True(t, ok)
	assert.Equal(t, KindREAL, ct)
}

func TestCommonType_Incompatible(t *testing.T) {
	_, ok := CommonType(KindINT, KindSTRING)
	assert.False(t, ok)

	_, ok = CommonType(KindBOOL, KindINT)
	assert.False(t, ok)
}

func TestCommonType_SmallestSupertype(t *testing.T) {
	// SINT and INT should give INT (not DINT)
	ct, ok := CommonType(KindSINT, KindINT)
	assert.True(t, ok)
	assert.Equal(t, KindINT, ct)

	// BYTE and DWORD should give DWORD (not LWORD)
	ct, ok = CommonType(KindBYTE, KindDWORD)
	assert.True(t, ok)
	assert.Equal(t, KindDWORD, ct)
}

func TestCategoryMembership(t *testing.T) {
	// IsAnyInt covers all signed and unsigned integer types
	for _, k := range []TypeKind{KindSINT, KindINT, KindDINT, KindLINT, KindUSINT, KindUINT, KindUDINT, KindULINT} {
		assert.True(t, IsAnyInt(k), "%s should be ANY_INT", k)
	}
	assert.False(t, IsAnyInt(KindREAL))
	assert.False(t, IsAnyInt(KindBOOL))

	// IsAnyReal covers REAL and LREAL
	assert.True(t, IsAnyReal(KindREAL))
	assert.True(t, IsAnyReal(KindLREAL))
	assert.False(t, IsAnyReal(KindINT))

	// IsAnyBit covers BOOL, BYTE, WORD, DWORD, LWORD
	for _, k := range []TypeKind{KindBOOL, KindBYTE, KindWORD, KindDWORD, KindLWORD} {
		assert.True(t, IsAnyBit(k), "%s should be ANY_BIT", k)
	}
	assert.False(t, IsAnyBit(KindINT))

	// IsAnyNum covers IsAnyInt + IsAnyReal
	for _, k := range []TypeKind{KindSINT, KindINT, KindDINT, KindLINT, KindUSINT, KindUINT, KindUDINT, KindULINT, KindREAL, KindLREAL} {
		assert.True(t, IsAnyNum(k), "%s should be ANY_NUM", k)
	}
	assert.False(t, IsAnyNum(KindBOOL))
	assert.False(t, IsAnyNum(KindSTRING))
}

func TestIsAnySigned(t *testing.T) {
	for _, k := range []TypeKind{KindSINT, KindINT, KindDINT, KindLINT} {
		assert.True(t, IsAnySigned(k), "%s should be ANY_SIGNED", k)
	}
	assert.False(t, IsAnySigned(KindUSINT))
	assert.False(t, IsAnySigned(KindREAL))
}

func TestIsAnyUnsigned(t *testing.T) {
	for _, k := range []TypeKind{KindUSINT, KindUINT, KindUDINT, KindULINT} {
		assert.True(t, IsAnyUnsigned(k), "%s should be ANY_UNSIGNED", k)
	}
	assert.False(t, IsAnyUnsigned(KindSINT))
}

func TestIsAnyString(t *testing.T) {
	assert.True(t, IsAnyString(KindSTRING))
	assert.True(t, IsAnyString(KindWSTRING))
	assert.False(t, IsAnyString(KindINT))
}

func TestIsAnyDate(t *testing.T) {
	assert.True(t, IsAnyDate(KindDATE))
	assert.True(t, IsAnyDate(KindDT))
	assert.True(t, IsAnyDate(KindTOD))
	assert.False(t, IsAnyDate(KindINT))
}

func TestIsAnyChar(t *testing.T) {
	assert.True(t, IsAnyChar(KindCHAR))
	assert.True(t, IsAnyChar(KindWCHAR))
	assert.False(t, IsAnyChar(KindSTRING))
}
