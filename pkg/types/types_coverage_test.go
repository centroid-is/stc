package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Comprehensive widening tests ---

func TestCanWiden_AllValidPairs(t *testing.T) {
	validPairs := []struct {
		from, to TypeKind
	}{
		// Signed chain
		{KindSINT, KindINT}, {KindSINT, KindDINT}, {KindSINT, KindLINT},
		{KindSINT, KindREAL}, {KindSINT, KindLREAL},
		{KindINT, KindDINT}, {KindINT, KindLINT},
		{KindINT, KindREAL}, {KindINT, KindLREAL},
		{KindDINT, KindLINT}, {KindDINT, KindLREAL},

		// Unsigned chain
		{KindUSINT, KindUINT}, {KindUSINT, KindUDINT}, {KindUSINT, KindULINT},
		{KindUSINT, KindREAL}, {KindUSINT, KindLREAL},
		{KindUINT, KindUDINT}, {KindUINT, KindULINT},
		{KindUINT, KindREAL}, {KindUINT, KindLREAL},
		{KindUDINT, KindULINT}, {KindUDINT, KindLREAL},

		// Real chain
		{KindREAL, KindLREAL},

		// Bit chain
		{KindBYTE, KindWORD}, {KindBYTE, KindDWORD}, {KindBYTE, KindLWORD},
		{KindWORD, KindDWORD}, {KindWORD, KindLWORD},
		{KindDWORD, KindLWORD},
	}

	for _, p := range validPairs {
		if !CanWiden(p.from, p.to) {
			t.Errorf("CanWiden(%s, %s) should be true", p.from, p.to)
		}
	}
}

func TestCanWiden_AllInvalidPairs(t *testing.T) {
	invalidPairs := []struct {
		from, to TypeKind
	}{
		// Reverse widening
		{KindINT, KindSINT}, {KindDINT, KindINT}, {KindLINT, KindDINT},
		{KindUINT, KindUSINT}, {KindLREAL, KindREAL},
		{KindWORD, KindBYTE}, {KindLWORD, KindDWORD},

		// Cross-category
		{KindSINT, KindUSINT}, {KindINT, KindUINT}, {KindDINT, KindUDINT},
		{KindUSINT, KindSINT}, {KindUINT, KindINT},

		// BOOL to anything
		{KindBOOL, KindINT}, {KindBOOL, KindBYTE}, {KindBOOL, KindREAL},

		// BIT to INT
		{KindBYTE, KindINT}, {KindWORD, KindDINT},

		// STRING/TIME conversions
		{KindSTRING, KindINT}, {KindTIME, KindINT},
		{KindINT, KindSTRING}, {KindINT, KindTIME},

		// LINT/ULINT cannot widen (precision loss)
		{KindLINT, KindLREAL}, {KindULINT, KindLREAL},

		// Same type is not widening
		{KindINT, KindINT}, {KindREAL, KindREAL}, {KindBOOL, KindBOOL},

		// No widening from these
		{KindLINT, KindSINT}, {KindULINT, KindUSINT},
		{KindLWORD, KindBYTE}, {KindLREAL, KindREAL},

		// DATE types
		{KindDATE, KindINT}, {KindTOD, KindINT}, {KindDT, KindINT},

		// CHAR types
		{KindCHAR, KindINT}, {KindWCHAR, KindINT},
	}

	for _, p := range invalidPairs {
		if CanWiden(p.from, p.to) {
			t.Errorf("CanWiden(%s, %s) should be false", p.from, p.to)
		}
	}
}

// --- CommonType exhaustive tests ---

func TestCommonType_AllCombinations(t *testing.T) {
	tests := []struct {
		a, b   TypeKind
		want   TypeKind
		wantOK bool
	}{
		// Same type
		{KindINT, KindINT, KindINT, true},
		{KindREAL, KindREAL, KindREAL, true},
		{KindBOOL, KindBOOL, KindBOOL, true},

		// Direct widening
		{KindSINT, KindINT, KindINT, true},
		{KindINT, KindSINT, KindINT, true}, // symmetric
		{KindINT, KindDINT, KindDINT, true},
		{KindDINT, KindINT, KindDINT, true},
		{KindREAL, KindLREAL, KindLREAL, true},

		// Requires common supertype search
		{KindSINT, KindDINT, KindDINT, true},
		{KindSINT, KindLINT, KindLINT, true},
		{KindUSINT, KindUDINT, KindUDINT, true},
		{KindBYTE, KindDWORD, KindDWORD, true},

		// Cross-category: both widen to REAL or LREAL
		{KindINT, KindREAL, KindREAL, true},
		{KindSINT, KindREAL, KindREAL, true},
		{KindDINT, KindLREAL, KindLREAL, true},

		// Incompatible types
		{KindINT, KindSTRING, KindInvalid, false},
		{KindBOOL, KindINT, KindInvalid, false},
		{KindSTRING, KindREAL, KindInvalid, false},
		{KindTIME, KindINT, KindInvalid, false},
		{KindBYTE, KindINT, KindInvalid, false}, // BIT vs INT

		// Cross-category through common target
		{KindSINT, KindINT, KindINT, true},

		// Signed and unsigned - may find common supertype via REAL/LREAL
		{KindSINT, KindUSINT, KindREAL, true},
		{KindINT, KindUINT, KindREAL, true},
	}

	for _, tt := range tests {
		ct, ok := CommonType(tt.a, tt.b)
		if ok != tt.wantOK {
			t.Errorf("CommonType(%s, %s) ok=%v, want %v", tt.a, tt.b, ok, tt.wantOK)
			continue
		}
		if ok && ct != tt.want {
			t.Errorf("CommonType(%s, %s) = %s, want %s", tt.a, tt.b, ct, tt.want)
		}
	}
}

