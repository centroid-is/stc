package interp

import (
	"math"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/types"
)

// Tc2_EtherCAT mocks: Go StandardFBs backed by the engine's ecat.Network
// (see SetNetwork). Input and output names follow tc2_ethercat.st.

func init() {
	ecatMockCtors["FB_ECGETSLAVESTATE"] = func(s *ecatServices) StandardFB { return newEcGetSlaveState(s) }
	ecatMockCtors["FB_ECGETALLSLAVESTATES"] = func(s *ecatServices) StandardFB { return newEcGetAllSlaveStates(s) }
	ecatMockCtors["FB_ECSETSLAVESTATE"] = func(s *ecatServices) StandardFB { return newEcSetSlaveState(s) }
	ecatMockCtors["FB_ECGETMASTERSTATE"] = func(s *ecatServices) StandardFB { return newEcGetMasterState(s) }
	ecatMockCtors["FB_ECGETALLSLAVECRCERRORS"] = func(s *ecatServices) StandardFB { return newEcGetAllSlaveCrcErrors(s) }
	ecatMockCtors["FB_ECGETSLAVECRCERROR"] = func(s *ecatServices) StandardFB { return newEcGetSlaveCrcError(s, false) }
	ecatMockCtors["FB_ECGETSLAVECRCERROREX"] = func(s *ecatServices) StandardFB { return newEcGetSlaveCrcError(s, true) }
	ecatMockCtors["FB_ECPHYSICALWRITECMD"] = func(s *ecatServices) StandardFB { return newEcPhysicalWriteCmd(s) }
}

func ecByte(n uint64) Value {
	return Value{Kind: ValInt, Int: int64(uint8(n)), IECType: types.KindBYTE}
}
func ecUint(n uint64) Value {
	return Value{Kind: ValInt, Int: int64(uint16(n)), IECType: types.KindUINT}
}
func ecWord(n uint64) Value {
	return Value{Kind: ValInt, Int: int64(uint16(n)), IECType: types.KindWORD}
}
func ecUdint(n uint64) Value {
	return Value{Kind: ValInt, Int: int64(uint32(n)), IECType: types.KindUDINT}
}

func ecSlaveState(state uint16, link uint8) Value {
	return Value{Kind: ValStruct, Struct: map[string]Value{
		"DEVICESTATE": ecByte(uint64(state)),
		"LINKSTATE":   ecByte(uint64(link)),
	}}
}

func ecCrcStruct(c [4]uint32, ports int) Value {
	m := map[string]Value{}
	for i := 0; i < ports; i++ {
		m["PORT"+string(rune('A'+i))] = ecUdint(uint64(c[i]))
	}
	return Value{Kind: ValStruct, Struct: m}
}

// ecatFB holds the inputs, the request handshake and the FB-specific outputs.
type ecatFB struct {
	s   *ecatServices
	req asyncReq
	in  map[string]Value
	out map[string]Value
}

// newEcatFB initialises the common inputs plus extra, and the outputs.
func newEcatFB(s *ecatServices, timeout time.Duration, extra, out map[string]Value) ecatFB {
	in := map[string]Value{
		"SNETID":   StringValue(""),
		"BEXECUTE": BoolValue(false),
		"TTIMEOUT": TimeValue(timeout),
	}
	for k, v := range extra {
		in[k] = v
	}
	return ecatFB{s: s, in: in, out: out}
}

func (f *ecatFB) SetInput(name string, v Value) { f.in[strings.ToUpper(name)] = v }
func (f *ecatFB) GetInput(name string) Value    { return f.in[strings.ToUpper(name)] }

func (f *ecatFB) GetOutput(name string) Value {
	switch n := strings.ToUpper(name); n {
	case "BBUSY":
		return BoolValue(f.req.busy)
	case "BERROR":
		return BoolValue(f.req.err)
	case "NERRID":
		return ecUdint(uint64(f.req.errID))
	default:
		return f.out[n]
	}
}

func (f *ecatFB) netID() string { return f.in["SNETID"].Str }
func (f *ecatFB) num(name string) uint64 {
	return uint64(f.in[name].Int)
}

// run drives the request with bExecute and no timeout.
func (f *ecatFB) run(dt time.Duration, begin func() uint32, complete func() (uint32, bool)) {
	f.req.run(f.s, f.in["BEXECUTE"].Bool, dt, 0, begin, complete)
}

