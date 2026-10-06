package interp

import (
	"math"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scanATSrc = `
TYPE E_Mode : (Idle := 0, Run := 5);
END_TYPE
PROGRAM P
VAR
	x AT %IW0 : INT;
	w AT %IW2 : WORD;
	r AT %ID4 : REAL;
	e AT %IW8 : E_Mode;
	q AT %QD0 : REAL := 2.5;
	b AT %QX5.3 : BOOL := TRUE;
	s AT %IB20 : STRING;
END_VAR
END_PROGRAM
`

func rtFrom(t *testing.T, src string) *Runtime {
	t.Helper()
	res := parser.Parse("t.st", src)
	require.Empty(t, res.Diags)
	rt, err := NewRuntime([]*ast.SourceFile{res.File})
	require.NoError(t, err)
	return rt
}

func TestScanATDeclaredType(t *testing.T) {
	rt := rtFrom(t, scanATSrc)
	e := rt.Engine("P")
	io := e.IOTable()
	io.SetWord(iomap.AreaInput, 0, 0xFFFB)
	io.SetWord(iomap.AreaInput, 2, 0xFFFB)
	io.SetDWord(iomap.AreaInput, 4, math.Float32bits(-1.25))
	io.SetWord(iomap.AreaInput, 8, 5)
	io.SetByte(iomap.AreaOutput, 5, 0x81) // neighbouring bits survive
	require.NoError(t, e.Tick(time.Millisecond))

	get := func(n string) Value {
		v, ok := e.env.Get(n)
		require.True(t, ok, n)
		return v
	}
	assert.Equal(t, int64(-5), get("X").Int)
	assert.Equal(t, int64(65531), get("W").Int)
	assert.Equal(t, ValReal, get("R").Kind)
	assert.Equal(t, -1.25, get("R").Real)
	assert.Equal(t, int64(5), get("E").Int)
	assert.Equal(t, "E_MODE", get("E").Enum)
	assert.Equal(t, ValString, get("S").Kind, "a STRING binding keeps its value")

	assert.Equal(t, math.Float32bits(2.5), io.GetDWord(iomap.AreaOutput, 0))
	assert.Equal(t, byte(0x89), io.GetByte(iomap.AreaOutput, 5))
	// writing FALSE clears only bit 3
	e.env.Set("B", BoolValue(false))
	require.NoError(t, e.Tick(time.Millisecond))
	assert.Equal(t, byte(0x81), io.GetByte(iomap.AreaOutput, 5))
}

// Bindings without a declared type keep address-width typing.
func TestScanATWidthFallback(t *testing.T) {
	e := &ScanCycleEngine{interp: New(), ioTable: iomap.NewIOTable(), env: NewEnv(nil)}
	e.ioTable.SetWord(iomap.AreaInput, 0, 0xFFFB)
	b := IOBinding{VarName: "X", Address: iomap.IOAddress{Area: iomap.AreaInput, Size: iomap.SizeWord}}
	assert.Equal(t, int64(65531), e.readIOValue(b).Int)
	e.writeIOValue(IOBinding{VarName: "Y", Address: iomap.IOAddress{Area: iomap.AreaOutput, Size: iomap.SizeWord}}, Value{Kind: ValInt, Int: 7})
	assert.Equal(t, uint16(7), e.ioTable.GetWord(iomap.AreaOutput, 0))
	// a typed binding whose value cannot be copied writes nothing
	str := IOBinding{VarName: "S", Address: iomap.IOAddress{Area: iomap.AreaOutput, ByteOffset: 4, Size: iomap.SizeByte}, Spec: &ast.StringType{}}
	e.writeIOValue(str, Value{Kind: ValString, Int: 9})
	assert.Equal(t, byte(0), e.ioTable.GetByte(iomap.AreaOutput, 4))
	e.ioTable.SetByte(iomap.AreaMemory, 3, 1)
	assert.Equal(t, 0, ioByteLen(ioCodec{}, Value{Kind: ValString}, nil, 0)-1)
	assert.Equal(t, []byte{1}, ioWindow(e.ioTable, iomap.AreaMemory, 3, 1))
}

func TestScanATWidthFallbackSizes(t *testing.T) {
	e := &ScanCycleEngine{interp: New(), ioTable: iomap.NewIOTable(), env: NewEnv(nil)}
	for _, sz := range []iomap.Size{iomap.SizeBit, iomap.SizeByte, iomap.SizeWord, iomap.SizeDWord, iomap.Size('L')} {
		b := IOBinding{VarName: "V", Address: iomap.IOAddress{Area: iomap.AreaMemory, ByteOffset: 8, BitOffset: 1, Size: sz}}
		e.writeIOValue(b, Value{Kind: ValInt, Int: 3, Bool: true})
		got := e.readIOValue(b)
		if sz == iomap.SizeBit {
			assert.True(t, got.Bool)
		} else if sz != iomap.Size('L') {
			assert.Equal(t, int64(3), got.Int, string(sz))
		}
	}
}