// --- Category membership comprehensive ---

func TestIsAnySigned_Exhaustive(t *testing.T) {
	signed := []TypeKind{KindSINT, KindINT, KindDINT, KindLINT}
	notSigned := []TypeKind{KindUSINT, KindUINT, KindUDINT, KindULINT, KindREAL, KindBOOL, KindSTRING, KindBYTE}

	for _, k := range signed {
		assert.True(t, IsAnySigned(k), "%s should be ANY_SIGNED", k)
	}
	for _, k := range notSigned {
		assert.False(t, IsAnySigned(k), "%s should not be ANY_SIGNED", k)
	}
}

func TestIsAnyUnsigned_Exhaustive(t *testing.T) {
	unsigned := []TypeKind{KindUSINT, KindUINT, KindUDINT, KindULINT}
	notUnsigned := []TypeKind{KindSINT, KindINT, KindDINT, KindLINT, KindREAL, KindBOOL}

	for _, k := range unsigned {
		assert.True(t, IsAnyUnsigned(k), "%s should be ANY_UNSIGNED", k)
	}
	for _, k := range notUnsigned {
		assert.False(t, IsAnyUnsigned(k), "%s should not be ANY_UNSIGNED", k)
	}
}

func TestIsAnyInt_Comprehensive(t *testing.T) {
	ints := []TypeKind{KindSINT, KindINT, KindDINT, KindLINT, KindUSINT, KindUINT, KindUDINT, KindULINT}
	notInts := []TypeKind{KindREAL, KindLREAL, KindBOOL, KindBYTE, KindSTRING, KindTIME}

	for _, k := range ints {
		assert.True(t, IsAnyInt(k), "%s should be ANY_INT", k)
	}
	for _, k := range notInts {
		assert.False(t, IsAnyInt(k), "%s should not be ANY_INT", k)
	}
}

func TestIsAnyReal_Comprehensive(t *testing.T) {
	assert.True(t, IsAnyReal(KindREAL))
	assert.True(t, IsAnyReal(KindLREAL))
	for _, k := range []TypeKind{KindINT, KindDINT, KindBOOL, KindSTRING, KindBYTE} {
		assert.False(t, IsAnyReal(k), "%s should not be ANY_REAL", k)
	}
}

func TestIsAnyNum_Comprehensive(t *testing.T) {
	nums := []TypeKind{KindSINT, KindINT, KindDINT, KindLINT, KindUSINT, KindUINT, KindUDINT, KindULINT, KindREAL, KindLREAL}
	notNums := []TypeKind{KindBOOL, KindBYTE, KindWORD, KindSTRING, KindTIME, KindDATE}

	for _, k := range nums {
		assert.True(t, IsAnyNum(k), "%s should be ANY_NUM", k)
	}
	for _, k := range notNums {
		assert.False(t, IsAnyNum(k), "%s should not be ANY_NUM", k)
	}
}

func TestIsAnyBit_Comprehensive(t *testing.T) {
	bits := []TypeKind{KindBOOL, KindBYTE, KindWORD, KindDWORD, KindLWORD}
	notBits := []TypeKind{KindINT, KindREAL, KindSTRING, KindSINT}

	for _, k := range bits {
		assert.True(t, IsAnyBit(k), "%s should be ANY_BIT", k)
	}
	for _, k := range notBits {
		assert.False(t, IsAnyBit(k), "%s should not be ANY_BIT", k)
	}
}

func TestIsAnyString_Comprehensive(t *testing.T) {
	assert.True(t, IsAnyString(KindSTRING))
	assert.True(t, IsAnyString(KindWSTRING))
	assert.False(t, IsAnyString(KindCHAR))
	assert.False(t, IsAnyString(KindINT))
}

