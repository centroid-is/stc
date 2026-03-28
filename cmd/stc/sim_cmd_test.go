package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSimCommandJSON(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "sim", "testdata/sim_test.st",
		"--cycles", "10", "--dt", "10ms",
		"--wave", "SENSOR:sine:100:1",
		"--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}

	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output is not valid JSON:\n%s", stdout)
	}

	// Parse and verify structure
	var result struct {
		Cycles    json.RawMessage `json:"cycles"`
		NumCycles int             `json:"num_cycles"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if result.NumCycles != 10 {
		t.Errorf("expected num_cycles=10, got %d", result.NumCycles)
	}

	// Verify cycles is a non-empty array
	var cycles []json.RawMessage
	if err := json.Unmarshal(result.Cycles, &cycles); err != nil {
		t.Fatalf("cycles is not an array: %v", err)
	}
	if len(cycles) == 0 {
		t.Error("expected non-empty cycles array")
	}
}

func TestSimCommandText(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "sim", "testdata/sim_test.st",
		"--cycles", "5", "--dt", "10ms",
		"--wave", "SENSOR:step:1:1")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}

	// Text output should contain "Simulation:" header and "Cycle" column
	if !strings.Contains(stdout, "Simulation:") {
		t.Errorf("text output should contain 'Simulation:' header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Cycle") {
		t.Errorf("text output should contain 'Cycle' column header, got:\n%s", stdout)
	}
}

func TestSimCommandHelp(t *testing.T) {
	stdout, _, exitCode := runStc(t, "sim", "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "simulation") {
		t.Errorf("help should mention simulation, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--cycles") {
		t.Errorf("help should mention --cycles flag, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--wave") {
		t.Errorf("help should mention --wave flag, got:\n%s", stdout)
	}
}

func TestSimCommandNoArgs(t *testing.T) {
	_, stderr, exitCode := runStc(t, "sim")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for missing file argument")
	}
	if !strings.Contains(stderr, "accepts 1 arg") && !strings.Contains(stderr, "Error") {
		t.Errorf("stderr should mention missing argument, got: %s", stderr)
	}
}

func TestSimCommandMultipleWaveforms(t *testing.T) {
	// Create a program with two inputs
	src := `PROGRAM MultiWave
VAR_INPUT
    A : REAL;
    B : REAL;
END_VAR
VAR_OUTPUT
    SUM : REAL;
END_VAR
    SUM := A + B;
END_PROGRAM
`
	tmpDir := t.TempDir()
	path := writeTestST(t, tmpDir, "multi.st", src)

	stdout, stderr, exitCode := runStc(t, "sim", path,
		"--cycles", "5", "--dt", "10ms",
		"--wave", "A:sine:10:1",
		"--wave", "B:step:5:1",
		"--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}

	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output is not valid JSON:\n%s", stdout)
	}
}
