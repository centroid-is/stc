package ecat

import (
	"reflect"
	"sort"
	"testing"
)

const (
	d1     = "TIID^Device 1 (EtherCAT)^"
	d1ebus = d1 + "DEMO.A1.00 (EK1200)^"
	d2     = "TIID^Device 2 (EtherCAT)^"
)

type wantSlot struct {
	path              string
	dir               Dir
	byte, bit, bitLen int
}

func checkSlots(t *testing.T, topo *Topology, cases []wantSlot) {
	t.Helper()
	for _, c := range cases {
		s, ok := topo.Slot(c.path)
		if !ok {
			t.Errorf("Slot(%q) not found", c.path)
			continue
		}
		if s.Dir != c.dir || s.Byte != c.byte || s.Bit != c.bit || s.BitLen != c.bitLen || s.Path != c.path {
			t.Errorf("Slot(%q) = %s byte %d bit %d len %d, want %s byte %d bit %d len %d",
				c.path, s.Dir, s.Byte, s.Bit, s.BitLen, c.dir, c.byte, c.bit, c.bitLen)
		}
	}
}

func TestLayoutDemo1Computed(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	checkSlots(t, topo, []wantSlot{
		{d1ebus + "DEMO.A1.01 (EL1008)^Channel 1^Input", DirIn, 0, 0, 1},
		{d1ebus + "DEMO.A1.01 (EL1008)^Channel 3^Input", DirIn, 2, 0, 1},
		{d1ebus + "DEMO.A1.01 (EL1008)^Channel 8^Input", DirIn, 7, 0, 1},
		{d1ebus + "DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Status^Enabled", DirIn, 8, 0, 1},
		{d1ebus + "DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Status^Tripped", DirIn, 8, 1, 1},
		{d1ebus + "DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Current", DirIn, 10, 0, 16},
		{d1ebus + "DEMO.A1.04 (EL2912)^Module 3 (DEVICEIO)^FIELDVOLTAGE Field Voltage Status^Fieldvoltage Overrange", DirIn, 12, 1, 1},
		{d1 + "DEMO.CN01.FD01 (ATV320 EtherCAT)^Inputs^RFR", DirIn, 15, 0, 16},
		{d1 + "DEMO.T1 (PS2001-2410)^PSU Inputs Device^Input undervoltage", DirIn, 18, 0, 1},
		{d1ebus + "DEMO.A1.02 (EL2008)^Channel 1^Output", DirOut, 0, 0, 1},
		{d1ebus + "DEMO.A1.03 (EL9222-5500)^OCP Outputs Channel 1^Control^Reset", DirOut, 8, 0, 1},
		{d1 + "DEMO.CN01.FD01 (ATV320 EtherCAT)^Outputs^LFR", DirOut, 12, 0, 16},
		{d1 + "DEMO.V1 (CTEU-EtherCAT Modular)^Module 1 (VAEM-L1-S-8-PT [16DO])^Outputs^C2 Output", DirOut, 16, 0, 8},
		// Slave pseudo-inputs follow all PDO data, 11 bytes per slave.
		{d1 + "DEMO.A1.00 (EK1200)^WcState^WcState", DirIn, 19, 0, 1},
		{d1ebus + "DEMO.A1.01 (EL1008)^WcState^WcState", DirIn, 30, 0, 1},
		{d1ebus + "DEMO.A1.01 (EL1008)^InfoData^State", DirIn, 31, 0, 16},
		{d1ebus + "DEMO.A1.01 (EL1008)^InfoData^AdsAddr", DirIn, 33, 0, 64},
		// Master pseudo-inputs at the end.
		{d1 + "Inputs^DevState", DirIn, 129, 0, 16},
		{d1 + "Inputs^SlaveCount", DirIn, 131, 0, 16},
		{d1 + "Inputs^Frm0State", DirIn, 133, 0, 16},
		{d1 + "Inputs^Frm0WcState", DirIn, 135, 0, 16},
		{d1 + "InfoData^AmsNetId", DirIn, 137, 0, 48},
		{d1 + "InfoData^ChangeCount", DirIn, 143, 0, 16},
	})
	m := topo.Masters[0]
	if m.InBytes != 145 || m.OutBytes != 17 {
		t.Errorf("image sizes in=%d out=%d, want 145/17", m.InBytes, m.OutBytes)
	}
	for _, s := range m.Slots() {
		if s.Master != m.Name {
			t.Errorf("slot %q master = %q", s.Path, s.Master)
		}
	}
	if s, _ := m.Slot(d1 + "InfoData^AmsNetId"); s.DataType != "AMSNETID" {
		t.Errorf("AmsNetId DataType = %q", s.DataType)
	}
	if s, _ := m.Slot(d1ebus + "DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Current"); s.DataType != "UINT" {
		t.Errorf("Current DataType = %q", s.DataType)
	}
	// Padding and unassigned PDOs have no slot.
	for _, p := range topo.Paths() {
		if p == d1ebus+"DEMO.A1.03 (EL9222-5500)^Unassigned^Spare" || p == d1ebus+"DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^" {
			t.Errorf("unexpected slot %q", p)
		}
	}
}

