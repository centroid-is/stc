package iomap

import "encoding/binary"

// IOTable provides flat byte-array storage for the three IEC 61131-3
// I/O memory areas: Input (%I), Output (%Q), and Memory/Marker (%M).
// All multi-byte values are stored in little-endian byte order per IEC convention.
type IOTable struct {
	I []byte // Input process image
	Q []byte // Output process image
	M []byte // Memory (marker) area
}

// NewIOTable creates an IOTable with default initial capacities:
// I=1024, Q=1024, M=4096 bytes.
func NewIOTable() *IOTable {
	return &IOTable{
		I: make([]byte, 1024),
		Q: make([]byte, 1024),
		M: make([]byte, 4096),
	}
}

// area returns the byte slice for the given area.
func (t *IOTable) area(a Area) *[]byte {
	switch a {
	case AreaInput:
		return &t.I
	case AreaOutput:
		return &t.Q
	case AreaMemory:
		return &t.M
	default:
		return &t.I
	}
}

// ensureCapacity grows the area slice if needed to accommodate the given byte count.
func (t *IOTable) ensureCapacity(a Area, needed int) {
	s := t.area(a)
	if needed <= len(*s) {
		return
	}
	// Grow to at least double current size or needed, whichever is larger
	newCap := len(*s) * 2
	if newCap < needed {
		newCap = needed
	}
	grown := make([]byte, newCap)
	copy(grown, *s)
	*s = grown
}

// GetBit returns the value of a single bit in the specified area.
func (t *IOTable) GetBit(a Area, byteOff, bitOff int) bool {
	t.ensureCapacity(a, byteOff+1)
	s := *t.area(a)
	return s[byteOff]&(1<<uint(bitOff)) != 0
}

// SetBit sets or clears a single bit in the specified area.
func (t *IOTable) SetBit(a Area, byteOff, bitOff int, v bool) {
	t.ensureCapacity(a, byteOff+1)
	s := *t.area(a)
	if v {
		s[byteOff] |= 1 << uint(bitOff)
	} else {
		s[byteOff] &^= 1 << uint(bitOff)
	}
}

// GetByte returns the byte at the given offset in the specified area.
func (t *IOTable) GetByte(a Area, off int) byte {
	t.ensureCapacity(a, off+1)
	return (*t.area(a))[off]
}

// SetByte sets the byte at the given offset in the specified area.
func (t *IOTable) SetByte(a Area, off int, v byte) {
	t.ensureCapacity(a, off+1)
	(*t.area(a))[off] = v
}

// GetWord returns a 16-bit word (little-endian) at the given byte offset.
func (t *IOTable) GetWord(a Area, off int) uint16 {
	t.ensureCapacity(a, off+2)
	s := *t.area(a)
	return binary.LittleEndian.Uint16(s[off:])
}

// SetWord sets a 16-bit word (little-endian) at the given byte offset.
func (t *IOTable) SetWord(a Area, off int, v uint16) {
	t.ensureCapacity(a, off+2)
	s := *t.area(a)
	binary.LittleEndian.PutUint16(s[off:], v)
}

// GetDWord returns a 32-bit double word (little-endian) at the given byte offset.
func (t *IOTable) GetDWord(a Area, off int) uint32 {
	t.ensureCapacity(a, off+4)
	s := *t.area(a)
	return binary.LittleEndian.Uint32(s[off:])
}

// SetDWord sets a 32-bit double word (little-endian) at the given byte offset.
func (t *IOTable) SetDWord(a Area, off int, v uint32) {
	t.ensureCapacity(a, off+4)
	s := *t.area(a)
	binary.LittleEndian.PutUint32(s[off:], v)
}

// Reset zeroes all three I/O areas, preserving their current capacity.
func (t *IOTable) Reset() {
	clear(t.I)
	clear(t.Q)
	clear(t.M)
}
