package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const (
	demoTestsDir = "../../pkg/twincat/testdata/sln/tests"
	inlineTsproj = "../../pkg/twincat/testdata/inline/Inline.tsproj"
)

func TestTestProjectDemo(t *testing.T) {
	stdout, stderr, code := runStc(t, "test", "--project", demoTsproj, demoTestsDir, "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
	var res struct {
		Total    int      `json:"total"`
		Passed   int      `json:"passed"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	if res.Total != 2 || res.Passed != 2 {
		t.Errorf("result %s", stdout)
	}
	for _, w := range res.Warnings {
		if strings.Contains(w, "FB_Motor") || strings.Contains(w, "FB_LibThing") {
			t.Errorf("project FB auto-stubbed: %s", w)
		}
	}

	// Text mode prints import warnings to stderr and passes.
	stdout, stderr, code = runStc(t, "test", "--project", demoTsproj, demoTestsDir)
	if code != 0 || !strings.Contains(stdout, "2 passed") || !strings.Contains(stderr, "Tc2_Missing") {
		t.Errorf("text: exit %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestTestProjectErrors(t *testing.T) {
	_, stderr, code := runStc(t, "test", "--project", "nope/Nope.tsproj", demoTestsDir)
	if code == 0 || !strings.Contains(stderr, "Nope.tsproj") {
		t.Errorf("missing project: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, "test", "--project", "../../testdata/parse/motor_control.st", demoTestsDir)
	if code == 0 || !strings.Contains(stderr, "tsproj") {
		t.Errorf("not a project: exit %d %s", code, stderr)
	}
	// An Error diagnostic during import aborts.
	dir := t.TempDir()
	p := filepath.Join(dir, "Bad.plcproj")
	writeTestFile(t, p, `<Project><ItemGroup><Compile Include="POUs\Bad.TcPOU" /></ItemGroup></Project>`)
	writeTestFile(t, filepath.Join(dir, "POUs", "Bad.TcPOU"), "<TcPlcObject><POU")
	_, stderr, code = runStc(t, "test", "--project", p, demoTestsDir)
	if code != 1 || !strings.Contains(stderr, "Bad.TcPOU") {
		t.Errorf("bad item: exit %d %s", code, stderr)
	}
}

type simJSON struct {
	NumCycles int   `json:"num_cycles"`
	Duration  int64 `json:"duration"`
	Cycles    []struct {
		Time int64 `json:"time"`
	} `json:"cycles"`
}

func runSimJSON(t *testing.T, args ...string) simJSON {
	t.Helper()
	stdout, stderr, code := runStc(t, append([]string{"sim", "--format", "json"}, args...)...)
	if code != 0 {
		t.Fatalf("sim %v: exit %d %s", args, code, stderr)
	}
	var r simJSON
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	return r
}

func TestSimProjectInline(t *testing.T) {
	r := runSimJSON(t, inlineTsproj, "--cycles", "10")
	if r.NumCycles != 10 || r.Duration != 200_000_000 || r.Cycles[1].Time-r.Cycles[0].Time != 20_000_000 {
		t.Errorf("default dt from task: %+v", r)
	}
	r = runSimJSON(t, inlineTsproj, "--cycles", "10", "--dt", "5ms")
	if r.Duration != 50_000_000 {
		t.Errorf("--dt override: %+v", r)
	}
}

// writeTwoProgramProject writes a plcproj whose task calls the second of two
// programs; only Second sets the GVL flag.
func writeTwoProgramProject(t *testing.T, call string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "Two.plcproj")
	writeTestFile(t, p, `<Project><ItemGroup>
<Compile Include="GVL_Sim.TcGVL" />
<Compile Include="First.TcPOU" />
<Compile Include="Second.TcPOU" />
<Compile Include="Task.TcTTO" />
</ItemGroup></Project>`)
	writeTestFile(t, filepath.Join(dir, "GVL_Sim.TcGVL"), `<TcPlcObject><GVL Name="GVL_Sim"><Declaration><![CDATA[VAR_GLOBAL
	hits : DINT;
END_VAR]]></Declaration></GVL></TcPlcObject>`)
	for _, name := range []string{"First", "Second"} {
		writeTestFile(t, filepath.Join(dir, name+".TcPOU"), fmt.Sprintf(`<TcPlcObject><POU Name="%[1]s"><Declaration><![CDATA[PROGRAM %[1]s
VAR_OUTPUT
	which%[1]s : DINT;
END_VAR]]></Declaration><Implementation><ST><![CDATA[GVL_Sim.hits := GVL_Sim.hits + 1;
which%[1]s := GVL_Sim.hits;]]></ST></Implementation></POU></TcPlcObject>`, name))
	}
	writeTestFile(t, filepath.Join(dir, "Task.TcTTO"), `<TcPlcObject><Task Name="T"><CycleTime>2000</CycleTime><Priority>1</Priority>
<PouCall><Name>`+call+`</Name></PouCall></Task></TcPlcObject>`)
	return p
}

func TestSimProjectTaskProgram(t *testing.T) {
	p := writeTwoProgramProject(t, "second")
	stdout, stderr, code := runStc(t, "sim", p, "--cycles", "3", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	if !strings.Contains(stdout, "whichSecond") || strings.Contains(stdout, "whichFirst") {
		t.Errorf("task program not selected:\n%s", stdout)
	}
	var r simJSON
	_ = json.Unmarshal([]byte(stdout), &r)
	if r.Duration != 6_000_000 {
		t.Errorf("duration %d, want 6ms from the 2ms task", r.Duration)
	}

	// A PouCall naming no PROGRAM falls back to the first PROGRAM.
	p = writeTwoProgramProject(t, "Missing")
	stdout, _, code = runStc(t, "sim", p, "--cycles", "1", "--format", "json")
	if code != 0 || !strings.Contains(stdout, "whichFirst") {
		t.Errorf("fallback: exit %d %s", code, stdout)
	}
}

func TestSimProjectErrors(t *testing.T) {
	_, stderr, code := runStc(t, "sim", "nope/Nope.tsproj")
	if code == 0 || !strings.Contains(stderr, "Nope.tsproj") {
		t.Errorf("missing project: exit %d %s", code, stderr)
	}
	// The broken fixture has a PROGRAM; Demo has none it can run without FBs,
	// so use a project with no PROGRAM at all.
	dir := t.TempDir()
	p := filepath.Join(dir, "NoProg.plcproj")
	writeTestFile(t, p, `<Project><ItemGroup><Compile Include="F.TcPOU" /></ItemGroup></Project>`)
	writeTestFile(t, filepath.Join(dir, "F.TcPOU"), `<TcPlcObject><POU Name="F"><Declaration><![CDATA[FUNCTION F : INT]]></Declaration><Implementation><ST><![CDATA[F := 1;]]></ST></Implementation></POU></TcPlcObject>`)
	_, stderr, code = runStc(t, "sim", p)
	if code == 0 || !strings.Contains(stderr, "no PROGRAM") {
		t.Errorf("no program: exit %d %s", code, stderr)
	}
}
