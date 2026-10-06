package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const brokenPlcproj = "../../pkg/twincat/testdata/broken/Broken.plcproj"

type checkDiag struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Pos      struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Col  int    `json:"col"`
	} `json:"pos"`
}

func TestCheckProjectDemo(t *testing.T) {
	_, stderr, code := runStc(t, "check", demoTsproj)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stderr, "0 error(s)") || !strings.Contains(stderr, "Demo.plcproj:") {
		t.Errorf("stderr: %s", stderr)
	}

	stdout, stderr, code := runStc(t, "check", demoTsproj, "--format", "json")
	if code != 0 {
		t.Fatalf("json exit %d: %s", code, stderr)
	}
	var ds []checkDiag
	if err := json.Unmarshal([]byte(stdout), &ds); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	codes := map[string]bool{}
	for _, d := range ds {
		codes[d.Code] = true
		if d.Code == "VEND020" && (!strings.HasSuffix(d.Pos.File, "Demo.plcproj") || d.Pos.Line == 0) {
			t.Errorf("VEND020 position %+v", d.Pos)
		}
		if d.Severity == "error" {
			t.Errorf("unexpected error %+v", d)
		}
	}
	if !codes["VEND020"] || !codes["SEMA039"] {
		t.Errorf("codes %v", codes)
	}
}

func TestCheckProjectBrokenJSON(t *testing.T) {
	stdout, _, code := runStc(t, "check", brokenPlcproj, "--format", "json")
	if code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, stdout)
	}
	var ds []checkDiag
	if err := json.Unmarshal([]byte(stdout), &ds); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	found := false
	for _, d := range ds {
		if d.Code == "SEMA010" && strings.HasSuffix(filepath.ToSlash(d.Pos.File), "POUs/MAIN.TcPOU") && d.Pos.Line == 11 {
			found = true
		}
	}
	if !found {
		t.Errorf("no SEMA010 at MAIN.TcPOU:11 in %s", stdout)
	}
}

func TestCheckProjectArgs(t *testing.T) {
	_, stderr, code := runStc(t, "check", demoTsproj, "../../testdata/parse/motor_control.st")
	if code == 0 || !strings.Contains(stderr, "project") {
		t.Errorf("mixed args: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, "check", demoTsproj, demoPlcproj)
	if code == 0 || !strings.Contains(stderr, "project") {
		t.Errorf("two projects: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, "check", demoTsproj, "--gvl-name", "X")
	if code == 0 || !strings.Contains(stderr, "--gvl-name") {
		t.Errorf("--gvl-name: exit %d %s", code, stderr)
	}
	_, stderr, code = runStc(t, "check", "nope/Nope.plcproj")
	if code == 0 || !strings.Contains(stderr, "Nope.plcproj") {
		t.Errorf("missing project: exit %d %s", code, stderr)
	}
}

func TestCheckProjectUpperCaseExtAndVendor(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, "../../pkg/twincat/testdata/broken", dir)
	upper := filepath.Join(dir, "Broken.PLCPROJ")
	if err := os.Rename(filepath.Join(dir, "Broken.plcproj"), upper); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runStc(t, "check", upper)
	if code != 1 || !strings.Contains(stderr, "MAIN.TcPOU:11") {
		t.Errorf("upper-case ext: exit %d %s", code, stderr)
	}

	// --vendor applies in project mode: Demo uses OOP and pointers that the
	// schneider target flags.
	_, stderr, _ = runStc(t, "check", demoTsproj, "--vendor", "schneider")
	if !strings.Contains(stderr, "VEND") || !strings.Contains(strings.ToLower(stderr), "schneider") {
		t.Errorf("--vendor schneider: %s", stderr)
	}
}

// copyDir copies a directory tree.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
