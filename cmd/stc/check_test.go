package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestST(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

func TestCheckCommandValid(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "valid.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "check", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 for valid file, got %d; stderr: %s; stdout: %s", exitCode, stderr, stdout)
	}
	if !strings.Contains(stderr, "0 error(s)") {
		t.Errorf("expected '0 error(s)' in summary, stderr: %s", stderr)
	}
}

func TestCheckCommandTypeError(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "type_error.st", `PROGRAM Main
VAR
    x : INT;
    s : STRING;
END_VAR
    x := s;
END_PROGRAM
`)

	_, stderr, exitCode := runStc(t, "check", file)
	if exitCode != 1 {
		t.Fatalf("expected exit 1 for type error, got %d", exitCode)
	}
	if !strings.Contains(stderr, "cannot assign") {
		t.Errorf("expected type mismatch error in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "1 error(s)") {
		t.Errorf("expected '1 error(s)' in summary, stderr: %s", stderr)
	}
}

func TestCheckCommandJSON(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "test.st", `PROGRAM Main
VAR
    x : INT;
    s : STRING;
END_VAR
    x := s;
END_PROGRAM
`)

	stdout, _, exitCode := runStc(t, "check", file, "--format", "json")
	// Exit code 1 because there are errors, but JSON should still be valid
	if exitCode != 1 {
		t.Fatalf("expected exit 1 for file with errors, got %d", exitCode)
	}

	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON, got:\n%s", stdout)
	}

	// Parse the JSON array
	var diags []struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		Pos      struct {
			File   string `json:"file"`
			Line   int    `json:"line"`
			Col    int    `json:"col"`
		} `json:"pos"`
	}
	if err := json.Unmarshal([]byte(stdout), &diags); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	hasSEMA001 := false
	for _, d := range diags {
		if d.Code == "SEMA001" {
			hasSEMA001 = true
			if d.Severity != "error" {
				t.Errorf("SEMA001 should be severity 'error', got %q", d.Severity)
			}
			if d.Pos.Line == 0 {
				t.Error("diagnostic should have a line number")
			}
		}
	}
	if !hasSEMA001 {
		t.Errorf("JSON output should contain SEMA001 diagnostic")
	}
}

func TestCheckCommandVendor(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "vendor.st", `FUNCTION_BLOCK MyFB
VAR
    counter : INT;
END_VAR

METHOD PUBLIC DoWork : BOOL
VAR_INPUT
    value : INT;
END_VAR
    DoWork := value > 0;
END_METHOD

END_FUNCTION_BLOCK
`)

	_, stderr, exitCode := runStc(t, "check", file, "--vendor", "schneider")
	// Vendor warnings should not cause exit 1
	if exitCode != 0 {
		t.Fatalf("expected exit 0 (warnings only), got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stderr, "not supported by schneider") {
		t.Errorf("expected vendor warning about schneider in stderr, got: %s", stderr)
	}
}

func TestCheckCommandNoFiles(t *testing.T) {
	_, stderr, exitCode := runStc(t, "check")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for no arguments")
	}
	if !strings.Contains(stderr, "no input files") {
		t.Errorf("expected helpful error about no input files, got: %s", stderr)
	}
}
