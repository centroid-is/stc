package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// elems lists the declared elements low..high of an array value.
func elems(v Value) []Value { return v.Array[v.ArrayLow:] }

// TestIOCodecArrayLowerBound is the CR-01 regression: arrays are stored
// with padding slots below ArrayLow, and the codec must map the first
// declared element, not the padding, to the first image bits.
func TestIOCodecArrayLowerBound(t *testing.T) {
	c, get := codecFixture(t)

	for _, name := range []string{"a1", "a0"} {
		t.Run(name, func(t *testing.T) {
			cur, spec := get(name)
			assert.Equal(t, 8, c.bitSize(cur, spec))
			got := c.decodeBytes(cur, spec, []byte{0x81}, 0)
			el := elems(got)
			require.Len(t, el, 8)
			assert.True(t, el[0].Bool, "first element is bit 0")
			for i := 1; i < 7; i++ {
				assert.False(t, el[i].Bool, i)
			}
			assert.True(t, el[7].Bool, "last element is bit 7")
			out := []byte{0}
			c.encodeBytes(got, spec, out, 0)
			assert.Equal(t, []byte{0x81}, out)
		})
	}

	cur, spec := get("w1")
	assert.Equal(t, 32, c.bitSize(cur, spec))
	got := c.decodeBytes(cur, spec, []byte{0x34, 0x12, 0x78, 0x56}, 0)
	assert.Equal(t, int64(0x1234), got.Array[1].Int)
	assert.Equal(t, int64(0x5678), got.Array[2].Int)
	out := make([]byte, 4)
	c.encodeBytes(got, spec, out, 0)
	assert.Equal(t, []byte{0x34, 0x12, 0x78, 0x56}, out)

	for name, first := range map[string]int{"wd1": 1, "wd0": 0} {
		cur, spec := get(name)
		assert.Equal(t, 32, c.bitSize(cur, spec), name)
		got := c.decodeBytes(cur, spec, []byte{0xCD, 0xAB, 0xFF, 0xFF}, 0)
		assert.Equal(t, int64(0xABCD), got.Array[first].Int, name)
		assert.Equal(t, int64(0xFFFF), got.Array[first+1].Int, name)
	}

	cur, spec = get("s1")
	assert.Equal(t, 16, c.bitSize(cur, spec))
	got = c.decodeBytes(cur, spec, []byte{0xFE, 0x03}, 0)
	assert.Equal(t, int64(-2), got.Array[1].Struct["K"].Int)
	assert.Equal(t, int64(3), got.Array[2].Struct["K"].Int)
	out = make([]byte, 2)
	c.encodeBytes(got, spec, out, 0)
	assert.Equal(t, []byte{0xFE, 0x03}, out)
}

const ioArrBindSrc = `
VAR_GLOBAL
	aIn  : ARRAY[1..8] OF BOOL;
	aOut : ARRAY[1..8] OF BOOL;
	wIn  : ARRAY[1..2] OF INT;
	wOut : ARRAY[1..2] OF WORD;
	zIn  : ARRAY[0..1] OF INT;
END_VAR
PROGRAM P
VAR
	x : INT;
END_VAR
x := x;
END_PROGRAM
`

