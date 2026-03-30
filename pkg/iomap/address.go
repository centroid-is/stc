// Package iomap provides I/O address parsing and a flat byte-array I/O table
// for IEC 61131-3 direct addressing (%I, %Q, %M areas).
package iomap

import (
	"fmt"
	"strconv"
	"strings"
)

// Area identifies the I/O memory area.
type Area byte

const (
	AreaInput  Area = 'I' // %I — input process image
	AreaOutput Area = 'Q' // %Q — output process image
	AreaMemory Area = 'M' // %M — memory (marker) area
)

// Size identifies the data width of an I/O address.
type Size byte

const (
	SizeBit   Size = 'X' // 1 bit
	SizeByte  Size = 'B' // 8 bits
	SizeWord  Size = 'W' // 16 bits
	SizeDWord Size = 'D' // 32 bits
)

// IOAddress represents a parsed IEC 61131-3 direct address such as %IX0.0 or %QW4.
type IOAddress struct {
	Area       Area
	Size       Size
	ByteOffset int
	BitOffset  int
	IsWildcard bool
}

// ByteSpan returns the byte offset and byte count for this address.
// For bit addresses, the byte count is 1 (the byte containing the bit).
func (a IOAddress) ByteSpan() (offset int, length int) {
	switch a.Size {
	case SizeBit:
		return a.ByteOffset, 1
	case SizeByte:
		return a.ByteOffset, 1
	case SizeWord:
		return a.ByteOffset, 2
	case SizeDWord:
		return a.ByteOffset, 4
	default:
		return a.ByteOffset, 1
	}
}

// String returns the canonical IEC 61131-3 address string.
// Bit addresses always include the X size prefix.
func (a IOAddress) String() string {
	if a.IsWildcard {
		return fmt.Sprintf("%%%c*", a.Area)
	}
	if a.Size == SizeBit {
		return fmt.Sprintf("%%%c%c%d.%d", a.Area, a.Size, a.ByteOffset, a.BitOffset)
	}
	return fmt.Sprintf("%%%c%c%d", a.Area, a.Size, a.ByteOffset)
}

// ParseAddress parses an IEC 61131-3 direct address string.
// It accepts forms like %IX0.0, %QW4, %MD48, %I*, %I0.0 (optional X).
// Parsing is case-insensitive.
func ParseAddress(s string) (IOAddress, error) {
	if len(s) == 0 {
		return IOAddress{}, fmt.Errorf("empty address string")
	}
	if s[0] != '%' {
		return IOAddress{}, fmt.Errorf("address must start with '%%', got %q", s)
	}
	if len(s) < 3 {
		return IOAddress{}, fmt.Errorf("address too short: %q", s)
	}

	upper := strings.ToUpper(s)
	pos := 1 // skip %

	// Parse area letter
	var addr IOAddress
	switch upper[pos] {
	case 'I':
		addr.Area = AreaInput
	case 'Q':
		addr.Area = AreaOutput
	case 'M':
		addr.Area = AreaMemory
	default:
		return IOAddress{}, fmt.Errorf("invalid area letter %q in %q, expected I, Q, or M", string(s[pos]), s)
	}
	pos++

	// Check for wildcard
	if pos < len(upper) && upper[pos] == '*' {
		addr.IsWildcard = true
		return addr, nil
	}

	// Parse optional size letter
	hasSizePrefix := false
	if pos < len(upper) {
		switch upper[pos] {
		case 'X':
			addr.Size = SizeBit
			hasSizePrefix = true
			pos++
		case 'B':
			addr.Size = SizeByte
			hasSizePrefix = true
			pos++
		case 'W':
			addr.Size = SizeWord
			hasSizePrefix = true
			pos++
		case 'D':
			addr.Size = SizeDWord
			hasSizePrefix = true
			pos++
		}
	}

	// Must have digits remaining
	if pos >= len(upper) {
		return IOAddress{}, fmt.Errorf("missing offset in address %q", s)
	}

	// Check for negative offset (the '-' char)
	if upper[pos] == '-' {
		return IOAddress{}, fmt.Errorf("negative offset not allowed in address %q", s)
	}

	// Find the dot position (if any) for bit offset
	rest := upper[pos:]
	dotIdx := strings.IndexByte(rest, '.')

	if dotIdx >= 0 {
		// Has a dot — parse byte offset and bit offset
		byteStr := rest[:dotIdx]
		bitStr := rest[dotIdx+1:]

		byteOff, err := strconv.Atoi(byteStr)
		if err != nil {
			return IOAddress{}, fmt.Errorf("invalid byte offset in %q: %w", s, err)
		}
		bitOff, err := strconv.Atoi(bitStr)
		if err != nil {
			return IOAddress{}, fmt.Errorf("invalid bit offset in %q: %w", s, err)
		}

		if bitOff < 0 || bitOff > 7 {
			return IOAddress{}, fmt.Errorf("bit offset %d out of range 0-7 in %q", bitOff, s)
		}

		// If no explicit size prefix and we have a dot, it is a bit address
		if !hasSizePrefix {
			addr.Size = SizeBit
		}

		// Bit offset only valid for bit-size addresses
		if addr.Size != SizeBit {
			return IOAddress{}, fmt.Errorf("bit offset not valid for size %c in %q", addr.Size, s)
		}

		addr.ByteOffset = byteOff
		addr.BitOffset = bitOff
	} else {
		// No dot — parse byte offset only
		byteOff, err := strconv.Atoi(rest)
		if err != nil {
			return IOAddress{}, fmt.Errorf("invalid byte offset in %q: %w", s, err)
		}

		if !hasSizePrefix {
			// No size prefix, no dot — default to byte
			addr.Size = SizeByte
		}

		addr.ByteOffset = byteOff
	}

	if addr.ByteOffset < 0 {
		return IOAddress{}, fmt.Errorf("negative byte offset in address %q", s)
	}

	return addr, nil
}
