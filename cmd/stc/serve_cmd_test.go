package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeDuration(t *testing.T) {
	stdout, stderr, code := runStc(t, "serve", demoTsproj, "--duration", "300ms", "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d %s", code, stderr)
	}
	st := decodeProjectJSON(t, stdout)
	if len(st.Tasks) != 1 || st.Tasks[0].Runs == 0 || st.SimTimeNS == 0 || st.Tasks[0].CycleNS != 1_000_000 {
		t.Errorf("status: %+v", st)
	}

	// --run-for is the 28-04 spelling; --cycle overrides the single task.
	stdout, stderr, code = runStc(t, "serve", "--project", demoTsproj, "--run-for", "50ms", "--cycle", "5ms", "--realtime")
	if code != 0 || !strings.Contains(stdout, "PlcTask  5ms") {
		t.Errorf("text status: exit %d %s %s", code, stdout, stderr)
	}
}

// serveInProcess runs `stc serve args` on a fresh root command with ctx.
func serveInProcess(ctx context.Context, args ...string) (string, string, error) {
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"serve"}, args...))
	err := root.ExecuteContext(ctx)
	return out.String(), errOut.String(), err
}

func TestServeCancelPersists(t *testing.T) {
	dir, files := writePersistProject(t)
	state := filepath.Join(dir, "state.json")
	writeTestFile(t, state, `{"version":1,"values":{"GVL_Cfg.p_cfg_Speed":42.5,"GVL_Cfg.nStarts":4}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	stdout, stderr, err := serveInProcess(ctx, append(files, "--persist", state, "--persist-interval", "20ms", "--format", "json")...)
	if err != nil {
		t.Fatalf("cancel is a clean stop: %v %s", err, stderr)
	}
	st := decodeProjectJSON(t, stdout)
	if st.Tasks[0].Runs == 0 {
		t.Errorf("no runs: %+v", st)
	}
	b, err := os.ReadFile(state)
	if err != nil || !strings.Contains(string(b), `"GVL_Cfg.p_cfg_Speed": 42.5`) || !strings.Contains(string(b), `"GVL_Cfg.nStarts": 5`) {
		t.Errorf("state restored, advanced and saved on stop: %v %s", err, b)
	}
}

func TestServeSIGINT(t *testing.T) {
	dir, files := writePersistProject(t)
	state := filepath.Join(dir, "state.json")
	cmd := exec.Command(stcBinary, append([]string{"serve", "--persist", state, "--persist-interval", "10ms", "--format", "json"}, files...)...)
	if coverDir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(state); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("no periodic save: %s", errOut.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("SIGINT exit: %v %s", err, errOut.String())
	}
	st := decodeProjectJSON(t, out.String())
	if st.Tasks[0].Runs == 0 {
		t.Errorf("status: %+v", st)
	}
}

func TestServeUsage(t *testing.T) {
	_, stderr, code := runStc(t, "serve")
	if code == 0 || !strings.Contains(stderr, "no project given") {
		t.Errorf("no project: exit %d %s", code, stderr)
	}
	stdout, _, code := runStc(t, "serve", "--help")
	for _, want := range []string{"--project", "--io", "--persist", "--persist-interval", "--duration", "--cycle", "--realtime"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Errorf("help lacks %s: exit %d", want, code)
		}
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{demoTsproj, "--cycle", "0s"}, "--cycle must be positive"},
		{[]string{demoTsproj, "--duration", "-1s"}, "--duration must not be negative"},
		{[]string{demoTsproj, "--duration", "1s", "--run-for", "1s"}, "give only one"},
		{[]string{"nope/Nope.tsproj", "--duration", "10ms"}, "Nope.tsproj"},
	} {
		_, stderr, code := runStc(t, append([]string{"serve"}, c.args...)...)
		if code == 0 || !strings.Contains(stderr, c.want) {
			t.Errorf("%v: exit %d %s", c.args, code, stderr)
		}
	}

	// A state file that cannot be written fails at stop.
	_, files := writePersistProject(t)
	_, _, err := serveInProcess(context.Background(), append(files, "--duration", "20ms",
		"--persist", filepath.Join(t.TempDir(), "missing", "s.json"))...)
	if err == nil || !strings.Contains(err.Error(), "writing state file") {
		t.Errorf("unwritable state: %v", err)
	}
	// A failing scan stops serve with the error.
	div := filepath.Join(t.TempDir(), "div.st")
	writeTestFile(t, div, "PROGRAM MAIN\nVAR\n\tz : INT;\n\tq : INT;\nEND_VAR\nq := 1 / z;\nEND_PROGRAM\n")
	_, _, err = serveInProcess(context.Background(), div, "--duration", "1s")
	if err == nil || !strings.Contains(err.Error(), "serve:") {
		t.Errorf("scan error: %v", err)
	}
}

func TestProjectSetupCycle(t *testing.T) {
	// Several tasks keep their own cycles: --cycle is refused.
	dir := t.TempDir()
	p := filepath.Join(dir, "Two.plcproj")
	writeTestFile(t, p, `<Project><ItemGroup>
<Compile Include="A.TcPOU" /><Compile Include="B.TcPOU" />
<Compile Include="TA.TcTTO" /><Compile Include="TB.TcTTO" />
</ItemGroup></Project>`)
	for _, n := range []string{"A", "B"} {
		writeTestFile(t, filepath.Join(dir, n+".TcPOU"), `<TcPlcObject><POU Name="`+n+`"><Declaration><![CDATA[PROGRAM `+n+`
VAR
	x : INT;
END_VAR]]></Declaration><Implementation><ST><![CDATA[x := x + 1;]]></ST></Implementation></POU></TcPlcObject>`)
		writeTestFile(t, filepath.Join(dir, "T"+n+".TcTTO"), `<TcPlcObject><Task Name="T`+n+`"><CycleTime>1000</CycleTime><Priority>1</Priority>
<PouCall><Name>`+n+`</Name></PouCall></Task></TcPlcObject>`)
	}
	_, _, err := serveInProcess(context.Background(), p, "--cycle", "5ms", "--duration", "10ms")
	if err == nil || !strings.Contains(err.Error(), "--cycle cannot override the 2 task cycles") {
		t.Errorf("multi-task --cycle: %v", err)
	}
	// .st files without tasks get the default MAIN task at --cycle.
	_, files := writePersistProject(t)
	stdout, _, err := serveInProcess(context.Background(), append(files, "--cycle", "2ms", "--duration", "10ms", "--format", "json")...)
	if err != nil || decodeProjectJSON(t, stdout).Tasks[0].CycleNS != 2_000_000 {
		t.Errorf("default task --cycle: %v %s", err, stdout)
	}
}
