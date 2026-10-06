package ecat

import (
	"sort"
	"strings"
)

// Dir is the process image direction of a PDO or slot.
type Dir int

const (
	DirIn  Dir = iota // TxPdo, slave to master (%I)
	DirOut            // RxPdo, master to slave (%Q)
)

// String returns "in" or "out".
func (d Dir) String() string {
	if d == DirOut {
		return "out"
	}
	return "in"
}

// Slot is the location of one linkable item in a master's process image.
type Slot struct {
	Master   string // master name, e.g. "Device 1 (EtherCAT)"
	Dir      Dir
	Byte     int    // byte offset in the master's In or Out image
	Bit      int    // bit within Byte, 0..7
	BitLen   int    // width in bits
	DataType string // EtherCAT data type, e.g. "BIT", "UINT", "AMSADDR"
	Path     string // TcLinkTo target, "TIID^master^..."
}

// layoutItem is one candidate slot inside a byte-aligned group (a PDO or a
// single pseudo-input).
type layoutItem struct {
	path     string
	piName   string // exact ProcessImage variable name
	group    string // ProcessImage name of the enclosing "__" group, or ""
	groupRel int    // relBit of the group's first entry
	relBit   int    // bit offset inside the group
	bitLen   int
	dataType string
	linkable bool
}

type layoutGroup struct {
	dir   Dir
	bits  int
	items []layoutItem
}

// buildLayout lays out every linkable item of m in bus order: PDO by PDO, bits
// packed in entry order, each PDO byte-aligned, inputs and outputs in separate
// images; then each slave's WcState/InfoData pseudo-inputs, then the master's.
// When the export has a ProcessImage its BitOffs win; items it does not name
// are appended after max(ByteSize, highest PI end).
func buildLayout(m *Master, pi *processImage) {
	var groups []layoutGroup
	for _, s := range m.Slaves {
		for _, p := range s.Pdos {
			groups = append(groups, pdoGroup(m.Name, s, p))
		}
	}
	for _, s := range m.Slaves {
		base := SlaveBasePath(m.Name, s)
		groups = append(groups,
			single(base+"^WcState^WcState", s.Name+".WcState.WcState", 1, "BIT"),
			single(base+"^InfoData^State", s.Name+".InfoData.State", 16, "UINT"),
			single(base+"^InfoData^AdsAddr", s.Name+".InfoData.AdsAddr", 64, "AMSADDR"))
	}
	mb := "TIID^" + m.Name + "^"
	for _, ps := range []struct {
		tail, dt string
		bits     int
	}{
		{"Inputs^DevState", "UINT", 16}, {"Inputs^SlaveCount", "UINT", 16},
		{"Inputs^Frm0State", "UINT", 16}, {"Inputs^Frm0WcState", "UINT", 16},
		{"InfoData^AmsNetId", "AMSNETID", 48}, {"InfoData^ChangeCount", "UINT", 16},
	} {
		groups = append(groups, single(mb+ps.tail, strings.ReplaceAll(ps.tail, "^", "."), ps.bits, ps.dt))
	}

	var areas [2]*piArea
	if pi != nil {
		areas = [2]*piArea{pi.Inputs, pi.Outputs}
	}
	var lookup [2]map[string]piVariable
	var cursor [2]int
	for d, a := range areas {
		lookup[d] = map[string]piVariable{}
		if a == nil {
			continue
		}
		end := a.ByteSize
		for _, v := range a.Variables {
			if _, dup := lookup[d][v.Name]; !dup {
				lookup[d][v.Name] = v
			}
			if e := (v.BitOffs + v.BitSize + 7) / 8; e > end {
				end = e
			}
		}
		cursor[d] = end
	}

	m.slots = nil
	m.index = map[string]int{}
	for _, g := range groups {
		block := -1
		for _, it := range g.items {
			if !it.linkable {
				continue
			}
			slot := Slot{Master: m.Name, Dir: g.dir, BitLen: it.bitLen, DataType: it.dataType, Path: it.path}
			abs := -1
			if v, ok := lookup[g.dir][it.piName]; ok {
				abs, slot.BitLen = v.BitOffs, v.BitSize
				if v.DataType != "" {
					slot.DataType = v.DataType
				}
			} else if v, ok := lookup[g.dir][it.group]; ok && it.group != "" {
				abs = v.BitOffs + it.relBit - it.groupRel
			}
			if abs < 0 {
				if block < 0 {
					block = cursor[g.dir]
					cursor[g.dir] += (g.bits + 7) / 8
				}
				abs = block*8 + it.relBit
			}
			slot.Byte, slot.Bit = abs/8, abs%8
			if _, dup := m.index[it.path]; !dup {
				m.index[it.path] = len(m.slots)
			}
			m.slots = append(m.slots, slot)
		}
	}
	m.InBytes, m.OutBytes = cursor[DirIn], cursor[DirOut]
}