func TestLayoutDemo2ProcessImage(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 2.xml"))
	if err != nil {
		t.Fatal(err)
	}
	checkSlots(t, topo, []wantSlot{
		{d2 + "D2.A1.01 (EL1008)^Channel 1^Input", DirIn, 25, 0, 1},
		{d2 + "D2.A1.01 (EL1008)^WcState^WcState", DirIn, 125, 0, 1},
		{d2 + "Inputs^DevState", DirIn, 150, 0, 16},
		{d2 + "InfoData^AmsNetId", DirIn, 162, 4, 48},
		{d2 + "D2.V1 (ATV320 EtherCAT)^Outputs^CMD", DirOut, 8, 0, 16},
		// Not in the ProcessImage: appended after max(ByteSize, PI end).
		{d2 + "D2.V1 (ATV320 EtherCAT)^Inputs^ETA", DirIn, 172, 0, 16},
		{d2 + "D2.V1 (ATV320 EtherCAT)^Outputs^LFR", DirOut, 12, 0, 16},
		{d2 + "D2.A1 (EK1100)^WcState^WcState", DirIn, 176, 0, 1},
		{d2 + "Inputs^SlaveCount", DirIn, 219, 0, 16},
	})
	m := topo.Masters[0]
	if m.InBytes != 227 || m.OutBytes != 14 {
		t.Errorf("image sizes in=%d out=%d, want 227/14", m.InBytes, m.OutBytes)
	}
	if m.InBytes < 172 || m.OutBytes < 4 {
		t.Errorf("image smaller than ProcessImage ByteSize")
	}
}

func TestLayoutGroupedEntriesUseParentVariable(t *testing.T) {
	// TwinCAT's ProcessImage lists "Status__X" entries as one "Status" struct.
	s := &Slave{Name: "S (EL9222-5500)", Pdos: []Pdo{{Name: "P", Dir: DirIn, Entries: []Entry{
		{Name: "Current", Index: "#x6000", BitLen: 16},
		{Name: "Status__Enabled", Index: "#x6000", BitLen: 1},
		{Name: "Status__", Index: "#x0", BitLen: 2},
		{Name: "Status__Diag", Index: "#x6000", BitLen: 1},
	}}}}
	m := &Master{Name: "M", Slaves: []*Slave{s}}
	buildLayout(m, &processImage{Inputs: &piArea{ByteSize: 2, Variables: []piVariable{
		{Name: "S (EL9222-5500).P.Status", DataType: "Status_X", BitSize: 16, BitOffs: 80},
	}}})
	cases := map[string][3]int{
		"TIID^M^S (EL9222-5500)^P^Status^Enabled": {10, 0, 1},
		"TIID^M^S (EL9222-5500)^P^Status^Diag":    {10, 3, 1},
		"TIID^M^S (EL9222-5500)^P^Current":        {12, 0, 16},
	}
	for p, w := range cases {
		got, ok := m.Slot(p)
		if !ok || got.Byte != w[0] || got.Bit != w[1] || got.BitLen != w[2] {
			t.Errorf("Slot(%q) = %+v %v, want %v", p, got, ok, w)
		}
	}
}

