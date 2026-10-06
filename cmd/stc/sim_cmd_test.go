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

const simSetGetSrc = `TYPE E_Mode : (Off, Slow, Fast); END_TYPE
TYPE ST_Cfg :
STRUCT
	gain : INT := 2;
	mode : E_Mode;
END_STRUCT
END_TYPE
FUNCTION_BLOCK FB_Acc
VAR_INPUT step : INT; END_VAR
VAR_OUTPUT total : INT; END_VAR
total := total + step;
END_FUNCTION_BLOCK
PROGRAM MAIN
VAR
	limit : INT := 100;
	count : INT;
	acc : FB_Acc;
	cfg : ST_Cfg;
	d : TIME;
END_VAR
IF count < limit THEN
	count := count + 1;
END_IF
acc(step := cfg.gain);
END_PROGRAM
`

func TestSimSetGetJSON(t *testing.T) {
	path := writeTestST(t, t.TempDir(), "setget.st", simSetGetSrc)
	stdout, stderr, exitCode := runStc(t, "sim", path, "--cycles", "3",
		"--set", "MAIN.limit=2", "--set", "MAIN.cfg.mode=\"Fast\"", "--set", "MAIN.d=T#5s",
		"--set", "MAIN.cfg={\"gain\": 5}",
		"--get", "MAIN.count", "--get", "MAIN.cfg", "--get", "MAIN.acc.total", "--get", "MAIN.d",
		"--format", "json")
	if exitCode != 0 {
		t.Fatalf("exit %d, stderr: %s", exitCode, stderr)
	}
	var res struct {
		NumCycles int                        `json:"num_cycles"`
		Get       map[string]json.RawMessage `json:"get"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	want := map[string]string{
		"MAIN.count":     `2`,
		"MAIN.cfg":       `{"gain":5,"mode":"Fast"}`,
		"MAIN.acc.total": `15`,
		"MAIN.d":         `{"ms":5000,"iso":"PT5S"}`,
	}
	for k, w := range want {
		var got any
		_ = json.Unmarshal(res.Get[k], &got)
		compact, _ := json.Marshal(got)
		var wantV any
		_ = json.Unmarshal([]byte(w), &wantV)
		wantC, _ := json.Marshal(wantV)
		if string(compact) != string(wantC) {
			t.Errorf("%s = %s, want %s", k, compact, wantC)
		}
	}
	if res.NumCycles != 3 {
		t.Errorf("num_cycles = %d", res.NumCycles)
	}
}

func TestSimSetGetText(t *testing.T) {
	path := writeTestST(t, t.TempDir(), "setget.st", simSetGetSrc)
	stdout, stderr, exitCode := runStc(t, "sim", path, "--cycles", "2",
		"--set", "MAIN.limit=16#1", "--get", "MAIN.count", "--get", "MAIN.cfg.mode")
	if exitCode != 0 {
		t.Fatalf("exit %d, stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "MAIN.count = 1\n") || !strings.Contains(stdout, `MAIN.cfg.mode = "Off"`) {
		t.Errorf("missing get lines:\n%s", stdout)
	}
}

func TestSimSetErrors(t *testing.T) {
	path := writeTestST(t, t.TempDir(), "setget.st", simSetGetSrc)
	cases := map[string][]string{
		"unknown root":        {"--set", "NOPE.x=1"},
		"out of range":        {"--set", "MAIN.limit=40000"},
		"expected PATH=VALUE": {"--set", "MAIN.limit"},
		"unknown variable":    {"--get", "MAIN.zz"},
	}
	for want, extra := range cases {
		args := append([]string{"sim", path, "--cycles", "1"}, extra...)
		_, stderr, exitCode := runStc(t, args...)
		if exitCode == 0 {
			t.Errorf("%v: expected failure", extra)
		}
		if !strings.Contains(stderr, want) {
			t.Errorf("%v: stderr %q does not contain %q", extra, stderr, want)
		}
	}
	bad := writeTestST(t, t.TempDir(), "bad.st", "PROGRAM P\nVAR a : ARRAY[1..Q.Z] OF INT; END_VAR\nEND_PROGRAM\n")
	_, stderr, exitCode := runStc(t, "sim", bad, "--cycles", "1")
	if exitCode == 0 || !strings.Contains(stderr, "initialisation error") {
		t.Errorf("init error: exit %d, stderr %q", exitCode, stderr)
	}
}

func TestSimUserFB(t *testing.T) {
	path := writeTestST(t, t.TempDir(), "fb.st", simSetGetSrc)
	stdout, stderr, exitCode := runStc(t, "sim", path, "--cycles", "4", "--get", "MAIN.acc.total", "--format", "json")
	if exitCode != 0 {
		t.Fatalf("exit %d, stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, `"MAIN.acc.total": 8`) {
		t.Errorf("user FB with TYPE default did not run:\n%s", stdout)
	}
}
