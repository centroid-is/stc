package iomap

import (
	"testing"
)

func TestParseAddress_ValidBitAddresses(t *testing.T) {
	tests := []struct {
		input      string
		area       Area
		size       Size
		byteOffset int
		bitOffset  int
	}{
		{"%IX0.0", AreaInput, SizeBit, 0, 0},
		{"%IX0.7", AreaInput, SizeBit, 0, 7},
		{"%QX0.0", AreaOutput, SizeBit, 0, 0},
		{"%I0.0", AreaInput, SizeBit, 0, 0},   // optional X
		{"%ix0.0", AreaInput, SizeBit, 0, 0},   // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			addr, err := ParseAddress(tt.input)
			if err != nil {
				t.Fatalf("ParseAddress(%q) error: %v", tt.input, err)
			}
			if addr.Area != tt.area {
				t.Errorf("Area = %c, want %c", addr.Area, tt.area)
			}
			if addr.Size != tt.size {
				t.Errorf("Size = %c, want %c", addr.Size, tt.size)
			}
			if addr.ByteOffset != tt.byteOffset {
				t.Errorf("ByteOffset = %d, want %d", addr.ByteOffset, tt.byteOffset)
			}
			if addr.BitOffset != tt.bitOffset {
				t.Errorf("BitOffset = %d, want %d", addr.BitOffset, tt.bitOffset)
			}
			if addr.IsWildcard {
				t.Error("IsWildcard should be false")
			}
		})
	}
}

func TestParseAddress_ValidNonBitAddresses(t *testing.T) {
	tests := []struct {
		input      string
		area       Area
		size       Size
		byteOffset int
	}{
		{"%IB0", AreaInput, SizeByte, 0},
		{"%IW0", AreaInput, SizeWord, 0},
		{"%IW2", AreaInput, SizeWord, 2},
		{"%ID0", AreaInput, SizeDWord, 0},
		{"%QB4", AreaOutput, SizeByte, 4},
		{"%QW8", AreaOutput, SizeWord, 8},
		{"%MD48", AreaMemory, SizeDWord, 48},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			addr, err := ParseAddress(tt.input)
			if err != nil {
				t.Fatalf("ParseAddress(%q) error: %v", tt.input, err)
			}
			if addr.Area != tt.area {
				t.Errorf("Area = %c, want %c", addr.Area, tt.area)
			}
			if addr.Size != tt.size {
				t.Errorf("Size = %c, want %c", addr.Size, tt.size)
			}
			if addr.ByteOffset != tt.byteOffset {
				t.Errorf("ByteOffset = %d, want %d", addr.ByteOffset, tt.byteOffset)
			}
			if addr.BitOffset != 0 {
				t.Errorf("BitOffset = %d, want 0", addr.BitOffset)
			}
		})
	}
}

func TestParseAddress_Wildcards(t *testing.T) {
	tests := []struct {
		input string
		area  Area
	}{
		{"%I*", AreaInput},
		{"%Q*", AreaOutput},
		{"%M*", AreaMemory},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			addr, err := ParseAddress(tt.input)
			if err != nil {
				t.Fatalf("ParseAddress(%q) error: %v", tt.input, err)
			}
			if addr.Area != tt.area {
				t.Errorf("Area = %c, want %c", addr.Area, tt.area)
			}
			if !addr.IsWildcard {
				t.Error("IsWildcard should be true")
			}
		})
	}
}

func TestParseAddress_Invalid(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"%ZZ0", "invalid area"},
		{"%IX0.8", "bit offset > 7"},
		{"%IW-1", "negative offset"},
		{"IX0.0", "missing % prefix"},
		{"", "empty string"},
		{"%", "just percent"},
		{"%I", "missing offset"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := ParseAddress(tt.input)
			if err == nil {
				t.Errorf("ParseAddress(%q) should return error for %s", tt.input, tt.desc)
			}
		})
	}
}

func TestParseAddress_String(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"%IX0.0", "%IX0.0"},
		{"%QW4", "%QW4"},
		{"%MD48", "%MD48"},
		{"%IB0", "%IB0"},
		{"%I0.0", "%IX0.0"}, // normalized to include X
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			addr, err := ParseAddress(tt.input)
			if err != nil {
				t.Fatalf("ParseAddress(%q) error: %v", tt.input, err)
			}
			got := addr.String()
			if got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIOAddress_ByteSpan(t *testing.T) {
	tests := []struct {
		size       Size
		byteOffset int
		wantOff    int
		wantLen    int
	}{
		{SizeBit, 3, 3, 1},
		{SizeByte, 5, 5, 1},
		{SizeWord, 2, 2, 2},
		{SizeDWord, 8, 8, 4},
	}

	for _, tt := range tests {
		addr := IOAddress{Area: AreaInput, Size: tt.size, ByteOffset: tt.byteOffset}
		off, length := addr.ByteSpan()
		if off != tt.wantOff || length != tt.wantLen {
			t.Errorf("ByteSpan() for size %c = (%d,%d), want (%d,%d)", tt.size, off, length, tt.wantOff, tt.wantLen)
		}
	}
}
