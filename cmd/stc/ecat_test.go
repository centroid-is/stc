package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ecatFixtures = "../../tests/ecat_fixtures"

func ecatIOArgs() []string {
	return []string{
		"--io", filepath.Join(ecatFixtures, "Demo Device 1.xml"),
		"--io", filepath.Join(ecatFixtures, "Demo Device 2.xml"),
	}
}

// ecatGVL copies demo_ect.st to a temp ECT.st so the GVL is named ECT.
func ecatGVL(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(ecatFixtures, "demo_ect.st"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "ECT.st")
	if err := os.WriteFile(p, src, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEcatValidateClean(t *testing.T) {
	args := append([]string{"ecat", "validate", "-D", "UNUSED"}, ecatIOArgs()...)
	args = append(args, filepath.Join(ecatFixtures, "demo_types.st"), ecatGVL(t))
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d\nstdout:%s\nstderr:%s", code, stdout, stderr)
	}
	for _, want := range []string{"VARIABLE", "ECT.A1_01.I1", "ECT.CN01.amsaddr", "Device 2 (EtherCAT)", "35 bindings, 0 errors, 0 warnings"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestEcatValidateBad(t *testing.T) {
	args := append([]string{"ecat", "validate"}, ecatIOArgs()...)
	args = append(args, filepath.Join(ecatFixtures, "demo_types.st"), filepath.Join(ecatFixtures, "demo_bad.st"))
	stdout, _, code := runStc(t, args...)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, stdout)
	}
	for _, want := range []string{"demo_bad.st:5:2: error: ECAT001", "ECAT002", "ECAT003", "ECAT004", "warning: ECAT005", "ECAT006", "2 bindings, 5 errors, 1 warnings"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestEcatValidateJSON(t *testing.T) {
	args := append([]string{"ecat", "validate", "--format", "json"}, ecatIOArgs()...)
	args = append(args, filepath.Join(ecatFixtures, "demo_types.st"), ecatGVL(t))
	stdout, stderr, code := runStc(t, args...)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	var res struct {
		Bindings []struct {
			Var, Link, Master, Dir, TypeName string
			Byte, Bit, BitLen                int
		} `json:"bindings"`
		Diagnostics []json.RawMessage `json:"diagnostics"`
		Images      map[string]struct {
			InBytes  int `json:"inBytes"`
			OutBytes int `json:"outBytes"`
		} `json:"images"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	if len(res.Bindings) != 35 || res.Diagnostics == nil || len(res.Diagnostics) != 0 {
		t.Fatalf("bindings %d diagnostics %v", len(res.Bindings), res.Diagnostics)
	}
	for _, m := range []string{"Device 1 (EtherCAT)", "Device 2 (EtherCAT)"} {
		if img, ok := res.Images[m]; !ok || img.InBytes == 0 || img.OutBytes == 0 {
			t.Errorf("image %q = %+v (present %v)", m, img, ok)
		}
	}
	b := res.Bindings[0]
	if b.Var != "ECT.A1_01.I1" || b.Dir != "in" || b.BitLen != 1 || b.TypeName != "BOOL" || b.Master != "Device 1 (EtherCAT)" {
		t.Errorf("first binding %+v", b)
	}
}

func TestEcatValidateJSONBad(t *testing.T) {
	args := append([]string{"ecat", "validate", "-f", "json"}, ecatIOArgs()...)
	args = append(args, filepath.Join(ecatFixtures, "demo_types.st"), filepath.Join(ecatFixtures, "demo_bad.st"))
	stdout, _, code := runStc(t, args...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	var res struct {
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if len(res.Diagnostics) != 6 || res.Diagnostics[0].Code != "ECAT001" {
		t.Errorf("diagnostics %+v", res.Diagnostics)
	}
}

func TestEcatValidateErrors(t *testing.T) {
	st := filepath.Join(ecatFixtures, "demo_types.st")
	missing := filepath.Join(t.TempDir(), "missing.xml")
	tests := []struct {
		name       string
		args       []string
		wantOut    string
		wantStderr string
	}{
		{"missing io flag", []string{"ecat", "validate", st}, "", "required flag"},
		{"no st files", append([]string{"ecat", "validate"}, ecatIOArgs()...), "", "requires at least 1 arg"},
		{"unreadable export", []string{"ecat", "validate", "--io", missing, st}, "", "error:"},
		{"unreadable export json", []string{"ecat", "validate", "--format", "json", "--io", missing, st}, `"error"`, ""},
		{"missing st file", append(append([]string{"ecat", "validate"}, ecatIOArgs()...), filepath.Join(t.TempDir(), "nope.st")), "", "error:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runStc(t, tt.args...)
			if code == 0 {
				t.Fatalf("exit 0, want non-zero")
			}
			if !strings.Contains(stdout, tt.wantOut) || !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stdout %q stderr %q", stdout, stderr)
			}
		})
	}
}

func TestEcatParseDiagnosticsReported(t *testing.T) {
	p := filepath.Join(t.TempDir(), "broken.st")
	if err := os.WriteFile(p, []byte("VAR_GLOBAL\n x : ;\nEND_VAR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runStc(t, append(append([]string{"ecat", "validate"}, ecatIOArgs()...), p)...)
	if code != 1 || !strings.Contains(stdout, "broken.st:") {
		t.Errorf("exit %d stdout %s", code, stdout)
	}
}
