package allen_bradley_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// TestABStubsParse verifies that all .st stub files parse without errors.
func TestABStubsParse(t *testing.T) {
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

func TestABStubDeclarationCounts(t *testing.T) {
	tests := []struct {
		file     string
		minDecls int
	}{
		{"profile.st", 4},      // TIMER, COUNTER, CONTROL types + MSG FB
		{"timers.st", 3},       // TONR, TOFR, RTO
		{"instructions.st", 12}, // ADD, SUB, MUL, DIV, MOV, CMP, EQU, NEQ, GRT, LES, GEQ, LEQ
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
