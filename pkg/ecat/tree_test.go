package ecat

import "testing"

func TestClassifyRole(t *testing.T) {
	cases := []struct {
		physics, model string
		want           Role
	}{
		{"YK", "", RoleOpen},
		{"YKY", "", RoleOpen},
		{"ykk", "", RoleOpen},
		{"KK", "", RoleTerminal},
		{"K", "", RoleTerminal},
		{"KY", "", RoleClose},
		{"KYY", "", RoleClose},
		{"YY", "EK1100", RolePlain},
		{"", "EK1110", RoleClose},
		{"", "EK1120", RoleClose},
		{"", "EK1210", RoleClose},
		{"", "EK1220", RoleClose},
		{"", "EK1100", RoleOpen},
		{"", "EK1101", RoleOpen},
		{"", "EK1200", RoleOpen},
		{"", "EK1501", RoleOpen},
		{"", "EK1521", RoleOpen},
		{"", "EK1100-0008", RolePlain},
		{"", "EL1008", RolePlain},
		{"", "", RolePlain},
	}
	for _, c := range cases {
		s := &Slave{Physics: c.physics, Model: c.model}
		if got := ClassifyRole(s); got != c.want {
			t.Errorf("ClassifyRole(physics=%q model=%q) = %v, want %v", c.physics, c.model, got, c.want)
		}
	}
}

func parentName(s *Slave) string {
	if s.Parent == nil {
		return ""
	}
	return s.Parent.Name
}

func TestAssignParentsDemo1(t *testing.T) {
	m := loadDemo1(t)
	want := map[string]struct {
		parent string
		ebus   bool
	}{
		"DEMO.A1.00 (EK1200)":              {"", false},
		"DEMO.A1.01 (EL1008)":              {"DEMO.A1.00 (EK1200)", true},
		"DEMO.A1.02 (EL2008)":              {"DEMO.A1.00 (EK1200)", true},
		"DEMO.A1.03 (EL9222-5500)":         {"DEMO.A1.00 (EK1200)", true},
		"DEMO.A1.04 (EL2912)":              {"DEMO.A1.00 (EK1200)", true},
		"DEMO.A1.05 (EK1110)":              {"DEMO.A1.00 (EK1200)", false},
		"DEMO.CN01.FD01 (ATV320 EtherCAT)": {"", false},
		"DEMO.A2 (EP2338-0002)":            {"", false},
		"DEMO.T1 (PS2001-2410)":            {"", false},
		"DEMO.V1 (CTEU-EtherCAT Modular)":  {"", false},
	}
	for _, s := range m.Slaves {
		w := want[s.Name]
		if parentName(s) != w.parent || s.IsEBus != w.ebus {
			t.Errorf("%s: parent=%q ebus=%v, want %q %v", s.Name, parentName(s), s.IsEBus, w.parent, w.ebus)
		}
	}
}

func TestAssignParentsDemo2Fallback(t *testing.T) {
	m, err := LoadConfig(fixture("Demo Device 2.xml"))
	if err != nil {
		t.Fatal(err)
	}
	// No Physics: the generator's fallback only knows open/close couplers, so
	// an EL terminal classifies as plain and stays at master level, while the
	// EK1110 still closes the EK1100 segment.
	got := []string{parentName(m.Slaves[0]), parentName(m.Slaves[1]), parentName(m.Slaves[2]), parentName(m.Slaves[3])}
	want := []string{"", "", "D2.A1 (EK1100)", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slave %s parent = %q, want %q", m.Slaves[i].Name, got[i], want[i])
		}
	}
}

func TestAssignParentsBranches(t *testing.T) {
	mk := func(name, physics string, phys int, prevPort string, prevPhys int, hasPrev bool) *Slave {
		return &Slave{Name: name, Physics: physics, Phys: phys, HasPhys: true,
			PrevPort: prevPort, PrevPhys: prevPhys, HasPrevPhys: hasPrev}
	}
	t.Run("port C terminal is master level and clears coupler", func(t *testing.T) {
		c := mk("C", "YK", 1, "", 0, false)
		a := mk("A", "KK", 2, "B", 1, true)
		b := mk("B", "KK", 3, "C", 2, true)
		d := mk("D", "KK", 4, "B", 3, true)
		assignParents([]*Slave{c, a, b, d})
		if a.Parent != c || !a.IsEBus {
			t.Errorf("A should nest under C")
		}
		if b.Parent != nil || b.IsEBus {
			t.Errorf("Port C terminal should be master level")
		}
		if d.Parent != nil {
			t.Errorf("coupler should be cleared after a branch")
		}
	})
	t.Run("terminal whose upstream is a plain box", func(t *testing.T) {
		c := mk("C", "YK", 1, "", 0, false)
		p := mk("P", "YY", 2, "B", 1, true)
		a := mk("A", "KK", 3, "B", 2, true)
		assignParents([]*Slave{c, p, a})
		if a.Parent != nil {
			t.Errorf("terminal chained from a non-E-bus box must be master level")
		}
		// Port B keeps the coupler open for a later terminal chained from it.
		b := mk("B", "KK", 4, "B", 1, true)
		assignParents([]*Slave{c, p, a, b})
		if b.Parent != c {
			t.Errorf("terminal chained from the coupler should still nest")
		}
	})
	t.Run("document order fallback when PreviousPort missing", func(t *testing.T) {
		c := &Slave{Name: "C", Physics: "YK"}
		a := &Slave{Name: "A", Physics: "KK"}
		b := &Slave{Name: "B", Physics: "KK"}
		x := &Slave{Name: "X", Physics: "KY"}
		y := &Slave{Name: "Y", Physics: "KK"}
		assignParents([]*Slave{c, a, b, x, y})
		if a.Parent != c || b.Parent != c || x.Parent != c || x.IsEBus {
			t.Errorf("document-order nesting broken")
		}
		if y.Parent != nil {
			t.Errorf("terminal after a close must be master level")
		}
	})
	t.Run("terminal with no coupler", func(t *testing.T) {
		a := &Slave{Name: "A", Physics: "KK"}
		assignParents([]*Slave{a})
		if a.Parent != nil || a.IsEBus {
			t.Errorf("orphan terminal should be master level")
		}
	})
}

