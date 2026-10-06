package devices

import (
	"math"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

// psuFull mirrors the PS2001-2410 export: "PSU Inputs" (Warning, Error,
// DC OK, REAL voltage and current), "PSU Inputs Device" and "PSU Outputs".
func psuFull(voltType string, voltLen int) *PSU {
	p := psu().(*PSU)
	p.Init(&ecat.Slave{Name: "P (PS2001-2410)"})
	p.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "PSU Inputs", Entry: "Warning", Dir: ecat.DirIn, Bit: 0, BitLen: 1, DataType: "BIT"},
		{Pdo: "PSU Inputs", Entry: "Error", Dir: ecat.DirIn, Bit: 1, BitLen: 1, DataType: "BIT"},
		{Pdo: "PSU Inputs", Entry: "DC OK", Dir: ecat.DirIn, Bit: 3, BitLen: 1, DataType: "BIT"},
		{Pdo: "PSU Inputs", Entry: "Output voltage", Dir: ecat.DirIn, Bit: 32, BitLen: voltLen, DataType: voltType},
		{Pdo: "PSU Inputs", Entry: "Output current", Dir: ecat.DirIn, Bit: 96, BitLen: 32, DataType: "REAL"},
		{Pdo: "PSU Inputs Device", Entry: "Input undervoltage", Dir: ecat.DirIn, Bit: 130, BitLen: 1, DataType: "BIT"},
		{Pdo: "PSU Outputs", Entry: "Disable output", Dir: ecat.DirOut, Bit: 0, BitLen: 1, DataType: "BIT"},
	}))
	return p
}

func f32(in []byte, bit int) float32 {
	return math.Float32frombits(uint32(ecat.Get(in, ecat.Field{Bit: bit, BitLen: 32})))
}

func TestPSUDefaultsAndSetPSU(t *testing.T) {
	p := psuFull("REAL", 32)
	in, out := make([]byte, 17), make([]byte, 1)
	in[0] = 0xff
	p.Step(0, out, in)
	if in[0]&0x0b != 0x08 || f32(in, 32) != 24 || f32(in, 96) != 0 || in[16]&4 != 0 {
		t.Fatalf("defaults: % x voltage %v", in, f32(in, 32))
	}
	if !p.State().DCOK {
		t.Error("State default DC OK")
	}
	p.SetPSU(PSUState{Warning: true, Error: true, InputUndervoltage: true, OutputVoltage: 23.1, OutputCurrent: 4.2})
	p.Step(0, out, in)
	if in[0]&0x0b != 0x03 || f32(in, 32) != float32(23.1) || f32(in, 96) != float32(4.2) || in[16]&4 == 0 {
		t.Errorf("SetPSU: % x", in)
	}
	p.SetPSU(PSUState{DCOK: true, OutputVoltage: 24, OutputCurrent: 3})
	out[0] = 1 // PLC disables the output
	p.Step(0, out, in)
	if in[0]&0x08 != 0 || f32(in, 32) != 0 || f32(in, 96) != 0 {
		t.Errorf("Disable output: % x", in)
	}
	out[0] = 0
	p.Step(0, out, in)
	if in[0]&0x08 == 0 || f32(in, 32) != 24 {
		t.Error("output did not come back after Disable output cleared")
	}
}

func TestPSUIntegerEncoding(t *testing.T) {
	p := psuFull("UINT", 16)
	in, out := make([]byte, 17), make([]byte, 1)
	p.SetPSU(PSUState{DCOK: true, OutputVoltage: 24.5})
	p.Step(0, out, in)
	if got := ecat.Get(in, ecat.Field{Bit: 32, BitLen: 16}); got != 24500 {
		t.Errorf("UINT voltage = %d, want 24500 mV", got)
	}
	p.SetPSU(PSUState{OutputVoltage: 1000, OutputCurrent: math.NaN()})
	p.Step(0, out, in)
	if got := ecat.Get(in, ecat.Field{Bit: 32, BitLen: 16}); got != 0xffff {
		t.Errorf("clamped voltage = %d", got)
	}
	if got := f32(in, 96); got != 0 {
		t.Errorf("NaN current = %v, want 0", got)
	}
	q := psuFull("LREAL", 64)
	q.Step(0, out, make([]byte, 17))
	if encodeAnalog(ecat.Field{DataType: "LREAL", BitLen: 64}, 2.5) != math.Float64bits(2.5) {
		t.Error("LREAL encoding")
	}
	if encodeAnalog(ecat.Field{DataType: "INT", BitLen: 16}, -3) != 0 {
		t.Error("negative clamps to 0")
	}
}

func TestPSUDemoFixture(t *testing.T) {
	topo, n := demoNet(t)
	const name = "DEMO.T1 (PS2001-2410)"
	d, ok := n.DeviceByName(name)
	if !ok {
		t.Fatal("no PSU")
	}
	p, ok := d.(*PSU)
	if !ok {
		t.Fatalf("PSU is %T", d)
	}
	slot := slotOf(t, topo, name, "PSU Inputs Device", "Input undervoltage")
	n.Step(0)
	if readSlot(n.Images().Get(dev1).In, slot) != 0 {
		t.Error("undervoltage set by default")
	}
	p.SetPSU(PSUState{DCOK: true, InputUndervoltage: true, OutputVoltage: 24})
	n.Step(0)
	if readSlot(n.Images().Get(dev1).In, slot) != 1 {
		t.Error("undervoltage not in image")
	}
}