// slaveFB is an ecatFB addressed to one slave by nSlaveAddr.
type slaveFB struct {
	ecatFB
	master string
	idx    int
	dev    ecat.Device
}

func (f *slaveFB) begin() uint32 {
	var code uint32
	f.master, f.idx, f.dev, code = f.s.resolveSlave(f.netID(), uint16(f.num("NSLAVEADDR")))
	return code
}

// masterFB is an ecatFB addressed to a master only.
type masterFB struct {
	ecatFB
	master string
}

func (f *masterFB) begin() uint32 {
	var code uint32
	f.master, code = f.s.resolveMaster(f.netID())
	return code
}

// ----- FB_EcGetSlaveState -----

type ecGetSlaveState struct{ slaveFB }

func newEcGetSlaveState(s *ecatServices) *ecGetSlaveState {
	return &ecGetSlaveState{slaveFB{ecatFB: newEcatFB(s, defaultEcTimeout,
		map[string]Value{"NSLAVEADDR": ecUint(0)},
		map[string]Value{"STATE": ecSlaveState(0, 0)})}}
}

func (f *ecGetSlaveState) Execute(dt time.Duration) {
	f.run(dt, f.begin, func() (uint32, bool) {
		st, link := f.s.net.SlaveState(f.master, f.idx)
		f.out["STATE"] = ecSlaveState(st, link)
		return 0, true
	})
}

// ----- FB_EcGetAllSlaveStates -----

type ecGetAllSlaveStates struct{ masterFB }

func newEcGetAllSlaveStates(s *ecatServices) *ecGetAllSlaveStates {
	return &ecGetAllSlaveStates{masterFB{ecatFB: newEcatFB(s, defaultEcTimeout,
		map[string]Value{"PSTATEBUF": {Kind: ValPointer}, "CBBUFLEN": ecUdint(0)},
		map[string]Value{"NSLAVES": ecUint(0)})}}
}

func (f *ecGetAllSlaveStates) Execute(dt time.Duration) {
	f.run(dt, f.begin, func() (uint32, bool) {
		n := f.s.net.SlaveCount(f.master)
		k := min(int(f.num("CBBUFLEN")/2), n)
		data := make([]byte, 0, 2*k)
		for i := 0; i < k; i++ {
			st, link := f.s.net.SlaveState(f.master, i)
			data = append(data, byte(st), link)
		}
		if err := f.s.writePtrBytes(f.in["PSTATEBUF"], data); err != nil {
			return adsErrInvalidParm, true
		}
		f.out["NSLAVES"] = ecUint(uint64(n))
		return 0, true
	})
}

// ----- FB_EcSetSlaveState -----

type ecSetSlaveState struct{ slaveFB }

func newEcSetSlaveState(s *ecatServices) *ecSetSlaveState {
	return &ecSetSlaveState{slaveFB{ecatFB: newEcatFB(s, 10*time.Second,
		map[string]Value{"NSLAVEADDR": ecUint(0), "REQSTATE": ecWord(0)},
		map[string]Value{"CURRSTATE": ecSlaveState(0, 0)})}}
}

func (f *ecSetSlaveState) Execute(dt time.Duration) {
	timeout := f.in["TTIMEOUT"].Time
	if timeout <= 0 {
		timeout = defaultEcTimeout
	}
	req := uint16(f.num("REQSTATE"))
	begin := func() uint32 {
		if code := f.begin(); code != 0 {
			return code
		}
		// Cannot fail: the slave was just resolved on this master.
		_ = f.s.net.RequestSlaveState(f.master, f.idx, req)
		return 0
	}
	f.req.run(f.s, f.in["BEXECUTE"].Bool, dt, timeout, begin, func() (uint32, bool) {
		st, link := f.s.net.SlaveState(f.master, f.idx)
		f.out["CURRSTATE"] = ecSlaveState(st, link)
		return 0, st == req
	})
}

// ----- FB_EcGetMasterState -----

type ecGetMasterState struct{ masterFB }

func newEcGetMasterState(s *ecatServices) *ecGetMasterState {
	return &ecGetMasterState{masterFB{ecatFB: newEcatFB(s, defaultEcTimeout, nil,
		map[string]Value{"STATE": ecWord(0)})}}
}

func (f *ecGetMasterState) Execute(dt time.Duration) {
	f.run(dt, f.begin, func() (uint32, bool) {
		f.out["STATE"] = ecWord(uint64(f.s.net.MasterState(f.master)))
		return 0, true
	})
}

