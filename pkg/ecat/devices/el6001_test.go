package devices

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

const (
	el6001Split = "DEMO.A3.01 (EL6001)" // COM Inputs/Outputs: split Status__*/Ctrl__* bits
	el6001Word  = "DEMO.A3.02 (EL6001)" // Inputs/Outputs: 16-bit Status/Ctrl words
)

func serialNet(t *testing.T) (*ecat.Topology, *ecat.Network) {
	t.Helper()
	topo, err := ecat.LoadProject(fixture("Demo Serial.xml"))
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

func el6001Dev(t *testing.T, n *ecat.Network, name string) *EL6001 {
	t.Helper()
	d, ok := n.DeviceByName(name)
	if !ok {
		t.Fatalf("no device %s", name)
	}
	e, ok := d.(*EL6001)
	if !ok {
		t.Fatalf("%s is %T", name, d)
	}
	return e
}

// serialSlots is the PLC's view of one EL6001: master image slots for the
// control/status bits and data bytes, for either PDO layout.
type serialSlots struct {
	img          *ecat.Image
	split        bool
	ctrl, status ecat.Slot
	cbits, sbits map[string]ecat.Slot
	dout, din    []ecat.Slot
}

func newSerialSlots(t *testing.T, topo *ecat.Topology, n *ecat.Network, slave string) *serialSlots {
	t.Helper()
	s := &serialSlots{img: n.Images().Get(dev1), split: slave == el6001Split, cbits: map[string]ecat.Slot{}, sbits: map[string]ecat.Slot{}}
	inPdo, outPdo := "Inputs", "Outputs"
	if s.split {
		inPdo, outPdo = "COM Inputs", "COM Outputs"
		for _, k := range []string{"Transmit request", "Receive accepted", "Init request", "Output length"} {
			s.cbits[k] = slotOf(t, topo, slave, outPdo, "Ctrl__"+k)
		}
		for _, k := range []string{"Transmit accepted", "Receive request", "Init accepted", "Buffer full", "Parity error", "Framing error", "Overrun error", "Input length"} {
			s.sbits[k] = slotOf(t, topo, slave, inPdo, "Status__"+k)
		}
	} else {
		s.ctrl = slotOf(t, topo, slave, outPdo, "Ctrl")
		s.status = slotOf(t, topo, slave, inPdo, "Status")
	}
	for i := 0; i < 22; i++ {
		s.dout = append(s.dout, slotOf(t, topo, slave, outPdo, "Data Out "+strconv.Itoa(i)))
	}
	// COM Inputs maps Data In 0..21 after the split status bits.
	for i := 0; i < 22; i++ {
		s.din = append(s.din, slotOf(t, topo, slave, inPdo, "Data In "+strconv.Itoa(i)))
	}
	return s
}

var ctrlBit = map[string]int{"Transmit request": 0, "Receive accepted": 1, "Init request": 2}
var statusBit = map[string]int{"Transmit accepted": 0, "Receive request": 1, "Init accepted": 2, "Buffer full": 3, "Parity error": 4, "Framing error": 5, "Overrun error": 6}

func (s *serialSlots) setCtrl(name string, v uint64) {
	if s.split {
		sl := s.cbits[name]
		ecat.WriteBits(s.img.Out, sl.Byte, sl.Bit, sl.BitLen, v)
		return
	}
	w := readSlot(s.img.Out, s.ctrl)
	if name == "Output length" {
		w = w&0x00ff | (v&0xff)<<8
	} else {
		b := uint64(1) << uint(ctrlBit[name])
		w &^= b
		if v != 0 {
			w |= b
		}
	}
	ecat.WriteBits(s.img.Out, s.ctrl.Byte, s.ctrl.Bit, s.ctrl.BitLen, w)
}

func (s *serialSlots) getCtrl(name string) uint64 {
	if s.split {
		return readSlot(s.img.Out, s.cbits[name])
	}
	w := readSlot(s.img.Out, s.ctrl)
	if name == "Output length" {
		return w >> 8 & 0xff
	}
	return w >> uint(ctrlBit[name]) & 1
}

func (s *serialSlots) stat(name string) uint64 {
	if s.split {
		return readSlot(s.img.In, s.sbits[name])
	}
	w := readSlot(s.img.In, s.status)
	if name == "Input length" {
		return w >> 8 & 0xff
	}
	return w >> uint(statusBit[name]) & 1
}

func (s *serialSlots) toggle(name string) { s.setCtrl(name, s.getCtrl(name)^1) }

func (s *serialSlots) writeData(b []byte) {
	for i, c := range b {
		sl := s.dout[i]
		ecat.WriteBits(s.img.Out, sl.Byte, sl.Bit, sl.BitLen, uint64(c))
	}
}

func (s *serialSlots) readData(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(readSlot(s.img.In, s.din[i]))
	}
	return out
}

