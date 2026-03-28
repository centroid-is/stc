# Phase 5: Testing Framework - Research

**Researched:** 2026-03-28
**Domain:** ST test DSL, test runner, JUnit XML output, source debug mapping
**Confidence:** HIGH

## Summary

Phase 5 builds the "killer feature" of stc: host-based unit testing of Structured Text without PLC hardware. The implementation requires changes across three layers: (1) lexer/parser extensions for `TEST_CASE`/`END_TEST_CASE`, assertion functions, and `ADVANCE_TIME`; (2) a test execution engine that wraps the existing interpreter with assertion tracking, I/O mocking, and deterministic time; (3) a CLI runner (`stc test`) with file discovery, JUnit XML output, JSON output, and source-mapped failure messages.

The existing codebase provides strong foundations. The interpreter (`pkg/interp`) already has scan-cycle execution with deterministic time (`ScanCycleEngine.Tick(dt)`), I/O injection (`SetInput`/`GetOutput`), and a stdlib function dispatch mechanism (`StdlibFunctions` map). The parser follows established patterns for adding new declaration types (see `parseProgram`, `parseFunctionBlock`). Source positions (`ast.Pos` with `File`, `Line`, `Col`) are already tracked on every AST node, which enables DBUG-01/DBUG-02 without additional infrastructure.

**Primary recommendation:** Implement TEST_CASE as a new top-level declaration type (like PROGRAM), treat assertions as stdlib functions registered in `StdlibFunctions`, treat ADVANCE_TIME as a special interpreter-level function, and build the test runner as a new `pkg/testing` package that orchestrates parse -> analyze -> interpret -> report.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- `TEST_CASE 'name' ... END_TEST_CASE` blocks in .st files -- new AST node types added to parser
- TcUnit-style assertions: `ASSERT_TRUE(expr)`, `ASSERT_FALSE(expr)`, `ASSERT_EQ(a, b)`, `ASSERT_NEAR(a, b, eps)`
- `ADVANCE_TIME(T#100ms)` function for explicit deterministic time advancement
- Inline I/O mocking: TEST_CASE declares VAR_INPUT/VAR_OUTPUT, sets inputs before assertions, reads outputs after
- `stc test <dir>` recursively discovers `*_test.st` files (convention over config)
- JUnit XML output with testsuite/testcase elements, stdout capture for assertions
- Failure messages reference original ST file:line:col using debug source maps (DBUG-01, DBUG-02)
- Exit 0 on all pass, exit 1 on any failure -- standard CI convention
- JSON output via `--format json`

### Claude's Discretion
- Internal test execution order (sequential is fine -- PLC tests are deterministic)
- JUnit XML schema details
- Assertion message formatting
- Test timeout implementation

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TEST-01 | Write tests in ST using TEST_CASE / ASSERT_EQ / ASSERT_TRUE / ASSERT_NEAR / ASSERT_FALSE | New AST node `TestCaseDecl`, assertion functions in stdlib dispatch |
| TEST-02 | CLI command `stc test <dir>` discovers and runs test files | File discovery via `filepath.Walk` with `*_test.st` filter (pattern from `discoverSTFiles`) |
| TEST-03 | Test runner outputs JUnit XML for CI integration | `pkg/testing/junit.go` emitter with `testsuites > testsuite > testcase` structure |
| TEST-04 | Test runner supports JSON output (`--format json`) | Structured JSON output alongside JUnit XML, following existing `--format json` pattern |
| TEST-05 | Tests support I/O mocking (inject input values, read output values) | TEST_CASE VarBlocks provide VAR_INPUT/VAR_OUTPUT; body sets inputs and reads outputs directly |
| TEST-06 | Tests support deterministic time advancement (ADVANCE_TIME) | ADVANCE_TIME as special function recognized by interpreter, delegates to `ScanCycleEngine.Tick(dt)` |
| TEST-07 | Test runner returns non-zero exit code on failure | `os.Exit(1)` when any assertion fails; exit 0 when all pass |
| DBUG-01 | Source maps from original ST lines to interpreter execution points | AST nodes already carry `Span` with `File:Line:Col`; RuntimeError already carries `Pos` |
| DBUG-02 | Test failure messages reference original ST file:line, not internal representation | Assertion functions capture caller `ast.Pos` from the `CallExpr` node, include in failure report |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/xml` | stdlib | JUnit XML output | Go standard library, no dependencies needed |
| `encoding/json` | stdlib | JSON test results output | Already used throughout project |
| `filepath` | stdlib | Recursive test file discovery | Already used in `discoverSTFiles` |
| `github.com/spf13/cobra` | 1.10.2 | CLI `stc test` subcommand | Already in go.mod, used by all other commands |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | 1.11.1 | Go-side unit tests for the test runner itself | Already in go.mod, used by all existing Go tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `encoding/xml` | `text/template` for XML | encoding/xml is type-safe and handles escaping; templates are error-prone for XML |
| Custom assertion dispatch | Extending `StdlibFunctions` map | StdlibFunctions is the established pattern; extending it is simpler than a separate dispatch |

**Installation:**
No new dependencies required. All needed packages are already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
pkg/
  ast/
    node.go          # Add KindTestCaseDecl, KindAssertStmt
    test_nodes.go    # NEW: TestCaseDecl node type
  lexer/
    token.go         # Add KwTestCase, KwEndTestCase
    keywords.go      # Add "TEST_CASE", "END_TEST_CASE" mappings
  parser/
    decl.go          # Add parseTestCase() alongside parseProgram()
  interp/
    interpreter.go   # Register ASSERT_* in StdlibFunctions, add ADVANCE_TIME
    assertions.go    # NEW: Assertion function implementations
    errors.go        # Add ErrAssertionFailed type
  testing/           # NEW PACKAGE
    runner.go        # Test discovery, orchestration, result collection
    result.go        # TestResult, TestSuiteResult types
    junit.go         # JUnit XML output
    json.go          # JSON output
cmd/
  stc/
    test_cmd.go      # NEW: replaces stub in stubs.go
    stubs.go         # Remove newTestCmd() stub
```

