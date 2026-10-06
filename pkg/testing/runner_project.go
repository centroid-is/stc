package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/scenario"
)

// This file is the project (plant) mode of the runner: with RunOpts.Plant
// set, every TEST_CASE runs against a fresh scenario.Plant built from the
// spec, on the plant's own interpreter, with the scenario built-ins (SET,
// GET, SIM_*, RUN_CYCLES and a plant-backed ADVANCE_TIME) registered.

// runProjectFile parses one test file and runs its TEST_CASEs against
// fresh Plants. A file that declares a FUNCTION, FUNCTION_BLOCK, TYPE,
// INTERFACE or GVL fails every case: in project mode declarations belong
// in the project (D-18).
func runProjectFile(filePath, baseDir string, spec *scenario.PlantSpec, defines map[string]bool) (*SuiteResult, error) {
	start := time.Now()
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}
	parsed := pipeline.Parse(filePath, string(content), defines)

	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		relPath = filePath
	}
	suite := &SuiteResult{Name: relPath}

	var testCases []*ast.TestCaseDecl
	var declared []string
	for _, d := range parsed.File.Declarations {
		switch d := d.(type) {
		case *ast.TestCaseDecl:
			testCases = append(testCases, d)
		case *ast.FunctionDecl:
			declared = append(declared, declName("FUNCTION", d.Name))
		case *ast.FunctionBlockDecl:
			declared = append(declared, declName("FUNCTION_BLOCK", d.Name))
		case *ast.TypeDecl:
			declared = append(declared, declName("TYPE", d.Name))
		case *ast.InterfaceDecl:
			declared = append(declared, declName("INTERFACE", d.Name))
		case *ast.GVLDecl:
			declared = append(declared, declName("VAR_GLOBAL", d.Name))
		}
	}

	if len(declared) > 0 {
		msg := fmt.Sprintf("project mode: test files may only contain TEST_CASEs; declare %s in the project instead",
			strings.Join(declared, ", "))
		if len(testCases) == 0 {
			suite.Tests = append(suite.Tests, TestResult{Name: relPath, File: filePath, Line: 1, Error: msg})
		}
		for _, tc := range testCases {
			suite.Tests = append(suite.Tests, TestResult{Name: tc.Name, File: filePath, Line: tc.Span().Start.Line, Error: msg})
		}
		suite.Duration = time.Since(start)
		return suite, nil
	}

	for _, tc := range testCases {
		suite.Tests = append(suite.Tests, executeProjectTestCase(tc, filePath, spec))
	}
	suite.Duration = time.Since(start)
	return suite, nil
}

// declName renders a declaration kind and name for the D-18 error.
func declName(kind string, id *ast.Ident) string {
	if id == nil || id.Name == "" {
		return kind
	}
	return kind + " " + id.Name
}

// executeProjectTestCase runs one TEST_CASE on a fresh Plant. The body runs
// directly on the plant's interpreter (never inside Project.Tick, so the
// built-ins can Tick, Get and Set without holding the Runtime mutex), in an
// environment whose parent is the interpreter's GlobalParent, so GVL paths
// resolve in the body.
func executeProjectTestCase(tc *ast.TestCaseDecl, filePath string, spec *scenario.PlantSpec) TestResult {
	start := time.Now()
	tr := TestResult{Name: tc.Name, File: filePath, Line: tc.Span().Start.Line}

	plant, err := spec.New()
	if err != nil {
		tr.Error = fmt.Sprintf("plant initialisation: %v", err)
		tr.Duration = time.Since(start)
		return tr
	}
	in := plant.Runtime().Interpreter()
	collector := &interp.AssertionCollector{}
	in.RegisterAssertions(collector)
	scenario.RegisterBuiltins(in, plant)

	env := interp.NewEnv(in.GlobalParent())
	in.RegisterInlineEnums(tc.Name, tc.VarBlocks)
	initializeTestEnv(in, env, tc.VarBlocks)

	if err := in.ExecStatements(env, tc.Body); err != nil {
		tr.Error = err.Error()
	}
	tr.Passed = !collector.HasFailures() && tr.Error == ""
	tr.Assertions = assertionsJSON(collector)
	tr.Duration = time.Since(start)
	return tr
}

// assertionsJSON converts the collected assertion results.
func assertionsJSON(collector *interp.AssertionCollector) []AssertionResultJSON {
	var out []AssertionResultJSON
	for _, ar := range collector.Results {
		pos := ""
		if ar.Pos.Line > 0 {
			pos = fmt.Sprintf("%s:%d:%d", ar.Pos.File, ar.Pos.Line, ar.Pos.Col)
		}
		out = append(out, AssertionResultJSON{Passed: ar.Passed, Message: ar.Message, Position: pos})
	}
	return out
}