func TestIsAnyDate_Comprehensive(t *testing.T) {
	assert.True(t, IsAnyDate(KindDATE))
	assert.True(t, IsAnyDate(KindDT))
	assert.True(t, IsAnyDate(KindTOD))
	assert.False(t, IsAnyDate(KindTIME))
	assert.False(t, IsAnyDate(KindINT))
}

func TestIsAnyChar_Comprehensive(t *testing.T) {
	assert.True(t, IsAnyChar(KindCHAR))
	assert.True(t, IsAnyChar(KindWCHAR))
	assert.False(t, IsAnyChar(KindSTRING))
	assert.False(t, IsAnyChar(KindINT))
}

// --- TypeKind.String() ---

func TestTypeKind_String_OutOfRange(t *testing.T) {
	k := TypeKind(9999)
	s := k.String()
	assert.Contains(t, s, "9999")
}

func TestTypeKind_String_AllKnown(t *testing.T) {
	for k := KindInvalid; k <= KindReference; k++ {
		s := k.String()
		assert.NotEmpty(t, s, "TypeKind(%d) should have a name", k)
	}
}

// --- Type equality edge cases ---

func TestPrimitiveType_NotEqualToOtherTypes(t *testing.T) {
	p := &PrimitiveType{Kind_: KindINT}
	assert.False(t, p.Equal(&ArrayType{ElementType: p}))
	assert.False(t, p.Equal(&StructType{Name: "S"}))
}

func TestArrayType_DimensionMismatch(t *testing.T) {
	elem := &PrimitiveType{Kind_: KindINT}
	a1 := &ArrayType{ElementType: elem, Dimensions: []ArrayDimension{{0, 9}}}
	a2 := &ArrayType{ElementType: elem, Dimensions: []ArrayDimension{{0, 9}, {0, 4}}}
	assert.False(t, a1.Equal(a2))
}

func TestArrayType_DimensionBoundsMismatch(t *testing.T) {
	elem := &PrimitiveType{Kind_: KindINT}
	a1 := &ArrayType{ElementType: elem, Dimensions: []ArrayDimension{{0, 9}}}
	a2 := &ArrayType{ElementType: elem, Dimensions: []ArrayDimension{{1, 10}}}
	assert.False(t, a1.Equal(a2))
}

func TestArrayType_NotEqualToPrimitive(t *testing.T) {
	arr := &ArrayType{ElementType: &PrimitiveType{Kind_: KindINT}}
	assert.False(t, arr.Equal(&PrimitiveType{Kind_: KindINT}))
}

func TestStructType_DifferentNames(t *testing.T) {
	s1 := &StructType{Name: "A"}
	s2 := &StructType{Name: "B"}
	assert.False(t, s1.Equal(s2))
	assert.False(t, s1.Equal(&PrimitiveType{Kind_: KindINT}))
}

func TestEnumType_Equality(t *testing.T) {
	e1 := &EnumType{Name: "E1"}
	e2 := &EnumType{Name: "E1"}
	e3 := &EnumType{Name: "E2"}
	assert.True(t, e1.Equal(e2))
	assert.False(t, e1.Equal(e3))
	assert.False(t, e1.Equal(&PrimitiveType{Kind_: KindINT}))
}

func TestFunctionBlockType_Equality(t *testing.T) {
	fb1 := &FunctionBlockType{Name: "FB1"}
	fb2 := &FunctionBlockType{Name: "FB1"}
	fb3 := &FunctionBlockType{Name: "FB2"}
	assert.True(t, fb1.Equal(fb2))
	assert.False(t, fb1.Equal(fb3))
	assert.False(t, fb1.Equal(&PrimitiveType{Kind_: KindINT}))
}

func TestFunctionType_Equality(t *testing.T) {
	f1 := &FunctionType{Name: "F1"}
	f2 := &FunctionType{Name: "F1"}
	f3 := &FunctionType{Name: "F2"}
	assert.True(t, f1.Equal(f2))
	assert.False(t, f1.Equal(f3))
	assert.False(t, f1.Equal(&PrimitiveType{Kind_: KindINT}))
}

func TestPointerType_Equality(t *testing.T) {
	p1 := &PointerType{BaseType: &PrimitiveType{Kind_: KindINT}}
	p2 := &PointerType{BaseType: &PrimitiveType{Kind_: KindINT}}
	p3 := &PointerType{BaseType: &PrimitiveType{Kind_: KindREAL}}
	assert.True(t, p1.Equal(p2))
	assert.False(t, p1.Equal(p3))
	assert.False(t, p1.Equal(&PrimitiveType{Kind_: KindINT}))
}

