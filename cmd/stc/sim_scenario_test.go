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
