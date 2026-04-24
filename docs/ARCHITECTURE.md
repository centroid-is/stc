# Architecture

Technical architecture of the stc compiler toolchain for developers.

## System Overview

```
                         stc.toml
                            |
                            v
  .st source ----> [Preprocessor] ----> [Lexer] ----> [Parser] ----> AST
                   (pkg/preprocess)    (pkg/lexer)   (pkg/parser)   (pkg/ast)
                                                                      |
                   +--------------------------------------------------+
                   |                    |                    |
                   v                    v                    v
              [Checker]            [Formatter]          [Emitter]
           (pkg/checker)         (pkg/format)          (pkg/emit)
                   |
          +--------+--------+
          |                 |
          v                 v
     [Interpreter]     [Analyzer]
    (pkg/interp)     (pkg/analyzer)
          |                 |
     +----+----+            v
     |         |       [Incremental]
     v         v      (pkg/incremental)
  [Testing] [Sim]
(pkg/testing)(pkg/sim)
```

### Data Flow

1. **Source text** enters the preprocessor, which evaluates `{IF}` / `{DEFINE}` directives and produces preprocessed text with a source map for position remapping.

2. The **lexer** tokenizes the preprocessed text into a stream of tokens, capturing trivia (whitespace, comments) for CST fidelity.

3. The **parser** consumes tokens via recursive descent with Pratt expression parsing. It produces a concrete syntax tree (`ast.SourceFile`) with error recovery -- broken code yields partial ASTs with `ErrorNode` markers plus diagnostics.

4. The **checker** runs two passes over the AST:
   - **Pass 1 (Resolver)**: Collects all declarations (PROGRAMs, FUNCTION_BLOCKs, FUNCTIONs, TYPEs) into the symbol table. Library stub files are loaded before user code.
   - **Pass 2 (Type Checker)**: Walks expression and statement bodies, resolves types, checks assignments, validates FB parameter usage, and emits vendor-aware warnings.

5. The **interpreter** executes the AST directly with PLC scan-cycle semantics: read inputs, execute body, write outputs. It maintains an environment (`Env`) per scope with variable storage, and `FBInstance` objects for function block state.

6. The **testing** package discovers `*_test.st` files, parses `TEST_CASE` declarations, and executes them through the interpreter with assertion collection.

7. The **sim** package runs closed-loop simulations with waveform generators driving inputs and optional plant models providing feedback.

8. The **emitter** walks the AST and produces vendor-flavored ST text (Beckhoff, Schneider, or portable).

9. The **formatter** walks the AST and re-emits ST with normalized style (indentation, keyword casing, spacing), preserving comments attached to nodes.

## Package Dependency Graph

Arrows indicate "imports" direction (A --> B means A imports B).

```
cmd/stc -------> pkg/analyzer, pkg/ast, pkg/diag, pkg/emit, pkg/format,
                 pkg/incremental, pkg/lint, pkg/lsp, pkg/parser, pkg/pipeline,
                 pkg/preprocess, pkg/project, pkg/sim, pkg/testing, pkg/vendor,
                 pkg/version

cmd/stc-mcp ---> pkg/analyzer, pkg/ast, pkg/diag, pkg/emit, pkg/format,
                 pkg/lint, pkg/parser, pkg/pipeline, pkg/testing

pkg/analyzer --> pkg/ast, pkg/checker, pkg/diag, pkg/project
pkg/checker ---> pkg/ast, pkg/diag, pkg/symbols, pkg/types
pkg/emit ------> pkg/ast
pkg/format ----> pkg/ast
pkg/incremental> pkg/ast, pkg/diag, pkg/parser, pkg/pipeline, pkg/source
pkg/interp ----> pkg/ast, pkg/types
pkg/iomap -----> pkg/ast
pkg/lexer -----> pkg/ast, pkg/source
pkg/lint ------> pkg/ast, pkg/diag
pkg/lsp -------> pkg/analyzer, pkg/ast, pkg/checker, pkg/diag, pkg/format,
                 pkg/parser, pkg/pipeline, pkg/symbols
pkg/parser ----> pkg/ast, pkg/lexer, pkg/source
pkg/pipeline --> pkg/ast, pkg/diag, pkg/parser, pkg/preprocess
pkg/preprocess > pkg/diag, pkg/source
pkg/sim -------> pkg/ast, pkg/interp, pkg/pipeline
pkg/symbols ---> pkg/ast, pkg/types
pkg/testing ---> pkg/ast, pkg/interp, pkg/iomap, pkg/pipeline
pkg/vendor ----> pkg/ast, pkg/parser, pkg/project
```

