# STC — Structured Text Compiler Toolchain
## Agent-Executable Implementation Plan

**Language**: Go
**Goal**: Make ST development feel like Rust/Go development — LSP, tests, CI, LLM agents
**Vendors**: Beckhoff (TwinCAT 3), Schneider (CODESYS-derived), later Allen Bradley
**License**: Pick MIT or Apache-2.0 (not GPL — avoids STruC++ fork issues)

---

## Why Go

- Single static binary — no runtime dependencies, trivial to distribute
- Fast compilation — agents iterate quickly
- `go test` built in — no test framework to configure
- `go.lsp.dev/protocol` — full LSP spec in Go, gopls is the reference implementation
- `participle` — struct-tag-based parser library, excellent for DSLs
- ANTLR4 has a Go target if needed later
- LLVM: `tinygo.org/x/go-llvm` provides CGo bindings to LLVM C API (LLVM 14-20). Not needed for MVP but available when you want native codegen
- Cross-compiles trivially (Linux/Windows/macOS from one machine)
- LLM agents write Go well — extensive training data

---

## Repository Structure

```
stc/
├── cmd/
│   ├── stc/           # main compiler CLI
│   ├── stc-lsp/       # LSP server binary
│   └── stc-mcp/       # MCP server for LLM agents
├── pkg/
│   ├── lexer/         # tokenizer
│   ├── parser/        # recursive descent parser → AST
│   ├── ast/           # AST node types
│   ├── preproc/       # {IF defined()} preprocessor
│   ├── sema/          # type checker, symbol tables
│   ├── ir/            # typed intermediate representation
│   ├── interp/        # reference interpreter (scan-cycle runtime)
│   ├── emitter/       # vendor ST emitters
│   │   ├── beckhoff/
│   │   ├── schneider/
│   │   └── portable/
│   ├── stdlib/        # TON, TOF, TP, CTU, R_TRIG, etc.
│   ├── testdsl/       # TEST_CASE / ASSERT parsing and execution
│   └── lsp/           # LSP handler logic
├── vscode/            # VS Code extension (TypeScript, thin wrapper)
│   ├── syntaxes/      # TextMate grammar (.tmLanguage.json)
│   ├── src/           # extension.ts — launches stc-lsp
│   └── package.json
├── testdata/          # golden test corpus
│   ├── parse/         # .st files + expected .ast.json
│   ├── sema/          # .st files + expected diagnostics
│   ├── interp/        # .st files + expected trace.json
│   ├── emit/          # .st files + expected .bk.st / .se.st
│   └── e2e/           # full pipeline tests
├── stdlib/            # IEC standard library in ST
├── examples/
│   ├── motor_control/ # portable motor FB + vendor adapters
│   ├── fill_station/  # simulation example
│   └── common_lib/    # shared library pattern demo
├── docs/
│   ├── ARCHITECTURE.md
│   ├── PORTABLE_SUBSET.md
│   ├── VENDOR_MATRIX.md
│   └── adr/           # architecture decision records
├── go.mod
├── go.sum
├── Makefile
└── .github/workflows/
    └── ci.yml
```

---

## Milestones — Each is independently shippable and testable

Every milestone has:
1. **End-to-end test** that proves it works
2. **Agent verification commands** — copy-paste into terminal
3. **Parallel work** markers — what can run simultaneously

---

### M0: Repository Bootstrap (Day 1)

**Deliverables:**
- Go module initialized
- CI pipeline (GitHub Actions: `go test ./...`, `go vet`, `golangci-lint`)
- Makefile with `build`, `test`, `lint`, `install` targets
- docs/ARCHITECTURE.md skeleton
- Empty but compiling packages for all `pkg/` directories

**E2E test:**
```go
// stc_test.go (root)
func TestSmoke(t *testing.T) {
    // Build stc binary
    cmd := exec.Command("go", "build", "./cmd/stc")
    require.NoError(t, cmd.Run())

    // Run --version
    out, err := exec.Command("./stc", "--version").Output()
    require.NoError(t, err)
    require.Contains(t, string(out), "stc")
}
```

