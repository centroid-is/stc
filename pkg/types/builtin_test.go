package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinTypeConstants(t *testing.T) {
	// All elementary type constants exist and have correct kinds
	constants := []struct {
		typ  Type
		kind TypeKind
		name string
	}{
		{TypeBOOL, KindBOOL, "BOOL"},
		{TypeBYTE, KindBYTE, "BYTE"},
		{TypeWORD, KindWORD, "WORD"},
		{TypeDWORD, KindDWORD, "DWORD"},
		{TypeLWORD, KindLWORD, "LWORD"},
		{TypeSINT, KindSINT, "SINT"},
		{TypeINT, KindINT, "INT"},
		{TypeDINT, KindDINT, "DINT"},
		{TypeLINT, KindLINT, "LINT"},
		{TypeUSINT, KindUSINT, "USINT"},
		{TypeUINT, KindUINT, "UINT"},
		{TypeUDINT, KindUDINT, "UDINT"},
		{TypeULINT, KindULINT, "ULINT"},
		{TypeREAL, KindREAL, "REAL"},
		{TypeLREAL, KindLREAL, "LREAL"},
		{TypeSTRING, KindSTRING, "STRING"},
		{TypeWSTRING, KindWSTRING, "WSTRING"},
		{TypeTIME, KindTIME, "TIME"},
		{TypeDATE, KindDATE, "DATE"},
		{TypeDT, KindDT, "DT"},
		{TypeTOD, KindTOD, "TOD"},
		{TypeCHAR, KindCHAR, "CHAR"},
		{TypeWCHAR, KindWCHAR, "WCHAR"},
	}

	for _, tc := range constants {
		assert.Equal(t, tc.kind, tc.typ.Kind(), "Kind for %s", tc.name)
		assert.Equal(t, tc.name, tc.typ.String(), "String for %s", tc.name)
	}
}

func TestBuiltinLookupElementaryType(t *testing.T) {
	// All elementary type names resolve
	names := []string{
		"BOOL", "BYTE", "WORD", "DWORD", "LWORD",
		"SINT", "INT", "DINT", "LINT",
		"USINT", "UINT", "UDINT", "ULINT",
		"REAL", "LREAL",
		"STRING", "WSTRING",
		"TIME", "DATE", "DT", "TOD",
		"CHAR", "WCHAR",
	}
	for _, name := range names {
		typ, ok := LookupElementaryType(name)
		require.True(t, ok, "LookupElementaryType(%q) should find type", name)
		assert.Equal(t, name, typ.String(), "LookupElementaryType(%q) should return correct type", name)
	}
}

func TestBuiltinLookupCaseInsensitive(t *testing.T) {
	// IEC identifiers are case-insensitive
	typ, ok := LookupElementaryType("int")
	require.True(t, ok)
	assert.Equal(t, KindINT, typ.Kind())

	typ, ok = LookupElementaryType("Int")
	require.True(t, ok)
	assert.Equal(t, KindINT, typ.Kind())

	typ, ok = LookupElementaryType("rEaL")
	require.True(t, ok)
	assert.Equal(t, KindREAL, typ.Kind())
}

func TestBuiltinLookupAliases(t *testing.T) {
	// DATE_AND_TIME and DT are aliases
	dt1, ok1 := LookupElementaryType("DATE_AND_TIME")
	dt2, ok2 := LookupElementaryType("DT")
	require.True(t, ok1)
	require.True(t, ok2)
	assert.True(t, dt1.Equal(dt2))

	// TIME_OF_DAY and TOD are aliases
	tod1, ok1 := LookupElementaryType("TIME_OF_DAY")
	tod2, ok2 := LookupElementaryType("TOD")
	require.True(t, ok1)
	require.True(t, ok2)
	assert.True(t, tod1.Equal(tod2))
}

func TestBuiltinLookupNotFound(t *testing.T) {
	_, ok := LookupElementaryType("NONEXISTENT")
	assert.False(t, ok)

	_, ok = LookupElementaryType("FB_Motor")
	assert.False(t, ok)
}

