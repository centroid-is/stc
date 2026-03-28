package format

import (
	"strings"
	"testing"
)

func TestFormatRoundTripLeadingLineComment(t *testing.T) {
	input := "PROGRAM Main\nVAR\n    // sensor input\n    x : INT;\nEND_VAR\nEND_PROGRAM\n"
	got := formatST(input)
	if !strings.Contains(got, "// sensor input") {
		t.Errorf("expected output to contain '// sensor input', got:\n%s", got)
	}
}

func TestFormatRoundTripTrailingBlockComment(t *testing.T) {
	input := "PROGRAM Main\nVAR\n    x : INT; (* units *)\nEND_VAR\nEND_PROGRAM\n"
	got := formatST(input)
	if !strings.Contains(got, "(* units *)") {
		t.Errorf("expected output to contain '(* units *)', got:\n%s", got)
	}
}

func TestFormatRoundTripMultipleLineComments(t *testing.T) {
	input := `PROGRAM Main
VAR
    // first comment
    x : INT;
    // second comment
    y : REAL;
END_VAR
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "// first comment") {
		t.Errorf("expected '// first comment' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "// second comment") {
		t.Errorf("expected '// second comment' in output, got:\n%s", got)
	}
}

func TestFormatRoundTripFileHeaderComment(t *testing.T) {
	input := "// file header\nPROGRAM Main\nEND_PROGRAM\n"
	got := formatST(input)
	if !strings.Contains(got, "// file header") {
		t.Errorf("expected output to contain '// file header', got:\n%s", got)
	}
}

func TestFormatRoundTripIdempotentWithComments(t *testing.T) {
	inputs := []string{
		"PROGRAM Main\nVAR\n    // sensor input\n    x : INT;\nEND_VAR\nEND_PROGRAM\n",
		"PROGRAM Main\nVAR\n    x : INT; (* units *)\nEND_VAR\nEND_PROGRAM\n",
		"// file header\nPROGRAM Main\nEND_PROGRAM\n",
		"PROGRAM Main\nVAR\n    // comment 1\n    x : INT;\n    // comment 2\n    y : REAL;\nEND_VAR\nEND_PROGRAM\n",
	}

	for i, input := range inputs {
		first := formatST(input)
		if first == "" {
			t.Fatalf("input %d: first format returned empty", i)
		}
		second := formatSTWith(first, DefaultFormatOptions())
		if first != second {
			t.Errorf("input %d: not idempotent with comments.\nFirst:\n%s\nSecond:\n%s", i, first, second)
		}
	}
}

func TestFormatRoundTripBodyComment(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42; (* set value *)
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "(* set value *)") {
		t.Errorf("expected output to contain '(* set value *)', got:\n%s", got)
	}
}
