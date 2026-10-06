package devices

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

const tick = 100 * time.Millisecond

func fixture(name string) string {
	return filepath.Join("..", "..", "..", "tests", "ecat_fixtures", name)
}

type rig struct {
	t    *testing.T
	topo *ecat.Topology
	n    *ecat.Network
	d    *ATV320
	base string
}

// newRig loads file, captures the ATV320 instance from a fresh registry and
// addresses entries through the topology's link paths.
func newRig(t *testing.T, file string) *rig {
	t.Helper()
	topo, err := ecat.LoadProject(fixture(file))
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, topo: topo}
	reg := ecat.NewRegistry()
	reg.Register(ATV320Vendor, ATV320Product, func() ecat.Device { r.d = NewATV320(); return r.d })
	r.n = ecat.NewNetwork(topo, reg)
	m := topo.Masters[0]
	for _, s := range m.Slaves {
		if s.Vendor == ATV320Vendor {
			r.base = ecat.SlaveBasePath(m.Name, s)
		}
	}
	if r.d == nil || r.base == "" {
		t.Fatal("no ATV320 slave")
	}
	return r
}

func (r *rig) slot(path string) (ecat.Slot, []byte) {
	r.t.Helper()
	s, ok := r.topo.Slot(path)
	if !ok {
		r.t.Fatalf("no slot %s", path)
	}
	img := r.n.Images().Get(s.Master)
	if s.Dir == ecat.DirOut {
		return s, img.Out
	}
	return s, img.In
}

func (r *rig) put(name string, v int) {
	r.t.Helper()
	s, buf := r.slot(r.base + "^Outputs^" + name)
	ecat.WriteBits(buf, s.Byte, s.Bit, s.BitLen, uint64(uint16(int16(v))))
}

func (r *rig) in(name string) uint16 {
	r.t.Helper()
	s, buf := r.slot(r.base + "^Inputs^" + name)
	return uint16(ecat.ReadBits(buf, s.Byte, s.Bit, s.BitLen))
}

func (r *rig) rfr() int16 { return int16(r.in("RFR")) }

func (r *rig) step(cmd int) {
	r.put("CMD", cmd)
	r.n.Step(tick)
}

// enable walks the drive to Operation enabled; the 0x0F step already ramps.
func (r *rig) enable() {
	r.t.Helper()
	r.step(0x00)
	r.step(0x06)
	r.step(0x07)
	r.step(0x0F)
	if r.d.State() != StOperationEnabled {
		r.t.Fatalf("state %v", r.d.State())
	}
}

func (r *rig) expect(eta, hmis uint16, rfr int16) {
	r.t.Helper()
	if got := r.in("ETA"); got != eta {
		r.t.Errorf("ETA = %#x, want %#x", got, eta)
	}
	if got := r.in("HMIS"); got != hmis {
		r.t.Errorf("HMIS = %d, want %d", got, hmis)
	}
	if got := r.rfr(); got != rfr {
		r.t.Errorf("RFR = %d, want %d", got, rfr)
	}
}

