package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var stcBinary string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	dir, err := os.MkdirTemp("", "stc-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	stcBinary = filepath.Join(dir, "stc")
	cmd := exec.Command("go", "build", "-o", stcBinary, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build stc: %v\n%s\n", err, string(out))
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// runStc runs the stc binary with the given args and returns stdout, stderr, and exit code.
func runStc(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(stcBinary, args...)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected error running stc: %v", err)
		}
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode
}

func TestCLI_Version(t *testing.T) {
	stdout, _, exitCode := runStc(t, "--version")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "stc") {
		t.Errorf("version output should contain 'stc', got: %s", stdout)
	}
}

func TestCLI_Help(t *testing.T) {
	stdout, _, exitCode := runStc(t, "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	for _, sub := range []string{"parse", "check", "test", "emit", "lint", "fmt", "pp"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("help output should contain subcommand %q", sub)
		}
	}
}

func TestCLI_ParseTextFormat(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "parse", "../../testdata/parse/motor_control.st")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "declaration") && !strings.Contains(stdout, "Parsed") {
		t.Errorf("expected summary with 'declaration' or 'Parsed', got: %s", stdout)
	}
}

func TestCLI_ParseJSONFormat(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "parse", "../../testdata/parse/motor_control.st", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output is not valid JSON:\n%s", stdout)
	}
	if !strings.Contains(stdout, "FunctionBlockDecl") {
		t.Errorf("JSON should contain FunctionBlockDecl, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "FB_Motor") {
		t.Errorf("JSON should contain FB_Motor, got:\n%s", stdout)
	}
}

func TestCLI_ParseBrokenFile(t *testing.T) {
	stdout, _, exitCode := runStc(t, "parse", "../../testdata/parse/broken_input.st", "--format", "json")
	if exitCode == 0 {
		t.Log("note: broken file may not produce error diagnostics depending on parser recovery")
	}
	// Output should still be valid JSON (partial AST produced).
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON even for broken input:\n%s", stdout)
	}
	// Should contain diagnostics or AST content.
	if !strings.Contains(stdout, "diagnostics") {
		t.Errorf("JSON should contain diagnostics field")
	}
}

func TestCLI_ParseNoArgs(t *testing.T) {
	_, stderr, exitCode := runStc(t, "parse")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for missing arguments")
	}
	if !strings.Contains(stderr, "requires at least 1 arg") && !strings.Contains(stderr, "Error") {
		t.Errorf("stderr should mention missing arguments, got: %s", stderr)
	}
}

func TestCLI_ParseNonexistentFile(t *testing.T) {
	_, stderr, exitCode := runStc(t, "parse", "nonexistent.st")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for nonexistent file")
	}
	if !strings.Contains(stderr, "no such file") && !strings.Contains(stderr, "not found") &&
		!strings.Contains(stderr, "cannot find") && !strings.Contains(stderr, "error") {
		t.Errorf("stderr should mention file error, got: %s", stderr)
	}
}

func TestCLI_StubCommands(t *testing.T) {
	for _, sub := range []string{"check", "test", "emit", "lint", "fmt"} {
		t.Run(sub, func(t *testing.T) {
			_, stderr, exitCode := runStc(t, sub)
			if exitCode != 0 {
				t.Fatalf("expected exit code 0 for stub command %q, got %d", sub, exitCode)
			}
			if !strings.Contains(stderr, "not yet implemented") {
				t.Errorf("stub %q should output 'not yet implemented', stderr: %s", sub, stderr)
			}
		})
	}
}

func TestCLI_StubCommandsJSON(t *testing.T) {
	stdout, _, exitCode := runStc(t, "check", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON, got: %s", stdout)
	}
	if !strings.Contains(stdout, "not yet implemented") {
		t.Errorf("JSON should contain 'not yet implemented', got: %s", stdout)
	}
}

func TestCLI_FormatFlag(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "parse", "../../testdata/parse/motor_control.st", "-f", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON with -f flag:\n%s", stdout)
	}
}
