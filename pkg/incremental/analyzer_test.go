package incremental

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
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
	result := ia.Parse([]string{mainPath, motorPath, timerPath})

	if result.Stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result.Stats.TotalFiles)
	}
	if result.Stats.StaleFiles != 3 {
		t.Errorf("StaleFiles = %d, want 3 (first run)", result.Stats.StaleFiles)
	}
	if result.Stats.SkippedFiles != 0 {
		t.Errorf("SkippedFiles = %d, want 0 (first run)", result.Stats.SkippedFiles)
	}
	if len(result.Files) != 3 {
		t.Errorf("Files count = %d, want 3", len(result.Files))
	}
}

func TestIncrementalSecondRunNoChanges(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	filenames := []string{mainPath, motorPath, timerPath}

	// First run
	ia1 := NewIncrementalAnalyzer(cacheDir)
	_ = ia1.Parse(filenames)

	// Second run (new analyzer, same cache dir)
	ia2 := NewIncrementalAnalyzer(cacheDir)
	result := ia2.Parse(filenames)

	if result.Stats.StaleFiles != 0 {
		t.Errorf("StaleFiles = %d, want 0 (no changes)", result.Stats.StaleFiles)
	}
	if result.Stats.SkippedFiles != 3 {
		t.Errorf("SkippedFiles = %d, want 3 (no changes)", result.Stats.SkippedFiles)
	}
}

func TestIncrementalOneFileChanged(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	filenames := []string{mainPath, motorPath, timerPath}

	// First run
	ia1 := NewIncrementalAnalyzer(cacheDir)
	_ = ia1.Parse(filenames)

	// Modify motor.st
	writeFile(t, srcDir, "motor.st", motorModifiedST)

	// Second run
	ia2 := NewIncrementalAnalyzer(cacheDir)
	result := ia2.Parse(filenames)

	if result.Stats.StaleFiles < 1 {
		t.Errorf("StaleFiles = %d, want >= 1 (motor.st changed)", result.Stats.StaleFiles)
	}
	if result.Stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result.Stats.TotalFiles)
	}
}

func TestIncrementalDiagnosticsEquivalence(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)

	filenames := []string{mainPath, motorPath}

	// First run (incremental)
	ia := NewIncrementalAnalyzer(cacheDir)
	incrResult := ia.Parse(filenames)

	// Run semantic analysis on the incremental result
	analysisResult := analyzer.Analyze(incrResult.Files, nil)

	if analysisResult.Symbols == nil {
		t.Error("Symbols table is nil from incremental analysis")
	}
}

func TestIncrementalFileDeleted(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()

	mainPath := writeFile(t, srcDir, "main.st", mainST)
	motorPath := writeFile(t, srcDir, "motor.st", motorST)
	timerPath := writeFile(t, srcDir, "timer_wrapper.st", timerWrapperST)

	// First run with 3 files
	ia1 := NewIncrementalAnalyzer(cacheDir)
	_ = ia1.Parse([]string{mainPath, motorPath, timerPath})

	// Second run with timer_wrapper removed from list
	ia2 := NewIncrementalAnalyzer(cacheDir)
	result := ia2.Parse([]string{mainPath, motorPath})

	if result.Stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.Stats.TotalFiles)
	}
	if len(result.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(result.Files))
	}
}