// ----- FB_EcGetAllSlaveCrcErrors -----

type ecGetAllSlaveCrcErrors struct{ masterFB }

func newEcGetAllSlaveCrcErrors(s *ecatServices) *ecGetAllSlaveCrcErrors {
	return &ecGetAllSlaveCrcErrors{masterFB{ecatFB: newEcatFB(s, defaultEcTimeout,
		map[string]Value{"PCRCERRORBUF": {Kind: ValPointer}, "CBBUFLEN": ecUdint(0)},
		map[string]Value{"NSLAVES": ecUint(0)})}}
}

// crcSum adds the four port counters, saturating at the UDINT maximum.
func crcSum(c [4]uint32) uint32 {
	var sum uint64
	for _, v := range c {
		sum += uint64(v)
	}
	if sum > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(sum)
}

func (f *ecGetAllSlaveCrcErrors) Execute(dt time.Duration) {
	f.run(dt, f.begin, func() (uint32, bool) {
		n := f.s.net.SlaveCount(f.master)
		k := min(int(f.num("CBBUFLEN")/4), n)
		data := make([]byte, 0, 4*k)
		for i := 0; i < k; i++ {
			v := crcSum(f.s.net.CrcErrors(f.master, i))
			data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
		}
		if err := f.s.writePtrBytes(f.in["PCRCERRORBUF"], data); err != nil {
			return adsErrInvalidParm, true
		}
		f.out["NSLAVES"] = ecUint(uint64(n))
		return 0, true
	})
}

// ----- FB_EcGetSlaveCrcError / FB_EcGetSlaveCrcErrorEx -----

type ecGetSlaveCrcError struct {
	slaveFB
	ports int
}

// newEcGetSlaveCrcError builds FB_EcGetSlaveCrcError (ports A..C, output
// crcError) or, with ex, FB_EcGetSlaveCrcErrorEx (ports A..D, CrcError).
// Both output names are CRCERROR once upper-cased.
func newEcGetSlaveCrcError(s *ecatServices, ex bool) *ecGetSlaveCrcError {
	ports, timeout := 3, defaultEcTimeout
	if ex {
		ports, timeout = 4, 0
	}
	return &ecGetSlaveCrcError{slaveFB: slaveFB{ecatFB: newEcatFB(s, timeout,
		map[string]Value{"NSLAVEADDR": ecUint(0)},
		map[string]Value{"CRCERROR": ecCrcStruct([4]uint32{}, ports)})}, ports: ports}
}

func (f *ecGetSlaveCrcError) Execute(dt time.Duration) {
	f.run(dt, f.begin, func() (uint32, bool) {
		f.out["CRCERROR"] = ecCrcStruct(f.s.net.CrcErrors(f.master, f.idx), f.ports)
		return 0, true
	})
}

// ----- FB_EcPhysicalWriteCmd -----

// CRC error counter registers of an ESC (write to clear).
const (
	escCrcFirst = 0x0300
	escCrcEnd   = 0x030C
)

type ecPhysicalWriteCmd struct{ masterFB }

func newEcPhysicalWriteCmd(s *ecatServices) *ecPhysicalWriteCmd {
	return &ecPhysicalWriteCmd{masterFB{ecatFB: newEcatFB(s, defaultEcTimeout,
		map[string]Value{
			"ADP": ecUint(0), "ADO": ecUint(0), "LEN": ecUdint(0),
			"ETYPE": IntValue(2), "PSRCBUF": {Kind: ValPointer},
		},
		map[string]Value{"WKC": ecUint(0)})}}
}

// Execute clears the CRC counters of slave adp when the write covers
// 0x0300..0x030B. An unknown adp is not an error: nobody answers, wkc 0.
func (f *ecPhysicalWriteCmd) Execute(dt time.Duration) {
	f.run(dt, f.begin, func() (uint32, bool) {
		i, _, err := f.s.net.Slave(f.master, uint16(f.num("ADP")))
		if err != nil {
			f.out["WKC"] = ecUint(0)
			return 0, true
		}
		ado, n := f.num("ADO"), f.num("LEN")
		if n > 0 && ado < escCrcEnd && ado+n > escCrcFirst {
			_ = f.s.net.ClearCrc(f.master, i)
		}
		f.out["WKC"] = ecUint(1)
		return 0, true
	})
}
