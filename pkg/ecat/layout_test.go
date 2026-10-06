package ecat

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
)

// recDev records its Layout and the views it is stepped with.
type recDev struct {
	layout  *Layout
	out, in []byte
}

func (d *recDev) Init(*Slave)                                 {}
func (d *recDev) Bind(l *Layout)                              { d.layout = l }
func (d *recDev) Step(_ time.Duration, out []byte, in []byte) { d.out, d.in = out, in }

func dev1Slave(t *testing.T, topo *Topology, model string) (*Master, *Slave) {
	t.Helper()
	m := topo.Master(dev1)
	for _, s := range m.Slaves {
		if s.Model == model {
			return m, s
		}
	}
	t.Fatalf("no slave %s", model)
	return nil, nil
}

func TestLayoutEL1008ByteAligned(t *testing.T) {
	reg := NewRegistry()
	reg.Register(2, 0x03f03052, func() Device { return &recDev{} })
	topo, n := loadNet(t, reg)
	m, s := dev1Slave(t, topo, "EL1008")
	d, ok := n.DeviceByName(s.Name)
	if !ok {
		t.Fatal("DeviceByName miss")
	}
	l := d.(*recDev).layout
	if l == nil {
		t.Fatal("Bind not called")
	}
	in, _ := slaveSpans(m, s)
	for ch := 1; ch <= 8; ch++ {
		pdo := "Channel " + string(rune('0'+ch))
		f, ok := l.Field(pdo, "Input")
		if !ok {
			t.Fatalf("no field %s", pdo)
		}
		slot, _ := m.Slot(LinkPath(m.Name, s, s.Pdos[ch-1], s.Pdos[ch-1].Entries[0]))
		want := (slot.Byte-in.lo)*8 + slot.Bit
		if f.Dir != DirIn || f.Bit != want || f.BitLen != 1 || f.Pdo != pdo || f.Entry != "Input" {
			t.Errorf("%s: got %+v want bit %d", pdo, f, want)
		}
	}
	if len(l.Fields(DirIn)) != 8 || len(l.Fields(DirOut)) != 0 {
		t.Errorf("fields in=%d out=%d", len(l.Fields(DirIn)), len(l.Fields(DirOut)))
	}
	if got := l.FindEntry(DirIn, "Input"); len(got) != 8 || got[2].Pdo != "Channel 3" {
		t.Errorf("FindEntry = %+v", got)
	}
	if _, ok := l.Field("Channel 9", "Input"); ok {
		t.Error("unexpected field")
	}
	// Writing through the field lands at the channel's master-image slot.
	n.Step(0)
	f, _ := l.Field("Channel 3", "Input")
	Put(d.(*recDev).in, f, 1)
	if got := readPath(t, topo, n, LinkPath(m.Name, s, s.Pdos[2], s.Pdos[2].Entries[0])); got != 1 {
		t.Errorf("channel 3 slot = %d", got)
	}
	if Get(d.(*recDev).in, f) != 1 {
		t.Error("Get after Put")
	}
}

func TestLayoutPackedAndPadding(t *testing.T) {
	s := &Slave{Name: "B (EP2338-0002)", Model: "EP2338-0002", Vendor: 2, Product: 0x09224052}
	for i := 1; i <= 8; i++ {
		s.Pdos = append(s.Pdos, Pdo{Name: "Channel " + itoa(i), Index: 0x1a00 + i, Dir: DirIn,
			Entries: []Entry{{Name: "Input", Index: "#x6000", SubIndex: 1, BitLen: 1}}})
	}
	for i := 9; i <= 16; i++ {
		s.Pdos = append(s.Pdos, Pdo{Name: "Channel " + itoa(i), Index: 0x1600 + i, Dir: DirOut,
			Entries: []Entry{{Name: "Output", Index: "#x7000", SubIndex: 1, BitLen: 1}}})
	}
	s.Pdos = append(s.Pdos, Pdo{Name: "Status", Index: 0x1a20, Dir: DirIn, Entries: []Entry{
		{Name: "Status__Tripped", Index: "#x6010", SubIndex: 1, BitLen: 1},
		{Name: "", Index: "#x0", BitLen: 7},
		{Name: "Value", Index: "#x6010", SubIndex: 2, BitLen: 16, DataType: "INT"},
	}})
	m := &Master{Name: "M", Slaves: []*Slave{s}}
	buildLayout(m, nil)
	topo := &Topology{Masters: []*Master{m}}
	reg := NewRegistry()
	reg.RegisterModel(2, regexp.MustCompile(`^EP2338`), func() Device { return &recDev{} })
	n := NewNetwork(topo, reg)
	if len(n.Diagnostics()) != 0 {
		t.Fatalf("diags: %v", n.Diagnostics())
	}
	d, err := n.Device("M", 0)
	if err != nil {
		t.Fatal(err)
	}
	l := d.(*recDev).layout
	ins, outs := l.Fields(DirIn), l.Fields(DirOut)
	if len(ins) != 10 || len(outs) != 8 {
		t.Fatalf("in=%d out=%d", len(ins), len(outs))
	}
	for i, f := range outs {
		if f.Bit != i || f.Pdo != "Channel "+itoa(9+i) {
			t.Errorf("out %d = %+v", i, f)
		}
	}
	// Each PDO is byte-aligned in the Phase 24 layout, so packed bits of
	// one PDO share a byte while separate PDOs start a new byte.
	for i := 0; i < 8; i++ {
		if ins[i].Bit != i*8 {
			t.Errorf("in %d bit = %d", i, ins[i].Bit)
		}
	}
	if f, ok := l.Field("Status", "Status__Tripped"); !ok || f.Bit != 64 {
		t.Errorf("tripped %+v", f)
	}
	if f, ok := l.Field("Status", "Value"); !ok || f.Bit != 72 || f.BitLen != 16 || f.DataType != "INT" {
		t.Errorf("value %+v", f)
	}
	n.Step(0)
	v, _ := l.Field("Status", "Value")
	Put(d.(*recDev).in, v, 0xBEEF)
	if Get(d.(*recDev).in, v) != 0xBEEF {
		t.Error("16-bit roundtrip")
	}
}

