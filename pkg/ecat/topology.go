// Package ecat loads TwinCAT EtherCATConfig exports ("Export Configuration
// File", Version 1.3) into masters, slaves and the PDO entries that TwinCAT
// places in the process image, and lays those entries out in per-master input
// and output images.
//
// The IO-tree rules (active PDO selection, E-bus nesting, Module segments and
// TcLinkTo link paths) are ported 1:1 from generate_gvl.py, the generator that
// produced the TcLinkTo pragmas in the target projects, so that every link it
// emits resolves against a Topology loaded here.
package ecat

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Limits that keep image allocation bounded for hostile or corrupt exports.
const (
	maxBitLen  = 4096
	maxBitOffs = 1 << 24
)

// Entry is one PDO entry. Name and Index are verbatim from the export.
type Entry struct {
	Name     string // entry name, used verbatim in the link path ("__" splits levels)
	Index    string // CoE index, e.g. "#x6000"; "#x0" marks padding
	SubIndex int
	BitLen   int
	DataType string // EtherCAT data type, "" when the export has none
}

// Padding reports whether the entry is a gap that occupies bits but cannot be
// linked.
func (e Entry) Padding() bool {
	idx := strings.ToLower(strings.TrimSpace(e.Index))
	return idx == "#x0" || e.Name == "" || strings.HasSuffix(e.Name, "__")
}

// Pdo is a process data object assigned to an enabled SyncManager.
type Pdo struct {
	Name    string // PDO name, used verbatim in the link path
	Index   int    // CoE index, e.g. 0x1a00
	Dir     Dir    // DirIn for TxPdo, DirOut for RxPdo
	Entries []Entry
}

// Slave is one EtherCAT box from the export.
type Slave struct {
	Index    int    // position in document (bus) order
	Name     string // full tree name, e.g. "DEMO.A1.01 (EL1008)"
	Model    string // token inside the trailing parentheses, "" if none
	Vendor   uint32
	Product  uint32
	Revision uint32

	HasVendor, HasProduct bool

	Phys    int // Info/PhysAddr
	HasPhys bool
	Physics string // Info/Physics port media, e.g. "YK", "KK", "KY", "YY"

	PrevPhys    int // PreviousPort/PhysAddr
	HasPrevPhys bool
	PrevPort    string // PreviousPort/Port: B = E-bus/line, C/D = branch

	Pdos []Pdo

	Parent *Slave // coupler the slave nests under in the IO tree, nil at master level
	IsEBus bool   // true when the slave sits on a coupler's E-bus
}

// Master is one EtherCAT device ("Device 1 (EtherCAT)") with its slaves and
// process image layout.
type Master struct {
	Name   string
	Slaves []*Slave
	NetID  [6]byte // AmsNetId reported through InfoData^AmsNetId; settable

	InBytes, OutBytes int // process image sizes

	slots []Slot
	index map[string]int
}

// DefaultNetID is the AmsNetId a master reports unless the caller sets one.
var DefaultNetID = [6]byte{192, 168, 0, 1, 1, 1}

// Topology is the set of masters of one project, in load order.
type Topology struct {
	Masters []*Master
}

