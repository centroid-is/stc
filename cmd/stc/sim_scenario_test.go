package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scenarioFiles are the fixture project files copied by scenarioProject;
// demo_ect.st becomes ECT.st so its GVL is named ECT.
var scenarioFiles = [][2]string{
	{"demo_types.st", "demo_types.st"},
	{"demo_ect.st", "ECT.st"},
	{"scenario/ECT_Diag.st", "ECT_Diag.st"},
	{"scenario/MAIN.st", "MAIN.st"},
	{"scenario/jam.toml", "jam.toml"},
}

// scenarioProject copies the scenario fixture project into a temp dir with
// its stc.toml (library path made absolute) and returns the dir, the .st
// paths and the --io glob for Demo Device 1/2.
func scenarioProject(t *testing.T) (dir string, st []string, ioGlob string) {
	t.Helper()
	dir = t.TempDir()
	for _, f := range scenarioFiles {
		b, err := os.ReadFile(filepath.Join(ecatFixtures, f[0]))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(dir, f[1]), string(b))
		if strings.HasSuffix(f[1], ".st") {
			st = append(st, filepath.Join(dir, f[1]))
		}
	}
	cfg, err := os.ReadFile(filepath.Join(ecatFixtures, "scenario", "stc.toml"))
	if err != nil {
		t.Fatal(err)
	}
	stubs, err := filepath.Abs("../../stdlib/vendor/beckhoff")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "stc.toml"), strings.Replace(string(cfg), "../../../stdlib/vendor/beckhoff", filepath.ToSlash(stubs), 1))
	abs, err := filepath.Abs(ecatFixtures)
	if err != nil {
		t.Fatal(err)
	}
	return dir, st, filepath.Join(abs, "Demo Device [12].xml")
}

// simScenarioJSON is the scenario part of the stc sim JSON result.
type simScenarioJSON struct {
	simProjectJSON
	Scenario struct {
		Name       string `json:"name"`
		Cycles     int    `json:"cycles"`
		SimTimeNS  int64  `json:"sim_time_ns"`
		Passed     bool   `json:"passed"`
		Assertions []struct {
			Step     int    `json:"step"`
			Path     string `json:"path"`
			Expected any    `json:"expected"`
			Actual   any    `json:"actual"`
			Pass     bool   `json:"pass"`
		} `json:"assertions"`
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	} `json:"scenario"`
	Outputs  map[string]any `json:"outputs"`
	EtherCAT []struct {
		Master string `json:"master"`
		Slave  int    `json:"slave"`
		Name   string `json:"name"`
		State  int    `json:"state"`
		Link   int    `json:"link"`
		WcBad  bool   `json:"wc_bad"`
	} `json:"ethercat"`
}

func decodeScenarioJSON(t *testing.T, stdout string) simScenarioJSON {
	t.Helper()
	var r simScenarioJSON
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	return r
}

func TestSimScenarioJam(t *testing.T) {
	dir, st, io := scenarioProject(t)
	args := append([]string{"sim", "--format", "json", "--io", io, "--scenario", filepath.Join(dir, "jam.toml"),
		"--set", "ECT_Diag.rSetpoint=1.5", "--get", "MAIN.xDriveFault"}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, stderr, stdout)
	}
	r := decodeScenarioJSON(t, stdout)
	if !r.Scenario.Passed || r.Scenario.Name != "jam" || len(r.Scenario.Assertions) != 8 {
		t.Fatalf("scenario = %+v", r.Scenario)
	}
	if r.Cycles != r.Scenario.Cycles || r.Cycles != 51 || r.SimTimeNS != 510_000_000 {
		t.Errorf("cycles %d / %d, sim time %d", r.Cycles, r.Scenario.Cycles, r.SimTimeNS)
	}
	if r.Get["MAIN.xDriveFault"] != true {
		t.Errorf("--get after the run = %v", r.Get)
	}
	if len(r.Tasks) != 1 || r.Tasks[0].Runs != 51 {
		t.Errorf("tasks = %+v", r.Tasks)
	}
}

