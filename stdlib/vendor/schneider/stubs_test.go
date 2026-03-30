package schneider_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// TestSchneiderStubsParse verifies that all .st stub files parse without errors.
func TestSchneiderStubsParse(t *testing.T) {
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

func TestSchneiderStubDeclarationCounts(t *testing.T) {
	tests := []struct {
		file     string
		minDecls int
	}{
		{"motion.st", 4},        // SE_AXIS_REF type + 3 motion FBs
		{"communication.st", 4}, // READ_VAR, WRITE_VAR, SEND_REQ, RCV_REQ
		{"system.st", 3},        // GetBit, SetBit, RTC
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
				t.Errorf("expected at least %d declarations, got %d", tt.minDecls, got)
			}
		})
	}
}
