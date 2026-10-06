package devices

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

func (r *rig) infoState() uint16 {
	r.t.Helper()
	s, buf := r.slot(r.base + "^InfoData^State")
	return uint16(ecat.ReadBits(buf, s.Byte, s.Bit, s.BitLen))
}

func (r *rig) request(s uint16) {
	r.t.Helper()
	if err := r.n.RequestSlaveState(r.m, r.idx, s); err != nil {
		r.t.Fatal(err)
	}
}

func TestATV320BootsPreOp(t *testing.T) {
	r := newRigPreOp(t, "atv320_device.xml")
	r.n.Step(tick)
	if r.infoState() != 0x0002 {
		t.Errorf("InfoData.State = %#x, want PreOp", r.infoState())
	}
	if s, _ := r.n.SlaveState(r.m, r.idx); s != ecat.StatePreOp {
		t.Errorf("SlaveState = %#x", s)
	}
}

func TestATV320PreOpOnlyForATV320(t *testing.T) {
	topo, err := ecat.LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	n := ecat.NewNetwork(topo, nil)
	n.Step(tick)
	m := topo.Masters[0]
	img := n.Images().Get(m.Name).In
	seen := map[uint64]int{}
	for _, s := range m.Slaves {
		slot, ok := topo.Slot(ecat.SlaveBasePath(m.Name, s) + "^InfoData^State")
		if !ok {
			continue
		}
		got := ecat.ReadBits(img, slot.Byte, slot.Bit, slot.BitLen)
		want := uint64(0x0008)
		if s.Vendor == ATV320Vendor && s.Product == ATV320Product {
			want = 0x0002
		}
		if got != want {
			t.Errorf("%s InfoData.State = %#x, want %#x", s.Name, got, want)
		}
		seen[want]++
	}
	if seen[0x0002] == 0 || seen[0x0008] == 0 {
		t.Fatalf("fixture must hold both an ATV320 and another slave: %v", seen)
	}
}

func TestATV320StateTransitions(t *testing.T) {
	r := newRigPreOp(t, "atv320_device.xml")
	for _, s := range []uint16{ecat.StateOP, ecat.StatePreOp, ecat.StateInit, ecat.StateSafeOp, ecat.StateOP} {
		prev := r.d.EcState()
		r.request(s)
		r.step(0)
		if r.d.EcState() != prev {
			t.Fatalf("-> %#x: changed after 1 step", s)
		}
		r.step(0)
		if r.d.EcState() != s || r.infoState() != s {
			t.Fatalf("-> %#x: state %#x InfoData %#x", s, r.d.EcState(), r.infoState())
		}
		r.step(0)
		if r.d.EcState() != s {
			t.Fatalf("-> %#x: state moved again", s)
		}
	}
}

func TestATV320SDOByState(t *testing.T) {
	r := newRigPreOp(t, "atv320_device.xml")
	if _, code := r.d.SDORead(0x2001, 0x05); code != 0 {
		t.Errorf("PreOp read abort %#x", code)
	}
	if code := r.d.SDOWrite(0x2042, 0x03, le16(480)); code != 0 {
		t.Errorf("PreOp write abort %#x", code)
	}
	r.d.StateDelay = 0
	r.d.RequestState(ecat.StateInit)
	if _, code := r.d.SDORead(0x2001, 0x05); code != ecat.AbortDeviceState {
		t.Errorf("INIT read abort %#x", code)
	}
	if code := r.d.SDOWrite(0x2001, 0x05, le16(1)); code != ecat.AbortDeviceState {
		t.Errorf("INIT write abort %#x", code)
	}
	r.d.RequestState(ecat.StateOP)
	if _, code := r.d.SDORead(0x2001, 0x05); code != 0 {
		t.Errorf("OP read abort %#x", code)
	}
}

func TestATV320NoExchangeBelowOP(t *testing.T) {
	r := newRigPreOp(t, "atv320_device.xml")
	start := r.d.State()
	r.d.SetDI(0x0005)
	r.put("LFR", 500)
	for _, c := range []int{0x00, 0x06, 0x07, 0x0F, 0x0F} {
		r.step(c)
	}
	if r.d.State() != start {
		t.Errorf("CiA402 moved below OP: %v -> %v", start, r.d.State())
	}
	for _, e := range []string{"ETA", "RFR", "LCR", "DI", "LFT", "HMIS"} {
		if got := r.in(e); got != 0 {
			t.Errorf("%s = %#x below OP, want 0", e, got)
		}
	}
	if r.d.RFR() != 0 {
		t.Errorf("RFR = %d", r.d.RFR())
	}
	r.d.SetSTO(true)
	r.step(0)
	if r.d.HMIS() != HMISSto || r.d.State() != StSwitchOnDisabled {
		t.Errorf("STO below OP: HMIS %d state %v", r.d.HMIS(), r.d.State())
	}
}

func TestATV320LeaveOPWhileEnabled(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.put("LFR", 500)
	r.enable()
	for i := 0; i < 5; i++ {
		r.step(0x0F)
	}
	if r.rfr() == 0 {
		t.Fatal("drive not running")
	}
	r.request(ecat.StatePreOp)
	r.step(0x0F)
	if r.d.State() != StOperationEnabled {
		t.Fatalf("left OP early: %v", r.d.State())
	}
	r.step(0x0F)
	if r.d.State() != StSwitchOnDisabled || r.d.RFR() != 0 || r.in("RFR") != 0 || r.in("ETA") != 0 {
		t.Errorf("after leaving OP: state %v RFR %d in RFR %d ETA %#x", r.d.State(), r.d.RFR(), r.in("RFR"), r.in("ETA"))
	}
	// Back in OP the master must walk the state machine again.
	goOP(t, r.n, r.m, r.idx)
	r.step(0x0F)
	if r.d.State() == StOperationEnabled {
		t.Error("re-enabled without shutdown/switch on")
	}
	r.enable()
}

func TestATV320OverrideWinsOverEcState(t *testing.T) {
	r := newRigPreOp(t, "atv320_device.xml")
	if err := r.n.SetSlaveState(r.m, r.idx, ecat.StateOP); err != nil {
		t.Fatal(err)
	}
	r.step(0)
	if r.infoState() != uint16(ecat.StateOP) || r.d.EcState() != ecat.StatePreOp {
		t.Errorf("InfoData %#x, model %#x", r.infoState(), r.d.EcState())
	}
	r.n.ClearFaults()
	r.step(0)
	if r.infoState() != ecat.StatePreOp {
		t.Errorf("after ClearFaults InfoData %#x", r.infoState())
	}
}