func TestLinkPath(t *testing.T) {
	m := loadDemo1(t)
	const base = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^"
	link := func(slave, pdo, entry string) string {
		s := slaveByName(t, m, slave)
		for _, p := range s.Pdos {
			if p.Name != pdo {
				continue
			}
			for _, e := range p.Entries {
				if e.Name == entry {
					return LinkPath(m.Name, s, p, e)
				}
			}
		}
		t.Fatalf("no %s/%s/%s", slave, pdo, entry)
		return ""
	}
	cases := []struct{ got, want string }{
		{link("DEMO.A1.01 (EL1008)", "Channel 3", "Input"),
			base + "DEMO.A1.01 (EL1008)^Channel 3^Input"},
		{link("DEMO.A1.03 (EL9222-5500)", "OCP Inputs Channel 1", "Status__Tripped"),
			base + "DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Status^Tripped"},
		{link("DEMO.A1.03 (EL9222-5500)", "OCP Outputs Channel 1", "Control__Reset"),
			base + "DEMO.A1.03 (EL9222-5500)^OCP Outputs Channel 1^Control^Reset"},
		{link("DEMO.A1.04 (EL2912)", "FIELDVOLTAGE Field Voltage Status", "Fieldvoltage Underrange"),
			base + "DEMO.A1.04 (EL2912)^Module 3 (DEVICEIO)^FIELDVOLTAGE Field Voltage Status^Fieldvoltage Underrange"},
		{link("DEMO.V1 (CTEU-EtherCAT Modular)", "Outputs", "C1 Output"),
			"TIID^Device 1 (EtherCAT)^DEMO.V1 (CTEU-EtherCAT Modular)^Module 1 (VAEM-L1-S-8-PT [16DO])^Outputs^C1 Output"},
		{link("DEMO.CN01.FD01 (ATV320 EtherCAT)", "Outputs", "CMD"),
			"TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)^Outputs^CMD"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("LinkPath =\n  %q\nwant\n  %q", c.got, c.want)
		}
	}
	if got := SlaveBasePath(m.Name, slaveByName(t, m, "DEMO.A1.05 (EK1110)")); got != base+"DEMO.A1.05 (EK1110)" {
		t.Errorf("SlaveBasePath(EK1110) = %q", got)
	}
}

func TestModuleSegmentNeedsIdentity(t *testing.T) {
	p := Pdo{Name: "Outputs"}
	if got := ModuleSegment(&Slave{Vendor: 29, Product: 572556, HasVendor: true, HasProduct: true}, p); got != "Module 1 (VAEM-L1-S-8-PT [16DO])" {
		t.Errorf("CTEU module = %q", got)
	}
	if got := ModuleSegment(&Slave{Vendor: 29, Product: 572556, HasVendor: true}, p); got != "" {
		t.Errorf("missing ProductCode must not match, got %q", got)
	}
	if got := ModuleSegment(&Slave{Vendor: 29, Product: 1, HasVendor: true, HasProduct: true}, p); got != "" {
		t.Errorf("other product must not match, got %q", got)
	}
}

func TestIECType(t *testing.T) {
	cases := []struct {
		e    Entry
		want string
	}{
		{Entry{DataType: "BIT", BitLen: 1}, "BOOL"},
		{Entry{DataType: "BOOL"}, "BOOL"},
		{Entry{DataType: "UINT", BitLen: 16}, "UINT"},
		{Entry{DataType: "INT"}, "INT"},
		{Entry{DataType: "REAL"}, "REAL"},
		{Entry{DataType: "LREAL"}, "LREAL"},
		{Entry{DataType: "USINT"}, "USINT"},
		{Entry{DataType: "Status_5E676BDB", BitLen: 16}, "WORD"},
		{Entry{BitLen: 1}, "BOOL"},
		{Entry{BitLen: 8}, "BYTE"},
		{Entry{BitLen: 32}, "DWORD"},
		{Entry{BitLen: 64}, "LWORD"},
		{Entry{BitLen: 12}, "BOOL"},
	}
	for _, c := range cases {
		if got := IECType(c.e); got != c.want {
			t.Errorf("IECType(%+v) = %q, want %q", c.e, got, c.want)
		}
	}
}

func TestRoleString(t *testing.T) {
	for r, want := range map[Role]string{RoleOpen: "open", RoleTerminal: "terminal", RoleClose: "close", RolePlain: "plain"} {
		if r.String() != want {
			t.Errorf("%d.String() = %q", r, r.String())
		}
	}
}