func single(path, piName string, bits int, dt string) layoutGroup {
	return layoutGroup{dir: DirIn, bits: bits, items: []layoutItem{{
		path: path, piName: piName, bitLen: bits, dataType: dt, linkable: true,
	}}}
}

func pdoGroup(master string, s *Slave, p Pdo) layoutGroup {
	prefix := s.Name + "."
	if mod := ModuleSegment(s, p); mod != "" {
		prefix += mod + "."
	}
	prefix += p.Name + "."
	g := layoutGroup{dir: p.Dir}
	groupStart := map[string]int{}
	for _, e := range p.Entries {
		it := layoutItem{relBit: g.bits, bitLen: e.BitLen, dataType: e.DataType, linkable: !e.Padding()}
		if i := strings.Index(e.Name, "__"); i > 0 {
			grp := e.Name[:i]
			if _, seen := groupStart[grp]; !seen {
				groupStart[grp] = g.bits
			}
			it.group, it.groupRel = prefix+grp, groupStart[grp]
		}
		if it.linkable {
			it.path = LinkPath(master, s, p, e)
			it.piName = prefix + strings.ReplaceAll(e.Name, "__", ".")
		}
		g.items = append(g.items, it)
		g.bits += e.BitLen
	}
	return g
}

// Slot resolves a link path within this master.
func (m *Master) Slot(path string) (Slot, bool) {
	i, ok := m.index[path]
	if !ok {
		return Slot{}, false
	}
	return m.slots[i], true
}

// Slots returns every slot in layout order.
func (m *Master) Slots() []Slot {
	return append([]Slot(nil), m.slots...)
}

// Slot resolves a TcLinkTo target "TIID^<master>^..." to its slot. The master
// is selected by the second segment.
func (t *Topology) Slot(path string) (Slot, bool) {
	segs := strings.SplitN(path, "^", 3)
	if len(segs) < 3 {
		return Slot{}, false
	}
	m := t.Master(segs[1])
	if m == nil {
		return Slot{}, false
	}
	return m.Slot(path)
}

// Paths returns every resolvable link path of every master, sorted.
func (t *Topology) Paths() []string {
	var out []string
	for _, m := range t.Masters {
		for p := range m.index {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// Image is one master's input and output process image.
type Image struct {
	In, Out []byte
}

// Images holds the process images of every master of a topology.
type Images struct {
	ByMaster map[string]*Image
	order    []string
}

// NewImages allocates zeroed images sized from each master's layout.
func NewImages(t *Topology) *Images {
	imgs := &Images{ByMaster: map[string]*Image{}}
	for _, m := range t.Masters {
		imgs.ByMaster[m.Name] = &Image{In: make([]byte, m.InBytes), Out: make([]byte, m.OutBytes)}
		imgs.order = append(imgs.order, m.Name)
	}
	return imgs
}

// Get returns the image of the named master, or nil.
func (i *Images) Get(master string) *Image {
	return i.ByMaster[master]
}

// Masters returns the master names in topology order.
func (i *Images) Masters() []string {
	return append([]string(nil), i.order...)
}

func bitsInRange(buf []byte, byteOff, bit, bitLen int) bool {
	if byteOff < 0 || bit < 0 || bit > 7 || bitLen <= 0 || bitLen > 64 {
		return false
	}
	return byteOff*8+bit+bitLen <= len(buf)*8
}

// ReadBits reads a little-endian bitLen-bit value (1..64) starting at bit
// `bit` of byte byteOff. Out-of-range reads return 0.
func ReadBits(buf []byte, byteOff, bit, bitLen int) uint64 {
	if !bitsInRange(buf, byteOff, bit, bitLen) {
		return 0
	}
	var v uint64
	pos := byteOff*8 + bit
	for i := 0; i < bitLen; i++ {
		p := pos + i
		if buf[p/8]&(1<<uint(p%8)) != 0 {
			v |= 1 << uint(i)
		}
	}
	return v
}

// WriteBits writes the low bitLen bits of v little-endian starting at bit
// `bit` of byte byteOff, leaving neighbouring bits untouched. Out-of-range
// writes are ignored.
func WriteBits(buf []byte, byteOff, bit, bitLen int, v uint64) {
	if !bitsInRange(buf, byteOff, bit, bitLen) {
		return
	}
	pos := byteOff*8 + bit
	for i := 0; i < bitLen; i++ {
		p := pos + i
		mask := byte(1 << uint(p%8))
		if v&(1<<uint(i)) != 0 {
			buf[p/8] |= mask
		} else {
			buf[p/8] &^= mask
		}
	}
}
