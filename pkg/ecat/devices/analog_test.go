package devices

import (
	"math"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

const (
	el3054Name = "DEMO.A2.01 (EL3054)"
	el3064Name = "DEMO.A2.02 (EL3064)"
)

func analogNet(t *testing.T) (*ecat.Topology, *ecat.Network) {
	t.Helper()
	topo, err := ecat.LoadProject(fixture("Demo Analog.xml"))
	if err != nil {
		t.Fatal(err)
	}
	reg := ecat.NewRegistry()
	Register(reg)
	n := ecat.NewNetwork(topo, reg)
	if len(n.Diagnostics()) != 0 {
		t.Fatalf("diagnostics: %v", n.Diagnostics())
	}
	return topo, n
}

func analogDev(t *testing.T, n *ecat.Network, name string) *Analog {
	t.Helper()
	d, ok := n.DeviceByName(name)
	if !ok {
		t.Fatalf("no device %s", name)
	}
	a, ok := d.(*Analog)
	if !ok {
		t.Fatalf("%s is %T, want *Analog", name, d)
	}
	return a
}

type aiRead struct {
	value                       int16
	under, over, err, toggleBit uint64
}

func readAI(t *testing.T, topo *ecat.Topology, n *ecat.Network, slave string, ch int) aiRead {
	t.Helper()
	img := n.Images().Get(dev1).In
	pdo := "AI Standard Channel " + itoa(ch)
	return aiRead{
		value:     int16(readSlotIn(t, topo, img, slave, pdo, "Value")),
		under:     readSlotIn(t, topo, img, slave, pdo, "Status__Underrange"),
		over:      readSlotIn(t, topo, img, slave, pdo, "Status__Overrange"),
		err:       readSlotIn(t, topo, img, slave, pdo, "Status__Error"),
		toggleBit: readSlotIn(t, topo, img, slave, pdo, "Status__TxPDO Toggle"),
	}
}

func readSlotIn(t *testing.T, topo *ecat.Topology, img []byte, slave, pdo, entry string) uint64 {
	t.Helper()
	return readSlot(img, slotOf(t, topo, slave, pdo, entry))
}

func TestAnalogEL3054CurrentScaling(t *testing.T) {
	cases := []struct {
		mA               float64
		value            int16
		under, over, err uint64
	}{
		{12, 16384, 0, 0, 0},
		{4, 0, 0, 0, 0},
		{20, 32767, 0, 0, 0},
		{3, 0, 1, 0, 0},
		{21, 32767, 0, 1, 0},
		{0, 0, 1, 0, 1},
		{-1, 0, 1, 0, 1},
		{math.NaN(), 0, 1, 0, 1},
		{math.Inf(1), 32767, 0, 1, 0},
	}
	for _, c := range cases {
		topo, n := analogNet(t)
		a := analogDev(t, n, el3054Name)
		if err := a.SetCurrent(1, c.mA); err != nil {
			t.Fatal(err)
		}
		n.Step(0)
		got := readAI(t, topo, n, el3054Name, 1)
		if got.value != c.value || got.under != c.under || got.over != c.over || got.err != c.err {
			t.Errorf("%v mA: got %+v, want value %d under %d over %d err %d", c.mA, got, c.value, c.under, c.over, c.err)
		}
		if other := readAI(t, topo, n, el3054Name, 2); other.value != 0 || other.err != 0 {
			t.Errorf("%v mA leaked into channel 2: %+v", c.mA, other)
		}
	}
}

func TestAnalogEL3064VoltageScaling(t *testing.T) {
	cases := []struct {
		v           float64
		value       int16
		under, over uint64
	}{
		{5, 16384, 0, 0},
		{0, 0, 0, 0},
		{10, 32767, 0, 0},
		{-0.5, 0, 1, 0},
		{10.5, 32767, 0, 1},
		{math.Inf(-1), 0, 1, 0},
	}
	for _, c := range cases {
		topo, n := analogNet(t)
		a := analogDev(t, n, el3064Name)
		if err := a.SetVoltage(2, c.v); err != nil {
			t.Fatal(err)
		}
		n.Step(0)
		got := readAI(t, topo, n, el3064Name, 2)
		if got.value != c.value || got.under != c.under || got.over != c.over || got.err != 0 {
			t.Errorf("%v V: got %+v", c.v, got)
		}
	}
	topo, n := analogNet(t)
	a := analogDev(t, n, el3064Name)
	if err := a.SetVoltage(1, math.NaN()); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if got := readAI(t, topo, n, el3064Name, 1); got.err != 1 || got.value != 0 {
		t.Errorf("NaN V: %+v", got)
	}
}

func TestAnalogToggleAndRaw(t *testing.T) {
	topo, n := analogNet(t)
	a := analogDev(t, n, el3054Name)
	if err := a.SetCurrent(3, 2); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	first := readAI(t, topo, n, el3054Name, 3)
	n.Step(0)
	second := readAI(t, topo, n, el3054Name, 3)
	if first.toggleBit == second.toggleBit {
		t.Error("TxPDO Toggle did not flip")
	}
	if err := a.SetRaw(3, -5); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if got := readAI(t, topo, n, el3054Name, 3); got.value != -5 || got.under != 0 || got.err != 0 {
		t.Errorf("SetRaw: %+v", got)
	}
	if a.Channels() != 4 {
		t.Errorf("Channels = %d", a.Channels())
	}
}

func TestAnalogErrors(t *testing.T) {
	_, n := analogNet(t)
	cur, volt := analogDev(t, n, el3054Name), analogDev(t, n, el3064Name)
	for _, err := range []error{
		cur.SetCurrent(0, 12), cur.SetCurrent(5, 12), cur.SetRaw(9, 1),
		cur.SetVoltage(1, 5), volt.SetCurrent(1, 12), volt.SetVoltage(5, 1),
	} {
		if err == nil || !strings.Contains(err.Error(), "DEMO.A2") {
			t.Errorf("err = %v", err)
		}
	}
}

// A packed layout with one 16-bit Status WORD per channel.
func TestAnalogStatusWord(t *testing.T) {
	a := &Analog{kind: analogCurrent}
	a.Init(&ecat.Slave{Name: "W (EL3054)"})
	a.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "AI Standard Channel 1", Entry: "Status", Dir: ecat.DirIn, Bit: 0, BitLen: 16},
		{Pdo: "AI Standard Channel 1", Entry: "Value", Dir: ecat.DirIn, Bit: 16, BitLen: 16},
		{Pdo: "AI Standard Channel 2", Entry: "Value", Dir: ecat.DirIn, Bit: 32, BitLen: 16},
	}))
	if err := a.SetCurrent(1, 0); err != nil {
		t.Fatal(err)
	}
	in := make([]byte, 6)
	a.Step(0, nil, in)
	st := uint16(in[0]) | uint16(in[1])<<8
	if st != 1|1<<6|1<<15 {
		t.Errorf("status word = %016b", st)
	}
	if err := a.SetCurrent(1, 21); err != nil {
		t.Fatal(err)
	}
	a.Step(0, nil, in)
	st = uint16(in[0]) | uint16(in[1])<<8
	if st != 1<<1 || in[2] != 0xff || in[3] != 0x7f {
		t.Errorf("overrange: status %016b value % x", st, in[2:4])
	}
	// Channel 2 has a Value but no status entry; it still works.
	if err := a.SetCurrent(2, 12); err != nil {
		t.Fatal(err)
	}
	a.Step(0, nil, in)
	if in[4] != 0x00 || in[5] != 0x40 {
		t.Errorf("channel 2 value % x", in[4:6])
	}
}

func TestAnalogRegistrations(t *testing.T) {
	reg := ecat.NewRegistry()
	Register(reg)
	for model, kind := range map[string]analogKind{"EL3052": analogCurrent, "EL3058": analogCurrent, "EL3062": analogVoltage, "EL3068": analogVoltage} {
		f, ok := reg.Lookup(&ecat.Slave{Vendor: VendorBeckhoff, Product: 0xbad, Model: model})
		if !ok {
			t.Errorf("no fallback for %s", model)
			continue
		}
		if a, isA := f().(*Analog); !isA || a.kind != kind {
			t.Errorf("%s fallback is %#v", model, f())
		}
	}
}
