package ecat

import (
	"testing"
	"time"
)

const dev2 = "Device 2 (EtherCAT)"

type stateDev struct {
	Passthrough
	state uint16
	reqs  []uint16
}

func (d *stateDev) EcState() uint16       { return d.state }
func (d *stateDev) RequestState(s uint16) { d.reqs = append(d.reqs, s) }

func TestServiceMasters(t *testing.T) {
	_, n := loadNet(t, nil)
	names := n.MasterNames()
	if len(names) != 2 || names[0] != dev1 || names[1] != dev2 {
		t.Fatalf("MasterNames = %v", names)
	}
	if got, err := n.MasterByNetID([6]byte{192, 168, 0, 1, 1, 1}); err != nil || got != dev1 {
		t.Errorf("MasterByNetID = %q, %v", got, err)
	}
	if _, err := n.MasterByNetID([6]byte{}); err == nil {
		t.Error("zero NetID must be an error")
	}
	if _, err := n.MasterByNetID([6]byte{9, 9, 9, 9, 9, 9}); err == nil {
		t.Error("unknown NetID must be an error")
	}
	if n.SlaveCount(dev1) != 10 || n.SlaveCount(dev2) != 4 || n.SlaveCount("nope") != 0 {
		t.Errorf("SlaveCount = %d %d %d", n.SlaveCount(dev1), n.SlaveCount(dev2), n.SlaveCount("nope"))
	}
}

func TestServiceSlaveLookup(t *testing.T) {
	_, n := loadNet(t, nil)
	if got := n.SlaveAddr(dev1, 1); got != 1002 {
		t.Fatalf("SlaveAddr = %d", got)
	}
	for _, bad := range []int{-1, 10} {
		if got := n.SlaveAddr(dev1, bad); got != 0 {
			t.Errorf("SlaveAddr(%d) = %d", bad, got)
		}
	}
	if n.SlaveAddr("nope", 0) != 0 {
		t.Error("unknown master addr")
	}
	for i := 0; i < n.SlaveCount(dev1); i++ {
		idx, dev, err := n.Slave(dev1, n.SlaveAddr(dev1, i))
		if err != nil || idx != i || dev == nil {
			t.Errorf("Slave(%d) = %d, %v, %v", i, idx, dev, err)
		}
	}
	if _, _, err := n.Slave(dev1, 4242); err == nil {
		t.Error("unknown address must be an error")
	}
	if _, _, err := n.Slave("nope", 1001); err == nil {
		t.Error("unknown master must be an error")
	}
}

func TestServiceSlaveAddrWithoutPhys(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	topo.Masters[0].Slaves[3].HasPhys = false
	n := NewNetwork(topo, nil)
	if got := n.SlaveAddr(dev1, 3); got != 1004 {
		t.Fatalf("SlaveAddr = %d, want 1004", got)
	}
	if idx, _, err := n.Slave(dev1, 1004); err != nil || idx != 3 {
		t.Fatalf("Slave(1004) = %d, %v", idx, err)
	}
}

func TestServicePlainStateRequest(t *testing.T) {
	topo, n := loadNet(t, nil)
	if s, link := n.SlaveState(dev1, 1); s != StateOP || link != 0 {
		t.Fatalf("initial = %#x, %#x", s, link)
	}
	if err := n.RequestSlaveState(dev1, 1, StatePreOp); err != nil {
		t.Fatal(err)
	}
	n.Step(time.Millisecond)
	if s, _ := n.SlaveState(dev1, 1); s != StateOP {
		t.Fatalf("after 1 step = %#x, want OP", s)
	}
	n.Step(time.Millisecond)
	if s, _ := n.SlaveState(dev1, 1); s != StatePreOp {
		t.Fatalf("after 2 steps = %#x, want PreOp", s)
	}
	if got := readPath(t, topo, n, a101+"^InfoData^State"); got != uint64(StatePreOp) {
		t.Errorf("InfoData.State = %#x", got)
	}
	// Override wins; ClearFaults keeps the requested state.
	_ = n.SetSlaveState(dev1, 1, StateSafeOp)
	if s, _ := n.SlaveState(dev1, 1); s != StateSafeOp {
		t.Errorf("override = %#x", s)
	}
	n.ClearFaults()
	if s, _ := n.SlaveState(dev1, 1); s != StatePreOp {
		t.Errorf("after ClearFaults = %#x", s)
	}
	// Zero delay applies at once.
	n.StateDelay = 0
	if err := n.RequestSlaveState(dev1, 1, StateOP); err != nil {
		t.Fatal(err)
	}
	if s, _ := n.SlaveState(dev1, 1); s != StateOP {
		t.Errorf("zero delay = %#x", s)
	}
	for _, c := range []struct {
		m string
		i int
	}{{"nope", 0}, {dev1, -1}, {dev1, 10}} {
		if err := n.RequestSlaveState(c.m, c.i, StateOP); err == nil {
			t.Errorf("RequestSlaveState(%q, %d) must fail", c.m, c.i)
		}
		if s, l := n.SlaveState(c.m, c.i); s != 0 || l != 0 {
			t.Errorf("SlaveState(%q, %d) = %#x, %#x", c.m, c.i, s, l)
		}
	}
}