func TestLayoutBoundsAndNil(t *testing.T) {
	f := Field{Bit: 9, BitLen: 8}
	Put(nil, f, 0xff) // must not panic
	if Get(nil, f) != 0 || Get([]byte{0xff}, f) != 0 {
		t.Error("out-of-range Get")
	}
	buf := []byte{0, 0}
	Put(buf, Field{Bit: -1, BitLen: 1}, 1)
	Put(buf, Field{Bit: 0, BitLen: 0}, 1)
	if buf[0] != 0 || buf[1] != 0 {
		t.Error("bad field wrote")
	}
	var l *Layout
	if _, ok := l.Field("a", "b"); ok || l.Fields(DirIn) != nil || l.FindEntry(DirIn, "x") != nil {
		t.Error("nil layout")
	}
	nl := NewLayout([]Field{{Pdo: "P", Entry: "E", Dir: DirOut, Bit: 3, BitLen: 1}, {Pdo: "P", Entry: "E", Bit: 9}})
	if f, ok := nl.Field("P", "E"); !ok || f.Bit != 3 {
		t.Errorf("first wins: %+v", f)
	}
	if len(nl.Fields(DirOut)) != 1 || len(nl.FindEntry(DirIn, "E")) != 1 {
		t.Error("NewLayout fields")
	}
}

func TestRegistryLookup(t *testing.T) {
	r := NewRegistry()
	el := &Slave{Model: "EL1008", Vendor: 2, Product: 0xdead}
	if _, ok := r.Lookup(el); ok {
		t.Error("empty registry hit")
	}
	if _, ok := r.New(el).(Passthrough); !ok {
		t.Error("New miss != Passthrough")
	}
	r.RegisterModel(2, regexp.MustCompile(`^EL100[48]$`), func() Device { return &recDev{} })
	r.RegisterModel(2, regexp.MustCompile(`^EL`), func() Device { return Passthrough{} })
	if f, ok := r.Lookup(el); !ok {
		t.Error("pattern miss")
	} else if _, isRec := f().(*recDev); !isRec {
		t.Error("first pattern should win")
	}
	if _, ok := r.Lookup(&Slave{Model: "EL1008", Vendor: 3}); ok {
		t.Error("pattern must be vendor scoped")
	}
	r.Register(2, 0xdead, func() Device { return Passthrough{} })
	if _, ok := r.New(el).(Passthrough); !ok {
		t.Error("exact match must win over pattern")
	}
	r.RegisterPassive(7, 9)
	if f, ok := r.Lookup(&Slave{Vendor: 7, Product: 9}); !ok {
		t.Error("passive miss")
	} else if _, ok := f().(Passthrough); !ok {
		t.Error("passive not Passthrough")
	}
}

func TestNetworkUnknownDeviceDiagnostics(t *testing.T) {
	topo, n := loadNet(t, NewRegistry())
	total := 0
	for _, m := range topo.Masters {
		total += len(m.Slaves)
	}
	diags := n.Diagnostics()
	if len(diags) != total {
		t.Fatalf("got %d diags for %d slaves", len(diags), total)
	}
	for _, d := range diags {
		if d.Code != CodeNoModel || d.Severity != diag.Warning || !strings.Contains(d.Message, "using passthrough") || d.Pos.File == "" {
			t.Errorf("diag %+v", d)
		}
	}
	full := NewRegistry()
	for _, m := range topo.Masters {
		for _, s := range m.Slaves {
			full.RegisterPassive(s.Vendor, s.Product)
		}
	}
	if d := NewNetwork(topo, full).Diagnostics(); len(d) != 0 {
		t.Errorf("full registry diags: %v", d)
	}
}

func TestNetworkDeviceAccessors(t *testing.T) {
	_, n := loadNet(t, NewRegistry())
	if d, err := n.Device(dev1, 1); err != nil || d == nil {
		t.Errorf("Device = %v, %v", d, err)
	}
	if _, err := n.Device(dev1, 99); err == nil {
		t.Error("bad index accepted")
	}
	if _, err := n.Device("nope", 0); err == nil {
		t.Error("bad master accepted")
	}
	if _, ok := n.DeviceByName("nope"); ok {
		t.Error("bad name found")
	}
	if _, ok := n.DeviceByName("DEMO.A1.01 (EL1008)"); !ok {
		t.Error("EL1008 not found by name")
	}
	if l, err := n.Layout(dev1, 1); err != nil || len(l.Fields(DirIn)) != 8 {
		t.Errorf("Layout = %v, %v", l, err)
	}
	if _, err := n.Layout(dev1, -1); err == nil {
		t.Error("Layout bad index accepted")
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