// recordPeer records writes and returns queued replies on Read.
type recordPeer struct {
	sent  []byte
	reply []byte
}

func (p *recordPeer) Write(tx []byte) { p.sent = append(p.sent, tx...) }
func (p *recordPeer) Read() []byte {
	r := p.reply
	p.reply = nil
	return r
}

func forBothLayouts(t *testing.T, f func(t *testing.T, topo *ecat.Topology, n *ecat.Network, d *EL6001, s *serialSlots)) {
	for _, name := range []string{el6001Split, el6001Word} {
		t.Run(name, func(t *testing.T) {
			topo, n := serialNet(t)
			d := el6001Dev(t, n, name)
			f(t, topo, n, d, newSerialSlots(t, topo, n, name))
		})
	}
}

func TestEL6001InitHandshake(t *testing.T) {
	forBothLayouts(t, func(t *testing.T, _ *ecat.Topology, n *ecat.Network, d *EL6001, s *serialSlots) {
		if d.DataLen() != 22 {
			t.Fatalf("data length %d, want 22", d.DataLen())
		}
		s.setCtrl("Init request", 1)
		n.Step(0)
		if s.stat("Init accepted") != 1 {
			t.Fatal("InitAccepted not set")
		}
		s.setCtrl("Init request", 0)
		n.Step(0)
		if s.stat("Init accepted") != 0 || s.stat("Transmit accepted") != 0 || s.stat("Receive request") != 0 {
			t.Fatal("terminal not ready after init")
		}
	})
}

func TestEL6001TransmitToggle(t *testing.T) {
	forBothLayouts(t, func(t *testing.T, _ *ecat.Topology, n *ecat.Network, d *EL6001, s *serialSlots) {
		p := &recordPeer{}
		d.SetPeer(p)
		if d.Peer() != p {
			t.Fatal("Peer() mismatch")
		}
		s.writeData([]byte("md\r"))
		s.setCtrl("Output length", 3)
		s.toggle("Transmit request")
		n.Step(0)
		if s.stat("Transmit accepted") != s.getCtrl("Transmit request") || string(p.sent) != "md\r" {
			t.Fatalf("TA %d TR %d sent %q", s.stat("Transmit accepted"), s.getCtrl("Transmit request"), p.sent)
		}
		// No toggle: no second transmission.
		n.Step(0)
		n.Step(0)
		if string(p.sent) != "md\r" {
			t.Fatalf("resent without toggle: %q", p.sent)
		}
		// Toggling again with the same data is a new request, acknowledged once.
		s.toggle("Transmit request")
		n.Step(0)
		n.Step(0)
		if s.stat("Transmit accepted") != s.getCtrl("Transmit request") || string(p.sent) != "md\rmd\r" {
			t.Fatalf("second toggle: sent %q", p.sent)
		}
		// OutputLength beyond the data bytes is clamped (T-25-07).
		s.writeData(bytes.Repeat([]byte{'x'}, 22))
		s.setCtrl("Output length", 200)
		s.toggle("Transmit request")
		n.Step(0)
		if len(p.sent) != 6+22 {
			t.Fatalf("clamp: sent %d bytes", len(p.sent))
		}
	})
}

