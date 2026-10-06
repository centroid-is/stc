package devices

import (
	"math"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

const el9222Demo = "DEMO.A1.03 (EL9222-5500)"

func el9222Dev(t *testing.T, n *ecat.Network) *EL9222 {
	t.Helper()
	d, ok := n.DeviceByName(el9222Demo)
	if !ok {
		t.Fatal("no EL9222 device")
	}
	e, ok := d.(*EL9222)
	if !ok {
		t.Fatalf("EL9222 is %T", d)
	}
	return e
}

// Demo Device 1 maps only Enabled, Tripped, Current and Control__Reset for
// channel 1; without a Switch entry the channel counts as switched on.
func TestEL9222DemoTripAndResetPulse(t *testing.T) {
	topo, n := demoNet(t)
	e := el9222Dev(t, n)
	img := n.Images().Get(dev1)
	read := func(entry string) uint64 {
		return readSlot(img.In, slotOf(t, topo, el9222Demo, "OCP Inputs Channel 1", entry))
	}
	reset := slotOf(t, topo, el9222Demo, "OCP Outputs Channel 1", "Control__Reset")
	setReset := func(v uint64) { ecat.WriteBits(img.Out, reset.Byte, reset.Bit, reset.BitLen, v) }

	if err := e.SetLoadCurrent(1, 2.5); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if read("Status__Enabled") != 1 || read("Status__Tripped") != 0 || read("Current") != 250 {
		t.Fatalf("initial: enabled %d tripped %d current %d", read("Status__Enabled"), read("Status__Tripped"), read("Current"))
	}
	if err := e.Trip(1); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if read("Status__Enabled") != 0 || read("Status__Tripped") != 1 || read("Current") != 0 {
		t.Fatal("trip not reported")
	}
	// Reset held high across the trip: no rising edge, still tripped.
	setReset(1)
	n.Step(0)
	setReset(0)
	if err := e.Trip(1); err != nil {
		t.Fatal(err)
	}
	setReset(1)
	n.Step(0)
	n.Step(0)
	if read("Status__Tripped") != 1 {
		// The rising edge happened before the second Trip; holding high must not clear it.
		t.Fatal("holding Reset cleared the trip")
	}
	setReset(0)
	n.Step(0)
	if read("Status__Tripped") != 1 {
		t.Fatal("Reset falling edge cleared the trip")
	}
	setReset(1)
	n.Step(0)
	if read("Status__Tripped") != 0 || read("Status__Enabled") != 1 {
		t.Fatal("Reset rising edge did not recover")
	}
	if !e.Enabled(1) || e.Tripped(1) {
		t.Error("accessors disagree with image")
	}
	// Cool-down blocks re-enable until cleared.
	if err := e.SetCoolDown(1, true); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if read("Status__Enabled") != 0 {
		t.Error("cool-down did not block enable")
	}
	if err := e.SetCoolDown(1, false); err != nil {
		t.Fatal(err)
	}
	n.Step(0)
	if read("Status__Enabled") != 1 {
		t.Error("enable did not return after cool-down cleared")
	}
}

// el9222Full builds the two-channel layout of the real EL9222-5500 export.
func el9222Full() *EL9222 {
	in := []struct {
		name string
		len  int
	}{
		{"Status__Enabled", 1}, {"Status__Tripped", 1}, {"", 2}, {"Status__Hardware Protection", 1}, {"", 2},
		{"Status__Current Level Warning", 1}, {"Status__Cool Down Lock", 1}, {"", 3}, {"Status__Diag", 1},
		{"Status__TxPDO State", 1}, {"Status__Input cycle counter", 2}, {"", 2}, {"Status__Error", 1},
		{"Status__State Reset", 1}, {"Status__State Switch", 1}, {"", 11},
	}
	var fields []ecat.Field
	for ch := 1; ch <= 2; ch++ {
		bit := (ch - 1) * 32
		for _, e := range in {
			if e.name != "" {
				fields = append(fields, ecat.Field{Pdo: "OCP Inputs Channel " + itoa(ch), Entry: e.name, Dir: ecat.DirIn, Bit: bit, BitLen: e.len})
			}
			bit += e.len
		}
		ob := (ch - 1) * 16
		fields = append(fields,
			ecat.Field{Pdo: "OCP Outputs Channel " + itoa(ch), Entry: "Control__Reset", Dir: ecat.DirOut, Bit: ob, BitLen: 1},
			ecat.Field{Pdo: "OCP Outputs Channel " + itoa(ch), Entry: "Control__Switch", Dir: ecat.DirOut, Bit: ob + 1, BitLen: 1})
	}
	e := &EL9222{}
	e.Init(&ecat.Slave{Name: "F (EL9222-5500)"})
	e.Bind(ecat.NewLayout(fields))
	return e
}

func TestEL9222FullStatusBits(t *testing.T) {
	e := el9222Full()
	in, out := make([]byte, 8), make([]byte, 4)
	st := func(ch int, entry string) uint64 {
		f, _ := e.Layout().Field("OCP Inputs Channel "+itoa(ch), "Status__"+entry)
		return ecat.Get(in, f)
	}
	ctl := func(ch int, reset, sw bool) {
		r, _ := e.Layout().Field("OCP Outputs Channel "+itoa(ch), "Control__Reset")
		s, _ := e.Layout().Field("OCP Outputs Channel "+itoa(ch), "Control__Switch")
		ecat.Put(out, r, b2u(reset))
		ecat.Put(out, s, b2u(sw))
	}
	e.Step(0, out, in)
	if st(1, "Enabled") != 0 {
		t.Error("enabled with Switch off")
	}
	ctl(1, false, true)
	ctl(2, false, true)
	e.Step(0, out, in)
	if st(1, "Enabled") != 1 || st(1, "State Switch") != 1 || st(1, "State Reset") != 0 {
		t.Error("switch on not reflected")
	}
	if err := e.Trip(1); err != nil {
		t.Fatal(err)
	}
	e.Step(0, out, in)
	for entry, want := range map[string]uint64{"Enabled": 0, "Tripped": 1, "Error": 1, "Diag": 1, "TxPDO State": 0} {
		if st(1, entry) != want {
			t.Errorf("tripped: %s = %d", entry, st(1, entry))
		}
	}
	if st(2, "Enabled") != 1 || st(2, "Tripped") != 0 || st(2, "Diag") != 0 {
		t.Error("channel 2 affected by channel 1 trip")
	}
	// Switch toggles alone do not clear the trip.
	ctl(1, false, false)
	e.Step(0, out, in)
	ctl(1, false, true)
	e.Step(0, out, in)
	if st(1, "Tripped") != 1 {
		t.Error("switch toggle cleared the trip")
	}
	// Reset pulse with Switch off: trip clears but stays disabled.
	ctl(1, true, false)
	e.Step(0, out, in)
	if st(1, "Tripped") != 0 || st(1, "Enabled") != 0 || st(1, "State Reset") != 1 {
		t.Error("reset with switch off")
	}
	ctl(1, false, true)
	e.Step(0, out, in)
	if st(1, "Enabled") != 1 {
		t.Error("switch on after reset did not enable")
	}
	// Warning and hardware protection.
	if err := e.SetWarning(2, true); err != nil {
		t.Fatal(err)
	}
	e.Step(0, out, in)
	if st(2, "Current Level Warning") != 1 || st(2, "Diag") != 1 || st(2, "Error") != 0 || st(2, "Enabled") != 1 {
		t.Error("warning bits")
	}
	if err := e.SetHardwareProtection(2, true); err != nil {
		t.Fatal(err)
	}
	if err := e.SetCoolDown(2, true); err != nil {
		t.Fatal(err)
	}
	e.Step(0, out, in)
	if st(2, "Hardware Protection") != 1 || st(2, "Error") != 1 || st(2, "Enabled") != 0 || st(2, "Cool Down Lock") != 1 {
		t.Error("hardware protection bits")
	}
}

func TestEL9222CycleCounter(t *testing.T) {
	e := el9222Full()
	in, out := make([]byte, 8), make([]byte, 4)
	f, _ := e.Layout().Field("OCP Inputs Channel 2", "Status__Input cycle counter")
	var seq []uint64
	for i := 0; i < 5; i++ {
		e.Step(0, out, in)
		seq = append(seq, ecat.Get(in, f))
	}
	if want := []uint64{1, 2, 3, 0, 1}; fmtU(seq) != fmtU(want) {
		t.Errorf("counter = %v, want %v", seq, want)
	}
}

func fmtU(v []uint64) string {
	s := ""
	for _, x := range v {
		s += itoa(int(x)) + " "
	}
	return s
}

func TestEL9222Errors(t *testing.T) {
	e := el9222Full()
	if e.Channels() != 2 {
		t.Errorf("Channels = %d", e.Channels())
	}
	for _, err := range []error{
		e.Trip(0), e.Trip(3), e.SetWarning(3, true), e.SetCoolDown(-1, true),
		e.SetHardwareProtection(9, true), e.SetLoadCurrent(3, 1), e.SetLoadCurrent(1, math.NaN()),
	} {
		if err == nil || !strings.Contains(err.Error(), "EL9222") {
			t.Errorf("err = %v", err)
		}
	}
	if e.Enabled(7) || e.Tripped(7) {
		t.Error("unknown channel reads true")
	}
}

func TestEL9222CurrentClamp(t *testing.T) {
	e := &EL9222{}
	e.Init(&ecat.Slave{Name: "C (EL9222-5500)"})
	e.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "OCP Inputs Channel 1", Entry: "Current", Dir: ecat.DirIn, Bit: 0, BitLen: 16},
	}))
	in := make([]byte, 2)
	for amps, want := range map[float64]uint64{-3: 0, 1.234: 123, 1000: 0xffff, math.Inf(1): 0xffff} {
		if err := e.SetLoadCurrent(1, amps); err != nil {
			t.Fatal(err)
		}
		e.Step(0, nil, in)
		if got := uint64(in[0]) | uint64(in[1])<<8; got != want {
			t.Errorf("%v A -> %d, want %d", amps, got, want)
		}
	}
}