Foundation packages with no internal dependencies: `pkg/source`, `pkg/diag`, `pkg/types`, `pkg/version`.

## Key Interfaces and Types

### Node (pkg/ast)

Every AST node implements the `Node` interface:

```go
type Node interface {
    Kind() NodeKind       // Discriminator (KindProgramDecl, KindIfStmt, etc.)
    Pos() Pos             // Source position (file, line, column)
    Children() []Node     // Child nodes for tree traversal
}
```

Nodes are categorized into declarations (`ProgramDecl`, `FunctionBlockDecl`, `FunctionDecl`, `InterfaceDecl`, `MethodDecl`, `PropertyDecl`, `TypeDecl`, `ActionDecl`, `TestCaseDecl`), statements (`AssignStmt`, `CallStmt`, `IfStmt`, `CaseStmt`, `ForStmt`, `WhileStmt`, `RepeatStmt`, `ReturnStmt`, `ExitStmt`, `ContinueStmt`), and expressions (`BinaryExpr`, `UnaryExpr`, `CallExpr`, `MemberAccessExpr`, `IndexExpr`, `LiteralExpr`, `IdentExpr`).

The `SourceFile` is the root node containing a slice of `Declaration` nodes.

### Value (pkg/interp)

Runtime values in the interpreter:

```go
type Value struct {
    Kind     ValueKind   // Bool, Int, Real, String, Time, Array, Struct, Enum, ...
    Bool     bool
    Int      int64
    Real     float64
    Str      string
    Dur      time.Duration
    Elements []Value           // For arrays
    Fields   map[string]Value  // For structs
}
```

### StandardFB (pkg/interp)

Interface for built-in function blocks (timers, counters, edge detectors, bistables):

```go
type StandardFB interface {
    Execute(dt time.Duration)
    SetInput(name string, v Value)
    GetOutput(name string) Value
    GetInput(name string) Value
}
```

`StdlibFBFactory` is a `map[string]func() StandardFB` that maps type names to constructors.

### FBInstance (pkg/interp)

Wraps either a `StandardFB` (for stdlib FBs) or an `Env` + `Decl` pair (for user-defined FBs):

```go
type FBInstance struct {
    TypeName   string
    FB         StandardFB              // Non-nil for stdlib FBs
    Env        *Env                    // Non-nil for user-defined FBs
    Decl       *ast.FunctionBlockDecl  // AST declaration
    ParentDecl *ast.FunctionBlockDecl  // For EXTENDS chain
}
```

### PlantModel (pkg/sim)

Interface for simulated physical systems:

```go
type PlantModel interface {
    Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value
}
```

Built-in models: `MotorModel`, `ValveModel`, `CylinderModel`.

### Config (pkg/project)

Project configuration loaded from `stc.toml`:

```go
type Config struct {
    Project ProjectConfig  // name, version
    Build   BuildConfig    // source_roots, vendor_target, library_paths
    Lint    LintConfig     // naming_convention
    Test    TestConfig     // mock_paths
}
```

## How to Add a New ST Language Feature

1. **Lexer** (`pkg/lexer/`): If the feature introduces new keywords or token types, add them to the keyword table and token type constants.

2. **AST** (`pkg/ast/`): Define new node types in the appropriate file (`decl.go` for declarations, `stmt.go` for statements, `expr.go` for expressions). Implement `Kind()`, `Pos()`, `Children()`, and the marker method (`declNode()`, `stmtNode()`, or `exprNode()`). Add JSON marshaling support in `json.go`.

3. **Parser** (`pkg/parser/`): Add parsing logic in the recursive descent parser. Follow the existing pattern of `parseXxx` methods. Add error recovery if the construct can appear in a position where recovery is needed.

