package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
)

// sdoBufSrc reads and writes an SDO through 1-based byte arrays (HI-01)
// and through ADR() of a struct member or array element (HI-02).
const sdoBufSrc = `
TYPE ST_Sdo :
STRUCT
	nHdr   : BYTE;
	nValue : UINT;
END_STRUCT
END_TYPE
PROGRAM P
VAR_INPUT
	gow, gor, gom, gox, gmw, gew, gou, gob : BOOL;
	net  : STRING;
	addr : UINT;
	idx  : WORD;
	sub  : BYTE;
	cb   : UDINT;
END_VAR
VAR
	wr   : FB_EcCoESdoWrite;
	rd   : FB_EcCoESdoRead;
	rm   : FB_EcCoESdoRead;
	rx   : FB_EcCoESdoRead;
	wm   : FB_EcCoESdoWrite;
	we   : FB_EcCoESdoWrite;
	ru   : FB_EcCoESdoRead;
	u    : UINT;
	rb   : FB_EcCoESdoRead;
	aBuf : ARRAY[1..4] OF BYTE;
	aSrc : ARRAY[1..2] OF BYTE := [16#34, 16#12];
	aDst : ARRAY[1..2] OF BYTE;
	st   : ST_Sdo;
	aw   : ARRAY[1..3] OF UINT;
	stW  : ST_Sdo := (nHdr := 7, nValue := 16#0BAD);
	awW  : ARRAY[0..1] OF UINT := [16#00F0, 16#0F00];
END_VAR
wr(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pSrcBuf := ADR(aSrc), cbBufLen := cb, bExecute := gow);
rd(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pDstBuf := ADR(aDst), cbBufLen := cb, bExecute := gor);
rm(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pDstBuf := ADR(st.nValue), cbBufLen := cb, bExecute := gom);
rx(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pDstBuf := ADR(aw[2]), cbBufLen := cb, bExecute := gox);
wm(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pSrcBuf := ADR(stW.nValue), cbBufLen := cb, bExecute := gmw);
we(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pSrcBuf := ADR(awW[1]), cbBufLen := cb, bExecute := gew);
ru(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pDstBuf := ADR(u), cbBufLen := cb, bExecute := gou);
rb(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pDstBuf := ADR(aBuf[1]), cbBufLen := cb, bExecute := gob);
END_PROGRAM
`

func newSdoBufRig(t *testing.T) *ecRig {
	r := newEcRig(t, sdoBufSrc)
	r.set("net", StringValue(ecNet3))
	r.set("addr", ecUint(1001))
	r.set("idx", ecWord(0x2001))
	r.set("sub", ecByte(5))
	r.set("cb", ecUdint(2))
	return r
}

// TestCoESdoOneBasedBuffers is the HI-01 regression: a 1-based byte array
// buffer maps SDO byte 0 to element 1, in both directions.
func TestCoESdoOneBasedBuffers(t *testing.T) {
	r := newSdoBufRig(t)
	r.pulseOn("gow", 3)
	r.done("wr", 0)
	r.pulseOn("gor", 3)
	r.done("rd", 0)
	a := envVal(t, r.e, "aDst")
	assert.Equal(t, int64(0x34), a.Array[1].Int, "SDO byte 0 is aDst[1]")
	assert.Equal(t, int64(0x12), a.Array[2].Int, "SDO byte 1 is aDst[2]")
	assert.Equal(t, int64(2), r.out("rd", "cbRead").Int)

	// The write sent 0x1234 byte-exact: read it back as a UINT.
	r.pulseOn("gou", 3)
	r.done("ru", 0)
	assert.Equal(t, int64(0x1234), envVal(t, r.e, "u").Int)
}

// TestCoESdoAdrMemberAndElement is the HI-02 regression: ADR() of a struct
// member or array element is a valid SDO buffer, as on a real PLC.
func TestCoESdoAdrMemberAndElement(t *testing.T) {
	r := newSdoBufRig(t)
	r.pulseOn("gmw", 3)
	r.done("wm", 0)
	r.pulseOn("gom", 3)
	r.done("rm", 0)
	st := envVal(t, r.e, "st").Struct
	assert.Equal(t, int64(0x0BAD), st["NVALUE"].Int)
	assert.Equal(t, int64(0), st["NHDR"].Int, "only the addressed member changes")

	r.pulseOn("gew", 3)
	r.done("we", 0)
	r.pulseOn("gox", 3)
	r.done("rx", 0)
	aw := envVal(t, r.e, "aw").Array
	assert.Equal(t, int64(0x0F00), aw[2].Int)
	assert.Equal(t, int64(0), aw[1].Int)
	assert.Equal(t, int64(0), aw[3].Int)

	// ADR(aBuf[1]) addresses the buffer from element 1 onwards.
	r.pulseOn("gow", 3)
	r.done("wr", 0)
	r.pulseOn("gob", 3)
	r.done("rb", 0)
	b := envVal(t, r.e, "aBuf").Array
	assert.Equal(t, []int64{0x34, 0x12, 0, 0}, []int64{b[1].Int, b[2].Int, b[3].Int, b[4].Int})
}

func TestEcatPtrRefErrors(t *testing.T) {
	s := &ecatServices{interp: New()}
	gone := Value{Kind: ValPointer, Ref: &RefPath{Env: NewEnv(nil), Var: "GONE", Steps: []RefStep{{Member: "M"}}}}
	_, err := s.readPtrBytes(gone, 1)
	assert.Error(t, err)
	assert.Error(t, s.writePtrBytes(gone, []byte{1}))
	// An element index outside the array falls back to the element path.
	env := NewEnv(nil)
	env.Define("ARR", Value{Kind: ValArray, Array: []Value{{Kind: ValInt, IECType: types.KindBYTE}}})
	bad := Value{Kind: ValPointer, Ref: &RefPath{Env: env, Var: "ARR", Steps: []RefStep{{Index: 5, IsIndex: true}}}}
	assert.Error(t, s.writePtrBytes(bad, []byte{1}))
}
