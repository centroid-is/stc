package testing

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/pipeline"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	// Find testdata relative to this test file
	dir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("testdata directory not found: %s", dir)
	}
	return dir
}

func TestRun_PassingTests(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "passing_test.st"), filepath.Join(dir, "passing_test.st"))

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 tests, got %d", result.Total)
	}
	if result.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if result.HasFailures() {
		t.Error("expected no failures")
	}

	// Check test names
	names := collectTestNames(result)
	if !containsName(names, "Addition works") {
		t.Errorf("expected test 'Addition works', got: %v", names)
	}
	if !containsName(names, "Boolean logic") {
		t.Errorf("expected test 'Boolean logic', got: %v", names)
	}
}

func TestRun_FailingTests(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "failing_test.st"), filepath.Join(dir, "failing_test.st"))

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test, got %d", result.Total)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if !result.HasFailures() {
		t.Error("expected failures")
	}

	// Check that failure includes position info
	for _, suite := range result.Suites {
		for _, tr := range suite.Tests {
			if !tr.Passed {
				hasPos := false
				for _, a := range tr.Assertions {
					if !a.Passed && a.Position != "" {
						hasPos = true
					}
				}
				if !hasPos {
					t.Error("failed assertion should have file:line:col position")
				}
			}
		}
	}
}

func TestRun_TimerTest(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "timer_test.st"), filepath.Join(dir, "timer_test.st"))

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test, got %d", result.Total)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d; failures: %+v", result.Failed, describeFailures(result))
	}
}

func TestRun_MultiAssert(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "multi_assert_test.st"), filepath.Join(dir, "multi_assert_test.st"))

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test, got %d", result.Total)
	}

	// Should have 3 assertions total, 1 failure
	if len(result.Suites) == 0 || len(result.Suites[0].Tests) == 0 {
		t.Fatal("expected at least 1 suite with 1 test")
	}
	tr := result.Suites[0].Tests[0]
	if len(tr.Assertions) != 3 {
		t.Errorf("expected 3 assertions collected, got %d", len(tr.Assertions))
	}
	failCount := 0
	for _, a := range tr.Assertions {
		if !a.Passed {
			failCount++
		}
	}
	if failCount != 1 {
		t.Errorf("expected 1 failure among assertions, got %d", failCount)
	}
}

func TestRun_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 tests, got %d", result.Total)
	}
	if result.HasFailures() {
		t.Error("empty directory should have no failures")
	}
}

func TestRun_IsolatedState(t *testing.T) {
	// Two TEST_CASE blocks in one file should not share state
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "passing_test.st"), filepath.Join(dir, "passing_test.st"))

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// Both tests should pass independently
	if result.Failed != 0 {
		t.Errorf("expected 0 failures for isolated tests, got %d", result.Failed)
	}
}

func TestFormatJUnit(t *testing.T) {
	result := &RunResult{
		Suites: []SuiteResult{
			{
				Name: "example_test.st",
				Tests: []TestResult{
					{Name: "Test passes", File: "example_test.st", Line: 1, Passed: true},
					{Name: "Test fails", File: "example_test.st", Line: 10, Passed: false,
						Assertions: []AssertionResultJSON{
							{Passed: false, Message: "expected 10, got 5", Position: "example_test.st:12:3"},
						},
					},
				},
			},
		},
		Total:  2,
		Passed: 1,
		Failed: 1,
	}

	data, err := FormatJUnit(result)
	if err != nil {
		t.Fatalf("FormatJUnit failed: %v", err)
	}

	xmlStr := string(data)

	// Must be valid XML
	var suites JUnitTestSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, xmlStr)
	}

	if suites.Tests != 2 {
		t.Errorf("expected 2 tests, got %d", suites.Tests)
	}
	if suites.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", suites.Failures)
	}
	if len(suites.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(suites.Suites))
	}
	if len(suites.Suites[0].TestCases) != 2 {
		t.Errorf("expected 2 testcases, got %d", len(suites.Suites[0].TestCases))
	}

	// Check XML header
	if !strings.HasPrefix(xmlStr, "<?xml") {
		t.Error("JUnit XML should start with XML declaration")
	}
}