### Pattern 1: TEST_CASE as Declaration Node
**What:** `TEST_CASE` is parsed as a new top-level declaration type, similar to `ProgramDecl`. It has a name (string literal), optional VarBlocks (for local test variables and I/O declarations), and a body of statements.
**When to use:** Always -- this is the locked decision.
**Example:**
```go
// AST node in pkg/ast/test_nodes.go
type TestCaseDecl struct {
    NodeBase
    Name      string       `json:"name"`       // From string literal: TEST_CASE 'name'
    VarBlocks []*VarBlock  `json:"var_blocks,omitempty"`
    Body      []Statement  `json:"body,omitempty"`
}
func (n *TestCaseDecl) declNode() {}
```

```
// ST syntax
TEST_CASE 'TON timer fires after elapsed time'
  VAR
    timer : TON;
    output : BOOL;
  END_VAR

  timer(IN := TRUE, PT := T#100ms);
  ADVANCE_TIME(T#50ms);
  timer(IN := TRUE, PT := T#100ms);
  ASSERT_FALSE(timer.Q);

  ADVANCE_TIME(T#60ms);
  timer(IN := TRUE, PT := T#100ms);
  ASSERT_TRUE(timer.Q);
END_TEST_CASE
```

### Pattern 2: Assertions as Interpreter Functions
**What:** ASSERT_TRUE, ASSERT_FALSE, ASSERT_EQ, ASSERT_NEAR are registered in the `StdlibFunctions` map. They evaluate their arguments using the existing expression evaluator and signal pass/fail via a test context.
**When to use:** For all assertion functions.
**Why:** This reuses the existing function call dispatch mechanism (`evalCall` in interpreter.go). The interpreter already checks `StdlibFunctions[calleeName]` before falling through to undefined function error.
**Key design decision:** Assertions should NOT abort the test on first failure. They should record the failure and continue executing, so that a single test can report multiple assertion failures. This matches TcUnit/GoogleTest behavior.

```go
// Assertion context passed through the interpreter
type AssertionCollector struct {
    Results []AssertionResult
}

type AssertionResult struct {
    Passed  bool
    Message string
    Pos     ast.Pos  // Source position of the ASSERT_* call
}
```

### Pattern 3: ADVANCE_TIME as Special Interpreter Function
**What:** ADVANCE_TIME needs access to the ScanCycleEngine to call `Tick(dt)`. It cannot be a simple stdlib function because it needs the engine reference.
**When to use:** During test execution.
**How:** The test runner creates a ScanCycleEngine (or a TestEngine that wraps it), and registers ADVANCE_TIME as a closure that captures the engine reference. The closure is added to StdlibFunctions before test execution.

