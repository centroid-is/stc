package beckhoff_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// TestBeckhoffStubsParse verifies that all .st stub files in this directory
// parse without errors through the stc parser pipeline.
func TestBeckhoffStubsParse(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading directory: %v", err)
	}

	stFiles := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".st") {
			continue
		}
		stFiles++
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join(dir, e.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			result := pipeline.Parse(path, string(content), nil)

			// Check for parse errors (filter out warnings)
			var errors []string
			for _, d := range result.Diags {
				if d.Severity == diag.Error {
					errors = append(errors, d.Message)
				}
			}

			if len(errors) > 0 {
				for _, e := range errors {
					t.Errorf("parse error: %s", e)
				}
			}

			if result.File == nil {
				t.Fatalf("parsed file is nil")
			}

			if len(result.File.Declarations) == 0 {
				t.Errorf("no declarations found in %s", e.Name())
			}
		})
	}

	if stFiles == 0 {
		t.Fatal("no .st files found in directory")
	}
}

// TestBeckhoffStubDeclarationCounts verifies the expected number of
// declarations in each stub file.
func TestBeckhoffStubDeclarationCounts(t *testing.T) {
	tests := []struct {
		file     string
		minDecls int
		desc     string
	}{
		{"tc2_mc2.st", 10, "10 motion control FBs"},
		{"tc2_system.st", 9, "6 FBs + 3 functions (MEMCPY, MEMSET, MEMMOVE)"},
		{"tc2_utilities.st", 3, "1 FB + 2 functions (CRC16, CRC32)"},
		{"tc3_eventlogger.st", 2, "2 event logger FBs"},
		{"common_types.st", 4, "AXIS_REF, MC_Direction, T_AmsNetId/Port, E_OpenPath types"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			content, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("reading %s: %v", tt.file, err)
			}

			result := pipeline.Parse(tt.file, string(content), nil)
			if result.File == nil {
				t.Fatalf("parsed file is nil")
			}

			got := len(result.File.Declarations)
			if got < tt.minDecls {
				t.Errorf("expected at least %d declarations (%s), got %d",
					tt.minDecls, tt.desc, got)
			}
		})
	}
}
