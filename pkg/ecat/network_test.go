package ecat

import (
	"testing"
	"time"
)

const (
	dev1   = "Device 1 (EtherCAT)"
	a101   = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)"
	a101In = a101 + "^Channel 1^Input"
)

func loadNet(t *testing.T, reg *Registry) (*Topology, *Network) {
	t.Helper()
	topo, err := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 2.xml"))
	if err != nil {
		t.Fatal(err)
	}
	return topo, NewNetwork(topo, reg)
}

// readPath reads a slot value through the topology, never via raw offsets.
func readPath(t *testing.T, topo *Topology, n *Network, path string) uint64 {
	t.Helper()
	s, ok := topo.Slot(path)
	if !ok {
		t.Fatalf("no slot %q", path)
	}
	img := n.Images().Get(s.Master)
	buf := img.In
	if s.Dir == DirOut {
		buf = img.Out
	}
	return ReadBits(buf, s.Byte, s.Bit, s.BitLen)
}

func writePath(t *testing.T, topo *Topology, n *Network, path string, v uint64) {
	t.Helper()
	s, ok := topo.Slot(path)
	if !ok {
		t.Fatalf("no slot %q", path)
	}
	img := n.Images().Get(s.Master)
	buf := img.In
	if s.Dir == DirOut {
		buf = img.Out
	}
	WriteBits(buf, s.Byte, s.Bit, s.BitLen, v)
}

func netIDValue(id [6]byte) uint64 {
	var v uint64
	for i, b := range id {
		v |= uint64(b) << (8 * uint(i))
	}
	return v
}

func TestNetworkHealthyPseudoInputs(t *testing.T) {
	topo, n := loadNet(t, nil)
	n.Step(10 * time.Millisecond)
	want := []struct {
		path string
		v    uint64
	}{
		{a101 + "^InfoData^State", 0x0008},
		{a101 + "^WcState^WcState", 0},
		{a101 + "^InfoData^AdsAddr", netIDValue(DefaultNetID) | 1002<<48},
		{"TIID^" + dev1 + "^Inputs^SlaveCount", 10},
		{"TIID^" + dev1 + "^Inputs^DevState", 0},
		{"TIID^" + dev1 + "^Inputs^Frm0State", 0},
		{"TIID^" + dev1 + "^Inputs^Frm0WcState", 0},
		{"TIID^" + dev1 + "^InfoData^ChangeCount", 0},
		{"TIID^" + dev1 + "^InfoData^AmsNetId", netIDValue([6]byte{192, 168, 0, 1, 1, 1})},
		{"TIID^Device 2 (EtherCAT)^Inputs^SlaveCount", 4},
	}
	for _, w := range want {
		if got := readPath(t, topo, n, w.path); got != w.v {
			t.Errorf("%s = %#x, want %#x", w.path, got, w.v)
		}
	}
}

func TestNetworkPortWithoutPhys(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	topo.Masters[0].Slaves[1].HasPhys = false
	n := NewNetwork(topo, nil)
	n.Step(0)
	got := readPath(t, topo, n, a101+"^InfoData^AdsAddr") >> 48
	if got != 1002 { // 1001 + Index 1
		t.Fatalf("port = %d, want 1002", got)
	}
}

func TestNetworkSetMasterNetID(t *testing.T) {
	topo, n := loadNet(t, nil)
	id := [6]byte{10, 0, 0, 5, 1, 1}
	if err := n.SetMasterNetID(dev1, id); err != nil {
		t.Fatal(err)
	}
	if err := n.SetMasterNetID("nope", id); err == nil {
		t.Fatal("unknown master accepted")
	}
	n.Step(0)
	if got := readPath(t, topo, n, a101+"^InfoData^AdsAddr"); got != netIDValue(id)|1002<<48 {
		t.Errorf("AdsAddr = %#x", got)
	}
	if got := readPath(t, topo, n, "TIID^"+dev1+"^InfoData^AmsNetId"); got != netIDValue(id) {
		t.Errorf("AmsNetId = %#x", got)
	}
}

func TestFaultAPI(t *testing.T) {
	topo, n := loadNet(t, nil)
	if err := n.SetSlaveState(dev1, 1, 0x0002); err != nil {
		t.Fatal(err)
	}
	if n.WcBad(dev1, 1) {
		t.Error("WcBad before SetWcState")
	}
	if err := n.SetWcState(dev1, 1, true); err != nil {
		t.Fatal(err)
	}
	if !n.WcBad(dev1, 1) || n.WcBad("nope", 0) {
		t.Error("WcBad after SetWcState")
	}
	if err := n.SetDevState(dev1, 0x0001); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if got := readPath(t, topo, n, a101+"^InfoData^State"); got != 2 {
		t.Errorf("State = %d", got)
	}
	if got := readPath(t, topo, n, a101+"^WcState^WcState"); got != 1 {
		t.Errorf("WcState = %d", got)
	}
	if got := readPath(t, topo, n, "TIID^"+dev1+"^Inputs^DevState"); got != 1 {
		t.Errorf("DevState = %d", got)
	}
	// Other slaves stay healthy.
	if got := readPath(t, topo, n, "TIID^"+dev1+"^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^InfoData^State"); got != 8 {
		t.Errorf("A1.02 State = %d", got)
	}
	n.ClearFaults()
	n.Step(0)
	if got := readPath(t, topo, n, a101+"^InfoData^State"); got != 8 {
		t.Errorf("cleared State = %d", got)
	}
	if got := readPath(t, topo, n, a101+"^WcState^WcState"); got != 0 {
		t.Errorf("cleared WcState = %d", got)
	}
	if got := readPath(t, topo, n, "TIID^"+dev1+"^Inputs^DevState"); got != 0 {
		t.Errorf("cleared DevState = %d", got)
	}
}

