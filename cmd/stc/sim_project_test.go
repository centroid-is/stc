package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// simProjectJSON is the project-mode JSON result of stc sim and stc serve.
type simProjectJSON struct {
	Cycles    int   `json:"cycles"`
	SimTimeNS int64 `json:"sim_time_ns"`
	Tasks     []struct {
		Name     string   `json:"name"`
		CycleNS  int64    `json:"cycle_ns"`
		Priority int      `json:"priority"`
		Programs []string `json:"programs"`
		Runs     uint64   `json:"runs"`
		Overruns uint64   `json:"overruns"`
	} `json:"tasks"`
	Get         map[string]any `json:"get"`
	Diagnostics []struct {
		Code string `json:"code"`
	} `json:"diagnostics"`
	Warnings []string `json:"warnings"`
}

func decodeProjectJSON(t *testing.T, stdout string) simProjectJSON {
	t.Helper()
	var st simProjectJSON
	if err := json.Unmarshal([]byte(stdout), &st); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	return st
}

func runSimProjectJSON(t *testing.T, args ...string) simProjectJSON {
	t.Helper()
	stdout, stderr, code := runStc(t, append([]string{"sim", "--format", "json"}, args...)...)
	if code != 0 {
		t.Fatalf("sim %v: exit %d %s", args, code, stderr)
	}
	return decodeProjectJSON(t, stdout)
}

const persistCfgGVL = `VAR_GLOBAL PERSISTENT
	p_cfg_Speed : REAL;
END_VAR
VAR_GLOBAL RETAIN
	nStarts : DINT;
END_VAR
`

const persistMainST = `PROGRAM MAIN
VAR
	n : DINT;
END_VAR
n := n + 1;
IF n = 1 THEN
	GVL_Cfg.nStarts := GVL_Cfg.nStarts + 1;
END_IF
END_PROGRAM
`

// writePersistProject writes GVL_Cfg.st and main.st and returns their paths.
func writePersistProject(t *testing.T) (dir string, files []string) {
	t.Helper()
	dir = t.TempDir()
	files = []string{filepath.Join(dir, "GVL_Cfg.st"), filepath.Join(dir, "main.st")}
	writeTestFile(t, files[0], persistCfgGVL)
	writeTestFile(t, files[1], persistMainST)
	return dir, files
}