func TestSimScenarioCyclesOverride(t *testing.T) {
	dir, st, io := scenarioProject(t)
	args := append([]string{"sim", "--format", "json", "--io", io, "--scenario", filepath.Join(dir, "jam.toml"), "--cycles", "5"}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	r := decodeScenarioJSON(t, stdout)
	if r.Cycles != 5 || r.Scenario.Cycles != 5 || !r.Scenario.Passed {
		t.Errorf("cycles %d, scenario %+v", r.Cycles, r.Scenario)
	}
	unfired := 0
	for _, d := range r.Scenario.Diagnostics {
		if d.Code == "SCN010" {
			unfired++
		}
	}
	if unfired != 8 {
		t.Errorf("SCN010 warnings = %d, want 8 (steps after cycle 5)", unfired)
	}
}

func TestSimScenarioUsageErrors(t *testing.T) {
	dir, st, io := scenarioProject(t)
	jam := filepath.Join(dir, "jam.toml")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"single file", []string{"sim", st[3], "--scenario", jam}, "--scenario requires project mode"},
		{"realtime", append([]string{"sim", "--realtime", "--duration", "1ms", "--scenario", jam}, st...), "cannot be combined with --realtime"},
		{"glob without match", append([]string{"sim", "--io", filepath.Join(dir, "nope*.xml"), "--scenario", jam}, st...), "matches no file"},
		{"bad glob", append([]string{"sim", "--io", "[", "--scenario", jam}, st...), "--io \"[\""},
		{"missing scenario", append([]string{"sim", "--io", io, "--scenario", filepath.Join(dir, "nope.toml")}, st...), "scenario failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runStc(t, tc.args...)
			if code != 1 || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d, stderr %q, want %q", code, stderr, tc.want)
			}
		})
	}
}

func TestSimScenarioLoadErrorJSON(t *testing.T) {
	dir, st, io := scenarioProject(t)
	args := append([]string{"sim", "--format", "json", "--io", io, "--scenario", filepath.Join(dir, "nope.toml")}, st...)
	stdout, _, code := runStc(t, args...)
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	r := decodeScenarioJSON(t, stdout)
	if r.SimTimeNS != 0 || len(r.Scenario.Diagnostics) != 1 || r.Scenario.Diagnostics[0].Code != "SCN001" {
		t.Errorf("result = %+v", r)
	}
}

func TestSimScenarioText(t *testing.T) {
	dir, st, io := scenarioProject(t)
	args := append([]string{"sim", "--io", io, "--scenario", filepath.Join(dir, "jam.toml")}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	for _, want := range []string{"Project: 51 cycles", "TASK", "scenario jam: 51 cycles", "PASS step 9: MAIN.xDriveFault", "\nPASS\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text output lacks %q:\n%s", want, stdout)
		}
	}
}

func TestSimScenarioWithoutIO(t *testing.T) {
	dir, st, _ := scenarioProject(t)
	writeTestFile(t, filepath.Join(dir, "set.toml"), `[scenario]
name = "plain"
[[step]]
cycle = 0
set = { path = "ECT_Diag.rSetpoint", value = 2.5 }
expect = { path = "ECT_Diag.rSetpoint", value = 2.5 }
`)
	args := append([]string{"sim", "--format", "json", "--scenario", filepath.Join(dir, "set.toml")}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d %s\n%s", code, stderr, stdout)
	}
	if r := decodeScenarioJSON(t, stdout); !r.Scenario.Passed || r.Cycles != 1 {
		t.Errorf("result = %+v", r)
	}
}

func TestSimScenarioReportDeterministic(t *testing.T) {
	dir, st, io := scenarioProject(t)
	args := append([]string{"sim", "--format", "json", "--io", io, "--scenario", filepath.Join(dir, "jam.toml")}, st...)
	out1, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	out2, _, _ := runStc(t, args...)
	if out1 != out2 {
		t.Fatal("two identical scenario runs differ")
	}
	r := decodeScenarioJSON(t, out1)
	if _, ok := r.Outputs["ECT.A1_02.O1"]; !ok {
		t.Errorf("outputs lack ECT.A1_02.O1: %v", r.Outputs)
	}
	found := false
	for i, s := range r.EtherCAT {
		if i > 0 && r.EtherCAT[i-1].Master == s.Master && r.EtherCAT[i-1].Slave >= s.Slave {
			t.Errorf("ethercat not sorted: %+v", r.EtherCAT)
		}
		if s.Name == "DEMO.A1.01 (EL1008)" {
			found = true
			if s.State != 17 || s.Link != 1 {
				t.Errorf("EL1008 = %+v", s)
			}
		}
	}
	if !found {
		t.Errorf("ethercat lacks the pulled EL1008: %+v", r.EtherCAT)
	}
}