func TestSafetyDiagFieldVoltage(t *testing.T) {
	topo, n := demoNet(t)
	const name = "DEMO.A1.04 (EL2912)"
	d, ok := n.DeviceByName(name)
	if !ok {
		t.Fatal("no EL2912")
	}
	s, ok := d.(*SafetyDiag)
	if !ok {
		t.Fatalf("EL2912 is %T", d)
	}
	pdo := "FIELDVOLTAGE Field Voltage Status"
	under, over := slotOf(t, topo, name, pdo, "Fieldvoltage Underrange"), slotOf(t, topo, name, pdo, "Fieldvoltage Overrange")
	img := n.Images().Get(dev1).In
	ecat.WriteBits(img, under.Byte, under.Bit, 1, 1)
	n.Step(0)
	if readSlot(img, under) != 0 || readSlot(img, over) != 0 {
		t.Error("field voltage not healthy by default")
	}
	s.SetFieldVoltage(true, false)
	n.Step(0)
	if readSlot(img, under) != 1 || readSlot(img, over) != 0 {
		t.Error("underrange stimulus")
	}
	s.SetFieldVoltage(false, true)
	n.Step(0)
	if readSlot(img, under) != 0 || readSlot(img, over) != 1 {
		t.Error("overrange stimulus")
	}
}

// EL1904 carries only FSoE data: the model binds, writes nothing, and the
// safety frames pass through untouched.
func TestSafetyDiagLeavesFSoEAlone(t *testing.T) {
	s := safetyDiag().(*SafetyDiag)
	s.Init(&ecat.Slave{Name: "S (EL1904)"})
	s.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "TxPDO", Entry: "FSOE__FSoE Slave CMD", Dir: ecat.DirIn, Bit: 0, BitLen: 8},
		{Pdo: "TxPDO", Entry: "FSOE__InputChannel1", Dir: ecat.DirIn, Bit: 8, BitLen: 1},
		{Pdo: "TxPDO", Entry: "FSOE__FSoE Slave CRC_0", Dir: ecat.DirIn, Bit: 16, BitLen: 16},
		{Pdo: "RxPDO", Entry: "FSOE__FSoE Master CMD", Dir: ecat.DirOut, Bit: 0, BitLen: 8},
	}))
	in := []byte{0x36, 0x01, 0xaa, 0x55}
	s.SetFieldVoltage(true, true)
	s.Step(0, []byte{0x2a}, in)
	if string(in) != string([]byte{0x36, 0x01, 0xaa, 0x55}) {
		t.Errorf("FSoE data changed: % x", in)
	}
}

func TestPSUSafetyRegistrations(t *testing.T) {
	reg := ecat.NewRegistry()
	Register(reg)
	for _, c := range []struct {
		s    *ecat.Slave
		want string
	}{
		{&ecat.Slave{Vendor: VendorBeckhoff, Product: ProductPS2001, Model: "PS2001-2410"}, "*devices.PSU"},
		{&ecat.Slave{Vendor: VendorBeckhoff, Product: 0xbad, Model: "PS2001-2410"}, "*devices.PSU"},
		{&ecat.Slave{Vendor: VendorBeckhoff, Product: ProductEL2912, Model: "x"}, "*devices.SafetyDiag"},
		{&ecat.Slave{Vendor: VendorBeckhoff, Product: ProductEP1918, Model: "x"}, "*devices.SafetyDiag"},
		{&ecat.Slave{Vendor: VendorBeckhoff, Product: ProductEL1904, Model: "x"}, "*devices.SafetyDiag"},
	} {
		f, ok := reg.Lookup(c.s)
		if !ok {
			t.Errorf("no model for %+v", c.s)
			continue
		}
		if got := typeName(f()); got != c.want {
			t.Errorf("%+v -> %s, want %s", c.s, got, c.want)
		}
	}
}

func typeName(v interface{}) string {
	switch v.(type) {
	case *PSU:
		return "*devices.PSU"
	case *SafetyDiag:
		return "*devices.SafetyDiag"
	}
	return "other"
}

func TestDemoOnlyDriveUnmodelled(t *testing.T) {
	_, n := demoNet(t)
	diags := n.Diagnostics()
	if len(diags) != 1 || diags[0].Code != ecat.CodeNoModel || !strings.Contains(diags[0].Message, "ATV320") {
		t.Errorf("diagnostics = %v, want one ECAT010 for the ATV320", diags)
	}
}

// Layout edge cases: fields outside "Channel n" PDOs, a channel without a
// Value, and integer entries wider than 32 bits.
func TestModelLayoutEdges(t *testing.T) {
	a := &Analog{kind: analogCurrent}
	a.Init(&ecat.Slave{Name: "E (EL3054)"})
	a.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "Device", Entry: "Value", Dir: ecat.DirIn, Bit: 0, BitLen: 16},
		{Pdo: "AI Standard Channel 2", Entry: "Status__Error", Dir: ecat.DirIn, Bit: 16, BitLen: 1},
	}))
	if a.Channels() != 2 || a.SetCurrent(1, 12) == nil || a.SetCurrent(2, 12) == nil {
		t.Error("channels without Value must be rejected")
	}
	in := []byte{0x11, 0x22, 0x01}
	a.Step(0, nil, in)
	if in[0] != 0x11 || in[1] != 0x22 || in[2] != 0x01 {
		t.Errorf("valueless channel written: % x", in)
	}
	e := &EL9222{}
	e.Init(&ecat.Slave{Name: "E (EL9222-5500)"})
	e.Bind(ecat.NewLayout([]ecat.Field{{Pdo: "Unassigned", Entry: "Spare", Dir: ecat.DirIn, Bit: 0, BitLen: 8}}))
	if e.Channels() != 0 {
		t.Errorf("EL9222 channels = %d", e.Channels())
	}
	if got := encodeAnalog(ecat.Field{DataType: "ULINT", BitLen: 64}, 1e12); got != 0xffffffff {
		t.Errorf("wide integer clamp = %x", got)
	}
}
