package ecat

import (
	"testing"
	"time"
)

// slotRec records its entry layout and the views it is stepped with.
type slotRec struct {
	inits   int
	layouts int
	in, out []EntrySlot
	vin     []byte
	vout    []byte
}

func (d *slotRec) Init(*Slave) { d.inits++ }
func (d *slotRec) SetLayout(in, out []EntrySlot) {
	if d.inits != 1 {
		panic("SetLayout before Init")
	}
	d.layouts++
	d.in, d.out = in, out
}
func (d *slotRec) Step(_ time.Duration, out, in []byte) { d.vout, d.vin = out, in }

const (
	atvVendor  = 0x0800005A
	atvProduct = 0x389
)

func atvRecorders(t *testing.T) (*Topology, *Network, []*slotRec) {
	t.Helper()
	var recs []*slotRec
	reg := NewRegistry()
	reg.Register(atvVendor, atvProduct, func() Device {
		r := &slotRec{}
		recs = append(recs, r)
		return r
	})
	topo, n := loadNet(t, reg)
	if len(recs) != 2 {
		t.Fatalf("got %d ATV320 devices, want 2", len(recs))
	}
	return topo, n, recs
}

func TestLayoutDemoATV320(t *testing.T) {
	topo, n, recs := atvRecorders(t)
	d := recs[0]
	var fd01 string
	for _, s := range topo.Master(dev1).Slaves {
		if s.Vendor == atvVendor {
			fd01 = SlaveBasePath(dev1, s)
		}
	}
	if d.layouts != 1 {
		t.Fatalf("SetLayout called %d times", d.layouts)
	}
	want := []struct {
		list []EntrySlot
		name string
		dir  Dir
		b    int
		idx  uint16
		sub  uint8
	}{
		{d.in, "ETA", DirIn, 0, 0x2002, 1},
		{d.in, "RFR", DirIn, 2, 0x2002, 2},
		{d.out, "CMD", DirOut, 0, 0x2003, 1},
		{d.out, "LFR", DirOut, 2, 0x2003, 2},
	}
	if len(d.in) != 2 || len(d.out) != 2 {
		t.Fatalf("in=%+v out=%+v", d.in, d.out)
	}
	for _, w := range want {
		e, ok := FindEntry(w.list, w.name)
		if !ok || e.Dir != w.dir || e.Byte != w.b || e.Bit != 0 || e.BitLen != 16 ||
			e.Index != w.idx || e.SubIndex != w.sub {
			t.Errorf("%s = %+v ok=%v", w.name, e, ok)
		}
	}
	if e, ok := FindEntry(d.in, "eta"); !ok || e.Name != "ETA" {
		t.Error("FindEntry is not case-insensitive")
	}
	if _, ok := FindEntry(d.in, "LCR"); ok {
		t.Error("unexpected LCR")
	}

	// Values written through the slots land at the topology paths and back.
	n.Step(0)
	eta, _ := FindEntry(d.in, "ETA")
	rfr, _ := FindEntry(d.in, "RFR")
	eta.Set(d.vin, 0x0237)
	rfr.Set(d.vin, 0xFE0C)
	if got := readPath(t, topo, n, fd01+"^Inputs^ETA"); got != 0x0237 {
		t.Errorf("ETA path = %#x", got)
	}
	if got := readPath(t, topo, n, fd01+"^Inputs^RFR"); got != 0xFE0C {
		t.Errorf("RFR path = %#x", got)
	}
	writePath(t, topo, n, fd01+"^Outputs^LFR", 500)
	writePath(t, topo, n, fd01+"^Outputs^CMD", 0x0F)
	n.Step(0)
	cmd, _ := FindEntry(d.out, "CMD")
	lfr, _ := FindEntry(d.out, "LFR")
	if cmd.Get(d.vout) != 0x0F || lfr.Get(d.vout) != 500 {
		t.Errorf("CMD=%#x LFR=%d", cmd.Get(d.vout), lfr.Get(d.vout))
	}
}

// Demo Device 2 has a ProcessImage that places CMD away from the packed
// position; slots must follow the master's real layout.
func TestLayoutFollowsProcessImage(t *testing.T) {
	topo, n, recs := atvRecorders(t)
	d := recs[1]
	m := topo.Master("Device 2 (EtherCAT)")
	var s *Slave
	for _, x := range m.Slaves {
		if x.Vendor == atvVendor {
			s = x
		}
	}
	in, out := slaveSpans(m, s)
	for _, list := range [][]EntrySlot{d.in, d.out} {
		for _, e := range list {
			var p Pdo
			for _, q := range s.Pdos {
				if q.Dir == e.Dir {
					p = q
				}
			}
			var ent Entry
			for _, x := range p.Entries {
				if x.Name == e.Name {
					ent = x
				}
			}
			slot, _ := m.Slot(LinkPath(m.Name, s, p, ent))
			lo := in.lo
			if e.Dir == DirOut {
				lo = out.lo
			}
			if e.Byte != slot.Byte-lo || e.Bit != slot.Bit || e.BitLen != slot.BitLen {
				t.Errorf("%s = %+v, slot %+v lo %d", e.Name, e, slot, lo)
			}
		}
	}
	n.Step(0)
	writePath(t, topo, n, SlaveBasePath(m.Name, s)+"^Outputs^LFR", 321)
	n.Step(0)
	lfr, _ := FindEntry(d.out, "LFR")
	if got := lfr.Get(d.vout); got != 321 {
		t.Errorf("LFR via slot = %d", got)
	}
}

func TestParseIndex(t *testing.T) {
	for in, want := range map[string]uint16{
		"#x6041": 0x6041, "#X2002": 0x2002, " 24641 ": 24641, "": 0,
		"#xZZ": 0, "#x10000": 0, "12a": 0,
	} {
		if got := parseIndex(in); got != want {
			t.Errorf("parseIndex(%q) = %#x, want %#x", in, got, want)
		}
	}
}

func TestEntrySlotOutOfRange(t *testing.T) {
	e := EntrySlot{Byte: 4, BitLen: 16}
	buf := make([]byte, 2)
	e.Set(buf, 0xFFFF)
	if e.Get(buf) != 0 || buf[0] != 0 || buf[1] != 0 {
		t.Errorf("out-of-range access touched buf %x", buf)
	}
}
