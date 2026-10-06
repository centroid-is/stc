package interp

import (
	"math"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
	_ "github.com/centroid-is/stc/pkg/ecat/devices" // registers the ATV320 model
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ecDev1 = "Device 1 (EtherCAT)" // ten slaves, ATV320 at 1007
	ecDev3 = "Device 3 (EtherCAT)" // one ATV320 at 1001
	ecNet1 = "5.1.2.3.1.1"
	ecNet3 = "5.1.2.4.1.1"
)

// ecatNet loads both ATV320 fixtures with distinct AmsNetIds.
func ecatNet(t *testing.T) *ecat.Network {
	t.Helper()
	topo, err := ecat.LoadProject(ecatFixture("atv320_device.xml"), ecatFixture("Demo Device 1.xml"))
	require.NoError(t, err)
	n := ecat.NewNetwork(topo, nil)
	require.NoError(t, n.SetMasterNetID(ecDev1, [6]byte{5, 1, 2, 3, 1, 1}))
	require.NoError(t, n.SetMasterNetID(ecDev3, [6]byte{5, 1, 2, 4, 1, 1}))
	return n
}

// ecatFBSrc calls every state, master and CRC mock from one trigger.
const ecatFBSrc = `
PROGRAM P
VAR_INPUT
	go   : BOOL;
	goss : BOOL; (* FB_EcSetSlaveState has its own trigger *)
	net  : STRING;
	addr : UINT;
	req  : WORD;
	tmo  : TIME;
	ado  : UINT;
	plen : UDINT;
	cb   : UDINT;
END_VAR
VAR
	gs  : FB_EcGetSlaveState;
	gas : FB_EcGetAllSlaveStates;
	gab : FB_EcGetAllSlaveStates;
	ss  : FB_EcSetSlaveState;
	gm  : FB_EcGetMasterState;
	gac : FB_EcGetAllSlaveCrcErrors;
	gcb : FB_EcGetAllSlaveCrcErrors;
	gc  : FB_EcGetSlaveCrcError;
	gcx : FB_EcGetSlaveCrcErrorEx;
	pw  : FB_EcPhysicalWriteCmd;
	states : ARRAY[0..15] OF ST_EcSlaveState;
	crcs   : ARRAY[0..15] OF UDINT;
	clr    : ARRAY[0..11] OF USINT;
	busy, err : BOOL;
	id : UDINT;
	ds, ls : BYTE;
	nsl : UINT;
	ms : WORD;
	pd : UDINT;
	wkc : UINT;
END_VAR
gs(sNetId := net, nSlaveAddr := addr, bExecute := go);
busy := gs.bBusy; err := gs.bError; id := gs.nErrId;
ds := gs.state.deviceState; ls := gs.state.linkState;
gas(sNetId := net, pStateBuf := ADR(states), cbBufLen := cb, bExecute := go);
nsl := gas.nSlaves;
gab(sNetId := net, cbBufLen := cb, bExecute := go);
ss(sNetId := net, nSlaveAddr := addr, reqState := req, tTimeout := tmo, bExecute := goss);
gm(sNetId := net, bExecute := go);
ms := gm.state;
gac(sNetId := net, pCrcErrorBuf := ADR(crcs), cbBufLen := cb, bExecute := go);
gcb(sNetId := net, cbBufLen := cb, bExecute := go);
gc(sNetId := net, nSlaveAddr := addr, bExecute := go);
gcx(sNetId := net, nSlaveAddr := addr, bExecute := go);
pd := gcx.CrcError.portD;
pw(sNetId := net, adp := addr, ado := ado, len := plen, pSrcBuf := ADR(clr), bExecute := go);
wkc := pw.wkc;
END_PROGRAM
`

type ecRig struct {
	t   *testing.T
	e   *ScanCycleEngine
	net *ecat.Network
}

func newEcRig(t *testing.T, src string) *ecRig {
	net := ecatNet(t)
	e := ecatEngine(t, src, net)
	e.SetInput("net", StringValue(ecNet1))
	e.SetInput("tmo", TimeValue(5*time.Second))
	e.SetInput("cb", ecUdint(64))
	tick(t, e, 1)
	return &ecRig{t: t, e: e, net: net}
}

func (r *ecRig) set(name string, v Value) { r.e.SetInput(name, v) }

// pulse drops go for one scan, then raises it for n scans.
func (r *ecRig) pulse(n int) { r.pulseOn("go", n) }

func (r *ecRig) pulseOn(trigger string, n int) {
	r.set(trigger, BoolValue(false))
	tick(r.t, r.e, 1)
	r.set(trigger, BoolValue(true))
	tick(r.t, r.e, n)
}

func (r *ecRig) fb(name string) StandardFB { return envVal(r.t, r.e, name).FBRef.FB }

