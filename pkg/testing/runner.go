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
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// RunOpts configures mock and library files for the test runner.
type RunOpts struct {
	LibraryFiles []*ast.SourceFile
	MockFiles    []*ast.SourceFile
}

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
// This is a backward-compatible wrapper around RunWithOpts.
func Run(dir string) (*RunResult, error) {
	return RunWithOpts(dir, RunOpts{})
}

// RunWithOpts discovers and executes all *_test.st files with mock/library support.
// LibraryFiles provide declaration-only FBs (stubs). MockFiles provide FB implementations
// that override library stubs. FBs that are only in library stubs (no mock, no body)
// produce zero-value outputs and generate fidelity warnings.
func RunWithOpts(dir string, opts RunOpts) (*RunResult, error) {
	start := time.Now()

	files, err := DiscoverTestFiles(dir)
	if err != nil {
		return nil, err
	}

	// Build external FB context from library and mock files
	extCtx := buildExternalContext(opts)

	result := &RunResult{}

	// Track auto-stubbed FB types across all files
	autoStubbed := make(map[string]bool)

	for _, file := range files {
		suiteResult, stubs, err := runFileWithOpts(file, dir, extCtx)
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
		for name := range stubs {
			autoStubbed[name] = true
		}
	}

	// Generate fidelity warnings for auto-stubbed FBs
	for name := range autoStubbed {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("auto-stub: FB type '%s' has no mock implementation; outputs are zero-valued", name))
	}
	sort.Strings(result.Warnings)

	result.Duration = time.Since(start)
	return result, nil
}

// externalContext holds FB declarations from library stubs and mock files.
type externalContext struct {
	// libraryFBs maps uppercase FB names to body-less declarations (stubs)
	libraryFBs map[string]*ast.FunctionBlockDecl
	// mockFBs maps uppercase FB names to declarations with bodies (mocks)
	mockFBs map[string]*ast.FunctionBlockDecl
}

// buildExternalContext extracts FB declarations from library and mock files.
func buildExternalContext(opts RunOpts) *externalContext {
	ext := &externalContext{
		libraryFBs: make(map[string]*ast.FunctionBlockDecl),
		mockFBs:    make(map[string]*ast.FunctionBlockDecl),
	}

	for _, f := range opts.LibraryFiles {
		for _, decl := range f.Declarations {
			if fb, ok := decl.(*ast.FunctionBlockDecl); ok && fb.Name != nil {
				ext.libraryFBs[strings.ToUpper(fb.Name.Name)] = fb
			}
		}
	}

	for _, f := range opts.MockFiles {
		for _, decl := range f.Declarations {
			if fb, ok := decl.(*ast.FunctionBlockDecl); ok && fb.Name != nil {
				ext.mockFBs[strings.ToUpper(fb.Name.Name)] = fb
			}
		}
	}

	return ext
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
	// ifaceDecls maps upper-case interface names to their InterfaceDecl.
	ifaceDecls map[string]*ast.InterfaceDecl
}

// runFile parses a single .st file and executes all TEST_CASE blocks.
// Kept for backward compatibility -- delegates to runFileWithOpts with no external context.
func runFile(filePath, baseDir string) (*SuiteResult, error) {
	suite, _, err := runFileWithOpts(filePath, baseDir, nil)
	return suite, err
}

// runFileWithOpts parses a single .st file and executes all TEST_CASE blocks
// with optional external FB context from library/mock files.
// Returns the suite result and a set of auto-stubbed FB type names.
func runFileWithOpts(filePath, baseDir string, extCtx *externalContext) (*SuiteResult, map[string]bool, error) {
	start := time.Now()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	parseResult := pipeline.Parse(filePath, string(content), nil)

	// Build file context: collect TYPE, FUNCTION_BLOCK, and FUNCTION declarations
	ctx := &fileContext{
		typeDecls:  make(map[string]ast.TypeSpec),
		fbDecls:    make(map[string]*ast.FunctionBlockDecl),
		funcDecls:  make(map[string]*ast.FunctionDecl),
		ifaceDecls: make(map[string]*ast.InterfaceDecl),
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
		case *ast.InterfaceDecl:
			if d.Name != nil {
				ctx.ifaceDecls[strings.ToUpper(d.Name.Name)] = d
			}
		}
	}

	// Merge external context: mock FBs override library stubs, which fill gaps
	autoStubbed := make(map[string]bool)
	if extCtx != nil {
		// First add library stubs for FB types not already declared in test file
		for name, fbDecl := range extCtx.libraryFBs {
			if _, exists := ctx.fbDecls[name]; !exists {
				// Library stub (no body) -- track as auto-stub candidate
				ctx.fbDecls[name] = fbDecl
				// If no mock overrides this, it remains auto-stubbed
				if _, hasMock := extCtx.mockFBs[name]; !hasMock {
					autoStubbed[fbDecl.Name.Name] = true
				}
			}
		}
		// Then add/override with mock FBs (these have bodies)
		for name, fbDecl := range extCtx.mockFBs {
			ctx.fbDecls[name] = fbDecl
			delete(autoStubbed, fbDecl.Name.Name) // mock replaces auto-stub
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
	return suite, autoStubbed, nil
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

	// Register SET_IO and GET_IO for I/O table injection/reading
	ioTable := iomap.NewIOTable()
	registerIOFunctions(interpreter, ioTable)

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
				// Register subrange constraints if applicable
				if srt, ok := vd.Type.(*ast.SubrangeType); ok {
					low := evalConstInt(srt.Low)
					high := evalConstInt(srt.High)
					env.DefineSubrange(n.Name, int64(low), int64(high))
				}
			}
		}
	}
}

