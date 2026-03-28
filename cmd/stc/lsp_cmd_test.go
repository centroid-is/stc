package main

import (
	"strings"
	"testing"
)

func TestLspHelp(t *testing.T) {
	stdout, stderr, exitCode := runStc(t, "lsp", "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "LSP server") {
		t.Errorf("expected help to mention 'LSP server', got: %s", stdout)
	}
	if !strings.Contains(stdout, "lsp") {
		t.Errorf("expected help to mention 'lsp' command, got: %s", stdout)
	}
}

func TestLspRegistered(t *testing.T) {
	// Verify the lsp command appears in root help
	stdout, _, exitCode := runStc(t, "--help")
	if exitCode != 0 {
		t.Fatalf("expected exit 0 for root --help, got %d", exitCode)
	}
	if !strings.Contains(stdout, "lsp") {
		t.Errorf("expected 'lsp' in root help output, got: %s", stdout)
	}
}
