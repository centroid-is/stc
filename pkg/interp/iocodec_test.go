package interp

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ioCodecSrc = `
TYPE E_Mode : (Idle := 0, Run := 5);
END_TYPE
TYPE E_Wide : (WA := 1, WB := 70000) DINT;
END_TYPE
TYPE ST_Inner :
STRUCT
	k : SINT;
END_STRUCT
END_TYPE
TYPE ST_Mix :
STRUCT
	b0 : BOOL;
	b1 : BOOL;
	n  : INT;
	inner : ST_Inner;
END_STRUCT
END_TYPE
TYPE AMSADDR :
STRUCT
	netId : ARRAY[0..5] OF BYTE;
	port  : UINT;
END_STRUCT
END_TYPE
VAR_GLOBAL
	vI  : INT;
	vU  : UINT;
	vS  : SINT;
	vD  : DINT;
	vL  : LINT;
	vR  : REAL;
	vLR : LREAL;
	vE  : E_Mode;
	vEW : E_Wide;
	vM  : ST_Mix;
	vA  : AMSADDR;
	vB  : BOOL;
	a1  : ARRAY[1..8] OF BOOL;
	a0  : ARRAY[0..7] OF BOOL;
	w1  : ARRAY[1..2] OF INT;
	wd1 : ARRAY[1..2] OF WORD;
	wd0 : ARRAY[0..1] OF WORD;
	s1  : ARRAY[1..2] OF ST_Inner;
END_VAR
`

// codecFixture returns a codec over ioCodecSrc's types and a reader of the
// GVL variables (current values and declared specs).
func codecFixture(t *testing.T) (ioCodec, func(name string) (Value, ast.TypeSpec)) {
	t.Helper()
	res := parser.Parse("codec.st", ioCodecSrc)
	require.Empty(t, res.Diags)
	rt, err := NewRuntime([]*ast.SourceFile{res.File})
	require.NoError(t, err)
	c := ioCodec{interp: rt.interp}
	specs := map[string]ast.TypeSpec{}
	for _, d := range res.File.Declarations {
		if g, ok := d.(*ast.GVLDecl); ok {
			for _, vb := range g.Blocks {
				for _, vd := range vb.Declarations {
					for _, n := range vd.Names {
						specs[n.Name] = vd.Type
					}
				}
			}
		}
	}
	env := rt.interp.GlobalParent()
	return c, func(name string) (Value, ast.TypeSpec) {
		v, ok := env.Get(name)
		require.True(t, ok, name)
		return v, specs[name]
	}
}

func le(n int, u uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, u)
	return b[:n]
}

func TestIOCodecScalars(t *testing.T) {
	c, get := codecFixture(t)
	cases := []struct {
		name string
		buf  []byte
		want Value
	}{
		{"vI", []byte{0xFB, 0xFF}, Value{Kind: ValInt, Int: -5, IECType: types.KindINT}},
		{"vU", []byte{0xFB, 0xFF}, Value{Kind: ValInt, Int: 65531, IECType: types.KindUINT}},
		{"vS", []byte{0xFB}, Value{Kind: ValInt, Int: -5, IECType: types.KindSINT}},
		{"vD", le(4, 0xFFFFFFFB), Value{Kind: ValInt, Int: -5, IECType: types.KindDINT}},
		{"vL", le(8, 0xFFFFFFFFFFFFFFFB), Value{Kind: ValInt, Int: -5, IECType: types.KindLINT}},
		{"vR", le(4, uint64(math.Float32bits(1.5))), Value{Kind: ValReal, Real: 1.5, IECType: types.KindREAL}},
		{"vLR", le(8, math.Float64bits(-2.25)), Value{Kind: ValReal, Real: -2.25, IECType: types.KindLREAL}},
		{"vB", []byte{0x01}, Value{Kind: ValBool, Bool: true, IECType: types.KindBOOL}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cur, spec := get(tc.name)
			got := c.decodeBytes(cur, spec, tc.buf, 0)
			assert.Equal(t, tc.want.Kind, got.Kind)
			assert.Equal(t, tc.want.Int, got.Int)
			assert.Equal(t, tc.want.Real, got.Real)
			assert.Equal(t, tc.want.Bool, got.Bool)
			assert.Equal(t, tc.want.IECType, got.IECType)
			wantBits := len(tc.buf) * 8
			if cur.Kind == ValBool {
				wantBits = 1
			}
			assert.Equal(t, wantBits, c.bitSize(cur, spec))
			// encode is the exact inverse
			out := make([]byte, len(tc.buf))
			c.encodeBytes(got, spec, out, 0)
			assert.Equal(t, tc.buf, out)
		})
	}
}