func TestSimScenarioFailingExpect(t *testing.T) {
	dir, st, io := scenarioProject(t)
	writeTestFile(t, filepath.Join(dir, "bad.toml"), `[scenario]
name = "bad"
[[step]]
cycle = 0
set = { path = "ECT.A1_01.I1", value = true }
expect = { path = "ECT.A1_02.O1", value = false, within = 2 }
`)
	args := append([]string{"sim", "--format", "json", "--io", io, "--scenario", filepath.Join(dir, "bad.toml")}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 1 || !strings.Contains(stderr, "assertion(s) failed") {
		t.Fatalf("exit %d %s", code, stderr)
	}
	r := decodeScenarioJSON(t, stdout)
	if len(r.Scenario.Assertions) != 1 {
		t.Fatalf("assertions = %+v", r.Scenario.Assertions)
	}
	a := r.Scenario.Assertions[0]
	if a.Pass || a.Expected != false || a.Actual != true {
		t.Errorf("assertion = %+v", a)
	}
	hasSCN009 := false
	for _, d := range r.Diagnostics {
		hasSCN009 = hasSCN009 || d.Code == "SCN009"
	}
	if !hasSCN009 {
		t.Errorf("top-level diagnostics lack SCN009: %+v", r.Diagnostics)
	}
}

func TestSimScenarioUnknownSlave(t *testing.T) {
	dir, st, io := scenarioProject(t)
	writeTestFile(t, filepath.Join(dir, "slave.toml"), `[scenario]
name = "slave"
[[step]]
cycle = 3
trip = { slave = "NOPE.X1", channel = 1 }
`)
	args := append([]string{"sim", "--format", "json", "--io", io, "--scenario", filepath.Join(dir, "slave.toml")}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 1 || !strings.Contains(stderr, "scenario validation failed") {
		t.Fatalf("exit %d %s", code, stderr)
	}
	r := decodeScenarioJSON(t, stdout)
	if r.SimTimeNS != 0 || r.Cycles != 0 || len(r.Scenario.Diagnostics) != 1 || r.Scenario.Diagnostics[0].Code != "SCN006" {
		t.Errorf("result = %+v", r)
	}
}

func TestSimScenarioTextSections(t *testing.T) {
	dir, st, io := scenarioProject(t)
	args := append([]string{"sim", "--io", io, "--scenario", filepath.Join(dir, "jam.toml")}, st...)
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	for _, want := range []string{"outputs (12):", "  ECT.A1_02.O1 = false", "ethercat (2 not healthy):",
		"Device 1 (EtherCAT) #1 DEMO.A1.01 (EL1008): state 17 link 1", "diagnostics (", "SEMA024"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text output lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stderr, "SEMA024") {
		t.Errorf("diagnostics printed twice (stderr): %s", stderr)
	}
}

func TestSimScenarioTickError(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "main.st"), "PROGRAM MAIN\nVAR a : ARRAY[1..2] OF INT; i : INT := 1; END_VAR\ni := i + 1;\na[i] := 1;\nEND_PROGRAM\n")
	writeTestFile(t, filepath.Join(dir, "gvl.st"), "VAR_GLOBAL\n\tx : INT;\nEND_VAR\n")
	writeTestFile(t, filepath.Join(dir, "s.toml"), "[scenario]\nname = \"t\"\ncycles = 5\n[[step]]\ncycle = 0\nset = { path = \"gvl.x\", value = 1 }\n")
	stdout, stderr, code := runStc(t, "sim", "--format", "json", "--scenario", filepath.Join(dir, "s.toml"),
		filepath.Join(dir, "gvl.st"), filepath.Join(dir, "main.st"))
	if code != 1 || !strings.Contains(stderr, "simulation error: tick 1") {
		t.Fatalf("exit %d %s\n%s", code, stderr, stdout)
	}
	if r := decodeScenarioJSON(t, stdout); r.Scenario.Cycles != 1 || r.Scenario.Passed {
		t.Errorf("scenario = %+v", r.Scenario)
	}
}
