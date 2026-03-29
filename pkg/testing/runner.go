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

// fileContext holds parsed declarations from a test file that are
// available to all TEST_CASE blocks in that file.
type fileContext struct {
	// typeDecls maps upper-case type names to their TypeSpec from TYPE blocks.
	typeDecls map[string]ast.TypeSpec
	// fbDecls maps upper-case FB names to their FunctionBlockDecl.
	fbDecls map[string]*ast.FunctionBlockDecl
	// funcDecls maps upper-case function names to their FunctionDecl.
	funcDecls map[string]*ast.FunctionDecl
}

// runFile parses a single .st file and executes all TEST_CASE blocks.
func runFile(filePath, baseDir string) (*SuiteResult, error) {
	start := time.Now()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	parseResult := pipeline.Parse(filePath, string(content), nil)

	// Build file context: collect TYPE, FUNCTION_BLOCK, and FUNCTION declarations
	ctx := &fileContext{
		typeDecls: make(map[string]ast.TypeSpec),
		fbDecls:   make(map[string]*ast.FunctionBlockDecl),
		funcDecls: make(map[string]*ast.FunctionDecl),
	}

	var testCases []*ast.TestCaseDecl
	for _, decl := range parseResult.File.Declarations {
		switch d := decl.(type) {
		case *ast.TestCaseDecl:
			testCases = append(testCases, d)
		case *ast.TypeDecl:
			if d.Name != nil {
				ctx.typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
			}
		case *ast.FunctionBlockDecl:
			if d.Name != nil {
				ctx.fbDecls[strings.ToUpper(d.Name.Name)] = d
			}
		case *ast.FunctionDecl:
			if d.Name != nil {
				ctx.funcDecls[strings.ToUpper(d.Name.Name)] = d
			}
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
		tr := executeTestCase(tc, filePath, ctx)
		suite.Tests = append(suite.Tests, tr)
	}

	suite.Duration = time.Since(start)
	return suite, nil
}

// executeTestCase runs a single TEST_CASE in isolation with its own
// interpreter, environment, and assertion collector.
func executeTestCase(tc *ast.TestCaseDecl, filePath string, ctx *fileContext) TestResult {
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

	// Register user-defined functions from the file context
	if ctx != nil {
		registerUserFunctions(interpreter, ctx)
		registerEnumTypes(interpreter, ctx)
	}

	// Create isolated environment
	env := interp.NewEnv(nil)

	// Initialize variables from VarBlocks
	initializeTestEnv(interpreter, env, tc.VarBlocks, ctx)

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

// registerUserFunctions registers user-defined FUNCTION declarations as
// callable functions in the interpreter.
func registerUserFunctions(interpreter *interp.Interpreter, ctx *fileContext) {
	for name, decl := range ctx.funcDecls {
		funcDecl := decl // capture for closure
		funcName := name
		interpreter.RegisterFunction(funcName, func(args []interp.Value, pos ast.Pos) (interp.Value, error) {
			return callUserFunction(interpreter, funcDecl, args)
		})
	}
}

// callUserFunction executes a user-defined FUNCTION with the given arguments.
func callUserFunction(parentInterp *interp.Interpreter, decl *ast.FunctionDecl, args []interp.Value) (interp.Value, error) {
	// Create a new environment for the function call
	env := interp.NewEnv(nil)

	// Initialize return variable (function name holds the return value)
	retTypeName := ""
	if decl.ReturnType != nil {
		retTypeName = typeNameFromSpec(decl.ReturnType)
	}
	retVal := interp.ZeroFromTypeSpec(decl.ReturnType)
	if decl.Name != nil {
		env.Define(decl.Name.Name, retVal)
	}

	// Map arguments to VAR_INPUT parameters
	argIdx := 0
	for _, vb := range decl.VarBlocks {
		if vb.Section == ast.VarInput {
			for _, vd := range vb.Declarations {
				for _, n := range vd.Names {
					if argIdx < len(args) {
						env.Define(n.Name, args[argIdx])
						argIdx++
					} else {
						env.Define(n.Name, interp.ZeroFromTypeSpec(vd.Type))
					}
				}
			}
		} else {
			// Initialize other var blocks
			for _, vd := range vb.Declarations {
				val := interp.ZeroFromTypeSpec(vd.Type)
				if vd.InitValue != nil {
					if iv, err := parentInterp.EvalExpr(env, vd.InitValue); err == nil {
						val = iv
					}
				}
				for _, n := range vd.Names {
					env.Define(n.Name, val)
				}
			}
		}
	}

	// Execute function body
	err := parentInterp.ExecStatements(env, decl.Body)
	if err != nil {
		// ErrReturn is normal function termination
		if err.Error() == "RETURN" {
			// Normal return
		} else {
			return interp.Value{}, err
		}
	}

	// Read return value from the function name variable
	if decl.Name != nil {
		if v, ok := env.Get(decl.Name.Name); ok {
			return v, nil
		}
	}

	_ = retTypeName
	return retVal, nil
}

// initializeTestEnv populates the environment from VarBlocks, following the
// same pattern as ScanCycleEngine.initializeEnv for FB and variable creation.
func initializeTestEnv(interpreter *interp.Interpreter, env *interp.Env, varBlocks []*ast.VarBlock, ctx *fileContext) {
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

			// Check if the type is a user-defined FB from the file context
			if ctx != nil {
				if fbDecl, ok := ctx.fbDecls[upperTypeName]; ok {
					for _, n := range vd.Names {
						inst := interp.NewUserFBInstance(typeName, fbDecl, interpreter, env)
						// Wire up parent FB declaration for EXTENDS chain
						if fbDecl.Extends != nil {
							parentName := strings.ToUpper(fbDecl.Extends.Name)
							if parentDecl, ok2 := ctx.fbDecls[parentName]; ok2 {
								inst.ParentDecl = parentDecl
								// Initialize parent variables in the instance env
								for _, pvb := range parentDecl.VarBlocks {
									for _, pvd := range pvb.Declarations {
										pval := interp.ZeroFromTypeSpec(pvd.Type)
										if pvd.InitValue != nil {
											if iv, err := interpreter.EvalExpr(inst.Env, pvd.InitValue); err == nil {
												pval = iv
											}
										}
										for _, pn := range pvd.Names {
											if _, exists := inst.Env.Get(pn.Name); !exists {
												inst.Env.Define(pn.Name, pval)
											}
										}
									}
								}
							}
						}
						val := interp.Value{Kind: interp.ValFBInstance, FBRef: inst}
						env.Define(n.Name, val)
					}
					continue
				}
			}

			// Check if the type is a user-defined TYPE (struct, enum, etc.)
			if ctx != nil {
				if typeSpec, ok := ctx.typeDecls[upperTypeName]; ok {
					val := interp.ZeroFromTypeSpec(typeSpec)
					if vd.InitValue != nil {
						if iv, err := interpreter.EvalExpr(env, vd.InitValue); err == nil {
							val = iv
						}
					}
					for _, n := range vd.Names {
						env.Define(n.Name, val)
					}
					continue
				}
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

// registerEnumTypes registers enum type declarations from the file context
// with the interpreter so that typed enum literals (e.g., Color#Green) can
// be resolved at runtime.
func registerEnumTypes(interpreter *interp.Interpreter, ctx *fileContext) {
	for typeName, typeSpec := range ctx.typeDecls {
		if enumType, ok := typeSpec.(*ast.EnumType); ok {
			values := make(map[string]int64)
			for i, ev := range enumType.Values {
				if ev.Name == nil {
					continue
				}
				memberName := strings.ToUpper(ev.Name.Name)
				// Use explicit init value if present, otherwise use position index
				if ev.Value != nil {
					if lit, ok := ev.Value.(*ast.Literal); ok && lit.LitKind == ast.LitInt {
						if n, err := parseInt(lit.Value); err == nil {
							values[memberName] = n
							continue
						}
					}
				}
				values[memberName] = int64(i)
			}
			interpreter.RegisterEnumType(typeName, values)
		}
	}
}

// parseInt parses an integer string, used for enum init values.
func parseInt(s string) (int64, error) {
	s = strings.ReplaceAll(s, "_", "")
	n := int64(0)
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int64(ch-'0')
		}
	}
	return n, nil
}

// typeNameFromSpec extracts the type name string from an AST TypeSpec.
func typeNameFromSpec(ts ast.TypeSpec) string {
	if nt, ok := ts.(*ast.NamedType); ok && nt.Name != nil {
		return nt.Name.Name
	}
	return ""
}
