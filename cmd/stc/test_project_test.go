package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scenarioTestProject copies the scenario fixture project and
// scenario_test.st (into a tests/ subdirectory) and returns the stc test
// arguments for plant mode.
func scenarioTestProject(t *testing.T) (testsDir string, args []string) {
	t.Helper()
	dir, st, ioGlob := scenarioProject(t)
	testsDir = filepath.Join(dir, "tests")
	b, err := os.ReadFile(filepath.Join(ecatFixtures, "scenario", "scenario_test.st"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(testsDir, "scenario_test.st"), string(b))
	args = []string{"test", testsDir}
	for _, p := range st {
		args = append(args, "--project", p)
	}
	return testsDir, append(args, "--io", ioGlob)
}

type testRunJSON struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Errors int `json:"errors"`
}

func TestTestProjectPlantMode(t *testing.T) {
	testsDir, args := scenarioTestProject(t)
	stdout, stderr, code := runStc(t, append(args, "--format", "json")...)
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	var res testRunJSON
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, stdout)
	}
	if res.Total != 8 || res.Passed != 8 {
		t.Errorf("result %s", stdout)
	}

	// Text and JUnit formats keep their shape.
	stdout, _, code = runStc(t, args...)
	if code != 0 || !strings.Contains(stdout, "8 tests, 8 passed") {
		t.Errorf("text: exit %d\n%s", code, stdout)
	}
	stdout, _, code = runStc(t, append(args, "--format", "junit")...)
	if code != 0 || !strings.Contains(stdout, "<testsuites") {
		t.Errorf("junit: exit %d\n%s", code, stdout)
	}

	// A broken assertion fails the run.
	p := filepath.Join(testsDir, "scenario_test.st")
	b, _ := os.ReadFile(p)
	broken := strings.Replace(string(b), "ASSERT_EQ(ECT_Diag.nScanCount, UDINT#10);", "ASSERT_EQ(ECT_Diag.nScanCount, UDINT#11);", 1)
	writeTestFile(t, p, broken)
	stdout, _, code = runStc(t, append(args, "--format", "json")...)
	if code != 1 {
		t.Fatalf("broken: exit %d\n%s", code, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil || res.Failed != 1 || res.Passed != 7 {
		t.Errorf("broken result %s", stdout)
	}
}

func TestTestProjectPlantModeNoIO(t *testing.T) {
	_, args := scenarioTestProject(t)
	args = args[:len(args)-2] // drop --io
	stdout, _, code := runStc(t, append(args, "--format", "json")...)
	if code != 1 || !strings.Contains(stdout, "no --io network loaded") {
		t.Errorf("exit %d\n%s", code, stdout)
	}
}

func TestTestProjectPlantModeErrors(t *testing.T) {
	testsDir, args := scenarioTestProject(t)

	// --io without --project is a usage error.
	_, stderr, code := runStc(t, "test", testsDir, "--io", args[len(args)-1])
	if code == 0 || !strings.Contains(stderr, "--io requires --project") {
		t.Errorf("--io alone: exit %d %s", code, stderr)
	}
	// An --io glob without a match.
	noMatch := append(append([]string(nil), args[:len(args)-1]...), filepath.Join(testsDir, "none*.xml"))
	_, stderr, code = runStc(t, noMatch...)
	if code == 0 || !strings.Contains(stderr, "matches no file") {
		t.Errorf("no match: exit %d %s", code, stderr)
	}
	// An --io file that is not an EtherCAT export.
	bad := filepath.Join(testsDir, "bad.xml")
	writeTestFile(t, bad, "<nope")
	badIO := append(append([]string(nil), args[:len(args)-1]...), bad)
	_, stderr, code = runStc(t, badIO...)
	if code == 0 || !strings.Contains(stderr, "--io") {
		t.Errorf("bad io: exit %d %s", code, stderr)
	}
	// Links that do not resolve against the network (Device 2 only).
	dev2 := filepath.Join(filepath.Dir(args[len(args)-1]), "Demo Device 2.xml")
	unresolved := append(append([]string(nil), args[:len(args)-1]...), dev2)
	_, stderr, code = runStc(t, unresolved...)
	if code == 0 || !strings.Contains(stderr, "do not resolve") {
		t.Errorf("unresolved: exit %d %s", code, stderr)
	}
	// A project source with errors.
	broken := filepath.Join(testsDir, "broken.st")
	writeTestFile(t, broken, "PROGRAM P\nVAR x : INT; END_VAR\nx := ;\nEND_PROGRAM\n")
	_, stderr, code = runStc(t, "test", testsDir, "--project", broken)
	if code == 0 || !strings.Contains(stderr, "broken.st") {
		t.Errorf("broken project: exit %d %s", code, stderr)
	}
}
