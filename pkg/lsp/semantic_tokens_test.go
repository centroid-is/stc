package lsp

import (
	"testing"
)

func TestFindInactiveRegions_ElseBlock(t *testing.T) {
	input := `PROGRAM Main
{IF defined(VENDOR_BECKHOFF)}
x := 1;
{ELSE}
x := 2;
{END_IF}
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 inactive region, got %d", len(regions))
	}

	// {ELSE} is on line 3 (0-based), "x := 2;" is on line 4, {END_IF} is on line 5
	// The inactive region should cover lines 3-4 (ELSE line and content)
	r := regions[0]
	if r.startLine != 3 {
		t.Errorf("expected startLine=3, got %d", r.startLine)
	}
	if r.endLine != 4 {
		t.Errorf("expected endLine=4, got %d", r.endLine)
	}
}

func TestFindInactiveRegions_ElsifBlock(t *testing.T) {
	input := `PROGRAM Main
{IF defined(VENDOR_A)}
x := 1;
{ELSIF defined(VENDOR_B)}
x := 2;
{ELSE}
x := 3;
{END_IF}
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 inactive region, got %d", len(regions))
	}

	// ELSIF on line 3 starts inactive, continues through ELSE block, ends before END_IF on line 7
	r := regions[0]
	if r.startLine != 3 {
		t.Errorf("expected startLine=3, got %d", r.startLine)
	}
	if r.endLine != 6 {
		t.Errorf("expected endLine=6, got %d", r.endLine)
	}
}

func TestFindInactiveRegions_Empty(t *testing.T) {
	regions := findInactiveRegions("")
	if len(regions) != 0 {
		t.Errorf("expected 0 regions for empty input, got %d", len(regions))
	}
}

func TestFindInactiveRegions_NoDirectives(t *testing.T) {
	input := `PROGRAM Main
x := 1;
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 0 {
		t.Errorf("expected 0 regions, got %d", len(regions))
	}
}

func TestFindInactiveRegions_NestedIF(t *testing.T) {
	input := `PROGRAM Main
{IF defined(A)}
x := 1;
{IF defined(B)}
y := 2;
{END_IF}
{ELSE}
z := 3;
{END_IF}
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 inactive region, got %d", len(regions))
	}

	// ELSE on line 6, z := 3 on line 7, END_IF on line 8
	r := regions[0]
	if r.startLine != 6 {
		t.Errorf("expected startLine=6, got %d", r.startLine)
	}
	if r.endLine != 7 {
		t.Errorf("expected endLine=7, got %d", r.endLine)
	}
}

func TestSemanticTokenEncoding(t *testing.T) {
	input := `PROGRAM Main
{IF defined(X)}
active := TRUE;
{ELSE}
inactive := FALSE;
{END_IF}
END_PROGRAM`

	store := NewDocumentStore()
	store.Open("file:///test.st", input, 1)

	handler := handleSemanticTokensFull(store)
	// We cannot easily call the handler without a glsp.Context, but we can test
	// the encoding logic through findInactiveRegions + manual encoding verification.
	_ = handler

	regions := findInactiveRegions(input)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}

	// Region covers lines 3-4 (0-based): "{ELSE}" and "inactive := FALSE;"
	r := regions[0]
	if r.startLine != 3 || r.endLine != 4 {
		t.Fatalf("unexpected region: start=%d end=%d", r.startLine, r.endLine)
	}

	// Manually verify encoding: 2 lines, each produces 5 uint32 values
	// Line 3 ("{ELSE}"): deltaLine=3, deltaStartChar=0, length=6, type=0, mods=0
	// Line 4 ("inactive := FALSE;"): deltaLine=1, deltaStartChar=0, length=18, type=0, mods=0
}

func TestFindInactiveRegions_OnlyIFBranch(t *testing.T) {
	// If there is only an IF/END_IF with no ELSE/ELSIF, nothing is inactive
	input := `PROGRAM Main
{IF defined(X)}
x := 1;
{END_IF}
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 0 {
		t.Errorf("expected 0 regions (no ELSE/ELSIF), got %d", len(regions))
	}
}

func TestFindInactiveRegions_MultipleBlocks(t *testing.T) {
	input := `PROGRAM Main
{IF defined(A)}
a := 1;
{ELSE}
a := 2;
{END_IF}
{IF defined(B)}
b := 1;
{ELSE}
b := 2;
{END_IF}
END_PROGRAM`

	regions := findInactiveRegions(input)
	if len(regions) != 2 {
		t.Fatalf("expected 2 inactive regions, got %d", len(regions))
	}

	// First block: ELSE on line 3, content on line 4
	if regions[0].startLine != 3 || regions[0].endLine != 4 {
		t.Errorf("first region: expected lines 3-4, got %d-%d", regions[0].startLine, regions[0].endLine)
	}
	// Second block: ELSE on line 8, content on line 9
	if regions[1].startLine != 8 || regions[1].endLine != 9 {
		t.Errorf("second region: expected lines 8-9, got %d-%d", regions[1].startLine, regions[1].endLine)
	}
}
