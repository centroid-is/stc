package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintCleanCode(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "clean.st", `PROGRAM Main
VAR
    counter : INT;
END_VAR
    counter := counter + 1;
END_PROGRAM
`)

	_, stderr, exitCode := runStc(t, "lint", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 for clean file, got %d; stderr: %s", exitCode, stderr)
	}
	if strings.Contains(stderr, "LINT") {
		t.Errorf("clean code should have no lint warnings, stderr: %s", stderr)
	}
}

func TestLintMagicNumber(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "magic.st", `PROGRAM Main
VAR
    x : INT;
    y : INT;
END_VAR
    x := y + 42;
END_PROGRAM
`)

	_, stderr, exitCode := runStc(t, "lint", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 (warnings only), got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "LINT001") && !strings.Contains(stderr, "magic number") {
		t.Errorf("expected magic number warning in stderr, got: %s", stderr)
	}
}

func TestLintNamingViolation(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "naming.st", `FUNCTION_BLOCK my_bad_name
END_FUNCTION_BLOCK
`)

	_, stderr, exitCode := runStc(t, "lint", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 (warnings only), got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "LINT005") && !strings.Contains(stderr, "PascalCase") {
		t.Errorf("expected naming warning in stderr, got: %s", stderr)
	}
}

func TestLintJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "json_test.st", `PROGRAM Main
VAR
    x : INT;
    y : INT;
END_VAR
    x := y + 42;
END_PROGRAM
`)

	stdout, _, exitCode := runStc(t, "lint", file, "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON, got:\n%s", stdout)
	}

	var diags []struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		Pos      struct {
			File string `json:"file"`
			Line int    `json:"line"`
			Col  int    `json:"col"`
		} `json:"pos"`
	}
	if err := json.Unmarshal([]byte(stdout), &diags); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	hasLint := false
	for _, d := range diags {
		if strings.HasPrefix(d.Code, "LINT") {
			hasLint = true
			break
		}
	}
	if !hasLint {
		t.Errorf("JSON output should contain LINT diagnostics")
	}
}

func TestLintNoArgs(t *testing.T) {
	_, stderr, exitCode := runStc(t, "lint")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for no arguments")
	}
	if !strings.Contains(stderr, "no input files") {
		t.Errorf("expected helpful error about no input files, got: %s", stderr)
	}
}

func TestLintNonexistentFile(t *testing.T) {
	_, _, exitCode := runStc(t, "lint", "nonexistent_file.st")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for nonexistent file")
	}
}

func TestLintBrokenFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "broken.st", `PROGRAM
    this is not valid ST ;;;
END_PROGRAM
`)

	_, _, exitCode := runStc(t, "lint", file)
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for file with parse errors")
	}
}

func TestLintMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := writeTestST(t, tmpDir, "file1.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := x + 42;
END_PROGRAM
`)
	file2 := writeTestST(t, tmpDir, "file2.st", `FUNCTION_BLOCK my_fb
END_FUNCTION_BLOCK
`)

	_, stderr, exitCode := runStc(t, "lint", file1, file2)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 (warnings only), got %d; stderr: %s", exitCode, stderr)
	}

	// Check diagnostics reference correct filenames
	if !strings.Contains(stderr, filepath.Base(file1)) {
		t.Errorf("stderr should reference file1, got: %s", stderr)
	}
	if !strings.Contains(stderr, filepath.Base(file2)) {
		t.Errorf("stderr should reference file2, got: %s", stderr)
	}
}
