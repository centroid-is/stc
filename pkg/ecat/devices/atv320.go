package devices

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// Schneider Electric ATV320 EtherCAT identity (ESI VendorId / ProductCode).
const (
	ATV320Vendor  uint32 = 0x0800005A
	ATV320Product uint32 = 0x389
)

// HMIS codes (SVNCore hmis_e) the model reports.
const (
	HMISRdy   uint16 = 2
	HMISNst   uint16 = 3
	HMISRun   uint16 = 4
	HMISAcc   uint16 = 5
	HMISDec   uint16 = 6
	HMISFst   uint16 = 8
	HMISFault uint16 = 23
	HMISSto   uint16 = 30
)

// Parameter defaults (ATV320 units: 0.1 Hz, 0.1 A, 0.1 s).
const (
	defHSP = 500
	defLSP = 0
	defFRS = 500
	defNCR = 20
	defACC = 30
	defDEC = 30
)

func init() {
	ecat.DefaultRegistry.Register(ATV320Vendor, ATV320Product, func() ecat.Device { return NewATV320() })
}

type rampMode uint8

const (
	rampHold rampMode = iota
	rampAcc
	rampDec
)

// ATV320 models an Altivar 320 on EtherCAT (CANopen over EtherCAT, ST301
// profile): CiA402 on CMD/ETA, LFR to RFR ramps at ACC/DEC, an LCR current
// estimate, HMIS and LFT codes, DI/OL1R passthrough and a stimulus API.
// Entries are matched by name; missing entries read 0 and are not written.
type ATV320 struct {
	// Tunables in drive units. ACC/DEC from the PDO override these when nonzero.
	HSP, LSP, FRS, NCR, ACC, DEC uint16
	// StateDelay is the number of Steps an EtherCAT state request takes
	// (ecat.DefaultStateDelay unless changed; <= 0 applies at once).
	StateDelay int

	sm        CiA402
	slots     map[string]ecat.EntrySlot
	rfr       int32
	rem       int64 // ramp remainder, units of 0.1 Hz * ns * 10
	remDen    int64 // denominator rem was accumulated against
	mode      rampMode
	lcr, hmis uint16
	eta       uint16
	lft, di   uint16
	ol1r      uint16
	sto       bool
	fault     bool

	cmd        uint16 // last CMD (0x6040)
	lfr        int16  // last LFR (0x2037:03)
	opMode     uint8  // modes of operation (0x6060)
	params     map[uint32]uint32
	eepromLeft int
	saveCount  int

	ecState    uint16 // EtherCAT state; the slave boots in PreOp
	reqState   uint16
	reqPending bool
	reqLeft    int
}

var (
	_ ecat.CoEDevice   = (*ATV320)(nil)
	_ ecat.StateDevice = (*ATV320)(nil)
)

// NewATV320 returns a drive with default parameters.
func NewATV320() *ATV320 {
	d := &ATV320{HSP: defHSP, LSP: defLSP, FRS: defFRS, NCR: defNCR, ACC: defACC, DEC: defDEC,
		slots: map[string]ecat.EntrySlot{}, hmis: HMISNst}
	d.opMode = 2 // velocity mode
	d.StateDelay = ecat.DefaultStateDelay
	d.ecState = ecat.StatePreOp
	d.params = make(map[uint32]uint32, len(atv320ParamDefaults))
	for k, v := range atv320ParamDefaults {
		d.params[k] = v
	}
	return d
}

// Init is a no-op; entry positions arrive through SetLayout.
func (d *ATV320) Init(*ecat.Slave) {}

// SetLayout records the entries the model reads and writes, by name.
func (d *ATV320) SetLayout(in, out []ecat.EntrySlot) {
	d.slots = map[string]ecat.EntrySlot{}
	for _, list := range [][]ecat.EntrySlot{in, out} {
		for _, e := range list {
			k := strings.ToUpper(e.Name)
			if _, dup := d.slots[k]; !dup {
				d.slots[k] = e
			}
		}
	}
}

func (d *ATV320) get(buf []byte, name string) uint64 {
	if e, ok := d.slots[name]; ok {
		return e.Get(buf)
	}
	return 0
}

func (d *ATV320) set(buf []byte, name string, v uint64) {
	if e, ok := d.slots[name]; ok {
		e.Set(buf, v)
	}
}

