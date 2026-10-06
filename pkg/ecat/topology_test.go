package ecat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(name string) string {
	return filepath.Join("..", "..", "tests", "ecat_fixtures", name)
}

func loadDemo1(t *testing.T) *Master {
	t.Helper()
	m, err := LoadConfig(fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return m
}

func slaveByName(t *testing.T, m *Master, name string) *Slave {
	t.Helper()
	for _, s := range m.Slaves {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("slave %q not found", name)
	return nil
}

func pdoNames(s *Slave) []string {
	var out []string
	for _, p := range s.Pdos {
		out = append(out, p.Dir.String()+":"+p.Name)
	}
	return out
}

func TestLoadConfigDemo1(t *testing.T) {
	m := loadDemo1(t)
	if m.Name != "Device 1 (EtherCAT)" {
		t.Errorf("master name = %q", m.Name)
	}
	want := []string{
		"DEMO.A1.00 (EK1200)", "DEMO.A1.01 (EL1008)", "DEMO.A1.02 (EL2008)",
		"DEMO.A1.03 (EL9222-5500)", "DEMO.A1.04 (EL2912)", "DEMO.A1.05 (EK1110)",
		"DEMO.CN01.FD01 (ATV320 EtherCAT)", "DEMO.A2 (EP2338-0002)",
		"DEMO.T1 (PS2001-2410)", "DEMO.V1 (CTEU-EtherCAT Modular)",
	}
	if len(m.Slaves) != len(want) {
		t.Fatalf("got %d slaves, want %d", len(m.Slaves), len(want))
	}
	for i, s := range m.Slaves {
		if s.Name != want[i] {
			t.Errorf("slave %d = %q, want %q", i, s.Name, want[i])
		}
		if s.Index != i {
			t.Errorf("slave %q Index = %d, want %d", s.Name, s.Index, i)
		}
		if !s.HasPhys || s.Phys != 1001+i {
			t.Errorf("slave %q phys = %d/%v", s.Name, s.Phys, s.HasPhys)
		}
	}
	if m.NetID != [6]byte{192, 168, 0, 1, 1, 1} {
		t.Errorf("default NetID = %v", m.NetID)
	}

	el1008 := m.Slaves[1]
	if el1008.Model != "EL1008" || el1008.Vendor != 2 || el1008.Product != 66072658 ||
		!el1008.HasVendor || !el1008.HasProduct || el1008.Physics != "KK" ||
		el1008.Revision != 0x00100000 {
		t.Errorf("EL1008 info = %+v", el1008)
	}
	if !el1008.HasPrevPhys || el1008.PrevPhys != 1001 || el1008.PrevPort != "B" {
		t.Errorf("EL1008 previous port = %d %q", el1008.PrevPhys, el1008.PrevPort)
	}
	if len(el1008.Pdos) != 8 || el1008.Pdos[2].Name != "Channel 3" || el1008.Pdos[2].Index != 0x1a02 ||
		el1008.Pdos[2].Dir != DirIn {
		t.Fatalf("EL1008 pdos = %v", pdoNames(el1008))
	}
	e := el1008.Pdos[2].Entries[0]
	if e.Name != "Input" || e.Index != "#x6020" || e.SubIndex != 1 || e.BitLen != 1 || e.DataType != "BIT" || e.Padding() {
		t.Errorf("EL1008 entry = %+v", e)
	}

	atv := slaveByName(t, m, "DEMO.CN01.FD01 (ATV320 EtherCAT)")
	if atv.Vendor != 0x0800005A || atv.Product != 0x389 {
		t.Errorf("hex VendorId/ProductCode parsed as %#x/%#x", atv.Vendor, atv.Product)
	}
	ep := slaveByName(t, m, "DEMO.A2 (EP2338-0002)")
	if ep.PrevPort != "D" || ep.PrevPhys != 1007 {
		t.Errorf("EP2338 prev = %q %d", ep.PrevPort, ep.PrevPhys)
	}
}

func TestLoadConfigActivePdos(t *testing.T) {
	m := loadDemo1(t)
	el9222 := slaveByName(t, m, "DEMO.A1.03 (EL9222-5500)")
	got := strings.Join(pdoNames(el9222), "|")
	want := "in:OCP Inputs Channel 1|out:OCP Outputs Channel 1"
	if got != want {
		t.Errorf("EL9222 pdos = %q, want %q (unassigned and Enable 0 PDOs must drop)", got, want)
	}
	pad := el9222.Pdos[0].Entries[2]
	if !pad.Padding() || pad.BitLen != 14 {
		t.Errorf("padding entry = %+v", pad)
	}
	// No Sm assignment at all: every PDO is kept.
	el2008 := slaveByName(t, m, "DEMO.A1.02 (EL2008)")
	if len(el2008.Pdos) != 8 || el2008.Pdos[0].Dir != DirOut {
		t.Errorf("EL2008 pdos = %v", pdoNames(el2008))
	}
	// TxPdos precede RxPdos.
	atv := slaveByName(t, m, "DEMO.CN01.FD01 (ATV320 EtherCAT)")
	if got := strings.Join(pdoNames(atv), "|"); got != "in:Inputs|out:Outputs" {
		t.Errorf("ATV320 pdos = %q", got)
	}
}

func TestEntryPadding(t *testing.T) {
	cases := []struct {
		e    Entry
		want bool
	}{
		{Entry{Name: "", Index: "#x0"}, true},
		{Entry{Name: "Status__", Index: "#x0"}, true},
		{Entry{Name: "", Index: "#x6000"}, true},
		{Entry{Name: "Input", Index: "#x6000"}, false},
	}
	for _, c := range cases {
		if got := c.e.Padding(); got != c.want {
			t.Errorf("Padding(%+v) = %v, want %v", c.e, got, c.want)
		}
	}
}

func TestModelOf(t *testing.T) {
	cases := map[string]string{
		"DEMO.A1.01 (EL1008)":              "EL1008",
		"DEMO.CN01.FD01 (ATV320 EtherCAT)": "ATV320 EtherCAT",
		"DEMO.T1":                          "",
		"X (a) (EL2008 ) ":                 "EL2008",
	}
	for in, want := range cases {
		if got := modelOf(in); got != want {
			t.Errorf("modelOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseInt(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"42", 42, true}, {" 7 ", 7, true}, {"#x1a00", 0x1a00, true}, {"#X10", 16, true},
		{"", 0, false}, {"abc", 0, false}, {"#xzz", 0, false}, {"-3", -3, true},
	}
	for _, c := range cases {
		got, ok := parseInt(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseInt(%q) = %d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestLoadProject(t *testing.T) {
	topo, err := LoadProject(fixture("Demo Device 2.xml"), fixture("Demo Device 1.xml"))
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(topo.Masters) != 2 || topo.Masters[0].Name != "Device 2 (EtherCAT)" || topo.Masters[1].Name != "Device 1 (EtherCAT)" {
		t.Fatalf("masters out of argument order: %v", topo.Masters)
	}
	if topo.Master("Device 1 (EtherCAT)") != topo.Masters[1] || topo.Master("nope") != nil {
		t.Errorf("Topology.Master lookup broken")
	}
	if _, err := LoadProject(fixture("Demo Device 1.xml"), fixture("Demo Device 1.xml")); err == nil ||
		!strings.Contains(err.Error(), "duplicate master") {
		t.Errorf("duplicate master err = %v", err)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	missing := filepath.Join(dir, "missing.xml")
	cases := []struct {
		name, path, want string
	}{
		{"missing", missing, "missing.xml"},
		{"malformed", write("bad.xml", "<EtherCATConfig><Config>"), "bad.xml"},
		{"no master", write("nomaster.xml", "<EtherCATConfig><Config></Config></EtherCATConfig>"), "no master name"},
		{"bitlen", write("bitlen.xml", `<EtherCATConfig><Config><Master><Info><Name>M</Name></Info></Master>
<Slave><Info><Name>S</Name></Info><ProcessData><TxPdo><Index>#x1a00</Index><Name>P</Name>
<Entry><Index>#x6000</Index><BitLen>5000</BitLen><Name>E</Name></Entry></TxPdo></ProcessData></Slave></Config></EtherCATConfig>`), "BitLen"},
		{"negative bitlen", write("neg.xml", `<EtherCATConfig><Config><Master><Info><Name>M</Name></Info></Master>
<Slave><Info><Name>S</Name></Info><ProcessData><TxPdo><Index>#x1a00</Index><Name>P</Name>
<Entry><Index>#x6000</Index><BitLen>-1</BitLen><Name>E</Name></Entry></TxPdo></ProcessData></Slave></Config></EtherCATConfig>`), "BitLen"},
		{"bitoffs", write("offs.xml", `<EtherCATConfig><Config><Master><Info><Name>M</Name></Info></Master>
<ProcessImage><Inputs><ByteSize>1</ByteSize><Variable><Name>X</Name><BitSize>1</BitSize><BitOffs>99999999</BitOffs></Variable></Inputs></ProcessImage>
</Config></EtherCATConfig>`), "BitOffs"},
		{"bytesize", write("size.xml", `<EtherCATConfig><Config><Master><Info><Name>M</Name></Info></Master>
<ProcessImage><Outputs><ByteSize>99999999</ByteSize></Outputs></ProcessImage>
</Config></EtherCATConfig>`), "ByteSize"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadConfig(c.path)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want containing %q", err, c.want)
			}
		})
	}
	if _, err := LoadProject(missing); err == nil {
		t.Errorf("LoadProject(missing) succeeded")
	}
}