func TestATV320Registered(t *testing.T) {
	topo, err := ecat.LoadProject(fixture("atv320_device.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ecat.DefaultRegistry.New(topo.Masters[0].Slaves[0]).(*ATV320); !ok {
		t.Fatal("DefaultRegistry does not build an ATV320")
	}
	n := ecat.NewNetwork(topo, nil)
	n.Step(tick)
	s, _ := topo.Slot(ecat.SlaveBasePath(topo.Masters[0].Name, topo.Masters[0].Slaves[0]) + "^Inputs^ETA")
	if got := ecat.ReadBits(n.Images().Get(s.Master).In, s.Byte, s.Bit, s.BitLen); got != 0x0250 {
		t.Errorf("ETA after power-up = %#x", got)
	}
}

func TestATV320StartupAndRampUp(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.put("ACC", 30)
	r.put("DEC", 30)
	r.step(0x00)
	r.expect(0x0250, HMISNst, 0)
	r.step(0x06)
	r.expect(0x0231, HMISRdy, 0)
	r.step(0x07)
	r.expect(0x0233, HMISRdy, 0)
	if r.in("LCR") != 0 {
		t.Error("LCR nonzero before OE")
	}
	for k := 1; k <= 30; k++ {
		r.step(0x0F)
		want := int16(k * 500 / 30)
		if r.rfr() != want {
			t.Fatalf("step %d: RFR %d, want %d", k, r.rfr(), want)
		}
		if k < 30 {
			r.expect(0x0237, HMISAcc, want)
		}
	}
	r.expect(0x0637, HMISRun, 500)
	if r.in("LCR") != 20 || r.d.LCR() != 20 {
		t.Errorf("LCR at FRS = %d", r.in("LCR"))
	}
	if r.d.RFR() != 500 || r.d.ETA() != 0x0637 || r.d.HMIS() != HMISRun {
		t.Error("getters disagree with image")
	}
}

func TestATV320RampDownAndStop(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.put("ACC", 10) // 1.0 s to HSP: 50 per tick
	r.put("DEC", 10)
	r.enable()
	for i := 0; i < 9; i++ {
		r.step(0x0F)
	}
	r.expect(0x0637, HMISRun, 500)
	r.put("LFR", 200)
	for i := 1; i <= 6; i++ {
		r.step(0x0F)
		if i < 6 {
			r.expect(0x0237, HMISDec, int16(500-50*i))
		}
	}
	r.expect(0x0637, HMISRun, 200)
	if r.in("LCR") != 6+14*200/500 {
		t.Errorf("LCR at 20 Hz = %d", r.in("LCR"))
	}
	r.put("LFR", 0)
	for i := 0; i < 4; i++ {
		r.step(0x0F)
	}
	r.expect(0x0637, HMISRdy, 0)
	if r.d.State() != StOperationEnabled || r.in("LCR") != 6 {
		t.Errorf("state %v LCR %d", r.d.State(), r.in("LCR"))
	}
}

func TestATV320NegativeAndReversal(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", -300)
	r.put("ACC", 10)
	r.put("DEC", 10)
	r.enable()
	if r.rfr() != -50 || r.in("HMIS") != HMISAcc {
		t.Fatalf("RFR %d HMIS %d", r.rfr(), r.in("HMIS"))
	}
	for i := 0; i < 5; i++ {
		r.step(0x0F)
	}
	r.expect(0x0637, HMISRun, -300)
	r.put("LFR", 300)
	var seen []int16
	for i := 0; i < 12; i++ {
		r.step(0x0F)
		seen = append(seen, r.rfr())
		if i < 6 && r.in("HMIS") != HMISDec {
			t.Errorf("tick %d HMIS %d, want dec", i, r.in("HMIS"))
		}
	}
	if seen[5] != 0 || seen[6] != 50 || seen[11] != 300 {
		t.Errorf("reversal profile %v", seen)
	}
	r.expect(0x0637, HMISRun, 300)
}

func TestATV320Clamp(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.d.LSP = 100
	r.put("ACC", 1) // 0.1 s: one tick to HSP
	r.put("DEC", 1)
	r.put("LFR", 800)
	r.enable()
	r.expect(0x0637, HMISRun, 500)
	for _, c := range []struct{ lfr, want int }{{50, 100}, {-50, -100}, {-900, -500}, {0, 0}} {
		r.put("LFR", c.lfr)
		r.step(0x0F)
		r.step(0x0F)
		if r.rfr() != int16(c.want) {
			t.Errorf("LFR %d: RFR %d, want %d", c.lfr, r.rfr(), c.want)
		}
	}
}

func TestATV320Halt(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.put("ACC", 1)
	r.put("DEC", 10)
	r.enable()
	r.step(0x10F)
	r.expect(0x0337, HMISDec, 450)
	for i := 0; i < 9; i++ {
		r.step(0x10F)
	}
	r.expect(0x0737, HMISRdy, 0)
	if r.d.State() != StOperationEnabled {
		t.Errorf("halt left OE: %v", r.d.State())
	}
	r.step(0x0F)
	r.expect(0x0637, HMISRun, 500)
}

func TestATV320QuickStop(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.put("ACC", 1)
	r.put("DEC", 30) // fast stop: 10x, 500 in 0.3 s
	r.enable()
	r.step(0x02)
	r.expect(0x0217, HMISFst, 334)
	r.step(0x02)
	r.expect(0x0217, HMISFst, 167)
	r.step(0x02)
	r.expect(0x0250, HMISNst, 0)
	if r.d.State() != StSwitchOnDisabled {
		t.Errorf("state %v", r.d.State())
	}
}

func TestATV320QuickStopAtStandstill(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 0)
	r.enable()
	r.step(0x02) // QSA is visible for one tick even at zero speed
	r.expect(0x0217, HMISFst, 0)
	r.step(0x02)
	r.expect(0x0250, HMISNst, 0)
}

