package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/scenario"
)

const plantSrc = `
{attribute 'qualified_only'}
VAR_GLOBAL
	xIn  : BOOL;
	nCnt : DINT;
END_VAR
PROGRAM MAIN
VAR
	xOut : BOOL;
END_VAR
GVL.nCnt := GVL.nCnt + 1;
xOut := GVL.xIn;
END_PROGRAM
`

// plantSpec builds a network-less PlantSpec from src (GVL named GVL).
func plantSpec(t *testing.T, src string) *scenario.PlantSpec {
	t.Helper()
	f := pipeline.Parse("GVL.st", src, nil).File
	spec, err := scenario.BuildPlantSpec(interp.ProjectSpec{Files: []*ast.SourceFile{f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &spec
}

func writeProjectTest(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunProjectMode(t *testing.T) {
	dir := t.TempDir()
	writeProjectTest(t, dir, "plant_test.st", `
TEST_CASE 'scans and GVL paths'
VAR
	n : DINT;
END_VAR
SET('GVL.xIn', TRUE);
RUN_CYCLES(3);
n := GVL.nCnt;
ASSERT_EQ(n, 3);
ASSERT_TRUE(GET('MAIN.xOut'));
ADVANCE_TIME(T#20ms);
ASSERT_EQ(GVL.nCnt, 5);
END_TEST_CASE

TEST_CASE 'fresh plant per case'
ASSERT_EQ(GVL.nCnt, 0);
ASSERT_FALSE(GVL.xIn);
END_TEST_CASE

TEST_CASE 'failing assertion'
ASSERT_TRUE(GVL.xIn, 'not set');
END_TEST_CASE

TEST_CASE 'no network'
SIM_TRIP('X', 1);
END_TEST_CASE
`)
	res, err := RunWithOpts(dir, RunOpts{Plant: plantSpec(t, plantSrc)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 4 || res.Passed != 2 || res.Failed != 1 || res.Errors != 1 {
		t.Fatalf("result %+v", res.Suites)
	}
	tests := res.Suites[0].Tests
	if tests[2].Passed || len(tests[2].Assertions) != 1 || tests[2].Assertions[0].Message != "not set" || tests[2].Assertions[0].Position == "" {
		t.Errorf("failing case %+v", tests[2])
	}
	if !strings.Contains(tests[3].Error, "no --io network loaded") {
		t.Errorf("no network: %q", tests[3].Error)
	}
}

func TestRunProjectModeRejectsDeclarations(t *testing.T) {
	dir := t.TempDir()
	writeProjectTest(t, dir, "a_test.st", `
FUNCTION F : INT
F := 1;
END_FUNCTION
FUNCTION_BLOCK FB
END_FUNCTION_BLOCK
TYPE T : STRUCT a : INT; END_STRUCT END_TYPE
INTERFACE I
END_INTERFACE
VAR_GLOBAL
	g : INT;
END_VAR
TEST_CASE 'one'
ASSERT_TRUE(TRUE);
END_TEST_CASE
`)
	writeProjectTest(t, dir, "b_test.st", `
FUNCTION_BLOCK OnlyDecl
END_FUNCTION_BLOCK
`)
	res, err := RunWithOpts(dir, RunOpts{Plant: plantSpec(t, plantSrc)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || res.Errors != 2 {
		t.Fatalf("result %+v", res.Suites)
	}
	msg := res.Suites[0].Tests[0].Error
	for _, want := range []string{"FUNCTION F", "FUNCTION_BLOCK FB", "TYPE T", "INTERFACE I", "VAR_GLOBAL", "declare"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in %q", want, msg)
		}
	}
	if b := res.Suites[1].Tests[0]; b.Name != "b_test.st" || !strings.Contains(b.Error, "OnlyDecl") {
		t.Errorf("declaration-only file %+v", b)
	}
	if declName("TYPE", nil) != "TYPE" {
		t.Error("declName without a name")
	}
}

func TestRunProjectModePlantError(t *testing.T) {
	dir := t.TempDir()
	writeProjectTest(t, dir, "p_test.st", "TEST_CASE 'x'\nASSERT_TRUE(TRUE);\nEND_TEST_CASE\n")
	spec := plantSpec(t, plantSrc)
	spec.Sources.Tasks = []interp.TaskSpec{{Name: "T", Cycle: 10 * time.Millisecond, Programs: []string{"NOPE"}}}
	res, err := RunWithOpts(dir, RunOpts{Plant: spec})
	if err != nil {
		t.Fatal(err)
	}
	if res.Errors != 1 || !strings.Contains(res.Suites[0].Tests[0].Error, "plant initialisation") {
		t.Fatalf("result %+v", res.Suites)
	}
}

func TestRunProjectFileReadError(t *testing.T) {
	if _, err := runProjectFile(filepath.Join(t.TempDir(), "missing_test.st"), ".", nil, nil); err == nil {
		t.Fatal("expected a read error")
	}
	// A directory named *_test.st is discovered and fails to read.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "x_test.st"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := runProjectFile(filepath.Join(dir, "x_test.st"), dir, nil, nil); err == nil {
		t.Fatal("expected a read error for a directory")
	}
}

// A project FUNCTION named like a scenario built-in (GET, SET) is called
// from project code; the built-ins resolve only in the TEST_CASE body, so
// the scan neither picks the built-in nor deadlocks on the Runtime mutex
// (review 2 HI-01).
func TestRunProjectModeUserFunctionNamedLikeBuiltin(t *testing.T) {
	src := `
{attribute 'qualified_only'}
VAR_GLOBAL
	nCnt : DINT;
END_VAR
FUNCTION GET : INT
VAR_INPUT
	x : STRING;
END_VAR
GET := 42;
END_FUNCTION
FUNCTION SET : INT
VAR_INPUT
	x : INT;
END_VAR
SET := x + 1;
END_FUNCTION
PROGRAM MAIN
VAR
	y : INT;
	z : INT;
END_VAR
y := GET('MAIN.y');
z := SET(4);
GVL.nCnt := GVL.nCnt + 1;
END_PROGRAM
`
	dir := t.TempDir()
	writeProjectTest(t, dir, "shadow_test.st", `
TEST_CASE 'project GET and SET'
RUN_CYCLES(2);
ASSERT_EQ(GET('MAIN.y'), 42);
ASSERT_EQ(GET('MAIN.z'), 5);
ASSERT_EQ(GVL.nCnt, 2);
SET('GVL.nCnt', 7);
ASSERT_EQ(GVL.nCnt, 7);
END_TEST_CASE
`)
	done := make(chan *RunResult, 1)
	go func() {
		res, err := RunWithOpts(dir, RunOpts{Plant: plantSpec(t, src)})
		if err != nil {
			t.Error(err)
		}
		done <- res
	}()
	select {
	case res := <-done:
		if res == nil {
			return
		}
		tc := res.Suites[0].Tests[0]
		if !tc.Passed {
			t.Fatalf("test failed: error=%q assertions=%+v", tc.Error, tc.Assertions)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("project FUNCTION GET deadlocked the scan")
	}
}