func (r *ecRig) out(fb, name string) Value { return r.fb(fb).GetOutput(name) }

// done asserts fb finished with error code want.
func (r *ecRig) done(fb string, want uint32) {
	r.t.Helper()
	assert.False(r.t, r.out(fb, "bBusy").Bool, fb)
	assert.Equal(r.t, want != 0, r.out(fb, "bError").Bool, fb)
	assert.Equal(r.t, int64(want), r.out(fb, "nErrId").Int, fb)
}

func TestEcGetSlaveStateLatency(t *testing.T) {
	r := newEcRig(t, ecatFBSrc)
	r.set("addr", ecUint(1002))
	require.NoError(t, r.net.SetLinkState(ecDev1, 1, ecat.LinkNotPresent))
	r.pulse(1)
	assert.True(t, envVal(t, r.e, "busy").Bool, "scan 1")
	tick(t, r.e, 1)
	assert.True(t, envVal(t, r.e, "busy").Bool, "scan 2")
	tick(t, r.e, 1)
	assert.False(t, envVal(t, r.e, "busy").Bool, "scan 3")
	assert.False(t, envVal(t, r.e, "err").Bool)
	assert.Equal(t, int64(ecat.StateOP), envVal(t, r.e, "ds").Int)
	assert.Equal(t, int64(ecat.LinkNotPresent), envVal(t, r.e, "ls").Int)

	// Outputs hold while bExecute stays TRUE and after it falls.
	tick(t, r.e, 2)
	assert.Equal(t, int64(ecat.StateOP), envVal(t, r.e, "ds").Int)

	// The ATV320 boots in PreOp.
	r.set("addr", ecUint(1007))
	r.pulse(3)
	assert.Equal(t, int64(ecat.StatePreOp), envVal(t, r.e, "ds").Int)

	// Unknown slave address: ADS 0x6; unknown master with two masters: 0x7.
	r.set("addr", ecUint(999))
	r.pulse(3)
	r.done("gs", adsErrPortNotFound)
	assert.Equal(t, int64(adsErrPortNotFound), envVal(t, r.e, "id").Int)
	r.set("net", StringValue(""))
	r.pulse(3)
	r.done("gs", adsErrMachineNotFound)
	// GetInput echoes inputs.
	assert.Equal(t, int64(999), r.fb("gs").GetInput("nSlaveAddr").Int)
	assert.Equal(t, Value{}, r.out("gs", "nosuch"))
}

func TestEcGetAllSlaveStates(t *testing.T) {
	r := newEcRig(t, ecatFBSrc)
	r.set("cb", ecUdint(64))
	r.pulse(3)
	r.done("gas", 0)
	assert.Equal(t, int64(10), envVal(t, r.e, "nsl").Int)
	states := envVal(t, r.e, "states").Array
	for i := 0; i < 10; i++ {
		want := int64(ecat.StateOP)
		if i == 6 {
			want = int64(ecat.StatePreOp)
		}
		assert.Equal(t, want, states[i].Struct["DEVICESTATE"].Int, i)
	}
	assert.Zero(t, states[10].Struct["DEVICESTATE"].Int)
	// No buffer: bad pointer is an error, not a panic.
	r.done("gab", adsErrInvalidParm)

	// A short buffer only receives the first entries.
	r2 := newEcRig(t, ecatFBSrc)
	r2.set("cb", ecUdint(4))
	r2.pulse(3)
	st2 := envVal(t, r2.e, "states").Array
	assert.Equal(t, int64(ecat.StateOP), st2[1].Struct["DEVICESTATE"].Int)
	assert.Zero(t, st2[2].Struct["DEVICESTATE"].Int)
	assert.Equal(t, int64(10), envVal(t, r2.e, "nsl").Int)
}

func TestEcSetSlaveState(t *testing.T) {
	r := newEcRig(t, ecatFBSrc)
	r.set("addr", ecUint(1007))
	r.set("req", ecWord(uint64(ecat.StateOP)))
	r.pulseOn("goss", 1)
	assert.True(t, r.out("ss", "bBusy").Bool)
	for i := 0; i < 10 && r.out("ss", "bBusy").Bool; i++ {
		tick(t, r.e, 1)
	}
	r.done("ss", 0)
	assert.Equal(t, int64(ecat.StateOP), r.out("ss", "currState").Struct["DEVICESTATE"].Int)
	st, _ := r.net.SlaveState(ecDev1, 6)
	assert.Equal(t, ecat.StateOP, st)

	// A plain device that is slower than tTimeout times out with 0x745.
	r.net.StateDelay = 100
	r.set("addr", ecUint(1002))
	r.set("req", ecWord(uint64(ecat.StatePreOp)))
	r.set("tmo", TimeValue(50*time.Millisecond))
	r.pulseOn("goss", 8)
	r.done("ss", adsErrTimeout)

	// tTimeout T#0S means 5 s of simulated time.
	r.net.StateDelay = 10000
	r.set("addr", ecUint(1003))
	r.set("tmo", TimeValue(0))
	r.pulseOn("goss", 1)
	n := 0
	for ; n < 1000 && r.out("ss", "bBusy").Bool; n++ {
		tick(t, r.e, 1)
	}
	r.done("ss", adsErrTimeout)
	assert.Equal(t, 501, n) // elapsed > 5 s after 501 scans of 10 ms

	// Unknown slave.
	r.set("addr", ecUint(4242))
	r.pulseOn("goss", 3)
	r.done("ss", adsErrPortNotFound)
	assert.Equal(t, 10*time.Second, newEcSetSlaveState(nil).GetInput("tTimeout").Time)
}

