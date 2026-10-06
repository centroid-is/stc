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
			File string `json:"file"`
			Line int    `json:"line"`
			Col  int    `json:"col"`
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

const symbolsFixture = `TYPE E_State : (Idle := 0, Run := 1); END_TYPE

{attribute 'OPC.UA.DA' := '1'}
TYPE ST_HMI :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_State : E_State;
END_STRUCT
END_TYPE

FUNCTION_BLOCK FB_Drive
VAR
	HMI : ST_HMI;
END_VAR
END_FUNCTION_BLOCK

PROGRAM MAIN
VAR
	n : INT;
END_VAR
END_PROGRAM
`

const symbolsGVL = `VAR_GLOBAL
	fb : ARRAY[1..3] OF FB_Drive;
END_VAR
`

func TestCheckSymbolsText(t *testing.T) {
	dir := t.TempDir()
	file := writeTestST(t, dir, "main.st", symbolsFixture)
	gvl := writeTestST(t, dir, "GVL.st", symbolsGVL)

	stdout, stderr, exitCode := runStc(t, "check", file, gvl, "--symbols")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	for _, want := range []string{
		"GVL\n",
		"  GVL.fb : ARRAY[1..3] OF FB_Drive\n",
		"MAIN\n",
		"  MAIN.n : INT\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "GVL.fb[1]") {
		t.Errorf("array elements must not be expanded:\n%s", stdout)
	}
	if !strings.Contains(stderr, "0 error(s)") {
		t.Errorf("diagnostic summary missing from stderr: %s", stderr)
	}

	// Without the flag stdout stays empty.
	stdout, _, _ = runStc(t, "check", file, gvl)
	if stdout != "" {
		t.Errorf("expected no stdout without --symbols, got %q", stdout)
	}
}

func TestCheckSymbolsJSON(t *testing.T) {
	dir := t.TempDir()
	file := writeTestST(t, dir, "main.st", symbolsFixture)
	gvl := writeTestST(t, dir, "GVL.st", symbolsGVL)

	stdout, stderr, exitCode := runStc(t, "check", file, gvl, "--symbols", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	var out struct {
		Diagnostics []map[string]any `json:"diagnostics"`
		Symbols     struct {
			Roots []struct {
				Name     string `json:"name"`
				Kind     string `json:"kind"`
				Children []struct {
					Path    string `json:"path"`
					Low     int    `json:"low"`
					High    int    `json:"high"`
					Element struct {
						Children []struct {
							Path     string `json:"path"`
							Children []struct {
								Path        string            `json:"path"`
								EnumStrings map[string]string `json:"enum_strings"`
							} `json:"children"`
						} `json:"children"`
					} `json:"element"`
				} `json:"children"`
			} `json:"roots"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if out.Diagnostics == nil {
		t.Errorf("diagnostics must be an array, not null")
	}
	if len(out.Symbols.Roots) != 2 || out.Symbols.Roots[0].Name != "GVL" || out.Symbols.Roots[1].Kind != "program" {
		t.Fatalf("unexpected roots: %+v", out.Symbols.Roots)
	}
	fb := out.Symbols.Roots[0].Children[0]
	if fb.Path != "GVL.fb" || fb.Low != 1 || fb.High != 3 {
		t.Errorf("unexpected array node: %+v", fb)
	}
	st := fb.Element.Children[0].Children[0]
	if st.Path != "GVL.fb[*].HMI.p_stat_State" || st.EnumStrings["1"] != "Run" {
		t.Errorf("unexpected enum member: %+v", st)
	}
}

func TestCheckSymbolsWithErrors(t *testing.T) {
	dir := t.TempDir()
	file := writeTestST(t, dir, "test.st", `PROGRAM Main
VAR
    x : INT;
    s : STRING;
END_VAR
    x := s;
END_PROGRAM
`)
	stdout, _, exitCode := runStc(t, "check", file, "--symbols")
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}
	if !strings.Contains(stdout, "  Main.x : INT\n") {
		t.Errorf("symbols must print before exiting:\n%s", stdout)
	}

	stdout, _, exitCode = runStc(t, "check", file, "--symbols", "--format", "json")
	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}
	var out struct {
		Diagnostics []map[string]any `json:"diagnostics"`
		Symbols     json.RawMessage  `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out.Diagnostics) == 0 || len(out.Symbols) == 0 {
		t.Errorf("expected diagnostics and symbols: %s", stdout)
	}
}