func TestIOCodecEnum(t *testing.T) {
	c, get := codecFixture(t)
	cur, spec := get("vE")
	got := c.decodeBytes(cur, spec, []byte{5, 0}, 0)
	assert.Equal(t, int64(5), got.Int)
	assert.Equal(t, "E_MODE", got.Enum)
	assert.Equal(t, types.KindINT, got.IECType)
	assert.Equal(t, 16, c.bitSize(cur, spec))
	// an unknown ordinal is kept as the integer
	got = c.decodeBytes(cur, spec, []byte{9, 0}, 0)
	assert.Equal(t, int64(9), got.Int)
	assert.Equal(t, "E_MODE", got.Enum)

	cur, spec = get("vEW")
	got = c.decodeBytes(cur, spec, le(4, 70000), 0)
	assert.Equal(t, int64(70000), got.Int)
	assert.Equal(t, types.KindDINT, got.IECType)
	out := make([]byte, 4)
	c.encodeBytes(got, spec, out, 0)
	assert.Equal(t, le(4, 70000), out)
}

func TestIOCodecStructAndArray(t *testing.T) {
	c, get := codecFixture(t)
	cur, spec := get("vM")
	// b0=1, b1=0 in bits 0..1; n=-5 in bits 2..17; inner.k=-2 in bits 18..25
	var raw uint64 = 1 | uint64(0xFFFB)<<2 | uint64(0xFE)<<18
	buf := le(4, raw)
	assert.Equal(t, 26, c.bitSize(cur, spec))
	got := c.decodeBytes(cur, spec, buf, 0)
	assert.True(t, got.Struct["B0"].Bool)
	assert.False(t, got.Struct["B1"].Bool)
	assert.Equal(t, int64(-5), got.Struct["N"].Int)
	assert.Equal(t, int64(-2), got.Struct["INNER"].Struct["K"].Int)
	out := make([]byte, 4)
	c.encodeBytes(got, spec, out, 0)
	assert.Equal(t, buf, out)

	cur, spec = get("vA")
	buf = []byte{192, 168, 1, 10, 1, 1, 0x53, 0x03}
	assert.Equal(t, 64, c.bitSize(cur, spec))
	got = c.decodeBytes(cur, spec, buf, 0)
	require.Len(t, got.Struct["NETID"].Array, 6)
	assert.Equal(t, int64(192), got.Struct["NETID"].Array[0].Int)
	assert.Equal(t, int64(10), got.Struct["NETID"].Array[3].Int)
	assert.Equal(t, int64(851), got.Struct["PORT"].Int)
	out = make([]byte, 8)
	c.encodeBytes(got, spec, out, 0)
	assert.Equal(t, buf, out)
}

func TestIOCodecBitOffsetAndSize(t *testing.T) {
	c, get := codecFixture(t)
	cur, spec := get("vB")
	buf := []byte{0x08}
	assert.True(t, c.decodeBytes(cur, spec, buf, 3).Bool)
	assert.False(t, c.decodeBytes(cur, spec, buf, 2).Bool)
	out := []byte{0}
	c.encodeBytes(BoolValue(true), spec, out, 5)
	assert.Equal(t, byte(0x20), out[0])
	assert.Equal(t, 0, c.bitSize(Value{Kind: ValString}, nil))
}
