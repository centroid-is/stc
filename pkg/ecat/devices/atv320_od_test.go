package devices

import (
	"encoding/binary"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

func le16(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }

func sdoU(t *testing.T, d *ATV320, index uint16, sub uint8) uint32 {
	t.Helper()
	b, code := d.SDORead(index, sub)
	if code != 0 {
		t.Fatalf("read %#04x:%02x abort %#x", index, sub, code)
	}
	var buf [4]byte
	copy(buf[:], b)
	return binary.LittleEndian.Uint32(buf[:])
}

// FB_ATV320's FB_Parameter objects with their defaults.
var fbATV320Params = []struct {
	name  string
	index uint16
	sub   uint8
	def   uint16
}{
	{"NPR", 0x2042, 0x0E, 75}, {"UNS", 0x2042, 0x02, 400}, {"FRS", 0x2042, 0x03, 500},
	{"NCR", 0x2042, 0x04, 20}, {"NSP", 0x2042, 0x05, 1430}, {"COS", 0x2042, 0x07, 80},
	{"ITH", 0x2042, 0x17, 20}, {"LFA", 0x2042, 0x3F, 0}, {"RSA", 0x2042, 0x2B, 0},
	{"TRA", 0x2042, 0x44, 0}, {"DCF", 0x2052, 0x1F, 4}, {"SSB", 0x203E, 0x29, 0},
	{"CLI", 0x203E, 0x02, 30}, {"HSP", 0x2001, 0x05, 500}, {"LSP", 0x2001, 0x06, 0},
}

func TestATV320ODParameters(t *testing.T) {
	if len(fbATV320Params) != 15 {
		t.Fatalf("%d parameters", len(fbATV320Params))
	}
	d := NewATV320()
	for _, p := range fbATV320Params {
		b, code := d.SDORead(p.index, p.sub)
		if code != 0 || len(b) != 2 || binary.LittleEndian.Uint16(b) != p.def {
			t.Errorf("%s default = %v, %#x; want %d", p.name, b, code, p.def)
			continue
		}
		v := p.def + 7
		if code := d.SDOWrite(p.index, p.sub, le16(v)); code != 0 {
			t.Errorf("%s write abort %#x", p.name, code)
		}
		if got := sdoU(t, d, p.index, p.sub); got != uint32(v) {
			t.Errorf("%s read back %d, want %d", p.name, got, v)
		}
	}
	if d.FRS != 507 || d.NCR != 27 || d.HSP != 507 || d.LSP != 7 {
		t.Errorf("model tunables FRS %d NCR %d HSP %d LSP %d", d.FRS, d.NCR, d.HSP, d.LSP)
	}
}

func TestATV320ODHSPDrivesRamp(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	if b, _ := r.d.SDORead(0x2001, 0x05); len(b) != 2 || b[0] != 0xF4 || b[1] != 0x01 {
		t.Fatalf("HSP bytes = %x", b)
	}
	if code := r.d.SDOWrite(0x2001, 0x05, []byte{0x58, 0x02}); code != 0 || r.d.HSP != 600 {
		t.Fatalf("HSP write %#x, HSP %d", code, r.d.HSP)
	}
	r.put("LFR", 700)
	r.enable()
	for i := 0; i < 40; i++ {
		r.step(0x0F)
	}
	if r.rfr() != 600 {
		t.Errorf("RFR = %d, want clamp at new HSP 600", r.rfr())
	}
	if sdoU(t, r.d, 0x2002, 0x03) != 600 || sdoU(t, r.d, 0x6041, 0x00) != uint32(r.d.ETA()) {
		t.Error("live RFR/ETA objects")
	}
	if sdoU(t, r.d, 0x2002, 0x29) != uint32(HMISRun) || sdoU(t, r.d, 0x2002, 0x05) != uint32(r.d.LCR()) {
		t.Error("live HMIS/LCR objects")
	}
}

func TestATV320ODAccDec(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	if code := r.d.SDOWrite(0x203C, 0x02, le16(10)); code != 0 {
		t.Fatal(code)
	}
	if code := r.d.SDOWrite(0x203C, 0x03, le16(5)); code != 0 || r.d.DEC != 5 {
		t.Fatal(code, r.d.DEC)
	}
	r.put("LFR", 500)
	r.enable()
	if r.rfr() != 50 {
		t.Errorf("ACC 1.0 s first tick RFR = %d, want 50", r.rfr())
	}
	if sdoU(t, r.d, 0x203C, 0x02) != 10 || sdoU(t, r.d, 0x203C, 0x03) != 5 {
		t.Error("ACC/DEC read back")
	}
}

func TestATV320ODReadOnlyAndMirrors(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.d.SetDI(0x0009)
	r.d.InjectFault(17)
	r.put("LFR", -123)
	r.put("OL1R", 0x0002)
	r.step(0x06)
	ro := []struct {
		index uint16
		sub   uint8
		want  uint32
	}{
		{0x6041, 0x00, uint32(r.d.ETA())}, {0x2002, 0x03, 0}, {0x2002, 0x05, 0},
		{0x2002, 0x29, uint32(HMISFault)}, {0x2029, 0x16, 17}, {0x2016, 0x03, 9}, {0x6061, 0x00, 2},
	}
	for _, o := range ro {
		if got := sdoU(t, r.d, o.index, o.sub); got != o.want {
			t.Errorf("%#04x:%02x = %d, want %d", o.index, o.sub, got, o.want)
		}
		if code := r.d.SDOWrite(o.index, o.sub, le16(1)); code != ecat.AbortReadOnly {
			t.Errorf("%#04x:%02x write abort %#x", o.index, o.sub, code)
		}
	}
	if sdoU(t, r.d, 0x6040, 0x00) != 0x06 || sdoU(t, r.d, 0x2037, 0x03) != uint32(uint16(0xFFFF-122)) ||
		sdoU(t, r.d, 0x2016, 0x0D) != 2 {
		t.Error("CMD/LFR/OL1R mirrors")
	}
	if code := r.d.SDOWrite(0x6040, 0, le16(0x0F)); code != 0 || sdoU(t, r.d, 0x6040, 0) != 0x0F {
		t.Error("CMD write")
	}
	if code := r.d.SDOWrite(0x2037, 3, le16(42)); code != 0 || sdoU(t, r.d, 0x2037, 3) != 42 {
		t.Error("LFR write")
	}
	if code := r.d.SDOWrite(0x2016, 0x0D, le16(5)); code != 0 || r.d.OL1R() != 5 {
		t.Error("OL1R write")
	}
	// Modes of operation: 1 byte, read back through 0x6061.
	if b, _ := r.d.SDORead(0x6060, 0); len(b) != 1 || b[0] != 2 {
		t.Errorf("0x6060 = %x", b)
	}
	if code := r.d.SDOWrite(0x6060, 0, []byte{3}); code != 0 || sdoU(t, r.d, 0x6061, 0) != 3 {
		t.Error("modes of operation")
	}
}

func TestATV320ODAborts(t *testing.T) {
	d := NewATV320()
	if _, code := d.SDORead(0x1234, 0); code != ecat.AbortNoObject {
		t.Errorf("unknown index read %#x", code)
	}
	if code := d.SDOWrite(0x1234, 0, nil); code != ecat.AbortNoObject {
		t.Errorf("unknown index write %#x", code)
	}
	if _, code := d.SDORead(0x2042, 0xEE); code != ecat.AbortNoSubIndex {
		t.Errorf("unknown subindex %#x", code)
	}
	if code := d.SDOWrite(0x2001, 0x05, []byte{1, 2, 3}); code != ecat.AbortLength {
		t.Errorf("UINT 3 bytes %#x", code)
	}
	if code := d.SDOWrite(0x2032, 0x01, []byte{1, 2, 3, 4, 5}); code != ecat.AbortLength {
		t.Errorf("UDINT 5 bytes %#x", code)
	}
	if code := d.SDOWrite(0x2001, 0x05, []byte{0x07}); code != 0 || d.HSP != 7 {
		t.Errorf("short write zero-extends: %#x HSP %d", code, d.HSP)
	}
}

func TestATV320ODMotorLock(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	r.enable()
	for _, o := range [][2]uint16{{0x2042, 0x03}, {0x2042, 0x0E}, {0x2052, 0x1F}} {
		if code := r.d.SDOWrite(o[0], uint8(o[1]), le16(1)); code != ecat.AbortDeviceState {
			t.Errorf("%#04x:%02x in Operation enabled abort %#x", o[0], o[1], code)
		}
	}
	if code := r.d.SDOWrite(0x2001, 0x05, le16(400)); code != 0 {
		t.Errorf("HSP in Operation enabled abort %#x", code)
	}
	r.step(0x06)
	if code := r.d.SDOWrite(0x2042, 0x03, le16(450)); code != 0 || r.d.FRS != 450 {
		t.Errorf("FRS after disable %#x", code)
	}
}

func TestATV320ODEEPROMSave(t *testing.T) {
	r := newRig(t, "atv320_device.xml")
	if code := r.d.SDOWrite(0x2032, 0x01, binary.LittleEndian.AppendUint32(nil, 0x65766173)); code != 0 {
		t.Fatal(code)
	}
	for i := 0; i < 5; i++ {
		if b, _ := r.d.SDORead(0x2032, 0x01); len(b) != 4 || binary.LittleEndian.Uint32(b) == 0 {
			t.Fatalf("step %d: save object %x, want busy", i, b)
		}
		r.step(0)
	}
	if got := sdoU(t, r.d, 0x2032, 0x01); got != 0 || r.d.SaveCount() != 1 {
		t.Fatalf("after 5 steps value %#x saves %d", got, r.d.SaveCount())
	}
	r.step(0)
	if r.d.SaveCount() != 1 {
		t.Error("idle step must not save")
	}
	// Writing 0 cancels a pending save.
	_ = r.d.SDOWrite(0x2032, 0x01, []byte{1})
	_ = r.d.SDOWrite(0x2032, 0x01, []byte{0})
	for i := 0; i < 6; i++ {
		r.step(0)
	}
	if r.d.SaveCount() != 1 {
		t.Errorf("cancelled save counted: %d", r.d.SaveCount())
	}
}
