package main

import (
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
