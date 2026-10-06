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
	"github.com/centroid-is/stc/pkg/scenario"
)

// RunOpts configures mock and library files for the test runner.
type RunOpts struct {
	LibraryFiles []*ast.SourceFile
	MockFiles    []*ast.SourceFile
	// ProjectFiles are the user sources of an imported project (stc test
	// --project). Their FUNCTION_BLOCKs (with bodies, methods and
	// properties), FUNCTIONs, TYPEs, INTERFACEs and GVLs are merged into
	// every test file through the same path as the test file's own
	// declarations, before them, so a test file declaration with the same
	// name wins. Project FBs are real implementations, never auto-stubs.
	ProjectFiles []*ast.SourceFile
	Defines      map[string]bool // Preprocessor defines (e.g., STC_TEST)
	// Plant, when set, switches to project mode (stc test --project with
	// .st sources or --io): every TEST_CASE runs on a fresh Plant built
	// from the spec, with the scenario built-ins registered. Library, mock
	// and project files are then ignored; the spec carries the sources.
	Plant *scenario.PlantSpec
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
		var suiteResult *SuiteResult
		var stubs map[string]bool
		if opts.Plant != nil {
			suiteResult, err = runProjectFile(file, dir, opts.Plant, opts.Defines)
		} else {
			suiteResult, stubs, err = runFileWithOpts(file, dir, extCtx, opts.Defines)
		}
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
	// projectFiles are merged like test-file declarations (RunOpts.ProjectFiles)
	projectFiles []*ast.SourceFile
	// typeDecls maps uppercase TYPE names to their specs, so that structs
	// and enums declared next to mock/library FBs resolve in tests too
	typeDecls map[string]ast.TypeSpec
	// typeAttrs maps uppercase TYPE names to their declaration attributes
	typeAttrs map[string][]*ast.Attribute
	// typeInits maps uppercase TYPE names to the TYPE's own default
	typeInits map[string]ast.Expr
	// funcDecls maps uppercase FUNCTION names to their declarations
	funcDecls map[string]*ast.FunctionDecl
}