```go
// In test runner setup, before executing each TEST_CASE:
interp.StdlibFunctions["ADVANCE_TIME"] = func(args []Value) (Value, error) {
    if len(args) != 1 || args[0].Kind != ValTime {
        return Value{}, &RuntimeError{Msg: "ADVANCE_TIME requires one TIME argument"}
    }
    engine.Tick(args[0].Time)
    return BoolValue(true), nil
}
```

### Pattern 4: Test Runner Orchestration
**What:** The test runner discovers `*_test.st` files, parses each, extracts `TestCaseDecl` nodes, and executes them sequentially.
**Pipeline:** `discover -> parse -> [optionally analyze] -> execute -> collect results -> report`

```go
// pkg/testing/runner.go
type Runner struct {
    Dir     string
    Format  string // "text", "json", "junit"
}

type TestResult struct {
    Name       string
    File       string
    Passed     bool
    Duration   time.Duration
    Assertions []AssertionResult
    Error      string // Runtime error, if any
}
```

### Pattern 5: Source Debug Mapping (DBUG-01, DBUG-02)
**What:** Every assertion failure includes the original ST file:line:col. This is already mostly built: `ast.CallExpr` nodes carry `Span` with `Start.File`, `Start.Line`, `Start.Col`. The assertion function implementation needs to receive the call-site position.
**How:** Modify `evalCall` to pass the `CallExpr.Span().Start` position to assertion functions, or have assertion functions registered with a wrapper that captures the AST position.

```go
// Enhanced assertion function signature that receives source position:
type AssertFunc func(args []Value, pos ast.Pos) (Value, error)
```

### Anti-Patterns to Avoid
- **Parsing assertions as special statement types:** Don't add KindAssertStmt to AST. Assertions are function calls and should parse as `CallExpr`. The interpreter dispatches them through `StdlibFunctions`. This avoids parser changes for each new assertion type.
- **Aborting test on first failure:** Don't use `ErrReturn` or `RuntimeError` for assertion failures. Collect all failures and report them together. Aborting hides subsequent failures and makes debugging harder.
- **Test-specific lexer tokens for ASSERT_*:** Don't add keywords. ASSERT_TRUE etc. parse as regular identifiers/function calls. Only TEST_CASE and END_TEST_CASE need keyword tokens (because they delimit a declaration block).
- **Coupling test runner to scan cycle engine:** Not every TEST_CASE needs a full ScanCycleEngine. Simple tests that don't use ADVANCE_TIME or FB instances can run with just the interpreter and an Env. Only create ScanCycleEngine when the test uses time-dependent features.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JUnit XML output | Custom XML string builder | `encoding/xml` with struct tags | Handles escaping, CDATA, encoding declaration correctly |
| File discovery | Custom directory walker | `filepath.Walk` with extension filter | Already proven pattern in `discoverSTFiles` |
| XML escaping | Manual `&amp;`, `&lt;` replacement | `encoding/xml.Marshal` | Edge cases in CDATA, attribute encoding |

**Key insight:** The heavy lifting for this phase is already done. The interpreter, scan cycle engine, stdlib FBs, and expression evaluator are all complete. The main work is wiring them together in a test-specific execution context.

## Common Pitfalls

### Pitfall 1: Assertion Source Position Loss
**What goes wrong:** Assertion failures report "runtime error at unknown position" instead of the ST file:line:col where the ASSERT_* call was made.
**Why it happens:** `StdlibFunctions` signature is `func(args []Value) (Value, error)`. It does not receive the call-site AST position. The `evalCall` method resolves the function and passes evaluated argument values but not the source position.
**How to avoid:** Either (a) create a separate assertion dispatch path in `evalCall` that passes `e.Span().Start` to assertion handlers, or (b) use a test-specific wrapper type that stores position alongside the assertion result.
**Warning signs:** Assertion failure messages missing file:line:col.

### Pitfall 2: ADVANCE_TIME Without Re-executing Program Body
**What goes wrong:** `ADVANCE_TIME(T#100ms)` advances the clock but does not re-execute the program body. Timer FBs need to be called again after time advances to update their state.
**Why it happens:** In a real PLC, ADVANCE_TIME would trigger scan cycles. In the test runner, it just moves the clock.
**How to avoid:** Document clearly that ADVANCE_TIME advances the virtual clock but the user must re-call FBs explicitly to process the time change. Alternatively, if the test wraps a PROGRAM, ADVANCE_TIME could trigger N scan cycle ticks. Decision: keep it simple -- ADVANCE_TIME just advances clock, user re-calls FBs.
**Warning signs:** Timer tests that expect Q=TRUE after ADVANCE_TIME without re-calling the timer FB.

