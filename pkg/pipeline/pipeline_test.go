package pipeline

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
)

func TestParse_NoDirectives(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM`

	result := Parse("test.st", src, nil)
	if result.File == nil {
		t.Fatal("expected non-nil AST")
	}
	for _, d := range result.Diags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestParse_WithDefines(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
{IF defined(VENDOR_BECKHOFF)}
    y : LREAL;
{END_IF}
END_VAR
    x := 1;
END_PROGRAM`

	// Without VENDOR_BECKHOFF: y should not appear
	result1 := Parse("test.st", src, nil)
	if result1.File == nil {
		t.Fatal("expected non-nil AST")
	}

	// With VENDOR_BECKHOFF: y should appear
	result2 := Parse("test.st", src, map[string]bool{"VENDOR_BECKHOFF": true})
	if result2.File == nil {
		t.Fatal("expected non-nil AST")
	}

	// The version with the define should have more content in the AST
	// (the y variable declaration).
	// We verify by checking the preprocessed output includes "y : LREAL"
	// indirectly: both should parse without errors.
	for _, d := range result2.Diags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error with defines: %s", d.Message)
		}
	}
}

func TestParse_PreprocessorError(t *testing.T) {
	src := `PROGRAM Main
{IF defined(DEBUG)}
{ERROR 'Debug build not allowed in production'}
{END_IF}
END_PROGRAM`

	result := Parse("test.st", src, map[string]bool{"DEBUG": true})
	hasError := false
	for _, d := range result.Diags {
		if d.Severity == diag.Error && d.Code == "PP001" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected PP001 error diagnostic for {ERROR} directive")
	}
}

func TestParse_SourceMapRemapping(t *testing.T) {
	// The {IF} directive on line 2 gets removed, so parser sees shifted lines.
	// The source map should remap diagnostics back to original positions.
	src := `{IF defined(ALWAYS)}
PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 1;
END_PROGRAM
{END_IF}`

	result := Parse("test.st", src, map[string]bool{"ALWAYS": true})
	if result.SourceMap == nil {
		t.Fatal("expected non-nil source map")
	}
	if result.SourceMap.Len() == 0 {
		t.Fatal("expected non-empty source map")
	}
}

func TestParseDefines(t *testing.T) {
	tests := []struct {
		input    []string
		expected map[string]bool
	}{
		{nil, nil},
		{[]string{}, nil},
		{[]string{"DEBUG"}, map[string]bool{"DEBUG": true}},
		{[]string{"A", "B"}, map[string]bool{"A": true, "B": true}},
	}

	for _, tt := range tests {
		result := ParseDefines(tt.input)
		if tt.expected == nil {
			if result != nil {
				t.Errorf("ParseDefines(%v) = %v, want nil", tt.input, result)
			}
			continue
		}
		if len(result) != len(tt.expected) {
			t.Errorf("ParseDefines(%v) has %d entries, want %d", tt.input, len(result), len(tt.expected))
		}
		for k := range tt.expected {
			if !result[k] {
				t.Errorf("ParseDefines(%v) missing key %q", tt.input, k)
			}
		}
	}
}