func TestEL9222Registration(t *testing.T) {
	reg := ecat.NewRegistry()
	Register(reg)
	for _, s := range []*ecat.Slave{
		{Vendor: VendorBeckhoff, Product: ProductEL9222, Model: "x"},
		{Vendor: VendorBeckhoff, Product: 0xbad, Model: "EL9227-5500"},
	} {
		f, ok := reg.Lookup(s)
		if !ok {
			t.Fatalf("no model for %+v", s)
		}
		if _, isE := f().(*EL9222); !isE {
			t.Errorf("%+v -> %T", s, f())
		}
	}
}

// TestEL9222ResetHighAtStartup is the LO-04 regression: Reset already
// high on the first Step is not a rising edge, so a trip injected before
// the first scan survives until Reset is pulsed.
func TestEL9222ResetHighAtStartup(t *testing.T) {
	topo, n := demoNet(t)
	e := el9222Dev(t, n)
	img := n.Images().Get(dev1)
	tripped := slotOf(t, topo, el9222Demo, "OCP Inputs Channel 1", "Status__Tripped")
	reset := slotOf(t, topo, el9222Demo, "OCP Outputs Channel 1", "Control__Reset")
	if err := e.Trip(1); err != nil {
		t.Fatal(err)
	}
	ecat.WriteBits(img.Out, reset.Byte, reset.Bit, reset.BitLen, 1)
	n.Step(0)
	n.Step(0)
	if readSlot(img.In, tripped) != 1 {
		t.Fatal("Reset held high from start-up cleared the trip")
	}
	ecat.WriteBits(img.Out, reset.Byte, reset.Bit, reset.BitLen, 0)
	n.Step(0)
	ecat.WriteBits(img.Out, reset.Byte, reset.Bit, reset.BitLen, 1)
	n.Step(0)
	if readSlot(img.In, tripped) != 0 {
		t.Fatal("a Reset pulse did not clear the trip")
	}
}
