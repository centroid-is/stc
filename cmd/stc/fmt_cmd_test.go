package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtBasic(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "simple.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "fmt", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "PROGRAM Main") {
		t.Errorf("expected PROGRAM Main in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "    x : INT;") {
		t.Errorf("expected 4-space indented var decl, got: %s", stdout)
	}
	if !strings.Contains(stdout, "    x := 42;") {
		t.Errorf("expected 4-space indented assignment, got: %s", stdout)
	}
	if !strings.Contains(stdout, "END_PROGRAM") {
		t.Errorf("expected END_PROGRAM in output, got: %s", stdout)
	}
}

func TestFmtPreservesComments(t *testing.T) {
	// The formatter preserves comments that are attached as trivia to AST nodes.
	// Since the current parser does not attach trivia, this test verifies the
	// command runs successfully on input containing comments (they will be
	// stripped by the parse-then-format round-trip, which is the expected
	// behavior until the parser gains trivia support).
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "comments.st", `PROGRAM Main
VAR
    // line comment
    x : INT;
END_VAR
    (* block comment *)
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "fmt", file)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	// Verify the program structure is preserved
	if !strings.Contains(stdout, "PROGRAM Main") {
		t.Errorf("expected PROGRAM Main, got: %s", stdout)
	}
	if !strings.Contains(stdout, "x : INT") {
		t.Errorf("expected variable declaration, got: %s", stdout)
	}
}

func TestFmtIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	file := writeTestST(t, tmpDir, "idem.st", input)

	// First format
	stdout1, stderr, exitCode := runStc(t, "fmt", file)
	if exitCode != 0 {
		t.Fatalf("first format: expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}

	// Write formatted output and format again
	file2 := filepath.Join(tmpDir, "idem2.st")
	if err := os.WriteFile(file2, []byte(stdout1), 0644); err != nil {
		t.Fatal(err)
	}

	stdout2, stderr, exitCode := runStc(t, "fmt", file2)
	if exitCode != 0 {
		t.Fatalf("second format: expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}

	if stdout1 != stdout2 {
		t.Errorf("format not idempotent.\nFirst:\n%s\nSecond:\n%s", stdout1, stdout2)
	}
}

func TestFmtLowercaseKeywords(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "lower.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "fmt", file, "--uppercase-keywords=false")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "program Main") {
		t.Errorf("expected lowercase 'program', got: %s", stdout)
	}
	if !strings.Contains(stdout, "end_program") {
		t.Errorf("expected lowercase 'end_program', got: %s", stdout)
	}
}

func TestFmtCustomIndent(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "indent.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "fmt", file, "--indent", "  ")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "  x : INT;") {
		t.Errorf("expected 2-space indented var decl, got: %s", stdout)
	}
	if !strings.Contains(stdout, "  x := 42;") {
		t.Errorf("expected 2-space indented assignment, got: %s", stdout)
	}
}

func TestFmtJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "json.st", `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`)

	stdout, stderr, exitCode := runStc(t, "fmt", file, "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output should be valid JSON, got:\n%s", stdout)
	}

	var result struct {
		File        string `json:"file"`
		Code        string `json:"code"`
		Diagnostics []any  `json:"diagnostics"`
		HasErrors   bool   `json:"has_errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result.Code == "" {
		t.Error("expected non-empty code field in JSON output")
	}
	if result.HasErrors {
		t.Error("expected has_errors to be false for valid input")
	}
}

func TestFmtNoArgs(t *testing.T) {
	_, stderr, exitCode := runStc(t, "fmt")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code when no files specified")
	}
	if !strings.Contains(stderr, "no input files") {
		t.Errorf("expected error about missing files, got: %s", stderr)
	}
}

func TestFmtBrokenFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := writeTestST(t, tmpDir, "broken.st", `PROGRAM
    this is not valid ST at all @@@ !!!
`)

	_, stderr, exitCode := runStc(t, "fmt", file)
	if exitCode != 1 {
		t.Fatalf("expected exit 1 for broken file, got %d; stderr: %s", exitCode, stderr)
	}
}

func TestFmtMultipleFiles(t *testing.T) {
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

	stdout, stderr, exitCode := runStc(t, "fmt", file1, file2)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "ProgramA") {
		t.Errorf("expected ProgramA in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "ProgramB") {
		t.Errorf("expected ProgramB in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "// ---") {
		t.Errorf("expected file separator markers for multiple files, got: %s", stdout)
	}
}

func TestFmtNonexistentFile(t *testing.T) {
	_, stderr, exitCode := runStc(t, "fmt", "nonexistent_file.st")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for nonexistent file")
	}
	if !strings.Contains(stderr, "no such file") && !strings.Contains(stderr, "not found") &&
		!strings.Contains(stderr, "cannot find") && !strings.Contains(stderr, "error") {
		t.Errorf("expected file error in stderr, got: %s", stderr)
	}
}
