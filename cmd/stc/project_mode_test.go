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

// TestSimProjectInline runs a project whose task calls no program: the
// default 10 ms MAIN task applies and --dt is refused in project mode.
func TestSimProjectInline(t *testing.T) {
	st := runSimProjectJSON(t, inlineTsproj, "--cycles", "10")
	if st.Cycles != 10 || st.SimTimeNS != 100_000_000 || len(st.Tasks) != 1 || st.Tasks[0].Runs != 10 {
		t.Errorf("default task: %+v", st)
	}
	_, stderr, code := runStc(t, "sim", inlineTsproj, "--dt", "5ms")
	if code == 0 || !strings.Contains(stderr, "--dt is not supported in project mode") {
		t.Errorf("--dt in project mode: exit %d %s", code, stderr)
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
	st := runSimProjectJSON(t, p, "--cycles", "3", "--get", "Second.whichSecond", "--get", "First.whichFirst")
	if st.SimTimeNS != 6_000_000 || st.Tasks[0].CycleNS != 2_000_000 {
		t.Errorf("sim time %d, want 6ms from the 2ms task", st.SimTimeNS)
	}
	if st.Get["Second.whichSecond"] != float64(3) || st.Get["First.whichFirst"] != float64(0) {
		t.Errorf("only the task program runs: %v", st.Get)
	}

	// A PouCall naming no PROGRAM fails the load.
	p = writeTwoProgramProject(t, "Missing")
	_, stderr, code := runStc(t, "sim", p, "--cycles", "1")
	if code == 0 || !strings.Contains(stderr, "unknown PROGRAM Missing") {
		t.Errorf("unknown task program: exit %d %s", code, stderr)
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
	if code == 0 || !strings.Contains(stderr, "no PROGRAM MAIN") {
		t.Errorf("no program: exit %d %s", code, stderr)
	}
}