func TestBuiltinFunctions_Arithmetic(t *testing.T) {
	for _, name := range []string{"ADD", "SUB", "MUL", "DIV"} {
		fn, ok := BuiltinFunctions[name]
		require.True(t, ok, "%s should be registered", name)
		assert.Equal(t, name, fn.Name)
		assert.Len(t, fn.Params, 2)
		// Parameters should have ANY_NUM constraint
		assert.NotNil(t, fn.Params[0].GenericConstraint, "%s param should have generic constraint", name)
		assert.True(t, fn.Params[0].GenericConstraint(KindINT), "%s should accept INT", name)
		assert.True(t, fn.Params[0].GenericConstraint(KindREAL), "%s should accept REAL", name)
		assert.False(t, fn.Params[0].GenericConstraint(KindBOOL), "%s should reject BOOL", name)
	}
}

func TestBuiltinFunctions_MOD(t *testing.T) {
	fn, ok := BuiltinFunctions["MOD"]
	require.True(t, ok)
	assert.Len(t, fn.Params, 2)
	// MOD is integer-only
	assert.True(t, fn.Params[0].GenericConstraint(KindINT))
	assert.False(t, fn.Params[0].GenericConstraint(KindREAL))
}

func TestBuiltinFunctions_Math(t *testing.T) {
	for _, name := range []string{"ABS", "SQRT", "SIN", "COS"} {
		fn, ok := BuiltinFunctions[name]
		require.True(t, ok, "%s should be registered", name)
		assert.Len(t, fn.Params, 1)
		assert.True(t, fn.Params[0].GenericConstraint(KindREAL))
		assert.False(t, fn.Params[0].GenericConstraint(KindINT))
	}
}

func TestBuiltinFunctions_MinMax(t *testing.T) {
	for _, name := range []string{"MIN", "MAX"} {
		fn, ok := BuiltinFunctions[name]
		require.True(t, ok)
		assert.Len(t, fn.Params, 2)
	}
}

func TestBuiltinFunctions_LIMIT(t *testing.T) {
	fn, ok := BuiltinFunctions["LIMIT"]
	require.True(t, ok)
	assert.Len(t, fn.Params, 3)
	assert.Equal(t, "MN", fn.Params[0].Name)
	assert.Equal(t, "IN", fn.Params[1].Name)
	assert.Equal(t, "MX", fn.Params[2].Name)
}

func TestBuiltinFunctions_SEL(t *testing.T) {
	fn, ok := BuiltinFunctions["SEL"]
	require.True(t, ok)
	assert.Len(t, fn.Params, 3)
	assert.Equal(t, KindBOOL, fn.Params[0].Type.Kind()) // selector is BOOL
}

func TestBuiltinFunctions_Conversions(t *testing.T) {
	conversions := []struct {
		name     string
		fromKind TypeKind
		toKind   TypeKind
	}{
		{"INT_TO_REAL", KindINT, KindREAL},
		{"REAL_TO_INT", KindREAL, KindINT},
		{"BOOL_TO_INT", KindBOOL, KindINT},
		{"INT_TO_STRING", KindINT, KindSTRING},
	}
	for _, tc := range conversions {
		fn, ok := BuiltinFunctions[tc.name]
		require.True(t, ok, "%s should be registered", tc.name)
		assert.Equal(t, tc.fromKind, fn.Params[0].Type.Kind(), "%s input type", tc.name)
		assert.Equal(t, tc.toKind, fn.ReturnType.Kind(), "%s return type", tc.name)
	}
}

func TestBuiltinFunctions_String(t *testing.T) {
	fn, ok := BuiltinFunctions["LEN"]
	require.True(t, ok)
	assert.Equal(t, KindINT, fn.ReturnType.Kind())
	assert.True(t, fn.Params[0].GenericConstraint(KindSTRING))

	fn, ok = BuiltinFunctions["CONCAT"]
	require.True(t, ok)
	assert.Len(t, fn.Params, 2)
	assert.Equal(t, KindSTRING, fn.ReturnType.Kind())
}

func TestBuiltinFunctions_MOVE(t *testing.T) {
	fn, ok := BuiltinFunctions["MOVE"]
	require.True(t, ok)
	assert.Len(t, fn.Params, 1)
}

func TestBuiltinFunctionCount(t *testing.T) {
	// Should have ~20+ standard function signatures
	assert.GreaterOrEqual(t, len(BuiltinFunctions), 20, "should have at least 20 builtin functions")
}
