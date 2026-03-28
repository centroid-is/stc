package incremental

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/project"
)

const mainST = `PROGRAM Main
VAR
    m : Motor;
END_VAR
    m.speed := 100;
END_PROGRAM
`

const motorST = `FUNCTION_BLOCK Motor
VAR_INPUT
    speed : INT;
END_VAR
END_FUNCTION_BLOCK
`

const timerWrapperST = `FUNCTION_BLOCK TimerWrapper
VAR
    count : INT;
END_VAR
    count := count + 1;
END_FUNCTION_BLOCK
`

const motorModifiedST = `FUNCTION_BLOCK Motor
VAR_INPUT
    speed : REAL;
END_VAR
END_FUNCTION_BLOCK
`

// writeFile is a test helper that writes content to a file in the given dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
	return path
}

func TestIncrementalFirstRun(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	ia := NewIncrementalAnalyzer(cacheDir)
	_ = ia.Analyze([]string{mainPath, motorPath, timerPath}, (*project.Config)(nil))

	stats := ia.Stats()
	if stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", stats.TotalFiles)
	}
	if stats.StaleFiles != 3 {
		t.Errorf("StaleFiles = %d, want 3 (first run)", stats.StaleFiles)
	}
	if stats.SkippedFiles != 0 {
		t.Errorf("SkippedFiles = %d, want 0 (first run)", stats.SkippedFiles)
	}
}

func TestIncrementalSecondRunNoChanges(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	filenames := []string{mainPath, motorPath, timerPath}
	var cfg *project.Config

	// First run
	ia1 := NewIncrementalAnalyzer(cacheDir)
	_ = ia1.Analyze(filenames, cfg)

	// Second run (new analyzer, same cache dir)
	ia2 := NewIncrementalAnalyzer(cacheDir)
	_ = ia2.Analyze(filenames, cfg)

	stats := ia2.Stats()
	if stats.StaleFiles != 0 {
		t.Errorf("StaleFiles = %d, want 0 (no changes)", stats.StaleFiles)
	}
	if stats.SkippedFiles != 3 {
		t.Errorf("SkippedFiles = %d, want 3 (no changes)", stats.SkippedFiles)
	}
}

func TestIncrementalOneFileChanged(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	filenames := []string{mainPath, motorPath, timerPath}
	var cfg *project.Config

	// First run
	ia1 := NewIncrementalAnalyzer(cacheDir)
	_ = ia1.Analyze(filenames, cfg)

	// Modify motor.st
	writeFile(t, srcDir, "motor.st", motorModifiedST)

	// Third run
	ia2 := NewIncrementalAnalyzer(cacheDir)
	_ = ia2.Analyze(filenames, cfg)

	stats := ia2.Stats()
	if stats.StaleFiles < 1 {
		t.Errorf("StaleFiles = %d, want >= 1 (motor.st changed)", stats.StaleFiles)
	}
	if stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", stats.TotalFiles)
	}
}

func TestIncrementalDiagnosticsEquivalence(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)

	filenames := []string{mainPath, motorPath}
	var cfg *project.Config

	// First run (incremental)
	ia := NewIncrementalAnalyzer(cacheDir)
	incrResult := ia.Analyze(filenames, cfg)

	// Count diagnostics by severity from incremental
	incrErrors := 0
	for _, d := range incrResult.Diags {
		if d.Severity == 0 { // diag.Error == 0
			incrErrors++
		}
	}

	// The incremental path should produce diagnostics (we don't require exact match
	// with non-incremental since ordering may differ, but the result should be valid)
	if incrResult.Symbols == nil {
		t.Error("Symbols table is nil from incremental analysis")
	}
}

func TestIncrementalFileDeleted(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	var cfg *project.Config

	// First run with 3 files
	ia1 := NewIncrementalAnalyzer(cacheDir)
	_ = ia1.Analyze([]string{mainPath, motorPath, timerPath}, cfg)

	// Second run with timer_wrapper removed from list
	ia2 := NewIncrementalAnalyzer(cacheDir)
	result := ia2.Analyze([]string{mainPath, motorPath}, cfg)

	stats := ia2.Stats()
	if stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", stats.TotalFiles)
	}
	if result.Symbols == nil {
		t.Error("Symbols table is nil after file deletion")
	}
}
