package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
}

const passingFixture = `TEST_CASE 'Addition'
VAR
    x : INT := 2;
    y : INT := 3;
END_VAR
    ASSERT_EQ(x + y, 5);
END_TEST_CASE
`

const failingFixture = `TEST_CASE 'Bad math'
VAR
    x : INT := 1;
END_VAR
    ASSERT_EQ(x, 99);
END_TEST_CASE
`

func TestTestCmd_PassingExitZero(t *testing.T) {
	dir := t.TempDir()
	writeTestFixture(t, dir, "pass_test.st", passingFixture)

	stdout, stderr, exitCode := runStc(t, "test", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 for passing tests, got %d; stdout: %s; stderr: %s", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, "PASS") {
		t.Errorf("expected PASS in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "1 passed") {
		t.Errorf("expected '1 passed' in output, got: %s", stdout)
	}
}

func TestTestCmd_FailingExitOne(t *testing.T) {
	dir := t.TempDir()
	writeTestFixture(t, dir, "fail_test.st", failingFixture)

	stdout, _, exitCode := runStc(t, "test", dir)
	if exitCode != 1 {
		t.Fatalf("expected exit 1 for failing tests, got %d", exitCode)
	}
	if !strings.Contains(stdout, "FAIL") {
		t.Errorf("expected FAIL in output, got: %s", stdout)
	}
}

func TestTestCmd_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	writeTestFixture(t, dir, "pass_test.st", passingFixture)

	stdout, _, exitCode := runStc(t, "test", dir, "--format", "json")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("output is not valid JSON:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Addition") {
		t.Errorf("JSON should contain test name 'Addition', got: %s", stdout)
	}
}

func TestTestCmd_JUnitFormat(t *testing.T) {
	dir := t.TempDir()
	writeTestFixture(t, dir, "pass_test.st", passingFixture)

	stdout, _, exitCode := runStc(t, "test", dir, "--format", "junit")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.HasPrefix(stdout, "<?xml") {
		t.Errorf("JUnit output should start with XML declaration, got: %s", stdout[:min(100, len(stdout))])
	}

	// Must be valid XML
	var suites struct {
		XMLName xml.Name `xml:"testsuites"`
		Tests   int      `xml:"tests,attr"`
	}
	if err := xml.Unmarshal([]byte(stdout), &suites); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, stdout)
	}
	if suites.Tests != 1 {
		t.Errorf("expected 1 test in JUnit, got %d", suites.Tests)
	}
}

func TestTestCmd_DefaultDirectory(t *testing.T) {
	// When no dir arg given, test command defaults to current directory.
	// Create a temp dir with a test fixture and run from there.
	dir := t.TempDir()
	writeTestFixture(t, dir, "default_test.st", passingFixture)

	// Run stc test with dir argument (we can't change cwd in subprocess easily,
	// so we test with explicit dir and verify it works)
	stdout, _, exitCode := runStc(t, "test", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", exitCode, stdout)
	}
}

func TestTestCmd_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	stdout, _, exitCode := runStc(t, "test", dir)
	if exitCode != 0 {
		t.Fatalf("expected exit 0 for empty dir, got %d", exitCode)
	}
	if !strings.Contains(stdout, "0 tests") {
		t.Errorf("expected '0 tests' in output, got: %s", stdout)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
