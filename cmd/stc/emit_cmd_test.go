package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmitBeckhoff(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "simple.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "emit", file, "--target", "beckhoff")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "PROGRAM") {
		t.Errorf("expected PROGRAM in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "END_PROGRAM") {
		t.Errorf("expected END_PROGRAM in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "x") {
		t.Errorf("expected variable x in output, got: %s", stdout)
	}
}

func TestEmitSchneider(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "oop.st", `FUNCTION_BLOCK MyFB
VAR
    counter : INT;
END_VAR
    counter := counter + 1;

METHOD PUBLIC DoWork : BOOL
VAR_INPUT
    value : INT;
END_VAR
    DoWork := value > 0;
END_METHOD

END_FUNCTION_BLOCK
`)

	stdout, stderr, exitCode := runStc(t, "emit", file, "--target", "schneider")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	// Schneider does not support OOP — methods should be filtered out
	if strings.Contains(stdout, "METHOD") {
		t.Errorf("schneider target should NOT contain METHOD, got: %s", stdout)
	}
	if !strings.Contains(stdout, "FUNCTION_BLOCK") {
		t.Errorf("expected FUNCTION_BLOCK in output, got: %s", stdout)
	}
}

func TestEmitPortable(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "simple.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "emit", file, "--target", "portable")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "PROGRAM") {
		t.Errorf("expected PROGRAM in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "END_PROGRAM") {
		t.Errorf("expected END_PROGRAM in output, got: %s", stdout)
	}
}

func TestEmitDefaultTarget(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "simple.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	// No --target flag should default to portable
	stdout, stderr, exitCode := runStc(t, "emit", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "PROGRAM") {
		t.Errorf("expected PROGRAM in output, got: %s", stdout)
	}
}

func TestEmitJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "simple.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "emit", file, "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON, got:\n%s", stdout)
	}

	var result struct {
		File        string `json:"file"`
		Code        string `json:"code"`
		Target      string `json:"target"`
		Diagnostics []any  `json:"diagnostics"`
		HasErrors   bool   `json:"has_errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result.Code == "" {
		t.Error("expected non-empty code field in JSON output")
	}
	if result.Target == "" {
		t.Error("expected target field in JSON output")
	}
	if result.HasErrors {
		t.Error("expected has_errors to be false for valid input")
	}
}

func TestEmitNonexistentFile(t *testing.T) {
	_, stderr, exitCode := runStc(t, "emit", "nonexistent_file.st")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for nonexistent file")
	}
	if !strings.Contains(stderr, "no such file") && !strings.Contains(stderr, "not found") &&
		!strings.Contains(stderr, "cannot find") && !strings.Contains(stderr, "error") {
		t.Errorf("expected file error in stderr, got: %s", stderr)
	}
}

func TestEmitBrokenFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "broken.st", `PROGRAM
    this is not valid ST at all @@@ !!!
`)

	_, stderr, exitCode := runStc(t, "emit", file)
	if exitCode != 1 {
		t.Fatalf("expected exit 1 for broken file, got %d; stderr: %s", exitCode, stderr)
	}
}

func TestEmitMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := writeTestST(t, tmpDir, "a.st", `PROGRAM ProgramA
VAR
    a : INT;
END_VAR
    a := 1;
END_PROGRAM
`)
	file2 := writeTestST(t, tmpDir, "b.st", `PROGRAM ProgramB
VAR
    b : INT;
END_VAR
    b := 2;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "emit", file1, file2, "--target", "beckhoff")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "ProgramA") {
		t.Errorf("expected ProgramA in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "ProgramB") {
		t.Errorf("expected ProgramB in output, got: %s", stdout)
	}
	// Multiple files should have separator markers
	if !strings.Contains(stdout, "// ---") {
		t.Errorf("expected file separator markers for multiple files, got: %s", stdout)
	}
}

func TestEmitNoArgs(t *testing.T) {
	_, stderr, exitCode := runStc(t, "emit")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code when no files specified")
	}
	if !strings.Contains(stderr, "no input files") && !strings.Contains(stderr, "usage") &&
		!strings.Contains(stderr, "Usage") && !strings.Contains(stderr, "required") {
		t.Errorf("expected helpful error about missing files, got: %s", stderr)
	}
}

func TestEmitJSONMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := writeTestST(t, tmpDir, "a.st", `PROGRAM ProgramA
VAR
    a : INT;
END_VAR
    a := 1;
END_PROGRAM
`)
	file2 := writeTestST(t, tmpDir, "b.st", `PROGRAM ProgramB
VAR
    b : INT;
END_VAR
    b := 2;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "emit", file1, file2, "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON, got:\n%s", stdout)
	}

	// Multiple files should produce a JSON array
	var results []struct {
		File   string `json:"file"`
		Code   string `json:"code"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("failed to parse JSON array: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