func TestEcGetMasterState(t *testing.T) {
	r := newEcRig(t, ecatFBSrc)
	r.pulse(3)
	r.done("gm", 0)
	assert.Equal(t, int64(0x0008), envVal(t, r.e, "ms").Int)
	require.NoError(t, r.net.SetMasterState(ecDev1, 0x0012))
	r.pulse(3)
	assert.Equal(t, int64(0x0012), envVal(t, r.e, "ms").Int)
	// The other master by its AmsNetId.
	r.set("net", StringValue(ecNet3))
	r.pulse(3)
	assert.Equal(t, int64(0x0008), envVal(t, r.e, "ms").Int)
	r.set("net", StringValue("1.1.1.1.1.1"))
	r.pulse(3)
	r.done("gm", adsErrMachineNotFound)
}

func TestEcCrcMocks(t *testing.T) {
	r := newEcRig(t, ecatFBSrc)
	require.NoError(t, r.net.SetCrcErrors(ecDev1, 2, [4]uint32{1, 2, 3, 4}))
	require.NoError(t, r.net.SetCrcErrors(ecDev1, 3, [4]uint32{math.MaxUint32, 1, 0, 0}))
	r.set("addr", ecUint(1003))
	r.pulse(3)
	r.done("gac", 0)
	crcs := envVal(t, r.e, "crcs").Array
	assert.Equal(t, int64(10), crcs[2].Int)
	assert.Equal(t, int64(math.MaxUint32), crcs[3].Int)
	assert.Zero(t, crcs[0].Int)
	r.done("gcb", adsErrInvalidParm)

	r.done("gc", 0)
	ce := r.out("gc", "crcError").Struct
	assert.Equal(t, []int64{1, 2, 3}, []int64{ce["PORTA"].Int, ce["PORTB"].Int, ce["PORTC"].Int})
	_, hasD := ce["PORTD"]
	assert.False(t, hasD)
	r.done("gcx", 0)
	assert.Equal(t, int64(4), envVal(t, r.e, "pd").Int)
	assert.Zero(t, newEcGetSlaveCrcError(nil, true).GetInput("tTimeout").Time)

	// Physical write outside the CRC registers: no effect, wkc 1.
	r.set("ado", ecUint(0x0100))
	r.set("plen", ecUdint(4))
	r.pulse(3)
	r.done("pw", 0)
	assert.Equal(t, int64(1), envVal(t, r.e, "wkc").Int)
	assert.Equal(t, [4]uint32{1, 2, 3, 4}, r.net.CrcErrors(ecDev1, 2))

	// Write-to-clear of 0x0300..0x030B.
	r.set("ado", ecUint(0x0300))
	r.set("plen", ecUdint(12))
	r.pulse(3)
	r.done("pw", 0)
	assert.Equal(t, int64(1), envVal(t, r.e, "wkc").Int)
	assert.Equal(t, [4]uint32{}, r.net.CrcErrors(ecDev1, 2))
	require.NoError(t, r.net.SetCrcErrors(ecDev1, 2, [4]uint32{5, 0, 0, 0}))
	r.set("plen", ecUdint(0))
	r.pulse(3)
	assert.Equal(t, [4]uint32{5, 0, 0, 0}, r.net.CrcErrors(ecDev1, 2))
	assert.Equal(t, [4]uint32{math.MaxUint32, 1, 0, 0}, r.net.CrcErrors(ecDev1, 3))

	// Unknown adp: nobody answers, no error.
	r.set("addr", ecUint(999))
	r.set("plen", ecUdint(12))
	r.pulse(3)
	r.done("pw", 0)
	assert.Zero(t, envVal(t, r.e, "wkc").Int)
	r.done("gc", adsErrPortNotFound)
}