func TestSimProjectDemo(t *testing.T) {
	stdout1, stderr, code := runStc(t, "sim", "--project", demoTsproj, "--cycles", "1000", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	stdout2, _, _ := runStc(t, "sim", demoTsproj, "--cycles", "1000", "--format", "json")
	if stdout1 != stdout2 {
		t.Errorf("--project and positional runs differ:\n%s\n---\n%s", stdout1, stdout2)
	}
	st := decodeProjectJSON(t, stdout1)
	if st.SimTimeNS != 1_000_000_000 || st.Cycles != 1000 {
		t.Errorf("sim time %d cycles %d, want 1s for 1000 x 1ms", st.SimTimeNS, st.Cycles)
	}
	if len(st.Tasks) != 1 || st.Tasks[0].CycleNS != 1_000_000 || st.Tasks[0].Runs != 1000 ||
		st.Tasks[0].Priority != 20 || st.Tasks[0].Overruns != 0 {
		t.Errorf("tasks: %+v", st.Tasks)
	}

	stdout, _, code := runStc(t, "sim", demoTsproj, "--cycles", "5")
	if code != 0 || !strings.Contains(stdout, "Project: 5 cycles, sim time 5ms") ||
		!strings.Contains(stdout, "PlcTask") {
		t.Errorf("text output: exit %d\n%s", code, stdout)
	}
}

func TestSimProjectSTFiles(t *testing.T) {
	_, files := writePersistProject(t)
	st := runSimProjectJSON(t, append(files, "--cycles", "4", "--get", "MAIN.n",
		"--set", "GVL_Cfg.p_cfg_Speed=1.5", "--get", "GVL_Cfg.p_cfg_Speed")...)
	if st.SimTimeNS != 40_000_000 || st.Tasks[0].Name != "PlcTask" || st.Tasks[0].CycleNS != 10_000_000 {
		t.Errorf("default task: %+v", st)
	}
	if st.Get["MAIN.n"] != float64(4) || st.Get["GVL_Cfg.p_cfg_Speed"] != 1.5 {
		t.Errorf("get: %v", st.Get)
	}
	stdout, stderr, code := runStc(t, "sim", files[0], files[1], "--cycles", "2", "--get", "MAIN.n")
	if code != 0 || !strings.Contains(stdout, "MAIN.n = 2") {
		t.Errorf("text get: exit %d %s %s", code, stdout, stderr)
	}
}

func TestSimProjectPersist(t *testing.T) {
	dir, files := writePersistProject(t)
	state := filepath.Join(dir, "state.json")
	args := append([]string{"--persist", state, "--cycles", "3", "--get", "GVL_Cfg.p_cfg_Speed", "--get", "GVL_Cfg.nStarts"}, files...)
	st := runSimProjectJSON(t, append(args, "--set", "GVL_Cfg.p_cfg_Speed=42.5")...)
	if st.Get["GVL_Cfg.p_cfg_Speed"] != 42.5 || st.Get["GVL_Cfg.nStarts"] != float64(1) {
		t.Fatalf("run 1: %v", st.Get)
	}
	data, err := os.ReadFile(state)
	if err != nil || !strings.Contains(string(data), `"GVL_Cfg.p_cfg_Speed": 42.5`) {
		t.Fatalf("state file: %v %s", err, data)
	}
	st = runSimProjectJSON(t, args...)
	if st.Get["GVL_Cfg.p_cfg_Speed"] != 42.5 || st.Get["GVL_Cfg.nStarts"] != float64(2) {
		t.Errorf("run 2 restores: %v", st.Get)
	}

	// Stale entries warn; a malformed file fails.
	writeTestFile(t, state, `{"version":1,"values":{"GVL_Cfg.gone":1,"GVL_Cfg.p_cfg_Speed":2}}`)
	st = runSimProjectJSON(t, args...)
	if st.Get["GVL_Cfg.p_cfg_Speed"] != float64(2) || len(st.Warnings) != 1 ||
		!strings.Contains(st.Warnings[0], "GVL_Cfg.gone: unknown path") {
		t.Errorf("stale entry: %+v", st)
	}
	writeTestFile(t, state, `{`)
	_, stderr, code := runStc(t, append([]string{"sim"}, args...)...)
	if code == 0 || !strings.Contains(stderr, "malformed state file") {
		t.Errorf("malformed: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, append([]string{"sim", "--persist-interval", "0s"}, args...)...)
	if code == 0 || !strings.Contains(stderr, "--persist-interval must be positive") {
		t.Errorf("interval: exit %d %s", code, stderr)
	}
}

// writeECatProject copies the Demo EtherCAT GVL as ECT.st with its types and
// a MAIN program into a temp dir.
func writeECatProject(t *testing.T, main string) []string {
	t.Helper()
	dir := t.TempDir()
	var files []string
	for src, dst := range map[string]string{"demo_types.st": "demo_types.st", "demo_ect.st": "ECT.st"} {
		b, err := os.ReadFile(filepath.Join(ecatFixtures, src))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, filepath.Join(dir, dst))
		writeTestFile(t, files[len(files)-1], string(b))
	}
	files = append(files, filepath.Join(dir, "main.st"))
	writeTestFile(t, files[len(files)-1], main)
	return files
}

func TestSimProjectIO(t *testing.T) {
	files := writeECatProject(t, "PROGRAM MAIN\nECT.CN01();\nECT.V1_C1 := 16#5A;\nEND_PROGRAM\n")
	io := []string{"--io", filepath.Join(ecatFixtures, "Demo Device 1.xml"), "--io", filepath.Join(ecatFixtures, "Demo Device 2.xml")}
	st := runSimProjectJSON(t, append(append(files, io...), "--cycles", "2", "--get", "ECT.Dev1_SlaveCount")...)
	if st.Get["ECT.Dev1_SlaveCount"] != float64(10) {
		t.Errorf("SlaveCount from the network: %v", st.Get)
	}

	// Without Device 2 the D2_I1 link does not resolve.
	_, stderr, code := runStc(t, append([]string{"sim", "--io", filepath.Join(ecatFixtures, "Demo Device 1.xml")}, files...)...)
	if code == 0 || !strings.Contains(stderr, "link target of ECT.D2_I1 not found") || !strings.Contains(stderr, "EtherCAT links do not resolve") {
		t.Errorf("unresolved link: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, append([]string{"sim", "--io", "nope.xml"}, files...)...)
	if code == 0 || !strings.Contains(stderr, "--io") {
		t.Errorf("missing export: exit %d %s", code, stderr)
	}
}

func TestSimProjectRealtime(t *testing.T) {
	dir, files := writePersistProject(t)
	state := filepath.Join(dir, "rt.json")
	st := runSimProjectJSON(t, append(files, "--realtime", "--duration", "200ms", "--persist", state,
		"--persist-interval", "50ms", "--set", "GVL_Cfg.p_cfg_Speed=3")...)
	if len(st.Tasks) != 1 || st.Tasks[0].Runs == 0 || st.SimTimeNS == 0 {
		t.Errorf("free-running: %+v", st)
	}
	if b, err := os.ReadFile(state); err != nil || !strings.Contains(string(b), `"GVL_Cfg.p_cfg_Speed": 3`) {
		t.Errorf("saved at stop: %v %s", err, b)
	}
}

func TestSimProjectUsage(t *testing.T) {
	_, files := writePersistProject(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--realtime", "--cycles", "5"}, "--cycles cannot be combined"},
		{[]string{"--duration", "1s"}, "--duration requires --realtime"},
		{[]string{"--realtime", "--duration", "-1s"}, "--duration must not be negative"},
		{[]string{"--cycles", "-1"}, "--cycles must not be negative"},
		{[]string{"--wave", "X:step"}, "--wave is not supported in project mode"},
		{[]string{"--set", "nope"}, "expected PATH=VALUE"},
		{[]string{"--set", "GVL_Cfg.missing=1"}, "--set GVL_Cfg.missing"},
		{[]string{"--get", "GVL_Cfg.missing"}, "--get GVL_Cfg.missing"},
	} {
		_, stderr, code := runStc(t, append(append([]string{"sim"}, files...), c.args...)...)
		if code == 0 || !strings.Contains(stderr, c.want) {
			t.Errorf("%v: exit %d %s", c.args, code, stderr)
		}
	}
	_, stderr, code := runStc(t, "sim", "--realtime")
	if code == 0 || !strings.Contains(stderr, "no project given") {
		t.Errorf("no inputs: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, "sim")
	if code == 0 || !strings.Contains(stderr, "accepts 1 arg") {
		t.Errorf("no args: exit %d %s", code, stderr)
	}
	// A broken source fails the load with its diagnostic.
	bad := filepath.Join(t.TempDir(), "bad.st")
	writeTestFile(t, bad, "PROGRAM MAIN\nx := ;\nEND_PROGRAM\n")
	_, stderr, code = runStc(t, "sim", "--project", bad)
	if code == 0 || !strings.Contains(stderr, "bad.st") {
		t.Errorf("broken source: exit %d %s", code, stderr)
	}
	// A failing scan reports the cycle.
	div := filepath.Join(t.TempDir(), "div.st")
	writeTestFile(t, div, "PROGRAM MAIN\nVAR\n\tz : INT;\n\tq : INT;\nEND_VAR\nq := 1 / z;\nEND_PROGRAM\n")
	_, stderr, code = runStc(t, "sim", "--project", div, "--cycles", "2")
	if code == 0 || !strings.Contains(stderr, "cycle 1") {
		t.Errorf("scan error: exit %d %s", code, stderr)
	}
}