func TestServiceStateDevice(t *testing.T) {
	sd := &stateDev{state: StatePreOp}
	topo, err := LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	s1 := topo.Masters[0].Slaves[1]
	reg := NewRegistry()
	reg.Register(s1.Vendor, s1.Product, func() Device { return sd })
	n := NewNetwork(topo, reg)
	n.Step(0)
	if got := readPath(t, topo, n, a101+"^InfoData^State"); got != uint64(StatePreOp) {
		t.Fatalf("InfoData.State = %#x", got)
	}
	if err := n.RequestSlaveState(dev1, 1, StateOP); err != nil {
		t.Fatal(err)
	}
	if len(sd.reqs) != 1 || sd.reqs[0] != StateOP {
		t.Fatalf("requests = %v", sd.reqs)
	}
	sd.state = StateOP
	if s, _ := n.SlaveState(dev1, 1); s != StateOP {
		t.Errorf("state = %#x", s)
	}
	_ = n.SetSlaveState(dev1, 1, StateInit)
	if s, _ := n.SlaveState(dev1, 1); s != StateInit {
		t.Errorf("override = %#x", s)
	}
}

func TestServiceLinkCrcMaster(t *testing.T) {
	_, n := loadNet(t, nil)
	if err := n.SetLinkState(dev1, 2, LinkNotPresent|LinkWithoutComm); err != nil {
		t.Fatal(err)
	}
	if _, link := n.SlaveState(dev1, 2); link != 3 {
		t.Errorf("link = %d", link)
	}
	c := [4]uint32{1, 2, 3, 4}
	if err := n.SetCrcErrors(dev1, 2, c); err != nil {
		t.Fatal(err)
	}
	if got := n.CrcErrors(dev1, 2); got != c {
		t.Errorf("crc = %v", got)
	}
	if err := n.ClearCrc(dev1, 2); err != nil {
		t.Fatal(err)
	}
	if got := n.CrcErrors(dev1, 2); got != ([4]uint32{}) {
		t.Errorf("cleared crc = %v", got)
	}
	if n.MasterState(dev1) != StateOP || n.MasterState("nope") != 0 {
		t.Errorf("master state = %#x %#x", n.MasterState(dev1), n.MasterState("nope"))
	}
	if err := n.SetMasterState(dev1, StateInit); err != nil {
		t.Fatal(err)
	}
	if n.MasterState(dev1) != StateInit {
		t.Errorf("master override = %#x", n.MasterState(dev1))
	}
	_ = n.SetCrcErrors(dev1, 2, c)
	n.ClearFaults()
	if _, link := n.SlaveState(dev1, 2); link != 0 {
		t.Errorf("link after clear = %d", link)
	}
	if n.CrcErrors(dev1, 2) != ([4]uint32{}) || n.MasterState(dev1) != StateOP {
		t.Error("ClearFaults must reset CRC and master state")
	}
	errs := []error{
		n.SetLinkState("nope", 0, 1),
		n.SetLinkState(dev1, 99, 1),
		n.SetCrcErrors(dev1, -1, c),
		n.ClearCrc(dev1, 99),
		n.SetMasterState("nope", 1),
	}
	for i, err := range errs {
		if err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
	if n.CrcErrors("nope", 0) != ([4]uint32{}) {
		t.Error("unknown CRC must be zero")
	}
}