func TestATV320FaultInjection(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.put("ACC", 1)
	r.enable()
	r.d.InjectFault(16)
	if r.d.RFR() != 0 || r.d.HMIS() != HMISFault {
		t.Fatal("fault did not freewheel at once")
	}
	r.step(0x0F)
	r.expect(0x021F, HMISFault, 0)
	if r.in("LFT") != 16 || r.d.State() != StFaultReactionActive {
		t.Errorf("LFT %d state %v", r.in("LFT"), r.d.State())
	}
	r.step(0x0F)
	r.expect(0x0218, HMISFault, 0)
	r.step(0x80) // rising edge while the fault is present: refused
	r.expect(0x0218, HMISFault, 0)
	r.d.ClearFault()
	r.step(0x80) // held bit: no edge
	if r.d.State() != StFault {
		t.Fatal("reset without edge")
	}
	r.step(0x00)
	r.step(0x80)
	r.expect(0x0250, HMISNst, 0)
	if r.in("LFT") != 16 || r.d.LFT() != 16 {
		t.Errorf("LFT after reset = %d", r.in("LFT"))
	}
	r.step(0x06)
	r.step(0x0F)
	r.step(0x0F)
	r.expect(0x0637, HMISRun, 500)
}

func TestATV320STO(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.put("ACC", 1)
	r.enable()
	r.d.SetSTO(true)
	r.step(0x0F)
	r.expect(0x0250, HMISSto, 0)
	r.step(0x06)
	r.step(0x06)
	r.expect(0x0250, HMISSto, 0)
	r.d.SetSTO(false)
	r.step(0x06)
	r.expect(0x0231, HMISRdy, 0)
}

func TestATV320DIAndOL1R(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.d.SetDI(0x0005)
	r.put("OL1R", 0x0003)
	r.step(0)
	if r.in("DI") != 0x0005 || r.d.OL1R() != 0x0003 {
		t.Errorf("DI %#x OL1R %#x", r.in("DI"), r.d.OL1R())
	}
}

func TestATV320Defaults(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.d.ACC, r.d.DEC, r.d.HSP, r.d.FRS = 0, 0, 0, 0 // all fall back to defaults
	r.put("LFR", 500)
	r.enable() // PDO ACC/DEC are 0 too: 3.0 s ramp
	if r.rfr() != 16 {
		t.Errorf("default ramp first tick = %d", r.rfr())
	}
	r.put("CMD", 0x0F)
	r.n.Step(0) // zero dt holds the ramp
	if r.rfr() != 16 || r.in("HMIS") != HMISAcc {
		t.Errorf("dt 0: RFR %d HMIS %d", r.rfr(), r.in("HMIS"))
	}
	r.d.ACC = 10
	r.step(0x0F)
	if r.rfr() != 66 {
		t.Errorf("tunable ACC: RFR %d", r.rfr())
	}
}

// On the demo export only ETA, RFR, CMD and LFR exist: the model must run and
// change nothing in the input image except ETA and RFR.
func TestATV320DemoFixture(t *testing.T) {
	topo, err := ecat.LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	base := ecat.NewNetwork(topo, ecat.NewRegistry())
	base.Step(tick)
	r := newRig(t, "Demo Device 1.xml")
	r.d.SetDI(0xFFFF)
	r.d.InjectFault(9)
	r.d.ClearFault()
	r.put("LFR", 500)
	r.enable()
	for i := 0; i < 40; i++ {
		r.step(0x0F)
	}
	if r.in("ETA") != 0x0637 || r.rfr() != 500 || r.d.HMIS() != HMISRun {
		t.Errorf("ETA %#x RFR %d HMIS %d", r.in("ETA"), r.rfr(), r.d.HMIS())
	}
	eta, _ := r.slot(r.base + "^Inputs^ETA")
	rfr, _ := r.slot(r.base + "^Inputs^RFR")
	allowed := map[int]bool{eta.Byte: true, eta.Byte + 1: true, rfr.Byte: true, rfr.Byte + 1: true}
	m := topo.Masters[0].Name
	want, got := base.Images().Get(m).In, r.n.Images().Get(m).In
	for i := range want {
		if want[i] != got[i] && !allowed[i] {
			t.Errorf("input byte %d changed: %#x -> %#x", i, want[i], got[i])
		}
	}
}

func TestATV320NoLayout(t *testing.T) {
	d := NewATV320()
	d.Init(nil)
	d.InjectFault(7)
	for i := 0; i < 3; i++ {
		d.Step(tick, nil, nil)
	}
	if d.State() != StFault || d.HMIS() != HMISFault {
		t.Errorf("state %v hmis %d", d.State(), d.HMIS())
	}
	d.SetLayout([]ecat.EntrySlot{{Name: "eta", BitLen: 16}, {Name: "ETA", Byte: 2, BitLen: 16}}, nil)
	buf := make([]byte, 4)
	d.Step(tick, nil, buf)
	if buf[0] != 0x18 || buf[2] != 0 {
		t.Errorf("first duplicate should win: %x", buf)
	}
}