// buildExternalContext extracts FB declarations from library and mock files.
func buildExternalContext(opts RunOpts) *externalContext {
	ext := &externalContext{
		libraryFBs: make(map[string]*ast.FunctionBlockDecl),
		mockFBs:    make(map[string]*ast.FunctionBlockDecl),
		typeDecls:  make(map[string]ast.TypeSpec),
		typeAttrs:  make(map[string][]*ast.Attribute),
		typeInits:  make(map[string]ast.Expr),
		funcDecls:  make(map[string]*ast.FunctionDecl),

		projectFiles: opts.ProjectFiles,
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
			switch d := decl.(type) {
			case *ast.FunctionBlockDecl:
				if d.Name != nil {
					ext.mockFBs[strings.ToUpper(d.Name.Name)] = d
				}
			case *ast.TypeDecl:
				if d.Name != nil {
					ext.typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
					ext.typeAttrs[strings.ToUpper(d.Name.Name)] = d.Attributes
					if d.InitValue != nil {
						ext.typeInits[strings.ToUpper(d.Name.Name)] = d.InitValue
					}
				}
			case *ast.FunctionDecl:
				if d.Name != nil {
					ext.funcDecls[strings.ToUpper(d.Name.Name)] = d
				}
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
	// typeAttrs maps upper-case type names to the attributes on their TYPE
	// declaration ({attribute 'to_string'} and friends).
	typeAttrs map[string][]*ast.Attribute
	// typeInits maps upper-case type names to the TYPE's own default
	// (TYPE T : INT := 5; END_TYPE).
	typeInits map[string]ast.Expr
	// fbDecls maps upper-case FB names to their FunctionBlockDecl.
	fbDecls map[string]*ast.FunctionBlockDecl
	// funcDecls maps upper-case function names to their FunctionDecl.
	funcDecls map[string]*ast.FunctionDecl
	// ifaceDecls maps upper-case interface names to their InterfaceDecl.
	ifaceDecls map[string]*ast.InterfaceDecl
	// gvlDecls lists the file's GVLs in source order. They are registered
	// afresh for every TEST_CASE so GVL state never leaks between cases.
	gvlDecls []*ast.GVLDecl
}

// collect records TYPE, FUNCTION_BLOCK, FUNCTION, INTERFACE and GVL
// declarations, replacing same-named earlier entries, and returns the
// TEST_CASEs in source order.
func (ctx *fileContext) collect(decls []ast.Declaration) []*ast.TestCaseDecl {
	var testCases []*ast.TestCaseDecl
	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.TestCaseDecl:
			testCases = append(testCases, d)
		case *ast.TypeDecl:
			if d.Name != nil {
				ctx.typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
				ctx.typeAttrs[strings.ToUpper(d.Name.Name)] = d.Attributes
				if d.InitValue != nil {
					ctx.typeInits[strings.ToUpper(d.Name.Name)] = d.InitValue
				}
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
		case *ast.GVLDecl:
			ctx.gvlDecls = append(ctx.gvlDecls, d)
		}
	}
	return testCases
}

// withoutGVLs returns the GVLs in base whose names do not appear in override.
func withoutGVLs(base, override []*ast.GVLDecl) []*ast.GVLDecl {
	names := map[string]bool{}
	for _, g := range override {
		if g.Name != nil {
			names[strings.ToUpper(g.Name.Name)] = true
		}
	}
	var out []*ast.GVLDecl
	for _, g := range base {
		if g.Name == nil || !names[strings.ToUpper(g.Name.Name)] {
			out = append(out, g)
		}
	}
	return out
}

// runFile parses a single .st file and executes all TEST_CASE blocks.
// Kept for backward compatibility -- delegates to runFileWithOpts with no external context.
func runFile(filePath, baseDir string) (*SuiteResult, error) {
	suite, _, err := runFileWithOpts(filePath, baseDir, nil, nil)
	return suite, err
}

// runFileWithOpts parses a single .st file and executes all TEST_CASE blocks
// with optional external FB context from library/mock files.
// Returns the suite result and a set of auto-stubbed FB type names.
func runFileWithOpts(filePath, baseDir string, extCtx *externalContext, defines map[string]bool) (*SuiteResult, map[string]bool, error) {
	start := time.Now()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	parseResult := pipeline.Parse(filePath, string(content), defines)

	// Build file context: collect TYPE, FUNCTION_BLOCK, and FUNCTION declarations
	ctx := &fileContext{
		typeDecls:  make(map[string]ast.TypeSpec),
		typeAttrs:  make(map[string][]*ast.Attribute),
		typeInits:  make(map[string]ast.Expr),
		fbDecls:    make(map[string]*ast.FunctionBlockDecl),
		funcDecls:  make(map[string]*ast.FunctionDecl),
		ifaceDecls: make(map[string]*ast.InterfaceDecl),
	}

	// Project declarations first, then the test file's own, so the test
	// file overrides by name (GVLs: a test-file GVL replaces a project GVL
	// of the same name).
	if extCtx != nil {
		for _, f := range extCtx.projectFiles {
			ctx.collect(f.Declarations)
		}
	}
	projectGVLs := ctx.gvlDecls
	ctx.gvlDecls = nil
	testCases := ctx.collect(parseResult.File.Declarations)
	ctx.gvlDecls = append(withoutGVLs(projectGVLs, ctx.gvlDecls), ctx.gvlDecls...)

	// Merge external context: mock FBs override library stubs, which fill gaps
	autoStubbed := make(map[string]bool)
	if extCtx != nil {
		// TYPE declarations from mock/library files; the test file wins on conflict
		for name, spec := range extCtx.typeDecls {
			if _, exists := ctx.typeDecls[name]; !exists {
				ctx.typeDecls[name] = spec
				ctx.typeAttrs[name] = extCtx.typeAttrs[name]
				if init, ok := extCtx.typeInits[name]; ok {
					ctx.typeInits[name] = init
				}
			}
		}
		// FUNCTION declarations likewise
		for name, decl := range extCtx.funcDecls {
			if _, exists := ctx.funcDecls[name]; !exists {
				ctx.funcDecls[name] = decl
			}
		}
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
		registerTypeDecls(interpreter, ctx)
	}

	// Register SET_IO and GET_IO for I/O table injection/reading
	ioTable := iomap.NewIOTable()
	registerIOFunctions(interpreter, ioTable)

	// Register the file's GVLs on this test case's interpreter. Types and
	// FB declarations are registered above, so GVL members of user types and
	// FB types are built correctly.
	if ctx != nil {
		interpreter.RegisterGVLs(ctx.gvlDecls)
	}

	// Create isolated environment; non qualified_only GVLs are its ancestors
	// so their variables resolve as bare names.
	env := interp.NewEnv(interpreter.GlobalParent())

	// Initialize variables from VarBlocks; inline VAR enums first so their
	// values resolve in initialisers and the body.
	interpreter.RegisterInlineEnums(tc.Name, tc.VarBlocks)
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

	tr.Assertions = assertionsJSON(collector)

	return tr
}

// registerUserFunctions registers the file's FUNCTION declarations with the
// interpreter, which binds named, positional and mixed arguments and applies
// declared defaults (see interp.CallFunction).
func registerUserFunctions(interpreter *interp.Interpreter, ctx *fileContext) {
	for _, decl := range ctx.funcDecls {
		interpreter.RegisterFunctionDecl(decl)
	}
}

// initializeTestEnv populates the environment from VarBlocks through the
// interpreter's shared instantiation path, exactly like program variables:
// stdlib and user FBs (with their EXTENDS chain) become live instances, other
// variables get their TYPE default and initialiser. Subrange variables also
// register their bounds.
func initializeTestEnv(interpreter *interp.Interpreter, env *interp.Env, varBlocks []*ast.VarBlock) {
	for _, vb := range varBlocks {
		for _, vd := range vb.Declarations {
			interpreter.InstantiateVar(env, vd)
			if srt, ok := vd.Type.(*ast.SubrangeType); ok {
				low := evalConstInt(srt.Low)
				high := evalConstInt(srt.High)
				for _, n := range vd.Names {
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

// registerTypeDecls hands the file's TYPE declarations to the interpreter so
// that zero-value construction can resolve user-defined named types wherever
// they appear -- including as an array element type or a struct member type,
// which the top-level lookup in initializeTestEnv does not cover.
func registerTypeDecls(interpreter *interp.Interpreter, ctx *fileContext) {
	if len(ctx.typeDecls) > 0 {
		decls := make(map[string]ast.TypeSpec, len(ctx.typeDecls))
		for name, ts := range ctx.typeDecls {
			decls[name] = ts
		}
		interpreter.TypeDecls = decls
	}
	if len(ctx.typeInits) > 0 {
		inits := make(map[string]ast.Expr, len(ctx.typeInits))
		for name, init := range ctx.typeInits {
			inits[name] = init
		}
		interpreter.TypeInits = inits
	}
	if len(ctx.fbDecls) > 0 {
		fbs := make(map[string]*ast.FunctionBlockDecl, len(ctx.fbDecls))
		for name, d := range ctx.fbDecls {
			fbs[name] = d
		}
		interpreter.FBDecls = fbs
	}
}

// registerEnumTypes registers enum type declarations from the file context
// with the interpreter, numbered with the shared IEC previous+1 rule and
// typed by their base type, so qualified (E.v), bare and typed (E#v) enum
// values resolve at runtime and TO_STRING honours {attribute 'to_string'}.
func registerEnumTypes(interpreter *interp.Interpreter, ctx *fileContext) {
	for typeName, typeSpec := range ctx.typeDecls {
		if enumType, ok := typeSpec.(*ast.EnumType); ok {
			interpreter.RegisterEnumDecl(typeName, enumType, ctx.typeAttrs[typeName])
		}
	}
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
