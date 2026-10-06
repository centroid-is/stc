package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestWrapInt(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		kind types.TypeKind
		want int64
	}{
		{"SINT over", 128, types.KindSINT, -128},
		{"SINT under", -129, types.KindSINT, 127},
		{"SINT in range", -5, types.KindSINT, -5},
		{"USINT over", 256, types.KindUSINT, 0},
		{"USINT under", -1, types.KindUSINT, 255},
		{"BYTE over", 256, types.KindBYTE, 0},
		{"BYTE under", -1, types.KindBYTE, 255},
		{"INT over", 32768, types.KindINT, -32768},
		{"INT under", -32769, types.KindINT, 32767},
		{"UINT over", 65536, types.KindUINT, 0},
		{"UINT under", -1, types.KindUINT, 65535},
		{"WORD over", 65536 + 9, types.KindWORD, 9},
		{"WORD under", -1, types.KindWORD, 65535},
		{"DINT over", 1 << 31, types.KindDINT, -2147483648},
		{"DINT under", -(1 << 31) - 1, types.KindDINT, 2147483647},
		{"UDINT over", 1 << 32, types.KindUDINT, 0},
		{"UDINT under", -1, types.KindUDINT, 4294967295},
		{"DWORD over", 1<<32 + 1, types.KindDWORD, 1},
		{"DWORD under", -1, types.KindDWORD, 4294967295},
		{"LINT unchanged", -1, types.KindLINT, -1},
		{"ULINT bit pattern", -1, types.KindULINT, -1},
		{"LWORD bit pattern", -1, types.KindLWORD, -1},
		{"unknown kind unchanged", 1 << 40, types.KindInvalid, 1 << 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, wrapInt(tt.n, tt.kind))
		})
	}
}

func TestStoreAs(t *testing.T) {
	t.Run("int slot wraps and keeps type and enum tag", func(t *testing.T) {
		dst := Value{Kind: ValInt, IECType: types.KindINT, Enum: "E"}
		got := storeAs(dst, IntValue(70000))
		assert.Equal(t, ValInt, got.Kind)
		assert.Equal(t, types.KindINT, got.IECType)
		assert.Equal(t, "E", got.Enum)
		assert.Equal(t, int64(4464), got.Int)
	})
	t.Run("enum value stored into plain int loses tag", func(t *testing.T) {
		dst := Zero(types.KindDINT)
		got := storeAs(dst, Value{Kind: ValInt, Int: 2, IECType: types.KindINT, Enum: "E"})
		assert.Equal(t, "", got.Enum)
		assert.Equal(t, types.KindDINT, got.IECType)
	})
	t.Run("untyped int slot keeps value type", func(t *testing.T) {
		dst := Value{Kind: ValInt}
		got := storeAs(dst, Value{Kind: ValInt, Int: 1 << 40, IECType: types.KindLINT})
		assert.Equal(t, types.KindLINT, got.IECType)
		assert.Equal(t, int64(1<<40), got.Int)
	})
	t.Run("int into real slot converts", func(t *testing.T) {
		got := storeAs(Zero(types.KindREAL), IntValue(3))
		assert.Equal(t, ValReal, got.Kind)
		assert.Equal(t, 3.0, got.Real)
		assert.Equal(t, types.KindREAL, got.IECType)
	})
	t.Run("lreal into real slot takes slot type", func(t *testing.T) {
		got := storeAs(Zero(types.KindREAL), RealValue(1.5))
		assert.Equal(t, types.KindREAL, got.IECType)
		assert.Equal(t, 1.5, got.Real)
	})
	t.Run("real into untyped real slot keeps value type", func(t *testing.T) {
		got := storeAs(Value{Kind: ValReal}, RealValue(1.5))
		assert.Equal(t, types.KindLREAL, got.IECType)
	})
	t.Run("non-numeric slots return v unchanged", func(t *testing.T) {
		v := StringValue("x")
		assert.Equal(t, v, storeAs(Zero(types.KindSTRING), v))
		b := BoolValue(true)
		assert.Equal(t, b, storeAs(Zero(types.KindBOOL), b))
		r := RealValue(2.5)
		assert.Equal(t, r, storeAs(Zero(types.KindINT), r))
	})
}