### Pitfall 3: I/O Mocking Scope Confusion
**What goes wrong:** VAR_INPUT/VAR_OUTPUT in a TEST_CASE are confused with the program's I/O. Test wants to mock a PROGRAM's inputs but the variables live in different scopes.
**Why it happens:** TEST_CASE has its own VarBlocks. A test for a PROGRAM needs to set the program's inputs, not the test's inputs.
**How to avoid:** For TEST_CASE, all variables declared in VarBlocks are local to the test. To test a PROGRAM, the user instantiates a ScanCycleEngine in the test, calls SetInput/GetOutput. But that is the Go API, not ST API. For the ST-level test: the test body directly assigns to variables and calls FBs. The test declares its own FB instances and wires them.
**Warning signs:** Tests that try to `SetInput` on a PROGRAM they haven't instantiated.

### Pitfall 4: StdlibFunctions Map Mutation
**What goes wrong:** ADVANCE_TIME is registered globally in StdlibFunctions and persists between tests, or between test files that should be isolated.
**Why it happens:** StdlibFunctions is a package-level map. Modifying it in one test affects all subsequent tests.
**How to avoid:** Either (a) create a copy of StdlibFunctions per test execution, (b) register and deregister ADVANCE_TIME around each test, or (c) give the test interpreter its own function lookup table. Option (c) is cleanest: add a `localFunctions` map to the Interpreter struct that takes priority over `StdlibFunctions`.
**Warning signs:** Tests pass individually but fail when run together.

### Pitfall 5: TEST_CASE Keyword Conflicts with Existing Identifiers
**What goes wrong:** Adding `TEST_CASE` and `END_TEST_CASE` as keywords breaks parsing of any ST file that happens to use `TEST_CASE` as an identifier.
**Why it happens:** The lexer keyword map is case-insensitive and global.
**How to avoid:** Only scan `*_test.st` files with the extended keyword set, or make TEST_CASE a contextual keyword that only activates as a declaration start (i.e., only when at top-level position). Simplest approach: add the keywords globally -- `TEST_CASE` is not a valid IEC 61131-3 identifier in practice, and `*_test.st` files are expected to use the test DSL.
**Warning signs:** Parse errors in non-test ST files after adding keywords.

## Code Examples

### Example 1: TestCaseDecl AST Node
```go
// pkg/ast/test_nodes.go
package ast

// TestCaseDecl represents a TEST_CASE 'name' ... END_TEST_CASE block.
type TestCaseDecl struct {
    NodeBase
    Name      string      `json:"name"`
    VarBlocks []*VarBlock `json:"var_blocks,omitempty"`
    Body      []Statement `json:"body,omitempty"`
}

func (n *TestCaseDecl) Children() []Node {
    var nodes []Node
    for _, v := range n.VarBlocks {
        nodes = append(nodes, v)
    }
    for _, s := range n.Body {
        nodes = append(nodes, s)
    }
    return nodes
}
func (n *TestCaseDecl) declNode() {}
```

### Example 2: Parser Extension for TEST_CASE
```go
// In pkg/parser/decl.go, add to parseDeclaration switch:
case lexer.KwTestCase:
    return p.parseTestCase()

// parseTestCase parses TEST_CASE 'name' ... END_TEST_CASE
func (p *Parser) parseTestCase() *ast.TestCaseDecl {
    startTok := p.advance() // consume TEST_CASE

    // Expect string literal for test name
    nameTok := p.expect(lexer.StringLiteral)
    name := strings.Trim(nameTok.Text, "'\"")

    p.match(lexer.Semicolon) // optional

    varBlocks := p.parseVarBlocks()
    body := p.parseStatements(lexer.KwEndTestCase)

    endTok := p.expect(lexer.KwEndTestCase)
    p.match(lexer.Semicolon)

    return &ast.TestCaseDecl{
        NodeBase: ast.NodeBase{
            NodeKind: ast.KindTestCaseDecl,
            NodeSpan: spanFromTokens(startTok, endTok),
        },
        Name:      name,
        VarBlocks: varBlocks,
        Body:      body,
    }
}
```