// Master returns the master with the given name, or nil.
func (t *Topology) Master(name string) *Master {
	for _, m := range t.Masters {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// --- XML mirror -------------------------------------------------------------

type xConfigFile struct {
	Config struct {
		Master struct {
			Name string `xml:"Info>Name"`
		} `xml:"Master"`
		Slaves       []xSlave      `xml:"Slave"`
		ProcessImage *processImage `xml:"ProcessImage"`
	} `xml:"Config"`
}

type xSlave struct {
	Info struct {
		Name        string `xml:"Name"`
		VendorID    string `xml:"VendorId"`
		ProductCode string `xml:"ProductCode"`
		RevisionNo  string `xml:"RevisionNo"`
		PhysAddr    string `xml:"PhysAddr"`
		Physics     string `xml:"Physics"`
	} `xml:"Info"`
	PreviousPort *struct {
		Port     string `xml:"Port"`
		PhysAddr string `xml:"PhysAddr"`
	} `xml:"PreviousPort"`
	ProcessData *struct {
		Tx    []xPdo  `xml:"TxPdo"`
		Rx    []xPdo  `xml:"RxPdo"`
		Other []xAnyS `xml:",any"`
	} `xml:"ProcessData"`
}

// xAnyS captures Sm0..SmN (and Send/Recv, which are skipped).
type xAnyS struct {
	XMLName xml.Name
	Enable  *string  `xml:"Enable"`
	Pdo     []string `xml:"Pdo"`
}

type xPdo struct {
	Index   string   `xml:"Index"`
	Name    string   `xml:"Name"`
	Entries []xEntry `xml:"Entry"`
}

type xEntry struct {
	Index    string `xml:"Index"`
	SubIndex string `xml:"SubIndex"`
	BitLen   string `xml:"BitLen"`
	Name     string `xml:"Name"`
	DataType string `xml:"DataType"`
}

// processImage mirrors Config/ProcessImage, which TwinCAT includes when the
// configuration has been activated. Its BitOffs are authoritative.
type processImage struct {
	Inputs  *piArea `xml:"Inputs"`
	Outputs *piArea `xml:"Outputs"`
}

type piArea struct {
	ByteSize  int          `xml:"ByteSize"`
	Variables []piVariable `xml:"Variable"`
}

type piVariable struct {
	Name     string `xml:"Name"`
	DataType string `xml:"DataType"`
	BitSize  int    `xml:"BitSize"`
	BitOffs  int    `xml:"BitOffs"`
}

// --- parsing helpers --------------------------------------------------------

var (
	modelRe = regexp.MustCompile(`\(([^)]*)\)\s*$`)
	smRe    = regexp.MustCompile(`^Sm\d+$`)
)

// modelOf returns the token inside the trailing parentheses of a box name.
func modelOf(name string) string {
	m := modelRe.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// parseInt accepts decimal or TwinCAT "#x" hex.
func parseInt(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if len(s) > 2 && s[0] == '#' && (s[1] == 'x' || s[1] == 'X') {
		v, err := strconv.ParseInt(s[2:], 16, 64)
		return v, err == nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}

// activePdoIndices ports active_pdo_indices: the PDO indices listed under an
// enabled SmN. An empty result means "no assignment, keep every PDO".
func activePdoIndices(sms []xAnyS) map[int]bool {
	active := map[int]bool{}
	for _, sm := range sms {
		if !smRe.MatchString(sm.XMLName.Local) {
			continue
		}
		if sm.Enable != nil && strings.TrimSpace(*sm.Enable) == "0" {
			continue
		}
		for _, p := range sm.Pdo {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				active[v] = true
			}
		}
	}
	return active
}

func convertSlave(i int, xs xSlave) (*Slave, error) {
	name := strings.TrimSpace(xs.Info.Name)
	s := &Slave{Index: i, Name: name, Model: modelOf(name), Physics: strings.TrimSpace(xs.Info.Physics)}
	if v, ok := parseInt(xs.Info.VendorID); ok {
		s.Vendor, s.HasVendor = uint32(v), true
	}
	if v, ok := parseInt(xs.Info.ProductCode); ok {
		s.Product, s.HasProduct = uint32(v), true
	}
	if v, ok := parseInt(xs.Info.RevisionNo); ok {
		s.Revision = uint32(v)
	}
	if v, ok := parseInt(xs.Info.PhysAddr); ok {
		s.Phys, s.HasPhys = int(v), true
	}
	if pp := xs.PreviousPort; pp != nil {
		s.PrevPort = strings.TrimSpace(pp.Port)
		if v, ok := parseInt(pp.PhysAddr); ok {
			s.PrevPhys, s.HasPrevPhys = int(v), true
		}
	}
	pd := xs.ProcessData
	if pd == nil {
		return s, nil
	}
	active := activePdoIndices(pd.Other)
	for _, grp := range []struct {
		pdos []xPdo
		dir  Dir
	}{{pd.Tx, DirIn}, {pd.Rx, DirOut}} {
		for _, xp := range grp.pdos {
			idx, ok := parseInt(xp.Index)
			// Only PDOs assigned to an enabled SyncManager are in the image.
			if len(active) > 0 && ok && !active[int(idx)] {
				continue
			}
			p := Pdo{Name: strings.TrimSpace(xp.Name), Index: int(idx), Dir: grp.dir}
			for _, xe := range xp.Entries {
				bl, _ := parseInt(xe.BitLen)
				if bl < 0 || bl > maxBitLen {
					return nil, fmt.Errorf("slave %q PDO %q entry %q: BitLen %d out of range 0..%d",
						name, p.Name, xe.Name, bl, maxBitLen)
				}
				sub, _ := parseInt(xe.SubIndex)
				p.Entries = append(p.Entries, Entry{
					Name:     strings.TrimSpace(xe.Name),
					Index:    strings.TrimSpace(xe.Index),
					SubIndex: int(sub),
					BitLen:   int(bl),
					DataType: strings.TrimSpace(xe.DataType),
				})
			}
			s.Pdos = append(s.Pdos, p)
		}
	}
	return s, nil
}

func validateImage(pi *processImage) error {
	if pi == nil {
		return nil
	}
	for _, a := range []*piArea{pi.Inputs, pi.Outputs} {
		if a == nil {
			continue
		}
		if a.ByteSize < 0 || a.ByteSize > maxBitOffs/8 {
			return fmt.Errorf("ProcessImage ByteSize %d out of range", a.ByteSize)
		}
		for _, v := range a.Variables {
			if v.BitOffs < 0 || v.BitOffs > maxBitOffs || v.BitSize < 0 || v.BitSize > maxBitLen {
				return fmt.Errorf("ProcessImage variable %q: BitOffs %d / BitSize %d out of range",
					v.Name, v.BitOffs, v.BitSize)
			}
		}
	}
	return nil
}

// LoadConfig parses one EtherCATConfig export into a Master with its IO-tree
// nesting and process image layout.
func LoadConfig(path string) (*Master, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ecat: %w", err)
	}
	var doc xConfigFile
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("ecat: %s: %w", path, err)
	}
	m := &Master{Name: strings.TrimSpace(doc.Config.Master.Name), NetID: DefaultNetID}
	if m.Name == "" {
		return nil, fmt.Errorf("ecat: %s: no master name (Config/Master/Info/Name)", path)
	}
	for i, xs := range doc.Config.Slaves {
		s, err := convertSlave(i, xs)
		if err != nil {
			return nil, fmt.Errorf("ecat: %s: %w", path, err)
		}
		m.Slaves = append(m.Slaves, s)
	}
	if err := validateImage(doc.Config.ProcessImage); err != nil {
		return nil, fmt.Errorf("ecat: %s: %w", path, err)
	}
	assignParents(m.Slaves)
	buildLayout(m, doc.Config.ProcessImage)
	return m, nil
}

// LoadProject loads several exports (one per master) in argument order.
func LoadProject(paths ...string) (*Topology, error) {
	t := &Topology{}
	for _, p := range paths {
		m, err := LoadConfig(p)
		if err != nil {
			return nil, err
		}
		if t.Master(m.Name) != nil {
			return nil, fmt.Errorf("ecat: %s: duplicate master %q", p, m.Name)
		}
		t.Masters = append(t.Masters, m)
	}
	return t, nil
}