func TestEL6001ChunkedReceive(t *testing.T) {
	forBothLayouts(t, func(t *testing.T, _ *ecat.Topology, n *ecat.Network, d *EL6001, s *serialSlots) {
		msg := []byte(BaaderMt1Reply) // 60 bytes
		d.SetPeer(&recordPeer{reply: msg})
		var got []byte
		for i, want := range []int{22, 22, 16} {
			rr := s.stat("Receive request")
			n.Step(0)
			if s.stat("Receive request") == rr {
				t.Fatalf("chunk %d: ReceiveRequest not toggled", i)
			}
			if il := int(s.stat("Input length")); il != want {
				t.Fatalf("chunk %d: InputLength %d, want %d", i, il, want)
			}
			got = append(got, s.readData(want)...)
			// Without the PLC's ReceiveAccepted toggle no new data appears.
			rr = s.stat("Receive request")
			n.Step(0)
			if s.stat("Receive request") != rr {
				t.Fatalf("chunk %d: new data before ReceiveAccepted", i)
			}
			s.toggle("Receive accepted")
		}
		if !bytes.Equal(got, msg) {
			t.Fatalf("received %q", got)
		}
		rr := s.stat("Receive request")
		n.Step(0)
		if s.stat("Receive request") != rr {
			t.Fatal("ReceiveRequest toggled with an empty FIFO")
		}
	})
}

func TestEL6001ErrorsAndOverrun(t *testing.T) {
	forBothLayouts(t, func(t *testing.T, _ *ecat.Topology, n *ecat.Network, d *EL6001, s *serialSlots) {
		d.SetErrors(true, false, true)
		n.Step(0)
		if s.stat("Parity error") != 1 || s.stat("Framing error") != 0 || s.stat("Overrun error") != 1 {
			t.Fatal("SetErrors bits")
		}
		d.SetErrors(false, true, false)
		n.Step(0)
		if s.stat("Parity error") != 0 || s.stat("Framing error") != 1 || s.stat("Overrun error") != 0 {
			t.Fatal("SetErrors bits after change")
		}
		d.SetErrors(false, false, false)

		// An oversized reply fills the 128-byte FIFO; the excess is dropped (T-25-08).
		d.SetPeer(&recordPeer{reply: bytes.Repeat([]byte{'a'}, 200)})
		n.Step(0)
		if s.stat("Overrun error") != 1 {
			t.Fatal("overrun not reported")
		}
		if s.stat("Buffer full") != 0 {
			t.Fatal("buffer full after the first chunk left the FIFO")
		}
		total := int(s.stat("Input length"))
		for i := 0; i < 10; i++ {
			rr := s.stat("Receive request")
			s.toggle("Receive accepted")
			n.Step(0)
			if s.stat("Receive request") != rr {
				total += int(s.stat("Input length"))
			}
		}
		if total != el6001FIFO {
			t.Fatalf("received %d bytes, want %d", total, el6001FIFO)
		}
		// Init clears the FIFO and the latched overrun.
		d.SetPeer(&recordPeer{reply: bytes.Repeat([]byte{'b'}, 140)})
		s.setCtrl("Init request", 0)
		n.Step(0)
		if s.stat("Overrun error") != 1 {
			t.Fatal("second overrun not reported")
		}
		s.setCtrl("Init request", 1)
		n.Step(0)
		s.setCtrl("Init request", 0)
		n.Step(0)
		if s.stat("Overrun error") != 0 || s.stat("Input length") != 0 {
			t.Fatal("init did not clear overrun and FIFO")
		}
	})
}

func TestEL6001BufferFull(t *testing.T) {
	topo, n := serialNet(t)
	d := el6001Dev(t, n, el6001Word)
	s := newSerialSlots(t, topo, n, el6001Word)
	p := &recordPeer{}
	d.SetPeer(p)
	// The PLC never acknowledges, so the FIFO fills behind the presented chunk.
	for i := 0; i < 8; i++ {
		p.reply = bytes.Repeat([]byte{'c'}, 30)
		n.Step(0)
	}
	if s.stat("Buffer full") != 1 || s.stat("Overrun error") != 1 {
		t.Fatalf("BufferFull %d Overrun %d", s.stat("Buffer full"), s.stat("Overrun error"))
	}
}

func TestEL6001NilPeerDropsBytes(t *testing.T) {
	topo, n := serialNet(t)
	_ = el6001Dev(t, n, el6001Split)
	s := newSerialSlots(t, topo, n, el6001Split)
	s.writeData([]byte("md\r"))
	s.setCtrl("Output length", 3)
	s.toggle("Transmit request")
	n.Step(0)
	if s.stat("Transmit accepted") != 1 {
		t.Fatal("transmit not acknowledged without a peer")
	}
}

