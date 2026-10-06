package interp

import (
	"math"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ioBindSrc = `
TYPE AMSADDR :
STRUCT
	netId : ARRAY[0..5] OF BYTE;
	port  : UINT;
END_STRUCT
END_TYPE
TYPE T_AmsNetIdArr : ARRAY[0..5] OF BYTE;
END_TYPE
TYPE E_Mode : (Idle := 0, Run := 5);
END_TYPE
TYPE ST_In :
STRUCT
	I1 : BOOL;
	W  : WORD;
END_STRUCT
END_TYPE
FUNCTION_BLOCK FB_Out
VAR
	Q : UINT;
END_VAR
END_FUNCTION_BLOCK
VAR_GLOBAL
	B  : BOOL;
	S  : SINT;
	I  : INT;
	U  : UINT;
	D  : DINT;
	UD : UDINT;
	L  : LINT;
	R  : REAL;
	LR : LREAL;
	E  : E_Mode;
	NA : T_AmsNetIdArr;
	AA : AMSADDR;
	ST1 : ST_In;
	FB1 : FB_Out;
	TXT : STRING;
END_VAR
PROGRAM P
VAR
	x : INT;
END_VAR
x := x;
END_PROGRAM
`

const ioMaster = "M"

func ioNet() *ecat.Network {
	topo := &ecat.Topology{Masters: []*ecat.Master{{Name: ioMaster, InBytes: 256, OutBytes: 256}}}
	return ecat.NewNetwork(topo, nil)
}

func bind(dir ecat.Dir, byteOff, bits int, typ string, steps ...string) ecat.Binding {
	return ecat.Binding{
		Var:  ecat.LinkedVar{Steps: steps, TypeName: typ, BitWidth: bits, Dir: dir},
		Slot: ecat.Slot{Master: ioMaster, Dir: dir, Byte: byteOff, BitLen: bits},
	}
}

func ioRead(t *testing.T, e *ScanCycleEngine, steps ...string) Value {
	t.Helper()
	p := &RefPath{Env: e.interp.lookupGVL(steps[0]), Var: steps[1]}
	for _, s := range steps[2:] {
		p = p.with(RefStep{Member: s})
	}
	v, err := readRef(p)
	require.NoError(t, err)
	return v
}

// ioScan runs the binder's scan-boundary copies around an empty scan.
func ioScan(e *ScanCycleEngine) {
	e.Initialize()
	e.ioBinder.preScan(time.Millisecond)
	e.ioBinder.postScan()
}

func TestIOBindCodecRoundTrip(t *testing.T) {
	f32 := uint64(math.Float32bits(1.5))
	f64 := math.Float64bits(-2.25)
	cases := []struct {
		name, typ string
		bits      int
		raw       uint64
		check     func(t *testing.T, v Value)
	}{
		{"B", "BOOL", 1, 1, func(t *testing.T, v Value) { assert.Equal(t, ValBool, v.Kind); assert.True(t, v.Bool) }},
		{"S", "SINT", 8, 0x80, func(t *testing.T, v Value) { assert.Equal(t, int64(-128), v.Int) }},
		{"I", "INT", 16, 0xFFFF, func(t *testing.T, v Value) { assert.Equal(t, int64(-1), v.Int) }},
		{"U", "UINT", 16, 0xFFFF, func(t *testing.T, v Value) { assert.Equal(t, int64(65535), v.Int) }},
		{"D", "DINT", 32, 0xFFFFFFFE, func(t *testing.T, v Value) { assert.Equal(t, int64(-2), v.Int) }},
		{"UD", "UDINT", 32, 0xFFFFFFFF, func(t *testing.T, v Value) { assert.Equal(t, int64(4294967295), v.Int) }},
		{"L", "LINT", 64, 0xFFFFFFFFFFFFFFFD, func(t *testing.T, v Value) { assert.Equal(t, int64(-3), v.Int) }},
		{"R", "REAL", 32, f32, func(t *testing.T, v Value) { assert.Equal(t, ValReal, v.Kind); assert.Equal(t, 1.5, v.Real) }},
		{"LR", "LREAL", 64, f64, func(t *testing.T, v Value) { assert.Equal(t, -2.25, v.Real) }},
		{"E", "E_Mode", 16, 5, func(t *testing.T, v Value) {
			assert.Equal(t, ValInt, v.Kind)
			assert.Equal(t, int64(5), v.Int)
			assert.Equal(t, "E_MODE", v.Enum)
		}},
		{"NA", "T_AmsNetIdArr", 48, 0x060504030201, func(t *testing.T, v Value) {
			require.Len(t, v.Array, 6)
			for i, el := range v.Array {
				assert.Equal(t, int64(i+1), el.Int)
			}
		}},
		{"AA", "AMSADDR", 64, 851<<48 | 0x060504030201, func(t *testing.T, v Value) {
			require.Len(t, v.Struct["NETID"].Array, 6)
			assert.Equal(t, int64(1), v.Struct["NETID"].Array[0].Int)
			assert.Equal(t, int64(6), v.Struct["NETID"].Array[5].Int)
			assert.Equal(t, int64(851), v.Struct["PORT"].Int)
		}},
	}
	e := gvlEngine(t, "IO.st", ioBindSrc)
	net := ioNet()
	var bs []ecat.Binding
	for i, c := range cases {
		bs = append(bs, bind(ecat.DirIn, i*8, c.bits, c.typ, "IO", c.name))
		bs = append(bs, bind(ecat.DirOut, i*8, c.bits, c.typ, "IO", c.name))
	}
	e.SetIOBinder(NewIOBinder(bs, net))
	img := net.Images().Get(ioMaster)
	for i, c := range cases {
		ecat.WriteBits(img.In, i*8, 0, c.bits, c.raw)
	}
	ioScan(e)
	require.Empty(t, e.ioBinder.Errors())
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, ioRead(t, e, "IO", c.name))
			assert.Equal(t, c.raw, ecat.ReadBits(img.Out, i*8, 0, c.bits), "encode is the inverse of decode")
		})
	}
}

