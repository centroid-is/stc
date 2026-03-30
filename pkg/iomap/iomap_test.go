package iomap

import (
	"testing"
)

func TestIOTable_BitRoundTrip(t *testing.T) {
	tbl := NewIOTable()

	// Set bit 3 of byte 0 in Input area
	tbl.SetBit(AreaInput, 0, 3, true)
	if !tbl.GetBit(AreaInput, 0, 3) {
		t.Error("GetBit should return true after SetBit(true)")
	}

	// Clear it
	tbl.SetBit(AreaInput, 0, 3, false)
	if tbl.GetBit(AreaInput, 0, 3) {
		t.Error("GetBit should return false after SetBit(false)")
	}
}

func TestIOTable_ByteRoundTrip(t *testing.T) {
	tbl := NewIOTable()

	tbl.SetByte(AreaOutput, 4, 0xAB)
	got := tbl.GetByte(AreaOutput, 4)
	if got != 0xAB {
		t.Errorf("GetByte = 0x%02X, want 0xAB", got)
	}
}

func TestIOTable_WordRoundTrip(t *testing.T) {
	tbl := NewIOTable()

	tbl.SetWord(AreaInput, 2, 0x1234)
	got := tbl.GetWord(AreaInput, 2)
	if got != 0x1234 {
		t.Errorf("GetWord = 0x%04X, want 0x1234", got)
	}

	// Verify little-endian storage
	lo := tbl.GetByte(AreaInput, 2)
	hi := tbl.GetByte(AreaInput, 3)
	if lo != 0x34 || hi != 0x12 {
		t.Errorf("Little-endian bytes: lo=0x%02X hi=0x%02X, want lo=0x34 hi=0x12", lo, hi)
	}
}

func TestIOTable_DWordRoundTrip(t *testing.T) {
	tbl := NewIOTable()

	tbl.SetDWord(AreaMemory, 48, 0xDEADBEEF)
	got := tbl.GetDWord(AreaMemory, 48)
	if got != 0xDEADBEEF {
		t.Errorf("GetDWord = 0x%08X, want 0xDEADBEEF", got)
	}
}

func TestIOTable_AutoGrow(t *testing.T) {
	tbl := NewIOTable()

	// Write beyond initial 1024 capacity for Input area
	tbl.SetByte(AreaInput, 2000, 0xFF)
	got := tbl.GetByte(AreaInput, 2000)
	if got != 0xFF {
		t.Errorf("GetByte after auto-grow = 0x%02X, want 0xFF", got)
	}
}

func TestIOTable_Reset(t *testing.T) {
	tbl := NewIOTable()

	tbl.SetByte(AreaInput, 0, 0xFF)
	tbl.SetByte(AreaOutput, 0, 0xFF)
	tbl.SetByte(AreaMemory, 0, 0xFF)

	tbl.Reset()

	if tbl.GetByte(AreaInput, 0) != 0 {
		t.Error("Input byte should be 0 after Reset")
	}
	if tbl.GetByte(AreaOutput, 0) != 0 {
		t.Error("Output byte should be 0 after Reset")
	}
	if tbl.GetByte(AreaMemory, 0) != 0 {
		t.Error("Memory byte should be 0 after Reset")
	}
}

func TestIOTable_BitDoesNotAffectOtherBits(t *testing.T) {
	tbl := NewIOTable()

	tbl.SetBit(AreaInput, 0, 3, true)
	tbl.SetBit(AreaInput, 0, 5, true)

	// Verify bit 3 is still set
	if !tbl.GetBit(AreaInput, 0, 3) {
		t.Error("Bit 3 should still be set")
	}
	// Verify bit 5 is set
	if !tbl.GetBit(AreaInput, 0, 5) {
		t.Error("Bit 5 should be set")
	}
	// Verify bit 0 is NOT set
	if tbl.GetBit(AreaInput, 0, 0) {
		t.Error("Bit 0 should not be set")
	}
}

func TestIOTable_WordAutoGrow(t *testing.T) {
	tbl := NewIOTable()

	// Write a word beyond initial capacity
	tbl.SetWord(AreaOutput, 2048, 0xCAFE)
	got := tbl.GetWord(AreaOutput, 2048)
	if got != 0xCAFE {
		t.Errorf("GetWord after auto-grow = 0x%04X, want 0xCAFE", got)
	}
}

func TestIOTable_DWordAutoGrow(t *testing.T) {
	tbl := NewIOTable()

	tbl.SetDWord(AreaMemory, 8192, 0x12345678)
	got := tbl.GetDWord(AreaMemory, 8192)
	if got != 0x12345678 {
		t.Errorf("GetDWord after auto-grow = 0x%08X, want 0x12345678", got)
	}
}
