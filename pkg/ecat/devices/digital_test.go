package devices

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

// slotOf resolves (slave, pdo, entry) to its slot in the master image.
func slotOf(t *testing.T, topo *ecat.Topology, slave, pdo, entry string) ecat.Slot {
	t.Helper()
	m := topo.Master(dev1)
	for _, s := range m.Slaves {
		if s.Name != slave {
			continue
		}
		for _, p := range s.Pdos {
			for _, e := range p.Entries {
				if p.Name == pdo && e.Name == entry {
					sl, ok := m.Slot(ecat.LinkPath(m.Name, s, p, e))
					if !ok {
						t.Fatalf("no slot for %s %s/%s", slave, pdo, entry)
					}
					return sl
				}
			}
		}
	}
	t.Fatalf("no entry %s %s/%s", slave, pdo, entry)
	return ecat.Slot{}
}

func digital(t *testing.T, n *ecat.Network, name string) *DigitalIO {
	t.Helper()
	d, ok := n.DeviceByName(name)
	if !ok {
		t.Fatalf("no device %s", name)
	}
	dio, ok := d.(*DigitalIO)
	if !ok {
		t.Fatalf("%s is %T, want *DigitalIO", name, d)
	}
	return dio
}

func readSlot(img []byte, s ecat.Slot) uint64 { return ecat.ReadBits(img, s.Byte, s.Bit, s.BitLen) }

func TestDigitalEL1008InputReachesImage(t *testing.T) {
	topo, n := demoNet(t)
	const name = "DEMO.A1.01 (EL1008)"
	d := digital(t, n, name)
	if err := d.SetInput(3, true); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	in := n.Images().Get(dev1).In
	for ch := 1; ch <= 8; ch++ {
		want := uint64(0)
		if ch == 3 {
			want = 1
		}
		if got := readSlot(in, slotOf(t, topo, name, "Channel "+itoa(ch), "Input")); got != want {
			t.Errorf("channel %d = %d, want %d", ch, got, want)
		}
	}
	if err := d.SetInput(3, false); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if readSlot(n.Images().Get(dev1).In, slotOf(t, topo, name, "Channel 3", "Input")) != 0 {
		t.Error("channel 3 not cleared")
	}
	if err := d.SetInput(9, true); err == nil || !strings.Contains(err.Error(), name) {
		t.Errorf("SetInput(9) err = %v", err)
	}
	if d.Output(1) {
		t.Error("EL1008 has no outputs")
	}
}

func TestDigitalEL2008OutputFromImage(t *testing.T) {
	topo, n := demoNet(t)
	const name = "DEMO.A1.02 (EL2008)"
	d := digital(t, n, name)
	s := slotOf(t, topo, name, "Channel 5", "Output")
	ecat.WriteBits(n.Images().Get(dev1).Out, s.Byte, s.Bit, s.BitLen, 1)
	n.Step(0)
	for ch := 1; ch <= 8; ch++ {
		if d.Output(ch) != (ch == 5) {
			t.Errorf("Output(%d) = %v", ch, d.Output(ch))
		}
	}
	if d.Output(0) || d.Output(9) {
		t.Error("out-of-range channel true")
	}
	if err := d.SetInput(1, true); err == nil {
		t.Error("SetInput on output-only EL2008 accepted")
	}
}

func TestDigitalEP2338Fixture(t *testing.T) {
	topo, n := demoNet(t)
	const name = "DEMO.A2 (EP2338-0002)"
	d := digital(t, n, name)
	if err := d.SetInput(1, true); err != nil {
		t.Fatal(err)
	}
	s := slotOf(t, topo, name, "Channel 9", "Output")
	ecat.WriteBits(n.Images().Get(dev1).Out, s.Byte, s.Bit, s.BitLen, 1)
	n.Step(0)
	if readSlot(n.Images().Get(dev1).In, slotOf(t, topo, name, "Channel 1", "Input")) != 1 {
		t.Error("EP2338 Channel 1 input not in image")
	}
	if !d.Output(1) {
		t.Error("EP2338 Output(1) should read Channel 9")
	}
}

