package beckhoff_mocks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// TestBeckhoffMocksParse verifies that all mock .st files parse without errors.
func TestBeckhoffMocksParse(t *testing.T) {
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
		})
	}

	if stFiles == 0 {
		t.Fatal("no .st files found in directory")
	}
}

// TestBeckhoffMocksHaveBodies verifies that all mock FBs have non-empty bodies.
func TestBeckhoffMocksHaveBodies(t *testing.T) {
	files := []string{"mc_mocks.st", "ads_mock.st"}

	for _, fname := range files {
		t.Run(fname, func(t *testing.T) {
			content, err := os.ReadFile(fname)
			if err != nil {
				t.Fatalf("reading %s: %v", fname, err)
			}

			result := pipeline.Parse(fname, string(content), nil)
			if result.File == nil {
				t.Fatalf("parsed file is nil")
			}

			for _, decl := range result.File.Declarations {
				fb, ok := decl.(*ast.FunctionBlockDecl)
				if !ok || fb.Name == nil {
					continue
				}

				if len(fb.Body) == 0 {
					t.Errorf("mock FB %s has empty body -- behavioral mocks must have implementations", fb.Name.Name)
				}
			}
		})
	}
}

// TestMcMocksContainExpectedFBs verifies the mc_mocks.st file has all 4 expected mocks.
func TestMcMocksContainExpectedFBs(t *testing.T) {
	content, err := os.ReadFile("mc_mocks.st")
	if err != nil {
		t.Fatalf("reading mc_mocks.st: %v", err)
	}

	result := pipeline.Parse("mc_mocks.st", string(content), nil)
	if result.File == nil {
		t.Fatalf("parsed file is nil")
	}

	expected := map[string]bool{
		"MC_MoveAbsolute": false,
		"MC_Power":        false,
		"MC_Home":         false,
		"MC_Stop":         false,
	}

	for _, decl := range result.File.Declarations {
		fb, ok := decl.(*ast.FunctionBlockDecl)
		if !ok || fb.Name == nil {
			continue
		}
		if _, want := expected[fb.Name.Name]; want {
			expected[fb.Name.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected mock FB %s not found in mc_mocks.st", name)
		}
	}
}

// TestAdsMockContainsADSREAD verifies the ads_mock.st file has the ADSREAD mock.
func TestAdsMockContainsADSREAD(t *testing.T) {
	content, err := os.ReadFile("ads_mock.st")
	if err != nil {
		t.Fatalf("reading ads_mock.st: %v", err)
	}

	result := pipeline.Parse("ads_mock.st", string(content), nil)
	if result.File == nil {
		t.Fatalf("parsed file is nil")
	}

	found := false
	for _, decl := range result.File.Declarations {
		fb, ok := decl.(*ast.FunctionBlockDecl)
		if !ok || fb.Name == nil {
			continue
		}
		if fb.Name.Name == "ADSREAD" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected ADSREAD mock not found in ads_mock.st")
	}
}
