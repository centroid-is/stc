package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/twincat"
)

const demoTsproj = "../twincat/testdata/sln/Demo/Demo solution.tsproj"

// demoOpts imports the Demo project and returns RunOpts with its user files
// and sibling library sources as ProjectFiles and its stubs as LibraryFiles.
func demoOpts(t *testing.T) RunOpts {
	t.Helper()
	m, _, err := twincat.Import(demoTsproj, twincat.Options{})
	if err != nil {
		t.Fatal(err)
	}
	project, stubs, _ := twincat.ParseForTest(m, nil)
	return RunOpts{ProjectFiles: project, LibraryFiles: stubs}
}

func TestProjectFilesDemo(t *testing.T) {
	opts := demoOpts(t)
	res, err := RunWithOpts("../twincat/testdata/sln/tests", opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || res.Passed != 2 {
		for _, s := range res.Suites {
			for _, tr := range s.Tests {
				t.Logf("%s: passed=%v err=%s %+v", tr.Name, tr.Passed, tr.Error, tr.Assertions)
			}
		}
		t.Fatalf("total %d passed %d", res.Total, res.Passed)
	}
	for _, w := range res.Warnings {
		for _, n := range []string{"FB_Motor", "FB_LibThing"} {
			if strings.Contains(w, "'"+n+"'") {
				t.Errorf("project FB auto-stubbed: %s", w)
			}
		}
	}
}

func TestProjectFilesLocalOverrideAndStubs(t *testing.T) {
	opts := demoOpts(t)
	opts.LibraryFiles = append(opts.LibraryFiles, parseST(t, "stub.st", `FUNCTION_BLOCK FB_Stubbed
VAR_OUTPUT done : BOOL; END_VAR
END_FUNCTION_BLOCK
`))
	dir := t.TempDir()
	writeST(t, filepath.Join(dir, "override_test.st"), `FUNCTION F_Add : INT
VAR_INPUT a : INT; b : INT; END_VAR
F_Add := a * b;
END_FUNCTION

TEST_CASE 'local declarations win'
VAR
    s : FB_Stubbed;
END_VAR
    ASSERT_EQ(F_Add(a := 2, b := 3), 6);
    ASSERT_EQ(GVL_Main.counter, 0);
    s();
    ASSERT_FALSE(s.done);
END_TEST_CASE
`)
	res, err := RunWithOpts(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed != 1 {
		t.Fatalf("result %+v", res.Suites)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "'FB_Stubbed'") {
			found = true
		}
	}
	if !found {
		t.Errorf("stub FB not reported as auto-stub: %v", res.Warnings)
	}
}

func parseST(t *testing.T, name, src string) *ast.SourceFile {
	t.Helper()
	r := pipeline.Parse(name, src, nil)
	if len(r.Diags) > 0 {
		t.Fatalf("%s: %v", name, r.Diags)
	}
	return r.File
}

func writeST(t *testing.T, p, src string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}
