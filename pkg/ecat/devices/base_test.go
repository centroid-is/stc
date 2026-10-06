package devices

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ecat"
)

func fixture(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "tests", "ecat_fixtures", name)
}

const dev1 = "Device 1 (EtherCAT)"

// demoNet loads Demo Device 1 with a fresh registry populated by Register.
func demoNet(t *testing.T) (*ecat.Topology, *ecat.Network) {
	t.Helper()
	topo, err := ecat.LoadProject(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	reg := ecat.NewRegistry()
	Register(reg)
	return topo, ecat.NewNetwork(topo, reg)
}

func testBase() *Base {
	b := &Base{}
	b.Init(&ecat.Slave{Name: "S (X)"})
	b.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "Channel 1", Entry: "Input", Dir: ecat.DirIn, Bit: 0, BitLen: 1},
		{Pdo: "Channel 2", Entry: "Input", Dir: ecat.DirIn, Bit: 1, BitLen: 1},
		{Pdo: "Status", Entry: "Value", Dir: ecat.DirIn, Bit: 8, BitLen: 16},
		{Pdo: "Channel 9", Entry: "Output", Dir: ecat.DirOut, Bit: 0, BitLen: 1},
		{Pdo: "Outputs", Entry: "Wide", Dir: ecat.DirOut, Bit: 1, BitLen: 1},
	}))
	return b
}

func TestBaseSetPersistsAcrossSteps(t *testing.T) {
	b := testBase()
	if err := b.Set("Channel 1", "Input", 1); err != nil {
		t.Fatal(err)
	}
	if err := b.Set("Status", "Value", 0x1234); err != nil {
		t.Fatal(err)
	}
	in, out := make([]byte, 3), make([]byte, 1)
	b.stepIO(out, in)
	if in[0] != 1 || in[1] != 0x34 || in[2] != 0x12 {
		t.Fatalf("in = % x", in)
	}
	in[0] = 0 // something else clears it; the override re-applies next Step
	b.stepIO(out, in)
	if v, ok := b.Get("Channel 1", "Input"); !ok || v != 1 {
		t.Errorf("Get after second step = %d %v", v, ok)
	}
	// Set after a Step writes the current view immediately.
	if err := b.Set("Channel 2", "Input", 1); err != nil || in[0] != 3 {
		t.Errorf("immediate set: in[0]=%x err=%v", in[0], err)
	}
	if err := b.Set("Channel 1", "Input", 0); err != nil || in[0] != 2 {
		t.Errorf("overwrite override: in[0]=%x", in[0])
	}
}

func TestBaseGetOutput(t *testing.T) {
	b := testBase()
	out := []byte{0x01}
	b.stepIO(out, make([]byte, 3))
	if v, ok := b.Get("Channel 9", "Output"); !ok || v != 1 {
		t.Errorf("Get output = %d %v", v, ok)
	}
	if b.Slave().Name != "S (X)" || b.Layout() == nil {
		t.Error("accessors")
	}
}

func TestBaseErrors(t *testing.T) {
	b := testBase()
	err := b.Set("Nope", "Input", 1)
	if err == nil || !strings.Contains(err.Error(), "S (X)") {
		t.Errorf("unknown entry err = %v", err)
	}
	if err := b.Set("Channel 9", "Output", 1); err == nil || !strings.Contains(err.Error(), "S (X)") {
		t.Errorf("output Set err = %v", err)
	}
	if _, ok := b.Get("Nope", "Input"); ok {
		t.Error("Get unknown ok")
	}
	// Short and empty views never panic and never write outside the view.
	if err := b.Set("Status", "Value", 0xffff); err != nil {
		t.Fatal(err)
	}
	b.stepIO(nil, nil)
	short := []byte{0}
	b.stepIO(short[:0], short[:1])
	if short[0] != 0 {
		t.Errorf("short view wrote %x", short[0])
	}
	if v, ok := b.Get("Status", "Value"); !ok || v != 0 {
		t.Errorf("short Get = %d %v", v, ok)
	}
	// Zero Base (no Init/Bind) is safe.
	var z Base
	if err := z.Set("a", "b", 1); err == nil {
		t.Error("zero Set")
	}
	if _, ok := z.ChannelField(ecat.DirIn, 1); ok {
		t.Error("zero ChannelField")
	}
	z.stepIO(nil, nil)
}

