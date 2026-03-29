package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseTestFile(t *testing.T, filename string) *ast.SourceFile {
	t.Helper()
	content, err := os.ReadFile(filename)
	require.NoError(t, err, "reading test file %s", filename)
	result := parser.Parse(filename, string(content))
	return result.File
}

func countErrors(diags []diag.Diagnostic) int {
	count := 0
	for _, d := range diags {
		if d.Severity == diag.Error {
			count++
		}
	}
	return count
}

func countWarnings(diags []diag.Diagnostic) int {
	count := 0
	for _, d := range diags {
		if d.Severity == diag.Warning {
			count++
		}
	}
	return count
}

func hasDiagCode(diags []diag.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestAnalyzeSingleFile(t *testing.T) {
	// A valid single-file program should produce 0 errors
	src := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	result := parser.Parse("test.st", src)
	require.NotNil(t, result.File)

	analysis := Analyze([]*ast.SourceFile{result.File}, nil)
	errors := countErrors(analysis.Diags)
	assert.Equal(t, 0, errors, "valid single-file program should have 0 errors, got diags: %v", analysis.Diags)
	assert.NotNil(t, analysis.Symbols, "symbol table should be populated")
}

func TestAnalyzeCrossFile(t *testing.T) {
	// Parse multi_file_a.st (declares FB_Motor) and multi_file_b.st (uses FB_Motor)
	fileA := parseTestFile(t, filepath.Join("testdata", "multi_file_a.st"))
	fileB := parseTestFile(t, filepath.Join("testdata", "multi_file_b.st"))

	analysis := Analyze([]*ast.SourceFile{fileA, fileB}, nil)

	// Should have no undeclared errors - FB_Motor from file A should resolve in file B
	for _, d := range analysis.Diags {
		if d.Severity == diag.Error && d.Code == "SEMA010" {
			t.Errorf("unexpected undeclared error: %s", d.Message)
		}
	}

	errors := countErrors(analysis.Diags)
	assert.Equal(t, 0, errors, "cross-file resolution should produce 0 errors, got diags: %v", analysis.Diags)
}

func TestAnalyzeWithTypeMismatch(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
    s : STRING;
END_VAR
    x := s;
END_PROGRAM
`
	result := parser.Parse("type_error.st", src)
	require.NotNil(t, result.File)

	analysis := Analyze([]*ast.SourceFile{result.File}, nil)
	assert.True(t, hasDiagCode(analysis.Diags, "SEMA001"),
		"should have SEMA001 type mismatch diagnostic, got: %v", analysis.Diags)

	// Verify position is set
	for _, d := range analysis.Diags {
		if d.Code == "SEMA001" {
			assert.Greater(t, d.Pos.Line, 0, "diagnostic should have line number")
		}
	}
}

func TestAnalyzeWithVendor(t *testing.T) {
	// Parse vendor_test.st which uses METHOD (OOP)
	file := parseTestFile(t, filepath.Join("testdata", "vendor_test.st"))

	cfg := &project.Config{
		Build: project.BuildConfig{
			VendorTarget: "schneider",
		},
	}
	analysis := Analyze([]*ast.SourceFile{file}, cfg)

	// Schneider does not support OOP, so expect VEND001 warning
	assert.True(t, hasDiagCode(analysis.Diags, "VEND001"),
		"should have VEND001 vendor OOP warning, got: %v", analysis.Diags)

	// Vendor warnings should not be errors
	for _, d := range analysis.Diags {
		if d.Code == "VEND001" {
			assert.Equal(t, diag.Warning, d.Severity,
				"vendor diagnostics should be warnings, not errors")
		}
	}
}

func TestAnalyzeNilConfig(t *testing.T) {
	src := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	result := parser.Parse("nil_config.st", src)
	require.NotNil(t, result.File)

	// Should not panic with nil config
	analysis := Analyze([]*ast.SourceFile{result.File}, nil)
	errors := countErrors(analysis.Diags)
	assert.Equal(t, 0, errors, "nil config should work without vendor checks")
}