// TestIOBindArrayLowerBound links 1-based and 0-based arrays with TcLinkTo
// bindings in both directions.
func TestIOBindArrayLowerBound(t *testing.T) {
	e := gvlEngine(t, "IO.st", ioArrBindSrc)
	net := ioNet()
	bs := []ecat.Binding{
		bind(ecat.DirIn, 0, 8, "ARRAY [1..8] OF BOOL", "IO", "AIN"),
		bind(ecat.DirIn, 2, 32, "ARRAY [1..2] OF INT", "IO", "WIN"),
		bind(ecat.DirIn, 6, 32, "ARRAY [0..1] OF INT", "IO", "ZIN"),
		bind(ecat.DirOut, 0, 8, "ARRAY [1..8] OF BOOL", "IO", "AOUT"),
		bind(ecat.DirOut, 2, 32, "ARRAY [1..2] OF WORD", "IO", "WOUT"),
	}
	e.SetIOBinder(NewIOBinder(bs, net))
	img := net.Images().Get(ioMaster)
	img.In[0] = 0x01
	copy(img.In[2:], []byte{0x34, 0x12, 0x78, 0x56, 0x01, 0x00, 0xFF, 0xFF})
	e.Initialize()
	gvl := e.interp.lookupGVL("IO")
	aOut, _ := gvl.Get("AOUT")
	aOut = aOut.Clone()
	aOut.Array[1] = BoolValue(true)
	gvl.Set("AOUT", aOut)
	wOut, _ := gvl.Get("WOUT")
	wOut = wOut.Clone()
	wOut.Array[1].Int = 0xBEEF
	wOut.Array[2].Int = 0x0102
	gvl.Set("WOUT", wOut)
	e.ioBinder.preScan(0)
	e.ioBinder.postScan()
	require.Empty(t, e.ioBinder.Errors())

	a := ioRead(t, e, "IO", "AIN")
	assert.True(t, a.Array[1].Bool, "aIn[1] is bit 0")
	for i := 2; i <= 8; i++ {
		assert.False(t, a.Array[i].Bool, i)
	}
	w := ioRead(t, e, "IO", "WIN")
	assert.Equal(t, int64(0x1234), w.Array[1].Int)
	assert.Equal(t, int64(0x5678), w.Array[2].Int)
	z := ioRead(t, e, "IO", "ZIN")
	assert.Equal(t, int64(1), z.Array[0].Int)
	assert.Equal(t, int64(-1), z.Array[1].Int)

	assert.Equal(t, byte(0x01), img.Out[0], "aOut[1] is written to bit 0")
	assert.Equal(t, []byte{0xEF, 0xBE, 0x02, 0x01}, img.Out[2:6])
}

const projArrSrc = `
VAR_GLOBAL
	aIn  AT %IB0 : ARRAY[1..8] OF BOOL;
	wIn  AT %IW2 : ARRAY[1..2] OF INT;
	zIn  AT %IW6 : ARRAY[0..1] OF WORD;
	aOut AT %QB0 : ARRAY[1..8] OF BOOL;
	wOut AT %QW2 : ARRAY[1..2] OF WORD;
	aw   AT %I*  : ARRAY[1..2] OF INT;
END_VAR
PROGRAM MAIN
VAR
	first : BOOL;
	last  : BOOL;
END_VAR
first := GVL.aIn[1];
last := GVL.aIn[8];
GVL.aOut[1] := TRUE;
GVL.wOut[1] := 16#1234;
GVL.wOut[2] := 16#5678;
END_PROGRAM
`

// TestProjectATArrayLowerBound binds 1-based and 0-based arrays with
// explicit and wildcard AT addresses.
func TestProjectATArrayLowerBound(t *testing.T) {
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{parseRT(t, "GVL.st", projArrSrc)}})
	require.NoError(t, err)
	addr, n, ok := p.IOSlot("GVL.aw")
	require.True(t, ok)
	assert.Equal(t, 4, n, "ARRAY[1..2] OF INT is 4 bytes")
	assert.Equal(t, iomap.AreaInput, addr.Area)

	require.NoError(t, p.SetIOBytes('I', 0, []byte{0x81, 0x00, 0x34, 0x12, 0x78, 0x56, 0x01, 0x00, 0x02, 0x00}))
	require.NoError(t, p.SetIOBytes('I', addr.ByteOffset, []byte{0xFF, 0xFF, 0x07, 0x00}))
	require.NoError(t, p.Tick())
	rt := p.Runtime()
	get := func(path string) Value {
		v, err := rt.Get(path)
		require.NoError(t, err, path)
		return v
	}
	assert.True(t, get("MAIN.first").Bool)
	assert.True(t, get("MAIN.last").Bool)
	assert.False(t, get("GVL.aIn[2]").Bool)
	assert.Equal(t, int64(0x1234), get("GVL.wIn[1]").Int)
	assert.Equal(t, int64(0x5678), get("GVL.wIn[2]").Int)
	assert.Equal(t, int64(1), get("GVL.zIn[0]").Int)
	assert.Equal(t, int64(2), get("GVL.zIn[1]").Int)
	assert.Equal(t, int64(-1), get("GVL.aw[1]").Int)
	assert.Equal(t, int64(7), get("GVL.aw[2]").Int)

	out, err := p.IOBytes('Q', 0, 6)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x00, 0x34, 0x12, 0x78, 0x56}, out)
}