**Agent verification:**
```bash
go build ./...
go test ./...
go vet ./...
test -f docs/ARCHITECTURE.md
```

---

### M1: Lexer (Days 2-3)

**Deliverables:**
- `pkg/lexer/` — tokenizes ST source into token stream
- Token types: keywords (IF, THEN, END_IF, FUNCTION_BLOCK, VAR, etc.), operators (:=, +, -, *, /, =, <>, <, >, <=, >=, AND, OR, NOT, MOD), literals (INT, REAL, STRING, BOOL, TIME, typed literals like INT#5), identifiers, comments (// and (* *)), preprocessor directives ({IF, {ELSE, {ENDIF})
- Position tracking (file, line, column) on every token
- JSON output mode for debugging

**E2E tests:**
```go
func TestLexer_MotorControl(t *testing.T) {
    src := `FUNCTION_BLOCK FB_Motor
VAR_INPUT
    Start : BOOL;
    Stop  : BOOL;
END_VAR
VAR_OUTPUT
    Running : BOOL;
END_VAR
IF Start AND NOT Stop THEN
    Running := TRUE;
END_IF;
END_FUNCTION_BLOCK`

    tokens, err := lexer.Tokenize("motor.st", src)
    require.NoError(t, err)
    require.Equal(t, token.FUNCTION_BLOCK, tokens[0].Type)
    require.Equal(t, "FB_Motor", tokens[1].Value)
    // No ILLEGAL tokens
    for _, tok := range tokens {
        require.NotEqual(t, token.ILLEGAL, tok.Type,
            "illegal token at %s:%d:%d: %q", tok.Pos.File, tok.Pos.Line, tok.Pos.Col, tok.Value)
    }
}

func TestLexer_PreprocessorDirectives(t *testing.T) {
    src := `{IF defined(VENDOR_BECKHOFF)}
    {attribute 'qualified_only'}
{END_IF}`
    tokens, err := lexer.Tokenize("test.st", src)
    require.NoError(t, err)
    require.Equal(t, token.PP_IF, tokens[0].Type)
}

func TestLexer_TypedLiterals(t *testing.T) {
    src := `x := INT#16#FF; y := REAL#3.14; z := T#5s;`
    tokens, err := lexer.Tokenize("test.st", src)
    require.NoError(t, err)
    // verify typed literal tokens parse correctly
}
```

**Golden tests:** `testdata/lexer/*.st` → `testdata/lexer/*.tokens.json`

**Agent verification:**
```bash
go test ./pkg/lexer/... -v -count=1
go test ./pkg/lexer/... -run TestGolden
```

**Parallel:** VS Code extension TextMate grammar (M1-vscode) can start simultaneously.

---

### M1-vscode: VS Code Extension — Syntax Highlighting (Days 2-3, parallel with M1)

**Deliverables:**
- `vscode/` directory with TypeScript extension
- TextMate grammar (`.tmLanguage.json`) for ST — fork from Serhioromano/vscode-st (MIT licensed)
- Add highlighting for `{IF defined(...)}` preprocessor directives
- Language configuration (brackets, comments, auto-closing)
- Package and test locally with `vsce package`

**E2E test:**
```bash
cd vscode && npm install && npm run compile && vsce package
# Produces stc-x.x.x.vsix
test -f *.vsix
```

**No Go dependency** — this is pure TypeScript. An agent can work on this completely independently.

---

### M2: Parser (Days 4-8)

**Deliverables:**
- `pkg/parser/` — recursive descent parser producing AST
- `pkg/ast/` — AST node types with JSON serialization
- Supported constructs:
  - PROGRAM, FUNCTION_BLOCK, FUNCTION
  - VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_TEMP, VAR_GLOBAL
  - Types: BOOL, BYTE, WORD, DWORD, SINT, INT, DINT, LINT, USINT, UINT, UDINT, ULINT, REAL, LREAL, STRING, WSTRING, TIME, DATE, DT, TOD
  - Arrays, structs, enums, subranges
  - IF/ELSIF/ELSE/END_IF, CASE/END_CASE, FOR/END_FOR, WHILE/END_WHILE, REPEAT/END_REPEAT
  - Assignments (:=), function calls, FB instantiation calls
  - METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS (CODESYS OOP)
  - Expressions with full operator precedence
  - {attribute '...'} pragmas (Beckhoff)
- **Error recovery**: on parse error, skip to next statement/declaration boundary, produce partial AST with error nodes. This is critical for LSP.
- `stc parse <file> --format json` CLI command

**E2E tests:**
```go
func TestParser_RoundTrip(t *testing.T) {
    // For each .st file in testdata/parse/
    files, _ := filepath.Glob("testdata/parse/*.st")
    for _, f := range files {
        t.Run(filepath.Base(f), func(t *testing.T) {
            src, _ := os.ReadFile(f)
            ast, diags := parser.Parse(filepath.Base(f), string(src))
            // No panics (checked by test framework)
            // Check against golden .ast.json
            golden := f + ".ast.json"
            if _, err := os.Stat(golden); err == nil {
                expected, _ := os.ReadFile(golden)
                actual, _ := json.MarshalIndent(ast, "", "  ")
                require.JSONEq(t, string(expected), string(actual))
            }
        })
    }
}

func TestParser_ErrorRecovery(t *testing.T) {
    // Broken ST — parser should NOT panic, should return partial AST + diagnostics
    src := `FUNCTION_BLOCK FB_Broken
VAR_INPUT
    x : BOOL
    // missing semicolon
END_VAR
IF x THEN
    y := ; // missing expression
END_IF;
END_FUNCTION_BLOCK`
    ast, diags := parser.Parse("broken.st", src)
    require.NotNil(t, ast, "parser must return partial AST")
    require.NotEmpty(t, diags, "parser must report diagnostics")
    // AST should still contain the FB declaration and IF statement
}

func TestParser_CODESYSOop(t *testing.T) {
    src := `FUNCTION_BLOCK FB_Motor EXTENDS FB_Base IMPLEMENTS IMotor
METHOD Start : BOOL
    Start := TRUE;
END_METHOD
END_FUNCTION_BLOCK`
    ast, diags := parser.Parse("oop.st", src)
    require.Empty(t, diags)
    fb := ast.POUs[0].(*ast.FunctionBlock)
    require.Equal(t, "FB_Base", fb.Extends)
    require.Equal(t, []string{"IMotor"}, fb.Implements)
    require.Len(t, fb.Methods, 1)
}
```

**Golden tests:** 20+ `.st` files covering all constructs, each with `.ast.json` golden output.

**Corpus test:** Parse your sanitized real-world ST files. Report: total, passed, failed-with-diagnostic, panicked (must be 0).

**Agent verification:**
```bash
go test ./pkg/parser/... -v -count=1
go test ./pkg/ast/... -v -count=1
go run ./cmd/stc parse testdata/parse/motor_control.st --format json | jq '.pous | length'
# Must output a number > 0
```

---

### M3: Preprocessor (Days 5-7, parallel with M2)

**Deliverables:**
- `pkg/preproc/` — processes `{IF defined(VENDOR_BECKHOFF)}` and friends
- Supported directives:
  - `{IF defined(NAME)}`, `{IF NOT defined(NAME)}`
  - `{ELSIF defined(NAME)}`
  - `{ELSE}`
  - `{END_IF}`
  - `{DEFINE NAME}` (file-local define)
  - `{ERROR "message"}`
- Source map: original file:line:col → preprocessed file:line:col (JSON)
- `stc pp <file> --define VENDOR_BECKHOFF` CLI command
- Integrates before parser: `stc parse` runs preprocessor first

**E2E tests:**
```go
func TestPreproc_VendorSelection(t *testing.T) {
    src := `FUNCTION_BLOCK FB_Motor
VAR_INPUT
    Start : BOOL;
END_VAR
{IF defined(VENDOR_BECKHOFF)}
    {attribute 'qualified_only'}
{ELSIF defined(VENDOR_SCHNEIDER)}
    // Schneider-specific init
{ELSE}
    {ERROR "Unsupported vendor"}
{END_IF}
END_FUNCTION_BLOCK`

    bk, err := preproc.Process(src, []string{"VENDOR_BECKHOFF"})
    require.NoError(t, err)
    require.Contains(t, bk.Output, "qualified_only")
    require.NotContains(t, bk.Output, "Schneider")

    se, err := preproc.Process(src, []string{"VENDOR_SCHNEIDER"})
    require.NoError(t, err)
    require.Contains(t, se.Output, "Schneider")
    require.NotContains(t, se.Output, "qualified_only")

    _, err = preproc.Process(src, []string{})
    require.Error(t, err) // should hit {ERROR}
}

func TestPreproc_SourceMap(t *testing.T) {
    src := "line1\n{IF defined(X)}\nline3\n{END_IF}\nline5"
    result, _ := preproc.Process(src, []string{"X"})
    // line3 in output maps to line 3 in original
    loc := result.SourceMap.OriginalLocation(2) // 0-indexed output line 2
    require.Equal(t, 3, loc.Line) // 1-indexed original line
}
```

**Agent verification:**
```bash
go test ./pkg/preproc/... -v -count=1
go run ./cmd/stc pp testdata/preproc/motor.st --define VENDOR_BECKHOFF > /tmp/bk.st
go run ./cmd/stc pp testdata/preproc/motor.st --define VENDOR_SCHNEIDER > /tmp/se.st
diff /tmp/bk.st /tmp/se.st  # must differ
```

---

### M4: Type Checker & Semantic Analysis (Days 9-14)

**Deliverables:**
- `pkg/sema/` — type checking, symbol resolution, diagnostics
- Symbol tables: global, per-POU, per-method scopes
- Type system: all IEC primitive types, arrays, structs, enums, FB instances
- Overload resolution for standard functions
- FB instance call validation (check VAR_INPUT/VAR_OUTPUT names)
- Cross-file resolution (multi-file projects)
- Vendor-aware diagnostics: warn when using non-portable constructs
- `stc check <files...> --format json` CLI command

**E2E tests:**
```go
func TestSema_TypeMismatch(t *testing.T) {
    src := `PROGRAM Main
VAR x : BOOL; y : INT; END_VAR
x := y; // type error: INT not assignable to BOOL
END_PROGRAM`
    diags := sema.Check(parser.MustParse(src))
    require.Len(t, diags, 1)
    require.Contains(t, diags[0].Message, "type")
    require.Equal(t, 3, diags[0].Pos.Line)
}

func TestSema_UndeclaredVariable(t *testing.T) {
    src := `PROGRAM Main
VAR x : BOOL; END_VAR
y := TRUE; // y not declared
END_PROGRAM`
    diags := sema.Check(parser.MustParse(src))
    require.Len(t, diags, 1)
    require.Contains(t, diags[0].Message, "undeclared")
}

func TestSema_FBInstanceCall(t *testing.T) {
    src := `PROGRAM Main
VAR timer : TON; END_VAR
timer(IN := TRUE, PT := T#5s);
IF timer.Q THEN
    // ok
END_IF;
END_PROGRAM`
    diags := sema.Check(parser.MustParse(src))
    require.Empty(t, diags) // TON with IN, PT, Q is valid
}

func TestSema_VendorWarning(t *testing.T) {
    src := `FUNCTION_BLOCK FB_X EXTENDS FB_Y
METHOD Foo : BOOL
    Foo := TRUE;
END_METHOD
END_FUNCTION_BLOCK`
    diags := sema.Check(parser.MustParse(src), sema.WithTarget("allen_bradley"))
    // AB doesn't support OOP — should warn
    require.NotEmpty(t, diags)
    require.Contains(t, diags[0].Message, "not supported")
}
```

**Agent verification:**
```bash
go test ./pkg/sema/... -v -count=1
go run ./cmd/stc check testdata/sema/good/*.st --format json | jq '.diagnostics | length'
# Must be 0 for good files
go run ./cmd/stc check testdata/sema/bad/*.st --format json | jq '.diagnostics | length'
# Must be > 0 for bad files
```

---

### M5: LSP Server (Days 10-16, overlaps M4)

**Deliverables:**
- `cmd/stc-lsp/` — binary that speaks LSP over stdio
- Uses `go.lsp.dev/protocol` or GLSP library
- Features:
  - `textDocument/publishDiagnostics` — real-time errors from parser + sema
  - `textDocument/completion` — keywords, types, declared variables, FB members
  - `textDocument/hover` — show type info, FB documentation
  - `textDocument/definition` — jump to declaration
  - `textDocument/references` — find usages
  - `textDocument/formatting` — keyword capitalization, indentation
- Preprocessor awareness: gray out inactive `{IF}` blocks via semantic tokens
- Wire into VS Code extension: update `vscode/src/extension.ts` to spawn `stc-lsp`

**E2E tests:**
```go
func TestLSP_Initialize(t *testing.T) {
    // Start LSP server, send initialize, verify capabilities
    conn := startLSPServer(t)
    result, err := conn.Initialize(context.Background(), &protocol.InitializeParams{})
    require.NoError(t, err)
    require.True(t, result.Capabilities.CompletionProvider != nil)
    require.True(t, result.Capabilities.HoverProvider != nil)
}

func TestLSP_Diagnostics(t *testing.T) {
    conn := startLSPServer(t)
    conn.Initialize(...)
    conn.DidOpen("test.st", `PROGRAM Main
VAR x : BOOL; END_VAR
x := 42; // type error
END_PROGRAM`)
    diags := conn.WaitForDiagnostics("test.st", 2*time.Second)
    require.Len(t, diags, 1)
    require.Contains(t, diags[0].Message, "type")
}

func TestLSP_Completion(t *testing.T) {
    conn := startLSPServer(t)
    // Open file with TON instance, trigger completion after "timer."
    conn.DidOpen("test.st", `PROGRAM Main
VAR timer : TON; END_VAR
timer.
END_PROGRAM`)
    completions := conn.Completion("test.st", 3, 6) // after the dot
    labels := extractLabels(completions)
    require.Contains(t, labels, "Q")
    require.Contains(t, labels, "ET")
    require.Contains(t, labels, "IN")
    require.Contains(t, labels, "PT")
}

func TestLSP_GotoDefinition(t *testing.T) {
    conn := startLSPServer(t)
    conn.DidOpen("test.st", `FUNCTION_BLOCK FB_Motor
VAR_INPUT Start : BOOL; END_VAR
END_FUNCTION_BLOCK
PROGRAM Main
VAR m : FB_Motor; END_VAR
m.Start := TRUE;
END_PROGRAM`)
    loc := conn.Definition("test.st", 6, 3) // on "Start" after "m."
    require.Equal(t, 2, loc.Range.Start.Line) // line where Start is declared
}
```

**Agent verification:**
```bash
go test ./pkg/lsp/... -v -count=1
go test ./cmd/stc-lsp/... -v -count=1
# Smoke test: start LSP, send initialize via stdio, check response
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | go run ./cmd/stc-lsp | jq '.result.capabilities'
```

---

### M6: Standard Library (Days 12-15, parallel with M5)

**Deliverables:**
- `pkg/stdlib/` — Go implementations of IEC standard FBs
- Implemented in Go (not ST) for the host interpreter:
  - Timers: TON, TOF, TP
  - Counters: CTU, CTD, CTUD
  - Edge detection: R_TRIG, F_TRIG
  - Bistable: SR, RS
  - Standard functions: ABS, SQRT, MIN, MAX, SEL, MUX, LIMIT, etc.
- Each FB has a Go struct implementing the scan-cycle interface
- Each FB has exhaustive tests matching IEC spec behavior

**E2E tests:**
```go
func TestTON_Basic(t *testing.T) {
    ton := stdlib.NewTON()
    dt := 100 * time.Millisecond
    preset := 500 * time.Millisecond

    // IN=FALSE: Q stays FALSE, ET stays 0
    ton.Execute(false, preset, dt)
    require.False(t, ton.Q)
    require.Equal(t, time.Duration(0), ton.ET)

    // IN=TRUE for 4 cycles (400ms < 500ms): Q still FALSE, ET rising
    for i := 0; i < 4; i++ {
        ton.Execute(true, preset, dt)
    }
    require.False(t, ton.Q)
    require.Equal(t, 400*time.Millisecond, ton.ET)

    // 5th cycle (500ms >= 500ms): Q becomes TRUE
    ton.Execute(true, preset, dt)
    require.True(t, ton.Q)
    require.Equal(t, 500*time.Millisecond, ton.ET)

    // IN goes FALSE: Q resets, ET resets
    ton.Execute(false, preset, dt)
    require.False(t, ton.Q)
    require.Equal(t, time.Duration(0), ton.ET)
}

func TestRTRIG(t *testing.T) {
    rt := stdlib.NewRTRIG()
    require.False(t, rt.Execute(false)) // no edge
    require.True(t, rt.Execute(true))   // rising edge
    require.False(t, rt.Execute(true))  // still high, no edge
    require.False(t, rt.Execute(false)) // falling, no edge
    require.True(t, rt.Execute(true))   // rising again
}
```

**Agent verification:**
```bash
go test ./pkg/stdlib/... -v -count=1 -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
# Target: >90% coverage on stdlib
```

---

### M7: Interpreter & Test Runner (Days 14-21)

**Deliverables:**
- `pkg/interp/` — reference interpreter that executes typed IR
- Scan-cycle model: `ReadInputs() → Execute() → WriteOutputs()` per cycle
- Deterministic time: interpreter controls dt, no wall-clock dependency
- `pkg/testdsl/` — parse TEST_CASE blocks in ST
- `stc test <dir>` CLI — discovers test files, runs them, outputs JUnit XML
- Test DSL syntax:
  ```
  TEST_CASE 'Motor starts on command'
  VAR motor : FB_Motor; END_VAR
  motor(Start := TRUE, Stop := FALSE);
  ASSERT_TRUE(motor.Running);
  motor(Start := FALSE, Stop := TRUE);
  ASSERT_FALSE(motor.Running);
  END_TEST_CASE
  ```

**E2E tests:**
```go
func TestInterp_ScanCycle(t *testing.T) {
    src := `PROGRAM Main
VAR_INPUT start : BOOL; END_VAR
VAR_OUTPUT running : BOOL; END_VAR
VAR timer : TON; END_VAR
timer(IN := start, PT := T#1s);
running := timer.Q;
END_PROGRAM`

    prog := compile(t, src)
    rt := interp.New(prog)

    // Set input, run 5 scans at 200ms each (total 1s)
    rt.SetInput("start", true)
    for i := 0; i < 5; i++ {
        rt.Tick(200 * time.Millisecond)
    }
    require.True(t, rt.GetOutput("running").(bool))
}

func TestTestRunner_JUnitOutput(t *testing.T) {
    // Run stc test on example test directory, verify JUnit XML
    cmd := exec.Command("go", "run", "./cmd/stc", "test", "testdata/e2e/tests/", "--format", "junit")
    out, err := cmd.Output()
    require.NoError(t, err)
    require.Contains(t, string(out), "<testsuite")
    require.Contains(t, string(out), "tests=")
}

func TestTestRunner_ExitCode(t *testing.T) {
    // Passing tests → exit 0
    cmd := exec.Command("go", "run", "./cmd/stc", "test", "testdata/e2e/tests_pass/")
    require.NoError(t, cmd.Run())

    // Failing tests → exit 1
    cmd = exec.Command("go", "run", "./cmd/stc", "test", "testdata/e2e/tests_fail/")
    err := cmd.Run()
    require.Error(t, err)
}
```

**Agent verification:**
```bash
go test ./pkg/interp/... -v -count=1
go test ./pkg/testdsl/... -v -count=1
go run ./cmd/stc test testdata/e2e/tests/ --format junit > /tmp/junit.xml
grep -q '<testsuite' /tmp/junit.xml
go run ./cmd/stc test testdata/e2e/tests/ --format json | jq '.passed'
```

---

### M8: Vendor ST Emitters (Days 18-22, overlaps M7)

**Deliverables:**
- `pkg/emitter/portable/` — emit clean, normalized portable ST
- `pkg/emitter/beckhoff/` — emit Beckhoff-flavored ST (with `{attribute}` pragmas)
- `pkg/emitter/schneider/` — emit Schneider-flavored ST
- `stc emit <file> --target beckhoff|schneider|portable`
- Round-trip stability: parse → emit → parse → emit should be stable

**E2E tests:**
```go
func TestEmitter_RoundTrip(t *testing.T) {
    files, _ := filepath.Glob("testdata/emit/*.st")
    for _, f := range files {
        for _, target := range []string{"beckhoff", "schneider", "portable"} {
            t.Run(filepath.Base(f)+"_"+target, func(t *testing.T) {
                src, _ := os.ReadFile(f)
                ast1, _ := parser.Parse(f, string(src))
                out1 := emitter.Emit(ast1, target)
                ast2, _ := parser.Parse(f, out1)
                out2 := emitter.Emit(ast2, target)
                require.Equal(t, out1, out2, "round-trip not stable")
            })
        }
    }
}

func TestEmitter_BeckhoffAttributes(t *testing.T) {
    src := `{IF defined(VENDOR_BECKHOFF)}
{attribute 'qualified_only'}
{END_IF}
FUNCTION_BLOCK FB_Motor
VAR_INPUT Start : BOOL; END_VAR
END_FUNCTION_BLOCK`
    ast, _ := parser.Parse("test.st", preproc.MustProcess(src, "VENDOR_BECKHOFF"))
    out := emitter.Emit(ast, "beckhoff")
    require.Contains(t, out, "{attribute 'qualified_only'}")
}
```

**Agent verification:**
```bash
go test ./pkg/emitter/... -v -count=1
go run ./cmd/stc emit testdata/emit/motor.st --target beckhoff > /tmp/bk.st
go run ./cmd/stc emit testdata/emit/motor.st --target schneider > /tmp/se.st
# Both must parse cleanly
go run ./cmd/stc parse /tmp/bk.st --format json | jq '.pous | length'
go run ./cmd/stc parse /tmp/se.st --format json | jq '.pous | length'
```

---

### M9: MCP Server (Days 20-23, parallel with M8)

**Deliverables:**
- `cmd/stc-mcp/` — MCP server exposing stc tools to LLM agents
- Tools exposed:
  - `stc_parse` — parse ST, return AST or diagnostics
  - `stc_check` — type check, return diagnostics
  - `stc_test` — run tests, return results
  - `stc_emit` — emit vendor ST
  - `stc_format` — format ST code
  - `stc_complete` — given partial ST, return completions (for agent code generation)
- JSON schemas for all inputs/outputs
- Minimal token footprint: tool descriptions are <100 tokens each

**E2E test:**
```go
func TestMCP_ParseTool(t *testing.T) {
    // Send MCP tool call, verify response
    server := startMCPServer(t)
    result := server.CallTool("stc_parse", map[string]any{
        "source": "PROGRAM Main\nVAR x : BOOL; END_VAR\nEND_PROGRAM",
    })
    require.True(t, result["success"].(bool))
    require.NotEmpty(t, result["ast"])
}

func TestMCP_CheckWithErrors(t *testing.T) {
    server := startMCPServer(t)
    result := server.CallTool("stc_check", map[string]any{
        "source": "PROGRAM Main\nVAR x : BOOL; END_VAR\nx := 42;\nEND_PROGRAM",
    })
    diags := result["diagnostics"].([]any)
    require.Len(t, diags, 1)
}
```

---

### M10: Full E2E — The "Common Library" Demo (Days 22-25)

**Deliverables:**
- `examples/common_lib/` — complete working example of:
  - `FB_MotorCore` — portable motor control FB (vendor-neutral)
  - `FB_MotorIO_Beckhoff` — Beckhoff I/O adapter
  - `FB_MotorIO_Schneider` — Schneider I/O adapter
  - `FB_MotorIO_Sim` — simulation adapter for host testing
  - Test suite that runs on host
  - `stc.toml` project manifest
- CI pipeline that:
  1. Runs `stc check` on all files
  2. Runs `stc test` with simulation adapter
  3. Emits Beckhoff-flavored ST
  4. Emits Schneider-flavored ST
  5. All green

**E2E test — THE acceptance test for the whole project:**
```go
func TestE2E_CommonLibrary(t *testing.T) {
    // 1. Check all ST files
    cmd := exec.Command("go", "run", "./cmd/stc", "check",
        "examples/common_lib/src/...",
        "--define", "VENDOR_SIM",
        "--format", "json")
    out, err := cmd.CombinedOutput()
    require.NoError(t, err, "stc check failed: %s", out)

    // 2. Run tests
    cmd = exec.Command("go", "run", "./cmd/stc", "test",
        "examples/common_lib/tests/",
        "--define", "VENDOR_SIM",
        "--format", "json")
    out, err = cmd.CombinedOutput()
    require.NoError(t, err, "stc test failed: %s", out)
    var result map[string]any
    json.Unmarshal(out, &result)
    require.Equal(t, float64(0), result["failed"])

    // 3. Emit Beckhoff
    cmd = exec.Command("go", "run", "./cmd/stc", "emit",
        "examples/common_lib/src/motor_core.st",
        "--target", "beckhoff")
    out, err = cmd.CombinedOutput()
    require.NoError(t, err, "emit beckhoff failed: %s", out)
    require.Contains(t, string(out), "FUNCTION_BLOCK")

    // 4. Emit Schneider
    cmd = exec.Command("go", "run", "./cmd/stc", "emit",
        "examples/common_lib/src/motor_core.st",
        "--target", "schneider")
    out, err = cmd.CombinedOutput()
    require.NoError(t, err, "emit schneider failed: %s", out)
    require.Contains(t, string(out), "FUNCTION_BLOCK")
}
```

**Agent verification:**
```bash
go test ./... -v -count=1 -timeout 5m
# This single command must pass ALL tests across ALL packages
# If it passes, the project is ready
```

---

## Parallelization Map

```
Day 1:          M0 (bootstrap)
Days 2-3:       M1 (lexer)          | M1-vscode (extension)
Days 4-8:       M2 (parser)         | M3 (preprocessor)
Days 9-14:      M4 (type checker)
Days 10-16:     M5 (LSP)            | M6 (stdlib)       [both need M4 partial]
Days 14-21:     M7 (interpreter)
Days 18-22:     M8 (emitters)       | M9 (MCP server)   [parallel]
Days 22-25:     M10 (E2E demo)
```

**3 agents can work simultaneously for most of the project:**
- Agent A: lexer → parser → type checker → interpreter
- Agent B: vscode extension → LSP server (needs parser from A)
- Agent C: preprocessor → stdlib → emitters → MCP server

---

## CI Pipeline (.github/workflows/ci.yml)

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest]
        go: ['1.22']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
      - run: go build ./...
      - run: go test ./... -v -count=1 -timeout 5m
      - run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - uses: golangci/golangci-lint-action@v4

  vscode:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: cd vscode && npm ci && npm run compile && npx vsce package
```

---

## Agent Instructions Template

Give this to Claude Code or any agent:

```
You are building `stc`, a Structured Text compiler toolchain in Go.

Repository: <repo-url>
Architecture: see docs/ARCHITECTURE.md
Current milestone: M<N>

Rules:
1. Every function must have a test. Write the test FIRST.
2. Use golden test files in testdata/ for parser and emitter tests.
3. All CLI output must support --format json.
4. Never panic — return errors.
5. Run `go test ./...` after every change. If it fails, fix it before moving on.
6. Run `go vet ./...` and fix warnings.
7. Update testdata/ golden files when output intentionally changes (go test -update).
8. Commit after each passing test addition.

Verify completion:
go build ./...
go test ./... -v -count=1
go vet ./...
```