// ep2338 builds a full 8-in/8-out EP2338 with packed bits, as the real
// exports lay it out.
func ep2338() *DigitalIO {
	var fields []ecat.Field
	for i := 1; i <= 8; i++ {
		fields = append(fields, ecat.Field{Pdo: "Channel " + itoa(i), Entry: "Input", Dir: ecat.DirIn, Bit: i - 1, BitLen: 1})
	}
	for i := 9; i <= 16; i++ {
		fields = append(fields, ecat.Field{Pdo: "Channel " + itoa(i), Entry: "Output", Dir: ecat.DirOut, Bit: i - 9, BitLen: 1})
	}
	d := &DigitalIO{}
	d.Init(&ecat.Slave{Name: "B (EP2338-1002)", Model: "EP2338-1002"})
	d.Bind(ecat.NewLayout(fields))
	return d
}

func TestDigitalEP2338PackedChannels(t *testing.T) {
	for ch := 1; ch <= 8; ch++ {
		d := ep2338()
		if err := d.SetInput(ch, true); err != nil {
			t.Fatal(err)
		}
		in, out := []byte{0}, []byte{1 << uint(ch-1)}
		d.Step(0, out, in)
		if in[0] != 1<<uint(ch-1) {
			t.Errorf("SetInput(%d): in = %08b", ch, in[0])
		}
		for o := 1; o <= 8; o++ {
			if d.Output(o) != (o == ch) {
				t.Errorf("out byte %08b: Output(%d) = %v", out[0], o, d.Output(o))
			}
		}
	}
}

func TestDigitalCTEUOutputBits(t *testing.T) {
	topo, n := demoNet(t)
	const name = "DEMO.V1 (CTEU-EtherCAT Modular)"
	d := digital(t, n, name)
	s := slotOf(t, topo, name, "Outputs", "C2 Output")
	ecat.WriteBits(n.Images().Get(dev1).Out, s.Byte, s.Bit, s.BitLen, 0x05)
	s1 := slotOf(t, topo, name, "Outputs", "C1 Output")
	ecat.WriteBits(n.Images().Get(dev1).Out, s1.Byte, s1.Bit, s1.BitLen, 0x80)
	n.Step(0)
	var on []int
	for i := 1; i <= 17; i++ {
		if d.Output(i) {
			on = append(on, i)
		}
	}
	if want := []int{8, 9, 11}; !equalInts(on, want) {
		t.Errorf("CTEU outputs on = %v, want %v", on, want)
	}
	if err := d.SetInput(1, true); err == nil || !strings.Contains(err.Error(), name) {
		t.Errorf("SetInput on output-only CTEU err = %v", err)
	}
}

func TestDemoDigitalModels(t *testing.T) {
	_, n := demoNet(t)
	for _, name := range []string{"DEMO.A1.01 (EL1008)", "DEMO.A1.02 (EL2008)", "DEMO.A2 (EP2338-0002)", "DEMO.V1 (CTEU-EtherCAT Modular)"} {
		digital(t, n, name)
	}
}

func TestDigitalRegistrations(t *testing.T) {
	reg := ecat.NewRegistry()
	Register(reg)
	for _, model := range []string{"EL1004", "EL1008", "EL1014", "EL1018", "EL2004", "EL2008", "EP2338-0002", "EP2338-1002"} {
		f, ok := reg.Lookup(&ecat.Slave{Vendor: VendorBeckhoff, Product: 0xbad, Model: model})
		if !ok {
			t.Errorf("no fallback for %s", model)
			continue
		}
		if _, isDIO := f().(*DigitalIO); !isDIO {
			t.Errorf("%s fallback is %T", model, f())
		}
	}
	if _, ok := reg.Lookup(&ecat.Slave{Vendor: VendorBeckhoff, Product: 0xbad, Model: "EL1009"}); ok {
		t.Error("EL1009 should not match")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