### Example 3: JUnit XML Output Structure
```go
// pkg/testing/junit.go
package testing

import "encoding/xml"

type JUnitTestSuites struct {
    XMLName xml.Name         `xml:"testsuites"`
    Tests   int              `xml:"tests,attr"`
    Failures int             `xml:"failures,attr"`
    Time    float64          `xml:"time,attr"`
    Suites  []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
    XMLName  xml.Name        `xml:"testsuite"`
    Name     string          `xml:"name,attr"`
    Tests    int             `xml:"tests,attr"`
    Failures int             `xml:"failures,attr"`
    Time     float64         `xml:"time,attr"`
    Cases    []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
    XMLName   xml.Name       `xml:"testcase"`
    Name      string         `xml:"name,attr"`
    Classname string         `xml:"classname,attr"`
    Time      float64        `xml:"time,attr"`
    Failure   *JUnitFailure  `xml:"failure,omitempty"`
}

type JUnitFailure struct {
    Message string `xml:"message,attr"`
    Type    string `xml:"type,attr"`
    Content string `xml:",chardata"`
}
```

### Example 4: Assertion Implementation
```go
// pkg/interp/assertions.go
package interp

import (
    "fmt"
    "math"
    "github.com/centroid-is/stc/pkg/ast"
)

// ErrAssertionFailed signals an assertion failure (not a runtime error).
// It is collected, not propagated -- test execution continues.
type ErrAssertionFailed struct {
    Message string
    Pos     ast.Pos
}

func (e *ErrAssertionFailed) Error() string {
    return fmt.Sprintf("%s:%d:%d: ASSERTION FAILED: %s",
        e.Pos.File, e.Pos.Line, e.Pos.Col, e.Message)
}

// assertEq checks that two values are equal.
func assertEq(args []Value) (bool, string) {
    if len(args) < 2 {
        return false, "ASSERT_EQ requires 2 arguments"
    }
    if valuesEqual(args[0], args[1]) {
        return true, ""
    }
    return false, fmt.Sprintf("expected %s, got %s", args[1].String(), args[0].String())
}

// assertNear checks float equality within epsilon.
func assertNear(args []Value) (bool, string) {
    if len(args) < 3 {
        return false, "ASSERT_NEAR requires 3 arguments (actual, expected, epsilon)"
    }
    actual := toFloat(args[0])
    expected := toFloat(args[1])
    eps := toFloat(args[2])
    if math.Abs(actual-expected) <= eps {
        return true, ""
    }
    return false, fmt.Sprintf("expected %g +/- %g, got %g", expected, eps, actual)
}
```