// evalConstInt extracts an integer from a constant expression AST node.
func evalConstInt(expr ast.Expr) int {
	if lit, ok := expr.(*ast.Literal); ok {
		if lit.LitKind == ast.LitInt {
			n := 0
			for _, ch := range lit.Value {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				}
			}
			return n
		}
	}
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		if unary.Op.Text == "-" {
			return -evalConstInt(unary.Operand)
		}
	}
	return 0
}

// validateImplements checks that an FB declares all methods required by
// its IMPLEMENTS interfaces. Returns a list of error messages, empty if valid.
func validateImplements(fbDecl *ast.FunctionBlockDecl, ctx *fileContext) []string {
	if ctx == nil || len(fbDecl.Implements) == 0 {
		return nil
	}
	var errors []string
	fbMethods := make(map[string]bool)
	for _, m := range fbDecl.Methods {
		if m.Name != nil {
			fbMethods[strings.ToUpper(m.Name.Name)] = true
		}
	}

	for _, iface := range fbDecl.Implements {
		ifaceName := strings.ToUpper(iface.Name)
		ifaceDecl, ok := ctx.ifaceDecls[ifaceName]
		if !ok {
			continue // Interface not found; skip (could be externally defined)
		}
		for _, m := range ifaceDecl.Methods {
			if m.Name != nil {
				methodName := strings.ToUpper(m.Name.Name)
				if !fbMethods[methodName] {
					errors = append(errors, fmt.Sprintf("FB '%s' missing method '%s' required by interface '%s'",
						fbDecl.Name.Name, m.Name.Name, iface.Name))
				}
			}
		}
	}
	return errors
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

// parseIOArea converts a string area identifier to an iomap.Area.
func parseIOArea(s string) (iomap.Area, error) {
	switch strings.ToUpper(s) {
	case "I":
		return iomap.AreaInput, nil
	case "Q":
		return iomap.AreaOutput, nil
	case "M":
		return iomap.AreaMemory, nil
	default:
		return 0, fmt.Errorf("invalid I/O area %q, expected I, Q, or M", s)
	}
}

// registerIOFunctions adds SET_IO and GET_IO to the interpreter for test I/O injection.
func registerIOFunctions(interpreter *interp.Interpreter, ioTable *iomap.IOTable) {
	interpreter.RegisterFunction("SET_IO", func(args []interp.Value, pos ast.Pos) (interp.Value, error) {
		// SET_IO(area_str, byte_offset, bit_offset, value)
		if len(args) < 4 {
			return interp.Value{}, fmt.Errorf("SET_IO requires 4 arguments (area, byte_offset, bit_offset, value)")
		}
		areaStr := args[0].Str
		byteOff := int(args[1].Int)
		bitOff := int(args[2].Int)
		value := args[3].Bool

		area, err := parseIOArea(areaStr)
		if err != nil {
			return interp.Value{}, err
		}

		ioTable.SetBit(area, byteOff, bitOff, value)
		return interp.BoolValue(true), nil
	})

	interpreter.RegisterFunction("GET_IO", func(args []interp.Value, pos ast.Pos) (interp.Value, error) {
		// GET_IO(area_str, byte_offset, bit_offset) -> BOOL
		if len(args) < 3 {
			return interp.Value{}, fmt.Errorf("GET_IO requires 3 arguments (area, byte_offset, bit_offset)")
		}
		areaStr := args[0].Str
		byteOff := int(args[1].Int)
		bitOff := int(args[2].Int)

		area, err := parseIOArea(areaStr)
		if err != nil {
			return interp.Value{}, err
		}

		val := ioTable.GetBit(area, byteOff, bitOff)
		return interp.BoolValue(val), nil
	})
}

// typeNameFromSpec extracts the type name string from an AST TypeSpec.
func typeNameFromSpec(ts ast.TypeSpec) string {
	if nt, ok := ts.(*ast.NamedType); ok && nt.Name != nil {
		return nt.Name.Name
	}
	return ""
}
