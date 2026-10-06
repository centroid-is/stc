package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bits(k types.TypeKind, v int64) Value { return Value{Kind: ValInt, Int: v, IECType: k} }

func TestBitShiftFunctions(t *testing.T) {
	cases := []struct {
		fn   string
		in   Value
		n    int64
		want int64
		kind types.TypeKind
	}{
		{"SHL", bits(types.KindBYTE, 0x10), 3, 0x80, types.KindBYTE},
		{"SHL", bits(types.KindBYTE, 0x10), 4, 0x00, types.KindBYTE},
		{"SHL", bits(types.KindBYTE, 0x10), 8, 0x00, types.KindBYTE},
		{"SHR", bits(types.KindWORD, 0x8000), 15, 1, types.KindWORD},
		{"SHR", bits(types.KindWORD, 0x8000), 16, 0, types.KindWORD},
		{"ROL", bits(types.KindBYTE, 0x81), 1, 0x03, types.KindBYTE},
		{"ROL", bits(types.KindBYTE, 0x81), 8, 0x81, types.KindBYTE},
		{"ROR", bits(types.KindBYTE, 0x81), 1, 0xC0, types.KindBYTE},
		{"ROR", bits(types.KindLWORD, 1), 1, -1 << 63, types.KindLWORD},
	}
	for _, c := range cases {
		got, err := StdlibFunctions[c.fn]([]Value{c.in, bits(types.KindINT, c.n)})
		require.NoError(t, err, c.fn)
		assert.Equal(t, c.want, got.Int, "%s(%#x, %d)", c.fn, c.in.Int, c.n)
		assert.Equal(t, c.kind, got.IECType, c.fn)
	}
	for _, args := range [][]Value{
		{bits(types.KindBYTE, 1)},
		{BoolValue(true), bits(types.KindINT, 1)},
	} {
		_, err := StdlibFunctions["SHL"](args)
		assert.Error(t, err)
	}

	eng := semRun(t, `PROGRAM Main
VAR m : BYTE; p : UINT; w : WORD; END_VAR
p := 2;
m := SHL(BYTE#16#10, p);
w := UINT_TO_WORD(UINT#65535);
END_PROGRAM
`)
	assert.Equal(t, int64(0x40), progVar(t, eng, "m").Int)
	assert.Equal(t, int64(0xFFFF), progVar(t, eng, "w").Int)
}

func TestTypedIntConversions(t *testing.T) {
	for name, c := range map[string]struct {
		in   Value
		want Value
	}{
		"UINT_TO_WORD": {bits(types.KindUINT, 65535), bits(types.KindWORD, 65535)},
		"DINT_TO_BYTE": {bits(types.KindDINT, 0x1FF), bits(types.KindBYTE, 0xFF)},
		"BOOL_TO_UINT": {BoolValue(true), bits(types.KindUINT, 1)},
		"WORD_TO_INT":  {bits(types.KindWORD, 0xFFFF), bits(types.KindINT, -1)},
		"UDINT_TO_LREAL": {bits(types.KindUDINT, 7),
			Value{Kind: ValReal, Real: 7, IECType: types.KindLREAL}},
	} {
		got, err := StdlibFunctions[name]([]Value{c.in})
		require.NoError(t, err, name)
		assert.Equal(t, c.want.Int, got.Int, name)
		assert.Equal(t, c.want.Real, got.Real, name)
		assert.Equal(t, c.want.IECType, got.IECType, name)
	}
	_, err := StdlibFunctions["UINT_TO_WORD"](nil)
	assert.Error(t, err)
	_, ok := StdlibFunctions["UINT_TO_UINT"]
	assert.False(t, ok)
}