func TestReferenceType_Equality(t *testing.T) {
	r1 := &ReferenceType{BaseType: &PrimitiveType{Kind_: KindINT}}
	r2 := &ReferenceType{BaseType: &PrimitiveType{Kind_: KindINT}}
	r3 := &ReferenceType{BaseType: &PrimitiveType{Kind_: KindREAL}}
	assert.True(t, r1.Equal(r2))
	assert.False(t, r1.Equal(r3))
	assert.False(t, r1.Equal(&PrimitiveType{Kind_: KindINT}))
}

// --- BuiltinFunctions signature lookup ---

func TestBuiltinFunctions_AllRegistered(t *testing.T) {
	expected := []string{
		"ADD", "SUB", "MUL", "DIV", "MOD",
		"ABS", "SQRT", "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "LN", "LOG", "EXP",
		"MIN", "MAX", "LIMIT", "SEL", "MUX", "MOVE",
		"LEN", "CONCAT", "LEFT", "RIGHT", "MID", "FIND",
		"INT_TO_REAL", "REAL_TO_INT", "DINT_TO_REAL", "REAL_TO_DINT",
		"INT_TO_DINT", "DINT_TO_INT", "INT_TO_STRING", "STRING_TO_INT",
		"REAL_TO_STRING", "STRING_TO_REAL",
		"BOOL_TO_INT", "INT_TO_BOOL", "BOOL_TO_STRING",
		"BYTE_TO_INT", "INT_TO_BYTE",
		"DINT_TO_LREAL", "LREAL_TO_DINT",
	}

	for _, name := range expected {
		_, ok := BuiltinFunctions[name]
		require.True(t, ok, "missing builtin function: %s", name)
	}
}

func TestBuiltinFunctions_MathConstraints(t *testing.T) {
	realOnlyFuncs := []string{"SQRT", "SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "LN", "LOG", "EXP"}
	for _, name := range realOnlyFuncs {
		fn := BuiltinFunctions[name]
		require.NotNil(t, fn, "missing %s", name)
		assert.NotNil(t, fn.Params[0].GenericConstraint)
		assert.True(t, fn.Params[0].GenericConstraint(KindREAL))
		assert.True(t, fn.Params[0].GenericConstraint(KindLREAL))
		assert.False(t, fn.Params[0].GenericConstraint(KindINT), "%s should reject INT", name)
		assert.False(t, fn.Params[0].GenericConstraint(KindBOOL), "%s should reject BOOL", name)
	}
}

func TestBuiltinFunctions_MUX_Params(t *testing.T) {
	fn := BuiltinFunctions["MUX"]
	require.NotNil(t, fn)
	assert.GreaterOrEqual(t, len(fn.Params), 2)
	// K param should be ANY_INT
	assert.NotNil(t, fn.Params[0].GenericConstraint)
	assert.True(t, fn.Params[0].GenericConstraint(KindINT))
}

func TestBuiltinFunctions_StringFuncs(t *testing.T) {
	funcs := map[string]int{
		"LEN": 1, "CONCAT": 2, "LEFT": 2, "RIGHT": 2, "MID": 3, "FIND": 2,
	}
	for name, paramCount := range funcs {
		fn, ok := BuiltinFunctions[name]
		require.True(t, ok, "missing %s", name)
		assert.Len(t, fn.Params, paramCount, "%s param count", name)
	}
}

// --- Lookup edge cases ---

func TestLookupElementaryType_AllAliases(t *testing.T) {
	aliases := map[string]TypeKind{
		"DATE_AND_TIME": KindDT,
		"DT":            KindDT,
		"TIME_OF_DAY":   KindTOD,
		"TOD":           KindTOD,
	}
	for name, want := range aliases {
		typ, ok := LookupElementaryType(name)
		require.True(t, ok, "LookupElementaryType(%q)", name)
		assert.Equal(t, want, typ.Kind(), "kind for %q", name)
	}
}

func TestLookupElementaryType_AllTypes(t *testing.T) {
	// Every type in the map should be lookup-able
	for name := range elementaryTypes {
		_, ok := LookupElementaryType(name)
		assert.True(t, ok, "should find %q", name)
	}
}

// --- Type String() methods ---

func TestComplexType_Strings(t *testing.T) {
	arr := &ArrayType{ElementType: &PrimitiveType{Kind_: KindINT}}
	assert.Contains(t, arr.String(), "ARRAY")
	assert.Contains(t, arr.String(), "INT")

	ptr := &PointerType{BaseType: &PrimitiveType{Kind_: KindBOOL}}
	assert.Equal(t, "POINTER TO BOOL", ptr.String())

	ref := &ReferenceType{BaseType: &PrimitiveType{Kind_: KindREAL}}
	assert.Equal(t, "REFERENCE TO REAL", ref.String())

	assert.Equal(t, "VOID", TypeVOID.String())
}