// sdoSrc follows FB_Parameter: read a parameter into a UINT, write one back.
const sdoSrc = `
PROGRAM P
VAR_INPUT
	gor, gow, gos, gon : BOOL;
	net  : STRING;
	addr : UINT;
	idx  : WORD;
	sub  : BYTE;
	cb   : UDINT;
	tmo  : TIME;
	wval : UINT;
END_VAR
VAR
	rd   : FB_EcCoESDoRead;
	wr   : FB_EcCoESDoWrite;
	sv   : FB_EcCoESdoWrite;
	rdn  : FB_EcCoESdoRead;
	wrn  : FB_EcCoESdoWrite;
	cur  : UINT := 16#BEEF;
	par  : UINT;
	save : UDINT := 16#65766173;
	rbusy, rerr : BOOL;
	rid, nread : UDINT;
END_VAR
par := wval;
rd(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pDstBuf := ADR(cur), cbBufLen := cb, bExecute := gor);
rbusy := rd.bBusy; rerr := rd.bError; rid := rd.nErrId; nread := rd.cbRead;
wr(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub,
	pSrcBuf := ADR(par), cbBufLen := cb, bExecute := gow);
sv(sNetId := net, nSlaveAddr := addr, nIndex := 16#2032, nSubIndex := 1,
	pSrcBuf := ADR(save), cbBufLen := 4, bExecute := gos);
rdn(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub, cbBufLen := cb, bExecute := gon);
wrn(sNetId := net, nSlaveAddr := addr, nIndex := idx, nSubIndex := sub, cbBufLen := cb, bExecute := gon);
END_PROGRAM
`

func newSdoRig(t *testing.T) *ecRig {
	r := newEcRig(t, sdoSrc)
	r.set("net", StringValue(ecNet3))
	r.set("addr", ecUint(1001))
	r.set("idx", ecWord(0x2001))
	r.set("sub", ecByte(5))
	r.set("cb", ecUdint(2))
	return r
}

func TestCoESDoWriteReadBack(t *testing.T) {
	r := newSdoRig(t)
	r.set("wval", ecUint(600))
	r.pulseOn("gow", 1)
	assert.True(t, r.out("wr", "bBusy").Bool, "scan 1")
	tick(t, r.e, 1)
	assert.True(t, r.out("wr", "bBusy").Bool, "scan 2")
	tick(t, r.e, 1)
	r.done("wr", 0)

	r.pulseOn("gor", 1)
	assert.True(t, envVal(t, r.e, "rbusy").Bool)
	assert.Equal(t, int64(0xBEEF), envVal(t, r.e, "cur").Int)
	tick(t, r.e, 2)
	assert.False(t, envVal(t, r.e, "rbusy").Bool)
	assert.False(t, envVal(t, r.e, "rerr").Bool)
	assert.Equal(t, int64(600), envVal(t, r.e, "cur").Int)
	assert.Equal(t, int64(2), envVal(t, r.e, "nread").Int)

	// cbBufLen larger than the target only fills the target.
	r.set("cb", ecUdint(100))
	r.set("wval", ecUint(450))
	r.pulseOn("gow", 3)
	r.done("wr", 0)
	r.pulseOn("gor", 3)
	assert.Equal(t, int64(450), envVal(t, r.e, "cur").Int)

	// EEPROM save object 0x2032:01 takes a UDINT.
	r.pulseOn("gos", 3)
	r.done("sv", 0)
}

func TestCoESDoErrors(t *testing.T) {
	r := newSdoRig(t)
	// Unknown object: abort 0x06020000, buffer untouched.
	r.set("idx", ecWord(0x5FFF))
	r.set("sub", ecByte(0))
	r.pulseOn("gor", 3)
	assert.True(t, envVal(t, r.e, "rerr").Bool)
	assert.Equal(t, int64(ecat.AbortNoObject), envVal(t, r.e, "rid").Int)
	assert.Equal(t, int64(0xBEEF), envVal(t, r.e, "cur").Int)
	r.pulseOn("gow", 3)
	r.done("wr", ecat.AbortNoObject)

	// Missing buffers are ADS errors, not panics.
	r.set("idx", ecWord(0x2001))
	r.set("sub", ecByte(5))
	r.pulseOn("gon", 3)
	r.done("rdn", adsErrInvalidParm)
	r.done("wrn", adsErrInvalidParm)

	// A slave without an object dictionary aborts with 0x06020000.
	r.set("net", StringValue(ecNet1))
	r.set("addr", ecUint(1002))
	r.pulseOn("gor", 3)
	r.done("rd", ecat.AbortNoObject)
	r.pulseOn("gow", 3)
	r.done("wr", ecat.AbortNoObject)

	// Unknown slave address.
	r.set("addr", ecUint(77))
	r.pulseOn("gor", 3)
	r.done("rd", adsErrPortNotFound)
	assert.Equal(t, Value{}, r.out("wr", "cbRead"))
}
