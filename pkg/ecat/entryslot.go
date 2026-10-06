package ecat

import "strings"

// EntrySlot locates one named PDO entry inside a slave's direction view (the
// out or in slice passed to Device.Step). Byte and Bit are relative to the
// start of that view, so ReadBits(view, e.Byte, e.Bit, e.BitLen) reads it.
type EntrySlot struct {
	Name     string
	Index    uint16
	SubIndex uint8
	BitLen   int
	Dir      Dir
	Byte     int
	Bit      int
}

// LayoutAware is implemented by device models that need the positions of
// their PDO entries. NewNetwork calls SetLayout once, right after Init, with
// the slave's input (TxPdo) and output (RxPdo) entries in PDO order. Padding
// and entries without a slot are omitted.
type LayoutAware interface {
	SetLayout(in, out []EntrySlot)
}

// FindEntry returns the first entry named name (case-insensitive).
func FindEntry(list []EntrySlot, name string) (EntrySlot, bool) {
	for _, e := range list {
		if strings.EqualFold(e.Name, name) {
			return e, true
		}
	}
	return EntrySlot{}, false
}

// Get reads the entry from a direction view; out-of-range reads return 0.
func (e EntrySlot) Get(buf []byte) uint64 {
	return ReadBits(buf, e.Byte, e.Bit, e.BitLen)
}

// Set writes the entry into a direction view; out-of-range writes are ignored.
func (e EntrySlot) Set(buf []byte, v uint64) {
	WriteBits(buf, e.Byte, e.Bit, e.BitLen, v)
}

// parseIndex parses a CoE index such as "#x6041" or "24641"; 0 when invalid.
func parseIndex(s string) uint16 {
	s = strings.ToLower(strings.TrimSpace(s))
	base := 10
	if strings.HasPrefix(s, "#x") {
		s, base = s[2:], 16
	}
	var v uint64
	for _, c := range s {
		var d uint64
		switch {
		case c >= '0' && c <= '9':
			d = uint64(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			d = uint64(c-'a') + 10
		default:
			return 0
		}
		v = v*uint64(base) + d
		if v > 0xFFFF {
			return 0
		}
	}
	return uint16(v)
}

// slaveEntries resolves s's PDO entries to view-relative slots, walking the
// PDOs the same way slaveSpans does.
func slaveEntries(m *Master, s *Slave, in, out span) (ins, outs []EntrySlot) {
	for _, p := range s.Pdos {
		for _, e := range p.Entries {
			if e.Padding() {
				continue
			}
			slot, ok := m.Slot(LinkPath(m.Name, s, p, e))
			if !ok {
				continue
			}
			es := EntrySlot{Name: e.Name, Index: parseIndex(e.Index), SubIndex: uint8(e.SubIndex),
				BitLen: slot.BitLen, Dir: slot.Dir, Bit: slot.Bit}
			if slot.Dir == DirOut {
				es.Byte = slot.Byte - out.lo
				outs = append(outs, es)
			} else {
				es.Byte = slot.Byte - in.lo
				ins = append(ins, es)
			}
		}
	}
	return ins, outs
}

// bindLayout hands a LayoutAware device its entry slots.
func bindLayout(m *Master, rt slaveRT) {
	if la, ok := rt.dev.(LayoutAware); ok {
		la.SetLayout(slaveEntries(m, rt.slave, rt.in, rt.out))
	}
}