func TestBaseChannelField(t *testing.T) {
	b := testBase()
	if f, ok := b.ChannelField(ecat.DirIn, 2); !ok || f.Pdo != "Channel 2" {
		t.Errorf("by name: %+v", f)
	}
	// No out PDO named "Channel 1": falls back to the first 1-bit out field.
	if f, ok := b.ChannelField(ecat.DirOut, 1); !ok || f.Pdo != "Channel 9" {
		t.Errorf("fallback: %+v", f)
	}
	if f, ok := b.ChannelField(ecat.DirOut, 2); !ok || f.Entry != "Wide" {
		t.Errorf("fallback 2: %+v", f)
	}
	for _, n := range []int{0, -1, 3} {
		if _, ok := b.ChannelField(ecat.DirOut, n); ok {
			t.Errorf("channel %d found", n)
		}
	}
	// "Channel 1" must not match "Channel 11".
	c := &Base{}
	c.Bind(ecat.NewLayout([]ecat.Field{
		{Pdo: "Channel 11", Entry: "Input", Dir: ecat.DirIn, Bit: 0, BitLen: 1},
		{Pdo: "Channel 1", Entry: "Input", Dir: ecat.DirIn, Bit: 1, BitLen: 1},
	}))
	if f, _ := c.ChannelField(ecat.DirIn, 1); f.Pdo != "Channel 1" {
		t.Errorf("suffix match: %+v", f)
	}
}

func TestRegistryKnowsEveryProduct(t *testing.T) {
	reg := ecat.NewRegistry()
	Register(reg)
	for _, r := range []*ecat.Registry{reg, ecat.DefaultRegistry} {
		for _, id := range knownIDs {
			if _, ok := r.Lookup(&ecat.Slave{Vendor: id.vendor, Product: id.product}); !ok {
				t.Errorf("no model for %s", id.name)
			}
		}
	}
	for _, model := range []string{"EK1100", "EK1110", "EK1200", "EL9011"} {
		f, ok := reg.Lookup(&ecat.Slave{Vendor: VendorBeckhoff, Product: 0xbad, Model: model})
		if !ok {
			t.Errorf("no fallback for %s", model)
			continue
		}
		if _, isPassive := f().(*Passive); !isPassive {
			t.Errorf("%s fallback is %T", model, f())
		}
	}
}

func TestPassiveCouplersNoDiagnostic(t *testing.T) {
	topo, n := demoNet(t)
	for _, d := range n.Diagnostics() {
		for _, model := range []string{"EK1200", "EK1110"} {
			if strings.Contains(d.Message, "("+model+")") {
				t.Errorf("unexpected diag %s", d.Message)
			}
		}
	}
	d, ok := n.DeviceByName("DEMO.A1.00 (EK1200)")
	if !ok {
		t.Fatal("EK1200 missing")
	}
	p, isPassive := d.(*Passive)
	if !isPassive || p.Slave().Model != "EK1200" {
		t.Fatalf("EK1200 is %T", d)
	}
	before := append([]byte(nil), n.Images().Get(dev1).In...)
	n.Step(0)
	n.Step(0)
	after := n.Images().Get(dev1).In
	// Only pseudo-inputs change; with no stimulus every PDO byte stays put.
	m := topo.Master(dev1)
	for _, s := range m.Slaves {
		for _, pd := range s.Pdos {
			for _, e := range pd.Entries {
				slot, ok := m.Slot(ecat.LinkPath(m.Name, s, pd, e))
				if ok && slot.Dir == ecat.DirIn && after[slot.Byte] != before[slot.Byte] {
					t.Errorf("%s changed", slot.Path)
				}
			}
		}
	}
}
