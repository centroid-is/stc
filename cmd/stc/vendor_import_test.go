package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	demoTsproj  = "../../pkg/twincat/testdata/sln/Demo/Demo solution.tsproj"
	demoPlcproj = "../../pkg/twincat/testdata/sln/Demo/Demo/Demo.plcproj"
)

// runStcIn runs the stc binary with dir as its working directory.
func runStcIn(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(stcBinary, args...)
	cmd.Dir = dir
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	if coverDir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	}
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running stc: %v", err)
		}
		exitCode = ee.ExitCode()
	}
	return so.String(), se.String(), exitCode
}

type importJSON struct {
	PlcName string `json:"plc_name"`
	AmsPort int    `json:"ams_port"`
	Tasks   []struct {
		Name        string   `json:"name"`
		CycleTimeNs int64    `json:"cycle_time_ns"`
		Programs    []string `json:"programs"`
	} `json:"tasks"`
	Libraries []struct {
		Name         string `json:"name"`
		ResolvedFrom string `json:"resolved_from"`
	} `json:"libraries"`
	Sources []struct {
		Path    string `json:"path"`
		RelPath string `json:"rel_path"`
		Kind    string `json:"kind"`
		Name    string `json:"name"`
	} `json:"sources"`
	Diagnostics []struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Pos      struct {
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"pos"`
	} `json:"diagnostics"`
}

func TestVendorImportJSON(t *testing.T) {
	stdout, stderr, code := runStc(t, "vendor", "import", demoTsproj, "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var got importJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	if got.PlcName != "Demo" || got.AmsPort != 851 {
		t.Errorf("plc %s port %d", got.PlcName, got.AmsPort)
	}
	if len(got.Tasks) != 1 || got.Tasks[0].CycleTimeNs != 1000000 || !reflect.DeepEqual(got.Tasks[0].Programs, []string{"MAIN"}) {
		t.Errorf("tasks %+v", got.Tasks)
	}
	from := map[string]string{}
	for _, l := range got.Libraries {
		from[l.Name] = l.ResolvedFrom
	}
	want := map[string]string{"DemoLib": "sibling", "Tc2_System": "stub", "Tc2_Standard": "builtin", "Tc2_Missing": "unresolved"}
	for k, v := range want {
		if from[k] != v {
			t.Errorf("library %s resolved_from %q, want %q", k, from[k], v)
		}
	}
	if len(got.Sources) != 8 || got.Sources[0].Kind != "gvl" || got.Sources[0].RelPath != "GVLs/GVL_Main.TcGVL" || !filepath.IsAbs(got.Sources[0].Path) {
		t.Errorf("sources %+v", got.Sources)
	}
	found := false
	for _, d := range got.Diagnostics {
		if d.Code == "VEND020" && strings.HasSuffix(d.Pos.File, "Demo.plcproj") && d.Pos.Line > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("no VEND020 in %+v", got.Diagnostics)
	}
}

func TestVendorImportText(t *testing.T) {
	stdout, stderr, code := runStc(t, "vendor", "import", demoTsproj)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, s := range []string{
		"PLC Demo (AMS port 851)",
		"Task PlcTask: cycle 1ms, priority 20, programs MAIN",
		"3 POU(s), 2 GVL(s), 2 DUT(s), 1 interface(s)",
		"7 librar",
		"DemoLib: sibling",
		"Tc2_Missing: unresolved",
		"1 task(s)",
	} {
		if !strings.Contains(stdout, s) {
			t.Errorf("stdout lacks %q:\n%s", s, stdout)
		}
	}
	if !strings.Contains(stderr, "VEND020") || !strings.Contains(stderr, "Tc2_Missing") {
		t.Errorf("stderr lacks warnings: %s", stderr)
	}
}

func TestVendorImportErrors(t *testing.T) {
	_, stderr, code := runStc(t, "vendor", "import", "nope/Nope.tsproj")
	if code == 0 || !strings.Contains(stderr, "Nope.tsproj") {
		t.Errorf("missing file: exit %d, stderr %s", code, stderr)
	}
	_, _, code = runStc(t, "vendor", "import")
	if code == 0 {
		t.Error("want non-zero exit without an argument")
	}
	// The broken fixture imports fine (no Error diagnostics at import time).
	_, stderr, code = runStc(t, "vendor", "import", "../../pkg/twincat/testdata/broken/Broken.plcproj")
	if code != 0 {
		t.Errorf("broken import exit %d: %s", code, stderr)
	}
	// An unreadable item is an Error diagnostic and exits 1.
	dir := t.TempDir()
	p := filepath.Join(dir, "Bad.plcproj")
	writeTestFile(t, p, `<Project><ItemGroup><Compile Include="POUs\Bad.TcPOU" /></ItemGroup></Project>`)
	writeTestFile(t, filepath.Join(dir, "POUs", "Bad.TcPOU"), "<TcPlcObject><POU")
	stdout, _, code := runStc(t, "vendor", "import", p, "--format", "json")
	if code != 1 || !strings.Contains(stdout, "VEND027") {
		t.Errorf("bad item: exit %d stdout %s", code, stdout)
	}
}

func TestVendorImportHelp(t *testing.T) {
	stdout, _, code := runStc(t, "vendor", "import", "--help")
	if code != 0 || !strings.Contains(stdout, "--out") {
		t.Errorf("help exit %d: %s", code, stdout)
	}
}

func TestVendorImportOut(t *testing.T) {
	out := t.TempDir()
	stdout, stderr, code := runStc(t, "vendor", "import", demoTsproj, "--out", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Wrote") {
		t.Errorf("stdout lacks the written summary: %s", stdout)
	}
	var files []string
	for _, d := range []string{"GVLs", "DUTs", "POUs"} {
		entries, err := os.ReadDir(filepath.Join(out, d))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			files = append(files, d+"/"+e.Name())
		}
	}
	if len(files) != 8 {
		t.Fatalf("written user files %v", files)
	}
	args := append([]string{"check"}, files...)
	_, stderr, code = runStcIn(t, out, args...)
	if code != 0 || !strings.Contains(stderr, "0 error(s)") {
		t.Errorf("check on --out files: exit %d\n%s", code, stderr)
	}

	// JSON mode with --out still prints the model.
	out2 := t.TempDir()
	stdout, _, code = runStc(t, "vendor", "import", demoTsproj, "--out", out2, "--format", "json")
	if code != 0 || !json.Valid([]byte(stdout)) {
		t.Errorf("json --out exit %d: %s", code, stdout)
	}

	// --out pointing at a file fails.
	file := filepath.Join(t.TempDir(), "f")
	writeTestFile(t, file, "")
	_, stderr, code = runStc(t, "vendor", "import", demoTsproj, "--out", file)
	if code == 0 {
		t.Errorf("want failure for --out file: %s", stderr)
	}
}

func TestVendorExtractJSON(t *testing.T) {
	stdout, stderr, code := runStc(t, "vendor", "extract", demoPlcproj, "--format", "json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var got struct {
		Stubs []struct {
			Name    string `json:"name"`
			RelPath string `json:"rel_path"`
			Kind    string `json:"kind"`
			Text    string `json:"text"`
		} `json:"stubs"`
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	var names []string
	for _, s := range got.Stubs {
		names = append(names, s.Name)
		if s.Text == "" || s.RelPath == "" || s.Kind == "" {
			t.Errorf("stub %+v", s)
		}
	}
	want := []string{"GVL_Main", "GVL_Quoted", "ST_Point", "E_Mode", "MAIN", "FB_Motor", "I_Motor", "F_Add"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names %v", names)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "VEND021" {
		t.Errorf("diagnostics %+v", got.Diagnostics)
	}
	stdout, _, code = runStc(t, "vendor", "extract", "nope.plcproj", "--format", "json")
	if code == 0 {
		t.Errorf("missing file: exit 0, %s", stdout)
	}
}

func writeTestFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
