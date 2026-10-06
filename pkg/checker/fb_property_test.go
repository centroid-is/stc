package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
)

func TestFBPropertyMemberAccess(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Motor
VAR
	speedSet : INT;
END_VAR
PROPERTY PUBLIC Speed : INT
GET
	Speed := speedSet;
END_GET
SET
	speedSet := Speed;
END_SET
END_PROPERTY
END_FUNCTION_BLOCK

PROGRAM MAIN
VAR
	m : FB_Motor;
	s : INT;
	b : BOOL;
END_VAR
s := m.Speed;
m.Speed := 3;
b := m.Speed;
s := m.Nope;
END_PROGRAM
`
	ds := runChecker(src)
	var codes []string
	for _, d := range ds {
		codes = append(codes, d.Code+": "+d.Message)
	}
	if countDiagCode(ds, CodeNoMember) != 1 {
		t.Fatalf("want exactly one no-member error (m.Nope), got %v", codes)
	}
	if countDiagCode(ds, CodeTypeMismatch) != 1 {
		t.Fatalf("want one type mismatch (INT property into BOOL), got %v", codes)
	}
}

func countDiagCode(ds []diag.Diagnostic, code string) int {
	n := 0
	for _, d := range ds {
		if d.Code == code {
			n++
		}
	}
	return n
}