func TestFormatJSON(t *testing.T) {
	result := &RunResult{
		Suites: []SuiteResult{
			{
				Name: "example_test.st",
				Tests: []TestResult{
					{Name: "Test passes", File: "example_test.st", Line: 1, Passed: true},
				},
			},
		},
		Total:  1,
		Passed: 1,
	}

	data, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("output is not valid JSON: %s", string(data))
	}
	if !strings.Contains(string(data), "Test passes") {
		t.Error("JSON should contain test name")
	}
}

func TestRunWithOpts_MockFB(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "mock_fb_test.st"), filepath.Join(dir, "mock_fb_test.st"))

	// Parse mock FB file
	mockPath := filepath.Join(testdataDir(t), "mock_mc_moveabsolute.st")
	mockFiles := parseFiles(t, mockPath)

	// Parse stub (library) file
	stubPath := filepath.Join(testdataDir(t), "stub_mc_moveabsolute.st")
	libFiles := parseFiles(t, stubPath)

	result, err := RunWithOpts(dir, RunOpts{
		LibraryFiles: libFiles,
		MockFiles:    mockFiles,
	})
	if err != nil {
		t.Fatalf("RunWithOpts failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test, got %d", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d; failures: %v", result.Passed, describeFailures(result))
	}
}

func TestRunWithOpts_AutoStubZeroValues(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "autostub_test.st"), filepath.Join(dir, "autostub_test.st"))

	// Only library stubs, no mocks -- should auto-stub with zero values
	stubPath := filepath.Join(testdataDir(t), "stub_mc_moveabsolute.st")
	libFiles := parseFiles(t, stubPath)

	result, err := RunWithOpts(dir, RunOpts{
		LibraryFiles: libFiles,
	})
	if err != nil {
		t.Fatalf("RunWithOpts failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test, got %d", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed (auto-stub zero values), got %d; failures: %v", result.Passed, describeFailures(result))
	}
}

func TestRunWithOpts_AutoStubWarning(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "autostub_test.st"), filepath.Join(dir, "autostub_test.st"))

	stubPath := filepath.Join(testdataDir(t), "stub_mc_moveabsolute.st")
	libFiles := parseFiles(t, stubPath)

	result, err := RunWithOpts(dir, RunOpts{
		LibraryFiles: libFiles,
	})
	if err != nil {
		t.Fatalf("RunWithOpts failed: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected fidelity warnings for auto-stubbed FB, got none")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "MC_MoveAbsolute") && strings.Contains(w, "auto-stub") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected auto-stub warning mentioning MC_MoveAbsolute, got: %v", result.Warnings)
	}
}

func TestRunWithOpts_SetIO_GetIO(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "io_inject_test.st"), filepath.Join(dir, "io_inject_test.st"))

	result, err := RunWithOpts(dir, RunOpts{})
	if err != nil {
		t.Fatalf("RunWithOpts failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 test, got %d", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d; failures: %v", result.Passed, describeFailures(result))
	}
}

func TestRun_BackwardCompatible(t *testing.T) {
	// Run (no opts) should still work exactly as before
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir(t), "passing_test.st"), filepath.Join(dir, "passing_test.st"))

	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 tests, got %d", result.Total)
	}
	if result.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", result.Passed)
	}
}

// --- helpers ---

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("failed to write %s: %v", dst, err)
	}
}

func collectTestNames(r *RunResult) []string {
	var names []string
	for _, s := range r.Suites {
		for _, tr := range s.Tests {
			names = append(names, tr.Name)
		}
	}
	return names
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func parseFiles(t *testing.T, paths ...string) []*ast.SourceFile {
	t.Helper()
	var result []*ast.SourceFile
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("failed to read %s: %v", p, err)
		}
		parsed := pipeline.Parse(p, string(content), nil)
		if parsed.File != nil {
			result = append(result, parsed.File)
		}
	}
	return result
}

func describeFailures(r *RunResult) []string {
	var out []string
	for _, s := range r.Suites {
		for _, tr := range s.Tests {
			if !tr.Passed {
				for _, a := range tr.Assertions {
					if !a.Passed {
						out = append(out, a.Message+" at "+a.Position)
					}
				}
				if tr.Error != "" {
					out = append(out, "error: "+tr.Error)
				}
			}
		}
	}
	return out
}
