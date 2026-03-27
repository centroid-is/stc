package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrimitiveTypeKinds(t *testing.T) {
	// Each IEC elementary type has a distinct TypeKind constant and String() returns its name.
	kinds := []struct {
		kind TypeKind
		name string
	}{
		{KindBOOL, "BOOL"},
		{KindBYTE, "BYTE"},
		{KindWORD, "WORD"},
		{KindDWORD, "DWORD"},
		{KindLWORD, "LWORD"},
		{KindSINT, "SINT"},
		{KindINT, "INT"},
		{KindDINT, "DINT"},
		{KindLINT, "LINT"},
		{KindUSINT, "USINT"},
		{KindUINT, "UINT"},
		{KindUDINT, "UDINT"},
		{KindULINT, "ULINT"},
		{KindREAL, "REAL"},
		{KindLREAL, "LREAL"},
		{KindSTRING, "STRING"},
		{KindWSTRING, "WSTRING"},
		{KindTIME, "TIME"},
		{KindDATE, "DATE"},
		{KindDT, "DT"},
		{KindTOD, "TOD"},
		{KindCHAR, "CHAR"},
		{KindWCHAR, "WCHAR"},
	}

	// Verify all kinds are distinct
	seen := make(map[TypeKind]string)
	for _, tc := range kinds {
		if prev, ok := seen[tc.kind]; ok {
			t.Fatalf("TypeKind %d is shared by %s and %s", tc.kind, prev, tc.name)
		}
		seen[tc.kind] = tc.name
		assert.Equal(t, tc.name, tc.kind.String(), "String() for %s", tc.name)
	}

	// Verify we have at least 23 distinct types
	assert.GreaterOrEqual(t, len(kinds), 23)
}

func TestInvalidType(t *testing.T) {
	assert.Equal(t, "Invalid", KindInvalid.String())
	p := &PrimitiveType{Kind_: KindInvalid}
	assert.Equal(t, KindInvalid, p.Kind())
	assert.Equal(t, "Invalid", p.String())
}

func TestVoidType(t *testing.T) {
	assert.Equal(t, "VOID", KindVoid.String())
}

func TestTypeInterface(t *testing.T) {
	// All concrete types implement the Type interface.
	var _ Type = &PrimitiveType{}
	var _ Type = &ArrayType{}
	var _ Type = &StructType{}
	var _ Type = &EnumType{}
	var _ Type = &FunctionBlockType{}
	var _ Type = &FunctionType{}
	var _ Type = &PointerType{}
	var _ Type = &ReferenceType{}
}

func TestPrimitiveTypeEqual(t *testing.T) {
	a := &PrimitiveType{Kind_: KindINT}
	b := &PrimitiveType{Kind_: KindINT}
	c := &PrimitiveType{Kind_: KindDINT}

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c))
}

func TestArrayType(t *testing.T) {
	elem := &PrimitiveType{Kind_: KindINT}
	arr := &ArrayType{
		ElementType: elem,
		Dimensions:  []ArrayDimension{{Low: 0, High: 9}},
	}
	assert.Equal(t, KindArray, arr.Kind())
	assert.Contains(t, arr.String(), "ARRAY")

	// Equal checks
	arr2 := &ArrayType{
		ElementType: &PrimitiveType{Kind_: KindINT},
		Dimensions:  []ArrayDimension{{Low: 0, High: 9}},
	}
	assert.True(t, arr.Equal(arr2))

	arr3 := &ArrayType{
		ElementType: &PrimitiveType{Kind_: KindDINT},
		Dimensions:  []ArrayDimension{{Low: 0, High: 9}},
	}
	assert.False(t, arr.Equal(arr3))
}

func TestStructType(t *testing.T) {
	s := &StructType{
		Name: "MyStruct",
		Members: []StructMember{
			{Name: "x", Type: &PrimitiveType{Kind_: KindREAL}},
			{Name: "y", Type: &PrimitiveType{Kind_: KindREAL}},
		},
	}
	assert.Equal(t, KindStruct, s.Kind())
	assert.Equal(t, "MyStruct", s.String())
}

func TestEnumType(t *testing.T) {
	e := &EnumType{
		Name:     "E_Color",
		BaseType: KindINT,
		Values:   []string{"RED", "GREEN", "BLUE"},
	}
	assert.Equal(t, KindEnum, e.Kind())
	assert.Equal(t, "E_Color", e.String())
}

func TestFunctionBlockType(t *testing.T) {
	fb := &FunctionBlockType{
		Name: "FB_Motor",
		Inputs: []Parameter{
			{Name: "Start", Type: &PrimitiveType{Kind_: KindBOOL}, Direction: DirInput},
		},
		Outputs: []Parameter{
			{Name: "Running", Type: &PrimitiveType{Kind_: KindBOOL}, Direction: DirOutput},
		},
	}
	assert.Equal(t, KindFunctionBlock, fb.Kind())
	assert.Equal(t, "FB_Motor", fb.String())
}

func TestFunctionType(t *testing.T) {
	fn := &FunctionType{
		Name:       "ADD",
		ReturnType: &PrimitiveType{Kind_: KindINT},
		Params: []Parameter{
			{Name: "IN1", Type: &PrimitiveType{Kind_: KindINT}, Direction: DirInput},
			{Name: "IN2", Type: &PrimitiveType{Kind_: KindINT}, Direction: DirInput},
		},
	}
	assert.Equal(t, KindFunction, fn.Kind())
	assert.Equal(t, "ADD", fn.String())
}

func TestPointerType(t *testing.T) {
	p := &PointerType{BaseType: &PrimitiveType{Kind_: KindINT}}
	assert.Equal(t, KindPointer, p.Kind())
	assert.Equal(t, "POINTER TO INT", p.String())

	p2 := &PointerType{BaseType: &PrimitiveType{Kind_: KindINT}}
	assert.True(t, p.Equal(p2))
}

func TestReferenceType(t *testing.T) {
	r := &ReferenceType{BaseType: &PrimitiveType{Kind_: KindDINT}}
	assert.Equal(t, KindReference, r.Kind())
	assert.Equal(t, "REFERENCE TO DINT", r.String())
}

func TestInvalidSentinel(t *testing.T) {
	require.NotNil(t, Invalid)
	assert.Equal(t, KindInvalid, Invalid.Kind())
}