func TestIOBindResolution(t *testing.T) {
	e := gvlEngine(t, "IO.st", ioBindSrc)
	net := ioNet()
	b := NewIOBinder([]ecat.Binding{
		bind(ecat.DirIn, 0, 1, "BOOL", "IO", "ST1", "I1"),
		bind(ecat.DirOut, 0, 16, "UINT", "IO", "FB1", "Q"),
		bind(ecat.DirIn, 2, 16, "INT", "P", "X"),
		bind(ecat.DirIn, 4, 1, "BOOL", "NOPE", "B"),
		bind(ecat.DirIn, 4, 1, "BOOL", "IO", "MISSING"),
		bind(ecat.DirIn, 4, 1, "BOOL", "IO", "ST1", "NOPE"),
		bind(ecat.DirIn, 4, 1, "BOOL", "IO", "FB1", "NOPE"),
		bind(ecat.DirIn, 4, 1, "BOOL", "IO"),
		bind(ecat.DirIn, 4, 48, "STRING", "IO", "TXT"),
		bind(ecat.DirIn, 4, 1, "BOOL", "IO", "FB1"),
		{Var: ecat.LinkedVar{Steps: []string{"IO", "B"}, TypeName: "BOOL", BitWidth: 1, Dir: ecat.DirIn},
			Slot: ecat.Slot{Master: "Z", Dir: ecat.DirIn, BitLen: 1}},
	}, net)
	e.SetIOBinder(b)
	img := net.Images().Get(ioMaster)
	ecat.WriteBits(img.In, 0, 0, 1, 1)
	ecat.WriteBits(img.In, 2, 0, 16, 0x1234)
	ioScan(e)
	assert.Len(t, b.Errors(), 8)
	assert.True(t, ioRead(t, e, "IO", "ST1", "I1").Bool)
	x, _ := e.env.Get("X")
	assert.Equal(t, int64(0x1234), x.Int)

	fb := ioRead(t, e, "IO", "FB1")
	fb.FBRef.Env.Set("Q", Value{Kind: ValInt, Int: 0xBEEF, IECType: fb.FBRef.Env.vars["Q"].IECType})
	ioScan(e)
	assert.Equal(t, uint64(0xBEEF), ecat.ReadBits(img.Out, 0, 0, 16))
	assert.Len(t, b.Errors(), 8, "resolution runs once")
}