func TestEL6001SmallImageLayout(t *testing.T) {
	// 5-byte process image: 8-bit Ctrl/Status with lengths in bits 4-6.
	var fields []ecat.Field
	fields = append(fields, ecat.Field{Pdo: "Outputs", Entry: "Ctrl", Dir: ecat.DirOut, Bit: 0, BitLen: 8})
	for i := 0; i < 5; i++ {
		fields = append(fields, ecat.Field{Pdo: "Outputs", Entry: "Data Out " + strconv.Itoa(4-i), Dir: ecat.DirOut, Bit: 8 + 8*(4-i), BitLen: 8})
	}
	fields = append(fields, ecat.Field{Pdo: "Inputs", Entry: "Status", Dir: ecat.DirIn, Bit: 0, BitLen: 8})
	for i := 0; i < 5; i++ {
		fields = append(fields, ecat.Field{Pdo: "Inputs", Entry: "Data In " + strconv.Itoa(i), Dir: ecat.DirIn, Bit: 8 + 8*i, BitLen: 8})
	}
	fields = append(fields, ecat.Field{Pdo: "Inputs", Entry: "Data In x", Dir: ecat.DirIn, Bit: 48, BitLen: 8})
	d := &EL6001{}
	d.Init(&ecat.Slave{Name: "small"})
	d.Bind(ecat.NewLayout(fields))
	if d.DataLen() != 5 {
		t.Fatalf("data length %d", d.DataLen())
	}
	p := &LoopbackPeer{}
	d.SetPeer(p)
	d.SetErrors(true, true, true) // not representable in the small image
	out, in := make([]byte, 6), make([]byte, 6)
	copy(out[1:], "hello")
	out[0] = 1 | 7<<4 // TR toggle, OutputLength 7 clamps to 5
	d.Step(0, out, in)
	if in[0]&1 != 1 || in[0]&2 != 2 || in[0]>>4&7 != 5 || string(in[1:6]) != "hello" {
		t.Fatalf("small image status %08b data %q", in[0], in[1:6])
	}
}

func TestEL6001Registration(t *testing.T) {
	reg := ecat.NewRegistry()
	Register(reg)
	for _, s := range []*ecat.Slave{
		{Vendor: VendorBeckhoff, HasVendor: true, Product: ProductEL6001, HasProduct: true, Model: "EL6001"},
		{Vendor: VendorBeckhoff, HasVendor: true, Product: 1, HasProduct: true, Model: "EL6002"},
	} {
		if _, ok := reg.New(s).(*EL6001); !ok {
			t.Errorf("%s not an EL6001 model", s.Model)
		}
	}
}

func TestEL6001PartialLayouts(t *testing.T) {
	// 8-bit Status with more data bytes than its 3-bit length can count:
	// chunks are capped at 7 bytes.
	var fields []ecat.Field
	fields = append(fields, ecat.Field{Pdo: "Inputs", Entry: "Status", Dir: ecat.DirIn, Bit: 0, BitLen: 8})
	for i := 0; i < 10; i++ {
		fields = append(fields, ecat.Field{Pdo: "Inputs", Entry: "Data In " + strconv.Itoa(i), Dir: ecat.DirIn, Bit: 8 + 8*i, BitLen: 8})
	}
	// Split control with only Transmit request mapped: the rest read as 0.
	fields = append(fields, ecat.Field{Pdo: "COM Outputs", Entry: "Ctrl__Transmit request", Dir: ecat.DirOut, Bit: 0, BitLen: 1})
	d := &EL6001{}
	d.Init(&ecat.Slave{Name: "partial"})
	d.Bind(ecat.NewLayout(fields))
	d.SetPeer(&recordPeer{reply: []byte("0123456789")})
	out, in := make([]byte, 1), make([]byte, 11)
	d.Step(0, out, in)
	if in[0]>>4&7 != 7 || string(in[1:8]) != "0123456" || in[8] != 0 {
		t.Fatalf("status %08b data %q", in[0], in[1:11])
	}
	out[0] = 1 // TR toggle with no Output length entry: acknowledged, nothing sent
	d.Step(0, out, in)
	if in[0]&1 != 1 {
		t.Fatal("transmit toggle not acknowledged")
	}
}
