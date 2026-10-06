package ecat

import (
	"regexp"
	"strings"
)

// Role is a box's place in the E-bus, read from its port media (Info/Physics)
// so that renaming a box never changes nesting. 'K' = E-bus, 'Y' = Ethernet,
// port A first.
type Role int

const (
	RoleOpen     Role = iota // Ethernet-in, E-bus-out: coupler or CX head (YK, YKY)
	RoleTerminal             // all E-bus ports (KK): EL/ES terminal
	RoleClose                // E-bus-in, Ethernet-out (KY): EK1110/EK1120 end extension
	RolePlain                // no E-bus port (YY): master-level EtherCAT box
)

// String returns the generator's role name.
func (r Role) String() string {
	return [...]string{"open", "terminal", "close", "plain"}[r]
}

// Model-prefix fallbacks used only when an export carries no Physics.
var (
	ebusCouplers = regexp.MustCompile(`^EK1(100|101|200|501|521)$`)
	ebusEnd      = regexp.MustCompile(`^EK1(110|120|210|220)$`)
)

// ClassifyRole ports classify_role from generate_gvl.py.
func ClassifyRole(s *Slave) Role {
	ph := strings.ToUpper(s.Physics)
	if ph != "" {
		if strings.Trim(ph, "K") == "" {
			return RoleTerminal
		}
		if !strings.Contains(ph, "K") {
			return RolePlain
		}
		if ph[0] != 'K' {
			return RoleOpen
		}
		return RoleClose
	}
	if ebusEnd.MatchString(s.Model) {
		return RoleClose
	}
	if ebusCouplers.MatchString(s.Model) {
		return RoleOpen
	}
	return RolePlain
}

// assignParents ports assign_parents: terminals nest under the open coupler
// they are daisy-chained to over Port B; a close extension is the last E-bus
// child and ends the segment; a branch (Port C/D) ends the E-bus.
func assignParents(slaves []*Slave) {
	byPhys := map[int]*Slave{}
	for _, s := range slaves {
		if s.HasPhys {
			byPhys[s.Phys] = s
		}
	}
	var coupler, prevInDoc *Slave
	for _, s := range slaves {
		var prev *Slave
		if s.HasPrevPhys {
			prev = byPhys[s.PrevPhys]
		}
		if prev == nil {
			prev = prevInDoc
		}
		port := s.PrevPort
		if port == "" {
			port = "B"
		}
		role := ClassifyRole(s)
		switch {
		case role == RoleClose:
			s.Parent = coupler
			coupler = nil
		case role == RoleOpen:
			s.Parent = nil
			coupler = s
		case role == RoleTerminal && coupler != nil && port == "B" &&
			prev != nil && (prev == coupler || prev.IsEBus):
			s.Parent = coupler
			s.IsEBus = true
		default:
			s.Parent = nil
			if port != "B" {
				coupler = nil
			}
		}
		prevInDoc = s
	}
}

// SlaveBasePath is the tree path down to the slave box:
// TIID^master^[coupler]^slave.
func SlaveBasePath(master string, s *Slave) string {
	segs := []string{"TIID", master}
	if s.Parent != nil {
		segs = append(segs, s.Parent.Name)
	}
	segs = append(segs, s.Name)
	return strings.Join(segs, "^")
}

// Vendor IDs used as keys in moduleMap.
const (
	vendorBeckhoff = 2
	vendorFesto    = 29
)

type moduleKey struct {
	vendor, product uint32
	pdo             string
}

// moduleMap ports MODULE_MAP: modular slaves nest some PDOs under a
// "Module N (...)" level that the export does not contain. Keyed by device
// identity (VendorId, ProductCode) plus PDO name.
var moduleMap = map[moduleKey]string{
	// EL2912 (TwinSAFE output): field-voltage diagnostics under Module 3.
	{vendorBeckhoff, 190853202, "FIELDVOLTAGE Field Voltage Status"}: "Module 3 (DEVICEIO)",
	// CTEU-EtherCAT Modular (Festo valve terminal): outputs under Module 1.
	{vendorFesto, 572556, "Outputs"}: "Module 1 (VAEM-L1-S-8-PT [16DO])",
}

// ModuleSegment returns the Module level inserted between the slave box and
// the PDO, or "" when the slave is not modular.
func ModuleSegment(s *Slave, p Pdo) string {
	if !s.HasVendor || !s.HasProduct {
		return ""
	}
	return moduleMap[moduleKey{s.Vendor, s.Product, p.Name}]
}

// LinkPath ports link_path: the TcLinkTo target of one PDO entry. Beckhoff
// flattens nested entries with "__"; in the IO tree those are separate levels.
func LinkPath(master string, s *Slave, p Pdo, e Entry) string {
	segs := []string{SlaveBasePath(master, s)}
	if mod := ModuleSegment(s, p); mod != "" {
		segs = append(segs, mod)
	}
	segs = append(segs, p.Name, strings.ReplaceAll(e.Name, "__", "^"))
	return strings.Join(segs, "^")
}

var iecTypeMap = map[string]string{
	"BIT": "BOOL", "BOOL": "BOOL",
	"SINT": "SINT", "USINT": "USINT", "BYTE": "BYTE",
	"INT": "INT", "UINT": "UINT", "WORD": "WORD",
	"DINT": "DINT", "UDINT": "UDINT", "DWORD": "DWORD",
	"LINT": "LINT", "ULINT": "ULINT", "LWORD": "LWORD",
	"REAL": "REAL", "LREAL": "LREAL",
}

var bitLenTypeMap = map[int]string{1: "BOOL", 8: "BYTE", 16: "WORD", 32: "DWORD", 64: "LWORD"}

// IECType ports iec_type: the IEC type for an entry, falling back to a
// size-based type, else BOOL.
func IECType(e Entry) string {
	if t, ok := iecTypeMap[e.DataType]; ok {
		return t
	}
	if t, ok := bitLenTypeMap[e.BitLen]; ok {
		return t
	}
	return "BOOL"
}
