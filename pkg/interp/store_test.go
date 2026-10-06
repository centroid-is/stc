package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestInitialisedIECType(t *testing.T) {
	t.Run("WORD initialiser keeps WORD type", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR w : WORD := 16#9; i : INT := 70000; x : LREAL := 1; END_VAR
END_PROGRAM
`)
		w := progVar(t, eng, "w")
		assert.Equal(t, types.KindWORD, w.IECType)
		assert.Equal(t, int64(9), w.Int)
		i := progVar(t, eng, "i")
		assert.Equal(t, types.KindINT, i.IECType)
		assert.Equal(t, int64(4464), i.Int)
		x := progVar(t, eng, "x")
		assert.Equal(t, ValReal, x.Kind)
		assert.Equal(t, 1.0, x.Real)
	})

	t.Run("bit 16 of an initialised WORD is a runtime error", func(t *testing.T) {
		err := bitRunErr(t, `
PROGRAM P
VAR w : WORD := 16#9; b : BOOL; END_VAR
b := w.16;
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "bit index 16 out of range for 16-bit value")
	})

	t.Run("FB VAR initialiser wraps on increment", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
FUNCTION_BLOCK FB_Count
VAR_OUTPUT o : USINT; END_VAR
VAR c : USINT := 255; END_VAR
c := c + 1;
o := c;
END_FUNCTION_BLOCK

PROGRAM P
VAR f : FB_Count; r : USINT := 7; END_VAR
f();
r := f.o;
END_PROGRAM
`)
		r := progVar(t, eng, "r")
		assert.Equal(t, int64(0), r.Int)
		assert.Equal(t, types.KindUSINT, r.IECType)
	})
}

func TestStoreSites(t *testing.T) {
	t.Run("REF= to INT array element and struct member wraps", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
TYPE S : STRUCT m : UINT; END_STRUCT END_TYPE

PROGRAM P
VAR
  arr : ARRAY[0..1] OF INT;
  s : S;
  r : REFERENCE TO INT;
  q : REFERENCE TO UINT;
END_VAR
r REF= arr[1];
r := 40000;
q REF= s.m;
q := -1;
END_PROGRAM
`)
		arr := progVar(t, eng, "arr")
		assert.Equal(t, int64(-25536), arr.Array[1].Int)
		assert.Equal(t, types.KindINT, arr.Array[1].IECType)
		s := progVar(t, eng, "s")
		assert.Equal(t, int64(65535), s.Struct["M"].Int)
	})

	t.Run("writeRef to a root variable wraps", func(t *testing.T) {
		env := NewEnv(nil)
		env.Define("B", Zero(types.KindBYTE))
		require.NoError(t, writeRef(&RefPath{Env: env, Var: "B"}, IntValue(257)))
		b, _ := env.Get("B")
		assert.Equal(t, int64(1), b.Int)
		assert.Equal(t, types.KindBYTE, b.IECType)
	})

	t.Run("FOR counter keeps its IEC type", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR i : SINT; n : DINT; END_VAR
FOR i := 1 TO 5 DO n := n + 1; END_FOR
END_PROGRAM
`)
		i := progVar(t, eng, "i")
		assert.Equal(t, types.KindSINT, i.IECType)
		assert.Equal(t, int64(6), i.Int)
		assert.Equal(t, int64(5), progVar(t, eng, "n").Int)
	})

	t.Run("FOR start value wraps to the counter type", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR i : SINT; n : DINT; END_VAR
FOR i := 130 TO -126 DO n := n + 1; END_FOR
END_PROGRAM
`)
		// 130 stored into SINT is -126: one pass, then -125 > -126 ends it.
		assert.Equal(t, types.KindSINT, progVar(t, eng, "i").IECType)
		assert.Equal(t, int64(1), progVar(t, eng, "n").Int)
	})

	t.Run("bit write keeps BYTE type", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR b : BYTE := 16#01; s : SINT; END_VAR
b.7 := TRUE;
s.7 := TRUE;
END_PROGRAM
`)
		b := progVar(t, eng, "b")
		assert.Equal(t, types.KindBYTE, b.IECType)
		assert.Equal(t, int64(129), b.Int)
		s := progVar(t, eng, "s")
		assert.Equal(t, types.KindSINT, s.IECType)
		assert.Equal(t, int64(-128), s.Int)
	})
}