### Example 5: Full ST Test File
```
(* motor_test.st - tests for motor control logic *)

TEST_CASE 'Motor starts when start button pressed'
  VAR
    startBtn : BOOL;
    motor : BOOL;
  END_VAR

  startBtn := TRUE;
  motor := startBtn;
  ASSERT_TRUE(motor);
END_TEST_CASE

TEST_CASE 'TON timer reaches preset time'
  VAR
    timer : TON;
  END_VAR

  timer(IN := TRUE, PT := T#500ms);
  ASSERT_FALSE(timer.Q);

  ADVANCE_TIME(T#250ms);
  timer(IN := TRUE, PT := T#500ms);
  ASSERT_FALSE(timer.Q);

  ADVANCE_TIME(T#300ms);
  timer(IN := TRUE, PT := T#500ms);
  ASSERT_TRUE(timer.Q);
END_TEST_CASE

TEST_CASE 'ASSERT_NEAR checks float tolerance'
  VAR
    result : REAL;
  END_VAR

  result := 3.14159;
  ASSERT_NEAR(result, 3.14, 0.01);
END_TEST_CASE
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| TcUnit (on PLC) | Host-based test runners (STruC++, this project) | 2024-2025 | No hardware needed for testing |
| Custom assert per type (TcUnit `ASSERT_EQ_INT`, `ASSERT_EQ_REAL`) | Generic ASSERT_EQ with runtime type dispatch | STruC++ design | Fewer assertions to learn, simpler API |
| Separate test config files | Convention-based `*_test.st` discovery | Go convention | Zero configuration for test discovery |

**Deprecated/outdated:**
- TcUnit's per-type assertion model (`ASSERT_EQ_INT`, `ASSERT_EQ_BOOL`) is verbose. Use generic `ASSERT_EQ` with runtime value comparison.

## Open Questions

1. **Should TEST_CASE support testing user-defined PROGRAMS?**
   - What we know: TEST_CASE declares its own variables and FB instances. Testing a PROGRAM requires instantiating and running it.
   - What's unclear: Should there be a `RUN_PROGRAM('ProgramName')` function, or should the user manually create FB instances?
   - Recommendation: For v1, TEST_CASE tests are self-contained. They declare their own FBs and logic. Testing full PROGRAMs via `stc test` is a Phase 6 (simulation) concern. Keep it simple.

2. **Should assertions accept an optional message parameter?**
   - What we know: TcUnit's assertions have optional message parameters. GoogleTest does too.
   - What's unclear: Should the API be `ASSERT_EQ(a, b)` or `ASSERT_EQ(a, b, 'custom message')`?
   - Recommendation: Support optional trailing string argument as custom message. Auto-generate a message if not provided. The variadic check is straightforward since we control the dispatch.

3. **Test timeout handling**
   - What we know: Infinite loops in ST code would hang the test runner.
   - What's unclear: How long should the default timeout be?
   - Recommendation: Use the existing `MaxLoopIterations` (1,000,000) on the interpreter. Add a `--timeout` CLI flag (default 30s wall-clock) as a safety net. Kill test execution if wall clock exceeds timeout.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify 1.11.1 |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./pkg/testing/... ./pkg/interp/... ./cmd/stc/... -run TestST -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | Parse and execute TEST_CASE with assertions | integration | `go test ./pkg/testing/... -run TestAssertions -count=1 -v` | Wave 0 |
| TEST-02 | `stc test <dir>` discovers *_test.st files | integration | `go test ./cmd/stc/... -run TestDiscovery -count=1 -v` | Wave 0 |
| TEST-03 | JUnit XML output | unit | `go test ./pkg/testing/... -run TestJUnitXML -count=1 -v` | Wave 0 |
| TEST-04 | JSON output | unit | `go test ./pkg/testing/... -run TestJSONOutput -count=1 -v` | Wave 0 |
| TEST-05 | I/O mocking via VAR_INPUT/VAR_OUTPUT | integration | `go test ./pkg/testing/... -run TestIOMocking -count=1 -v` | Wave 0 |
| TEST-06 | ADVANCE_TIME deterministic time | integration | `go test ./pkg/testing/... -run TestAdvanceTime -count=1 -v` | Wave 0 |
| TEST-07 | Non-zero exit on failure | integration | `go test ./cmd/stc/... -run TestExitCode -count=1 -v` | Wave 0 |
| DBUG-01 | Source position tracking through interpreter | unit | `go test ./pkg/interp/... -run TestSourcePos -count=1 -v` | Wave 0 |
| DBUG-02 | Failure messages include file:line:col | integration | `go test ./pkg/testing/... -run TestFailurePosition -count=1 -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/testing/... ./pkg/interp/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `pkg/testing/runner_test.go` -- covers TEST-01, TEST-02, TEST-05, TEST-06
- [ ] `pkg/testing/junit_test.go` -- covers TEST-03
- [ ] `pkg/testing/json_test.go` -- covers TEST-04
- [ ] `cmd/stc/test_cmd_test.go` -- covers TEST-07, integration tests
- [ ] `pkg/interp/assertions_test.go` -- covers DBUG-01, DBUG-02
- [ ] Test fixture files: `testdata/passing_test.st`, `testdata/failing_test.st`, `testdata/timer_test.st`

## Sources

### Primary (HIGH confidence)
- Project source code: `pkg/interp/`, `pkg/parser/`, `pkg/ast/`, `pkg/lexer/`, `cmd/stc/` -- direct code inspection
- Existing test patterns: `pkg/interp/scan_test.go`, `pkg/interp/stdlib_timers_test.go` -- established Go test conventions
- [TcUnit assertion API](https://deepwiki.com/tcunit/TcUnit/3-user-guide) -- reference for assertion naming and behavior
- [JUnit XML format spec](https://github.com/testmoapp/junitxml) -- canonical JUnit XML schema reference
- [JUnit XSD schema](https://github.com/windyroad/JUnit-Schema/blob/master/JUnit.xsd) -- formal schema definition

### Secondary (MEDIUM confidence)
- STruC++ test runner design (from FEATURES.md research) -- validates architecture approach

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies needed, all patterns established in codebase
- Architecture: HIGH - direct extension of existing parser/interpreter/CLI patterns
- Pitfalls: HIGH - identified from code analysis of actual interpreter dispatch mechanism

**Research date:** 2026-03-28
**Valid until:** 2026-04-28 (stable domain, no external dependency changes expected)
