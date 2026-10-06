package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVendorCmd_Help(t *testing.T) {
	stdout, _, exitCode := runStc(t, "vendor", "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit 0 for vendor --help, got %d", exitCode)
	}
	if !strings.Contains(stdout, "extract") {
		t.Errorf("vendor --help should list extract subcommand, got: %s", stdout)
	}
}

func TestVendorExtractCmd_MissingArg(t *testing.T) {
	_, _, exitCode := runStc(t, "vendor", "extract")
	if exitCode == 0 {
		t.Error("expected non-zero exit for missing argument")
	}
}

func TestVendorExtractCmd_DemoOrder(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "vendor", "extract", "../../pkg/twincat/testdata/sln/Demo/Demo/Demo.plcproj")
	if exitCode != 0 {
		t.Fatalf("exit %d: %s", exitCode, stderr)
	}
	names := []string{"GVL_Main", "GVL_Quoted", "ST_Point", "E_Mode", "MAIN", "FB_Motor", "I_Motor", "F_Add"}
	last := -1
	for _, n := range names {
		i := strings.Index(stdout, "(* "+n+" *)")
		if i <= last {
			t.Fatalf("%s out of order in:\n%s", n, stdout)
		}
		last = i
	}
	if !strings.Contains(stderr, "VEND021") {
		t.Errorf("stderr lacks VEND021: %s", stderr)
	}
}

func TestVendorExtractCmd_OutputDir(t *testing.T) {
	out := t.TempDir()
	_, stderr, exitCode := runStc(t, "vendor", "extract", "-o", out, "../../pkg/twincat/testdata/sln/Demo/Demo/Demo.plcproj")
	if exitCode != 0 {
		t.Fatalf("exit %d: %s", exitCode, stderr)
	}
	raw, err := os.ReadFile(filepath.Join(out, "FB_Motor.st"))
	if err != nil || !strings.Contains(string(raw), "END_FUNCTION_BLOCK") {
		t.Errorf("FB_Motor.st: %v %s", err, raw)
	}
}

func TestVendorExtractCmd_MissingFile(t *testing.T) {
	_, _, exitCode := runStc(t, "vendor", "extract", "nope.plcproj")
	if exitCode == 0 {
		t.Error("expected non-zero exit for a missing plcproj")
	}
}