func orDefault(v, def uint16) int64 {
	if v == 0 {
		return int64(def)
	}
	return int64(v)
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// target clamps the reference frequency to [LSP, HSP] in magnitude.
func (d *ATV320) target(lfr int16) int32 {
	t := int32(lfr)
	hsp, lsp := int32(orDefault(d.HSP, defHSP)), int32(d.LSP)
	switch {
	case t > hsp:
		return hsp
	case t < -hsp:
		return -hsp
	case t > 0 && t < lsp:
		return lsp
	case t < 0 && t > -lsp:
		return -lsp
	}
	return t
}

// ramp moves rfr toward target. Rate in 0.1 Hz/s is HSP*10/time (time in
// 0.1 s), times 10 for a fast stop. Integer math with a carried remainder
// keeps results exact and platform independent.
func (d *ATV320) ramp(dt time.Duration, target int32, acc, dec int64, fast bool) rampMode {
	cur := d.rfr
	if cur == target {
		d.rem, d.mode = 0, rampHold
		return rampHold
	}
	mode, goal, tm := rampAcc, target, acc
	if cur != 0 && (target == 0 || (target > 0) != (cur > 0) || abs32(target) < abs32(cur)) {
		mode, tm = rampDec, dec
		if (target > 0) != (cur > 0) {
			goal = 0
		}
	}
	mult := int64(10)
	if fast {
		mult = 100
	}
	den := tm * int64(time.Second)
	if mode != d.mode || den != d.remDen {
		d.rem, d.mode, d.remDen = 0, mode, den
	}
	if dt <= 0 {
		return mode
	}
	d.rem += orDefault(d.HSP, defHSP) * mult * dt.Nanoseconds()
	step := d.rem / den
	d.rem -= step * den
	dist := int64(goal) - int64(cur)
	if dist < 0 {
		dist = -dist
	}
	if step >= dist {
		d.rfr, d.rem = goal, 0
	} else if goal > cur {
		d.rfr = cur + int32(step)
	} else {
		d.rfr = cur - int32(step)
	}
	return mode
}

// Step runs one cycle: read outputs, advance CiA402 and the ramp, write inputs.
func (d *ATV320) Step(dt time.Duration, out []byte, in []byte) {
	d.stepEEPROM()
	d.stepEcState()
	if d.ecState != ecat.StateOP {
		d.stepNoExchange(in)
		return
	}
	d.cmd = uint16(d.get(out, "CMD"))
	d.lfr = int16(d.get(out, "LFR"))
	cmd, lfr := d.cmd, d.lfr
	d.ol1r = uint16(d.get(out, "OL1R"))
	acc := orDefault(uint16(d.get(out, "ACC")), d.ACC)
	if acc == 0 {
		acc = defACC
	}
	dec := orDefault(uint16(d.get(out, "DEC")), d.DEC)
	if dec == 0 {
		dec = defDEC
	}

	prev := d.sm.State()
	d.sm.Step(cmd, d.fault)
	if d.sto {
		d.sm.Disable()
	}

	tgt := int32(0)
	mode := rampHold
	switch st := d.sm.State(); st {
	case StOperationEnabled:
		if !d.sm.Halt() {
			tgt = d.target(lfr)
		}
		mode = d.ramp(dt, tgt, acc, dec, false)
	case StQuickStopActive:
		mode = d.ramp(dt, 0, acc, dec, true)
		if d.rfr == 0 && prev == StQuickStopActive {
			d.sm.QuickStopDone()
		}
	default:
		d.rfr, d.rem, d.mode = 0, 0, rampHold
	}

	d.status(tgt, mode)

	d.set(in, "ETA", uint64(d.eta))
	d.set(in, "RFR", uint64(uint16(int16(d.rfr))))
	d.set(in, "LCR", uint64(d.lcr))
	d.set(in, "DI", uint64(d.di))
	d.set(in, "LFT", uint64(d.lft))
	d.set(in, "HMIS", uint64(d.hmis))
}

// status derives LCR, HMIS and ETA from the CiA402 state and the ramp.
func (d *ATV320) status(tgt int32, mode rampMode) {
	st := d.sm.State()
	d.lcr = 0
	if st == StOperationEnabled {
		ncr, frs := int64(d.NCR), orDefault(d.FRS, defFRS)
		d.lcr = uint16(ncr*3/10 + ncr*7*int64(abs32(d.rfr))/(10*frs))
	}
	switch {
	case d.fault || st == StFault || st == StFaultReactionActive:
		d.hmis = HMISFault
	case d.sto:
		d.hmis = HMISSto
	case st == StQuickStopActive:
		d.hmis = HMISFst
	case st == StNotReady || st == StSwitchOnDisabled:
		d.hmis = HMISNst
	case st != StOperationEnabled || d.rfr == 0 && tgt == 0:
		d.hmis = HMISRdy
	case d.rfr == tgt:
		d.hmis = HMISRun
	case mode == rampAcc:
		d.hmis = HMISAcc
	default:
		d.hmis = HMISDec
	}
	d.eta = d.sm.ETA(st == StOperationEnabled && d.rfr == tgt)
}

// stepNoExchange runs a cycle below OP: CMD and LFR are ignored, the CiA402
// machine holds (STO still disables), the motor is stopped and no process
// data is exchanged, so every input entry reads 0.
func (d *ATV320) stepNoExchange(in []byte) {
	d.rfr, d.rem, d.mode = 0, 0, rampHold
	if d.sto {
		d.sm.Disable()
	}
	d.status(0, rampHold)
	for _, e := range d.slots {
		if e.Dir == ecat.DirIn {
			e.Set(in, 0)
		}
	}
}

// EcState returns the slave's EtherCAT state.
func (d *ATV320) EcState() uint16 { return d.ecState }

// RequestState starts a transition to s; it completes after StateDelay Steps.
func (d *ATV320) RequestState(s uint16) {
	if d.StateDelay <= 0 {
		d.reqPending = false
		d.setEcState(s)
		return
	}
	d.reqState, d.reqPending, d.reqLeft = s, true, d.StateDelay
}

func (d *ATV320) stepEcState() {
	if !d.reqPending {
		return
	}
	d.reqLeft--
	if d.reqLeft <= 0 {
		d.reqPending = false
		d.setEcState(d.reqState)
	}
}

// setEcState applies a state change. Leaving OP is a communication loss: an
// enabled drive drops to Switch on disabled and the motor freewheels.
func (d *ATV320) setEcState(s uint16) {
	if d.ecState == ecat.StateOP && s != ecat.StateOP {
		d.sm.Disable()
		d.rfr, d.rem, d.mode = 0, 0, rampHold
	}
	d.ecState = s
}

// InjectFault raises a drive fault with code lft (lft_e). The motor
// freewheels at once; ETA shows Fault reaction active on the next Step, then
// Fault. Fault reset is refused until ClearFault.
func (d *ATV320) InjectFault(lft uint16) {
	d.lft, d.fault = lft, true
	d.rfr, d.rem, d.mode = 0, 0, rampHold
	d.hmis = HMISFault
}

// ClearFault removes the fault condition so a fault reset can succeed. LFT
// keeps the last fault code.
func (d *ATV320) ClearFault() { d.fault = false }

// SetDI sets the logic input word reported in DI.
func (d *ATV320) SetDI(bits uint16) { d.di = bits }

// SetSTO switches Safe Torque Off: the drive drops to Switch on disabled and
// refuses to leave it until STO is released.
func (d *ATV320) SetSTO(on bool) { d.sto = on }

// RFR returns the output frequency in 0.1 Hz.
func (d *ATV320) RFR() int16 { return int16(d.rfr) }

// ETA returns the last status word written.
func (d *ATV320) ETA() uint16 { return d.eta }

// HMIS returns the drive state code (hmis_e).
func (d *ATV320) HMIS() uint16 { return d.hmis }

// LFT returns the last fault code (lft_e).
func (d *ATV320) LFT() uint16 { return d.lft }

// LCR returns the motor current estimate in 0.1 A.
func (d *ATV320) LCR() uint16 { return d.lcr }

// OL1R returns the last logic/relay output word written by the master.
func (d *ATV320) OL1R() uint16 { return d.ol1r }

// State returns the CiA402 state.
func (d *ATV320) State() CiA402State { return d.sm.State() }
