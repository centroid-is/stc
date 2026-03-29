package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// DiscoverTestFiles finds all *_test.st files under dir recursively.
// Returns sorted paths.
func DiscoverTestFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), "_test.st") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// Run discovers and executes all *_test.st files in the given directory.
func Run(dir string) (*RunResult, error) {
	start := time.Now()

	files, err := DiscoverTestFiles(dir)
	if err != nil {
		return nil, err
	}

	result := &RunResult{}

	for _, file := range files {
		suiteResult, err := runFile(file, dir)
		if err != nil {
			return nil, fmt.Errorf("running %s: %w", file, err)
		}
		result.Suites = append(result.Suites, *suiteResult)
		for _, tr := range suiteResult.Tests {
			result.Total++
			if tr.Error != "" {
				result.Errors++
			} else if tr.Passed {
				result.Passed++
			} else {
				result.Failed++
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// runFile parses a single .st file and executes all TEST_CASE blocks.
func runFile(filePath, baseDir string) (*SuiteResult, error) {
	start := time.Now()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	parseResult := pipeline.Parse(filePath, string(content), nil)

	// Extract TestCaseDecl nodes
	var testCases []*ast.TestCaseDecl
	for _, decl := range parseResult.File.Declarations {
		if tc, ok := decl.(*ast.TestCaseDecl); ok {
			testCases = append(testCases, tc)
		}
	}

	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		relPath = filePath
	}

	suite := &SuiteResult{
		Name: relPath,
	}

	for _, tc := range testCases {
		tr := executeTestCase(tc, filePath)
		suite.Tests = append(suite.Tests, tr)
	}

	suite.Duration = time.Since(start)
	return suite, nil
}

// executeTestCase runs a single TEST_CASE in isolation with its own
// interpreter, environment, and assertion collector.
func executeTestCase(tc *ast.TestCaseDecl, filePath string) TestResult {
	start := time.Now()

	// Fresh interpreter and collector per test case
	interpreter := interp.New()
	collector := &interp.AssertionCollector{}
	interpreter.RegisterAssertions(collector)

	// Track virtual clock for ADVANCE_TIME
	var clock time.Duration
	interpreter.RegisterAdvanceTime(func(dt time.Duration) {
		clock += dt
		// Set interpreter.dt so subsequent FB calls see this delta
		interpreter.SetDt(dt)
	})

	// Create isolated environment
	env := interp.NewEnv(nil)

	// Initialize variables from VarBlocks
	initializeTestEnv(interpreter, env, tc.VarBlocks)

	// Execute test body
	var runtimeErr string
	err := interpreter.ExecStatements(env, tc.Body)
	if err != nil {
		// Check if it's a runtime error vs control flow
		runtimeErr = err.Error()
	}

	// Build result
	passed := !collector.HasFailures() && runtimeErr == ""
	tr := TestResult{
		Name:     tc.Name,
		File:     filePath,
		Line:     tc.Span().Start.Line,
		Passed:   passed,
		Duration: time.Since(start),
		Error:    runtimeErr,
	}

	// Convert assertion results
	for _, ar := range collector.Results {
		pos := ""
		if ar.Pos.Line > 0 {
			pos = fmt.Sprintf("%s:%d:%d", ar.Pos.File, ar.Pos.Line, ar.Pos.Col)
		}
		tr.Assertions = append(tr.Assertions, AssertionResultJSON{
			Passed:   ar.Passed,
			Message:  ar.Message,
			Position: pos,
		})
	}

	return tr
}

// initializeTestEnv populates the environment from VarBlocks, following the
// same pattern as ScanCycleEngine.initializeEnv for FB and variable creation.
func initializeTestEnv(interpreter *interp.Interpreter, env *interp.Env, varBlocks []*ast.VarBlock) {
	for _, vb := range varBlocks {
		for _, vd := range vb.Declarations {
			typeName := typeNameFromSpec(vd.Type)
			upperTypeName := strings.ToUpper(typeName)

			// Check if the type is a stdlib FB
			if factory, ok := interp.StdlibFBFactory[upperTypeName]; ok {
				for _, n := range vd.Names {
					fb := factory()
					val := interp.MakeFBInstanceValue(typeName, fb)
					env.Define(n.Name, val)
				}
				continue
			}

			// Resolve zero value from the type spec
			val := interp.ZeroFromTypeSpec(vd.Type)

			// Evaluate init value if present
			if vd.InitValue != nil {
				if iv, err := interpreter.EvalExpr(env, vd.InitValue); err == nil {
					val = iv
				}
			}

			for _, n := range vd.Names {
				env.Define(n.Name, val)
			}
		}
	}
}

// typeNameFromSpec extracts the type name string from an AST TypeSpec.
func typeNameFromSpec(ts ast.TypeSpec) string {
	if nt, ok := ts.(*ast.NamedType); ok && nt.Name != nil {
		return nt.Name.Name
	}
	return ""
}