func TestFaultAPIErrors(t *testing.T) {
	_, n := loadNet(t, nil)
	cases := []error{
		n.SetSlaveState("nope", 0, 1),
		n.SetSlaveState(dev1, -1, 1),
		n.SetSlaveState(dev1, 10, 1),
		n.SetWcState("nope", 0, true),
		n.SetWcState(dev1, 99, true),
		n.SetDevState("nope", 1),
	}
	for i, err := range cases {
		if err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestNetworkDefaultKeepsInjectedInputs(t *testing.T) {
	topo, n := loadNet(t, nil)
	writePath(t, topo, n, a101In, 1)
	n.Step(time.Millisecond)
	if got := readPath(t, topo, n, a101In); got != 1 {
		t.Fatalf("injected input clobbered: %d", got)
	}
}

type recDevice struct {
	slave   *Slave
	in, out []byte
	steps   int
	dt      time.Duration
}

func (d *recDevice) Init(s *Slave) { d.slave = s }
func (d *recDevice) Step(dt time.Duration, out, in []byte) {
	d.steps++
	d.dt, d.out, d.in = dt, out, in
	for i := range in {
		in[i] = 0xFF
	}
}

func TestRegistryDeviceGetsOwnRegion(t *testing.T) {
	reg := NewRegistry()
	var made []*recDevice
	// EL1008: 8 one-bit inputs, no outputs.
	reg.Register(2, 66072658, func() Device { d := &recDevice{}; made = append(made, d); return d })
	topo, n := loadNet(t, reg)
	if len(made) != 2 { // one EL1008 per demo master
		t.Fatalf("made %d devices", len(made))
	}
	n.Step(5 * time.Millisecond)
	d := made[0]
	if d.slave == nil || d.slave.Name != "DEMO.A1.01 (EL1008)" {
		t.Fatalf("Init slave = %+v", d.slave)
	}
	if d.steps != 1 || d.dt != 5*time.Millisecond {
		t.Fatalf("steps=%d dt=%v", d.steps, d.dt)
	}
	// Each 1-bit PDO is byte-aligned, so eight PDOs span eight bytes.
	if len(d.out) != 0 || len(d.in) != 8 {
		t.Fatalf("len(out)=%d len(in)=%d", len(d.out), len(d.in))
	}
	for ch := 1; ch <= 8; ch++ {
		p := a101 + "^Channel " + string(rune('0'+ch)) + "^Input"
		if got := readPath(t, topo, n, p); got != 1 {
			t.Errorf("%s = %d", p, got)
		}
	}
	// The neighbouring EL2008 and the pseudo-inputs are untouched by the device.
	if got := readPath(t, topo, n, a101+"^InfoData^State"); got != 8 {
		t.Errorf("State = %d", got)
	}
}

func TestRegistryOutputRegion(t *testing.T) {
	reg := NewRegistry()
	var d *recDevice
	// ATV320 has both an RxPdo and a TxPdo.
	reg.Register(134217818, 905, func() Device {
		if d == nil {
			d = &recDevice{}
			return d
		}
		return &recDevice{}
	})
	topo, n := loadNet(t, reg)
	base := "TIID^" + dev1 + "^DEMO.CN01.FD01 (ATV320 EtherCAT)"
	writePath(t, topo, n, base+"^Outputs^CMD", 0x000F)
	n.Step(0)
	if len(d.out) == 0 || len(d.in) == 0 {
		t.Fatalf("out=%d in=%d", len(d.out), len(d.in))
	}
	if d.out[0] != 0x0F {
		t.Errorf("out[0] = %#x", d.out[0])
	}
	if got := readPath(t, topo, n, base+"^Inputs^ETA"); got != 0xFFFF {
		t.Errorf("ETA = %#x", got)
	}
}

func TestRegistryFallback(t *testing.T) {
	reg := NewRegistry()
	s := &Slave{Vendor: 1, Product: 2}
	if _, ok := reg.New(s).(Passthrough); !ok {
		t.Fatal("unknown slave did not get Passthrough")
	}
	if _, ok := DefaultRegistry.New(s).(Passthrough); !ok {
		t.Fatal("DefaultRegistry fallback")
	}
	var p Passthrough
	p.Init(s)
	p.Step(0, nil, nil)
}

func TestNetworkSpansEmptyForCoupler(t *testing.T) {
	reg := NewRegistry()
	var d *recDevice
	reg.Register(2, 78655570, func() Device { // EK1200, no PDOs
		if d == nil {
			d = &recDevice{}
			return d
		}
		return &recDevice{}
	})
	_, n := loadNet(t, reg)
	n.Step(0)
	if d == nil || len(d.in) != 0 || len(d.out) != 0 {
		t.Fatalf("coupler region not empty: %+v", d)
	}
}

func TestSlaveSpansSkipsUnlaidEntries(t *testing.T) {
	m := &Master{Name: "M", index: map[string]int{}}
	s := &Slave{Name: "S", Pdos: []Pdo{{Name: "P", Entries: []Entry{{Name: "E", Index: "#x6000", BitLen: 8}}}}}
	in, out := slaveSpans(m, s)
	if in != (span{}) || out != (span{}) {
		t.Fatalf("in=%v out=%v", in, out)
	}
	if v := view(make([]byte, 2), span{1, 4}); len(v) != 0 {
		t.Fatalf("out-of-range view len %d", len(v))
	}
}