4. **Checker** (`pkg/checker/`): Add type-checking logic in the appropriate pass. Pass 1 (`resolve.go`) for declarations, Pass 2 for expressions and statements.

5. **Interpreter** (`pkg/interp/`): Add evaluation logic in `interpreter.go` for expressions or statement execution.

6. **Emitter** (`pkg/emit/`): Add emission logic to reproduce the construct in vendor-flavored ST.

7. **Formatter** (`pkg/format/`): Add formatting logic to produce consistently styled output.

8. **Tests**: Add unit tests at each level (parser, checker, interpreter) and integration tests in `tests/`.

## How to Add a New CLI Command

1. Create a new file in `cmd/stc/` (e.g., `mycommand.go`).

2. Define a `newMyCmd() *cobra.Command` function following the existing pattern:
   - Set `Use`, `Short`, `Long` descriptions
   - Add command-specific flags
   - Implement `RunE` handler
   - Support `--format json` via the persistent `format` flag

3. Register the command in `cmd/stc/main.go` by adding `newMyCmd()` to the `rootCmd.AddCommand(...)` call.

4. Add tests in `cmd/stc/mycommand_test.go`.

5. If the command should be available via MCP, add a tool handler in `cmd/stc-mcp/tools.go`.

## How to Add a New Stdlib Function Block

1. Create a new file `pkg/interp/stdlib_myblock.go`.

2. Define a struct implementing `StandardFB`:
   ```go
   type MyBlock struct {
       // Internal state
   }
   func (b *MyBlock) Execute(dt time.Duration) { /* ... */ }
   func (b *MyBlock) SetInput(name string, v Value) { /* ... */ }
   func (b *MyBlock) GetOutput(name string) Value { /* ... */ }
   func (b *MyBlock) GetInput(name string) Value { /* ... */ }
   ```

3. Register the constructor in `StdlibFBFactory` via an `init()` function:
   ```go
   func init() {
       StdlibFBFactory["MY_BLOCK"] = func() StandardFB { return &MyBlock{} }
   }
   ```

4. Add the type signature in `pkg/types/builtin.go` so the checker knows the FB's inputs and outputs.

5. Add tests in `pkg/interp/stdlib_myblock_test.go`.

## How to Add Vendor Stubs

1. Create a `.st` file with `FUNCTION_BLOCK` declarations that have `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT` blocks but no body statements. Place it in `stdlib/vendor/<vendor>/`.

2. Supporting types (structs, enums) go in the same file or a separate `common_types.st`.

3. Add a test in `stdlib/vendor/<vendor>/stubs_test.go` that parses the stub file and verifies it produces no parse errors.

4. Users reference stubs via `[build.library_paths]` in their `stc.toml`.

## Testing Strategy

### Layers

- **Unit tests**: Each `pkg/` package has `*_test.go` files testing individual functions and types. Run with `go test ./...`.

- **Integration tests**: `cmd/stc/*_test.go` tests the CLI commands end-to-end by invoking the binary with test fixtures in `cmd/stc/testdata/`.

- **ST test suites**: `tests/` contains ST test files exercising the full pipeline (parse, check, interpret, assert). Run with `stc test tests/` or `go run ./cmd/stc test tests/`.

- **Corpus tests**: `tests/corpus/` contains real-world ST files for parse-only validation. `tests/corpus_test.go` verifies they parse without panics.

- **Adversarial tests**: `tests/adversarial/` and `pkg/interp/adversarial_test.go` test edge cases and malformed inputs.

### Coverage

Coverage thresholds are enforced via `.testcoverage.yml`:
- Overall: 85%
- Critical packages (parser, lexer, checker, interp, types, emit): 94-95%

CI runs coverage checks on every PR via the `coverage.yml` workflow.

### CI Workflows

| Workflow | File | Purpose |
|----------|------|---------|
| CI | `ci.yml` | Build, test, vet on Linux/macOS/Windows; golangci-lint |
| Coverage | `coverage.yml` | Coverage thresholds with Codecov upload |
| Release | `release.yml` | Cross-platform binaries on GitHub release |
| ST Tests | `st-tests.yml` | Run ST test suites on all platforms |