func TestTopologySlotLookup(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 2.xml"))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := topo.Slot(d1ebus + "DEMO.A1.01 (EL1008)^Channel 3^Input")
	if !ok || s.Master != "Device 1 (EtherCAT)" || s.Dir != DirIn || s.Byte != 2 || s.Bit != 0 || s.BitLen != 1 {
		t.Errorf("Channel 3 slot = %+v %v", s, ok)
	}
	for _, bad := range []string{"", "TIID", "TIID^Device 9 (EtherCAT)^X", d1 + "nope", d2 + "Inputs^Nope"} {
		if _, ok := topo.Slot(bad); ok {
			t.Errorf("Slot(%q) unexpectedly resolved", bad)
		}
	}
	paths := topo.Paths()
	if !sort.StringsAreSorted(paths) || len(paths) == 0 {
		t.Errorf("Paths not sorted")
	}
	again, _ := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 2.xml"))
	for i := range topo.Masters {
		if !reflect.DeepEqual(topo.Masters[i].Slots(), again.Masters[i].Slots()) {
			t.Errorf("layout of %s not deterministic", topo.Masters[i].Name)
		}
	}
	if s, ok := topo.Masters[1].Slot(d1 + "Inputs^DevState"); ok {
		t.Errorf("Device 2 resolved a Device 1 path: %+v", s)
	}
}

func TestImages(t *testing.T) {
	topo, _ := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 2.xml"))
	imgs := NewImages(topo)
	if got := imgs.Masters(); !reflect.DeepEqual(got, []string{"Device 1 (EtherCAT)", "Device 2 (EtherCAT)"}) {
		t.Errorf("Masters = %v", got)
	}
	img := imgs.Get("Device 1 (EtherCAT)")
	if img == nil || len(img.In) != 145 || len(img.Out) != 17 {
		t.Fatalf("image = %+v", img)
	}
	if imgs.Get("nope") != nil {
		t.Errorf("Get(unknown) != nil")
	}
	if DirIn.String() != "in" || DirOut.String() != "out" {
		t.Errorf("Dir.String broken")
	}
}

func TestReadWriteBits(t *testing.T) {
	for _, n := range []int{1, 8, 16, 48, 64} {
		buf := make([]byte, 12)
		for i := range buf {
			buf[i] = 0xA5
		}
		var v uint64 = 0x0123456789ABCDEF
		if n < 64 {
			v &= 1<<uint(n) - 1
		}
		WriteBits(buf, 2, 0, n, v)
		if got := ReadBits(buf, 2, 0, n); got != v {
			t.Errorf("%d-bit round trip = %#x, want %#x", n, got, v)
		}
		if buf[1] != 0xA5 || (n <= 64 && 2+(n+7)/8 < len(buf) && buf[2+(n+7)/8] != 0xA5) {
			t.Errorf("%d-bit write disturbed neighbours: % x", n, buf)
		}
	}
	// Little-endian layout.
	buf := make([]byte, 4)
	WriteBits(buf, 0, 0, 16, 0x1234)
	if buf[0] != 0x34 || buf[1] != 0x12 {
		t.Errorf("not little-endian: % x", buf)
	}
	// Single bits 0..7 leave neighbours alone.
	for bit := 0; bit < 8; bit++ {
		b := []byte{0x00, 0xFF}
		WriteBits(b, 0, bit, 1, 1)
		if b[0] != 1<<uint(bit) || ReadBits(b, 0, bit, 1) != 1 {
			t.Errorf("set bit %d: % x", bit, b)
		}
		WriteBits(b[1:], 0, bit, 1, 0)
		if b[1] != 0xFF&^(1<<uint(bit)) || b[0] != 1<<uint(bit) {
			t.Errorf("clear bit %d: % x", bit, b)
		}
	}
	// Unaligned multi-bit value.
	b := make([]byte, 3)
	WriteBits(b, 0, 4, 12, 0xABC)
	if b[0] != 0xC0 || b[1] != 0xAB || ReadBits(b, 0, 4, 12) != 0xABC {
		t.Errorf("unaligned: % x", b)
	}
}

func TestReadWriteBitsBounds(t *testing.T) {
	buf := []byte{0xFF, 0xFF}
	cases := []struct{ off, bit, n int }{
		{-1, 0, 8}, {0, -1, 1}, {0, 8, 1}, {1, 0, 16}, {2, 0, 1}, {0, 0, 0}, {0, 0, 65},
	}
	for _, c := range cases {
		if got := ReadBits(buf, c.off, c.bit, c.n); got != 0 {
			t.Errorf("ReadBits(%v) = %#x, want 0", c, got)
		}
		WriteBits(buf, c.off, c.bit, c.n, 0)
		if buf[0] != 0xFF || buf[1] != 0xFF {
			t.Errorf("WriteBits(%v) wrote out of range: % x", c, buf)
		}
	}
	if ReadBits(nil, 0, 0, 1) != 0 {
		t.Errorf("nil buffer read")
	}
}
