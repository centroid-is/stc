# Architecture Research

**Domain:** IEC 61131-3 Structured Text Compiler Toolchain
**Researched:** 2026-03-26
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Consumer Layer                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │   CLI    │  │   LSP    │  │   MCP    │  │  Skills  │               │
│  │ (stc)   │  │  Server  │  │  Server  │  │  Layer   │               │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘               │
│       │              │             │              │                     │
├───────┴──────────────┴─────────────┴──────────────┴─────────────────────┤
│                       Compiler API (Go package)                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Workspace / Project Model (file resolution, crate graph)       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────────┤
│                       Compiler Core Pipeline                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │ Preproc  │→ │  Lexer   │→ │  Parser  │→ │   AST    │               │
│  └──────────┘  └──────────┘  └──────────┘  └────┬─────┘               │
│                                                  │                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────┴─────┐               │
│  │ Indexer  │← │  Type    │← │ Resolver │← │ Symbol   │               │
│  │ (symbols)│  │ Checker  │  │ (names)  │  │  Table   │               │
│  └────┬─────┘  └──────────┘  └──────────┘  └──────────┘               │
│       │                                                                │
├───────┴─────────────────────────────────────────────────────────────────┤
│                         Output Backends                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │Interpre- │  │ C++ Code │  │ Vendor   │  │ PLCopen  │               │
│  │  ter     │  │   Gen    │  │ ST Emit  │  │ XML I/O  │               │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘               │
├─────────────────────────────────────────────────────────────────────────┤
│                        Runtime / Test Layer                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                             │
│  │ Scan     │  │  Test    │  │  Plant   │                             │
│  │ Cycle    │  │  Runner  │  │  Model   │                             │
│  │ Engine   │  │          │  │  (sim)   │                             │
│  └──────────┘  └──────────┘  └──────────┘                             │
└─────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| Preprocessor | Pragma handling, file inclusion, conditional compilation, vendor-specific directive stripping | Single-pass text transform before lexing |
| Lexer | Tokenization of ST source into token stream | Hand-written scanner (not generated; needed for pragma feedback and error recovery) |
| Parser | Recursive descent parse of token stream into CST/AST | Hand-written recursive descent with error recovery (critical for LSP partial ASTs) |
| AST | Typed tree structure representing the program | Immutable node types with position spans; supports incomplete/error nodes |
| Symbol Table | Maps identifiers to declarations with scope chains | Hierarchical scope model (global -> POU -> block -> local) |
| Name Resolver | Resolves identifier references to declarations | Walk AST, populate symbol table, link references |
| Type Checker | Validates type correctness, coercion rules, assignment compatibility | Walk resolved AST, apply IEC 61131-3 type promotion rules |
| Indexer | Builds cross-file index for go-to-definition, find-references | Indexes declarations by name, type, scope; updated incrementally |
| Interpreter | Executes typed AST directly for test runner | Tree-walking interpreter with scan cycle semantics |
| C++ Codegen | Emits C++17 from typed AST | AST visitor producing C++ source strings |
| Vendor ST Emitter | Re-emits ST in vendor-specific dialect | AST visitor with vendor-specific formatting rules |
| PLCopen XML I/O | Imports/exports PLCopen XML format | XML parser/serializer mapping to/from AST |
| Scan Cycle Engine | Simulates PLC scan cycle (input read -> execute -> output write) | Deterministic loop with configurable cycle time |
| Test Runner | Discovers and executes ST test programs, produces JUnit XML | Orchestrates scan cycle engine with assertions and mocking |
| Plant Model | Simulates physical process for closed-loop testing | Pluggable simulation interface (sensor injection, actuator reading) |
| Workspace Model | File discovery, project configuration, multi-file compilation units | Resolves library paths, manages file dependencies |
| LSP Server | Language Server Protocol implementation | Wraps compiler API, manages document state, incremental updates |
| MCP Server | Model Context Protocol for LLM agents | Wraps CLI commands as MCP tools |
| CLI | Command-line interface (`stc check`, `stc test`, `stc emit`, etc.) | Thin wrapper over compiler API with `--format json` support |

## Recommended Project Structure

```
cmd/
├── stc/                # CLI entry point
│   └── main.go         # Cobra command tree
├── stc-lsp/            # LSP server entry point
│   └── main.go         # LSP main loop
└── stc-mcp/            # MCP server entry point
    └── main.go         # MCP handler

pkg/
├── compiler/           # Compiler API facade
│   ├── compiler.go     # Top-level Compile(), Check(), Parse() functions
│   └── workspace.go    # Project/workspace model
├── source/             # Source text management
│   ├── source.go       # Source file representation with positions
│   └── span.go         # Position/span types
├── preprocess/         # Preprocessor
│   └── preprocess.go   # Pragma handling, file inclusion
├── lexer/              # Lexical analysis
│   ├── lexer.go        # Scanner
│   ├── token.go        # Token types
│   └── keywords.go     # Keyword table
├── parser/             # Syntax analysis
│   ├── parser.go       # Recursive descent parser
│   └── error.go        # Error recovery strategies
├── ast/                # Abstract syntax tree
│   ├── node.go         # Node interface and base types
│   ├── decl.go         # Declaration nodes (PROGRAM, FUNCTION_BLOCK, etc.)
│   ├── stmt.go         # Statement nodes (IF, FOR, WHILE, CASE, etc.)
│   ├── expr.go         # Expression nodes
│   ├── types.go        # Type declaration nodes
│   └── visitor.go      # Visitor pattern interface
├── symbols/            # Symbol table and name resolution
│   ├── scope.go        # Scope chain
│   ├── symbol.go       # Symbol types
│   └── resolve.go      # Name resolution pass
├── checker/            # Type checking and semantic analysis
│   ├── checker.go      # Type checker
│   ├── types.go        # Type system (IEC 61131-3 type lattice)
│   └── coerce.go       # Type coercion/promotion rules
├── diag/               # Diagnostics
│   ├── diagnostic.go   # Error/warning/info types with spans
│   └── reporter.go     # Diagnostic collection and formatting
├── index/              # Cross-file indexing
│   └── index.go        # Declaration index for IDE features
├── interp/             # Tree-walking interpreter
│   ├── interp.go       # Interpreter core
│   ├── value.go        # Runtime value representation
│   ├── scan.go         # Scan cycle engine
│   └── stdlib.go       # IEC standard library implementations (TON, CTU, etc.)
├── emit/               # Output backends
│   ├── cpp/            # C++17 code generation
│   │   ├── emitter.go  # C++ emitter
│   │   └── runtime.go  # Runtime header generation
│   ├── st/             # Vendor ST re-emission
│   │   ├── emitter.go  # Base ST emitter
│   │   ├── beckhoff.go # TwinCAT-specific formatting
│   │   └── codesys.go  # CODESYS-specific formatting
│   └── plcopen/        # PLCopen XML
│       ├── import.go   # XML -> AST
│       └── export.go   # AST -> XML
├── testrun/            # Test runner
│   ├── runner.go       # Test discovery and execution
│   ├── mock.go         # I/O mocking framework
│   ├── assert.go       # Assertion helpers
│   └── junit.go        # JUnit XML output
├── lsp/                # LSP protocol implementation
│   ├── server.go       # LSP server core
│   ├── handler.go      # Request/notification handlers
│   ├── document.go     # Document state management
│   └── features/       # Individual LSP features
│       ├── diagnostics.go
│       ├── completion.go
│       ├── definition.go
│       ├── hover.go
│       └── rename.go
└── mcp/                # MCP server implementation
    ├── server.go       # MCP server core
    └── tools.go        # Tool definitions wrapping CLI commands

internal/
├── version/            # Version info
└── testdata/           # Shared test fixtures
```

### Structure Rationale

- **cmd/ vs pkg/:** Go convention. `cmd/` for entry points, `pkg/` for importable library code. All compiler logic lives in `pkg/` so the LSP, MCP, and CLI all share the same compiler API.
- **compiler/ as facade:** Single entry point for all consumers. Hides pipeline orchestration. This is the boundary between "compiler internals" and "compiler users."
- **Pipeline packages (lexer/, parser/, ast/, symbols/, checker/):** Each stage is an independent package with clear input/output types. No circular dependencies. Each can be tested in isolation.
- **emit/ as pluggable backends:** Vendor emitters and C++ codegen are separate from the core pipeline. New vendors add a new file in `emit/st/`, not changes to core.
- **interp/ contains both interpreter and scan cycle:** The interpreter is the execution engine; the scan cycle engine wraps it to simulate PLC behavior. They are tightly coupled.
- **lsp/ separate from compiler/:** The LSP server is a consumer of the compiler API, not part of it. It manages document state, incremental updates, and protocol translation.

## Architectural Patterns

### Pattern 1: Compiler as Library (Facade Pattern)

**What:** The compiler is a Go package (`pkg/compiler/`) that exposes a clean API. CLI, LSP, MCP, and tests all consume this same API. No compiler logic lives in `cmd/`.
**When to use:** Always. This is the foundational pattern.
**Trade-offs:** Slightly more upfront design to define the API surface, but massive payoff in reuse and testability.

**Example:**
```go
// pkg/compiler/compiler.go
package compiler

// ParseResult contains the AST and any diagnostics from parsing.
type ParseResult struct {
    AST   *ast.File
    Diags []diag.Diagnostic
}

// CheckResult contains type-checked information and diagnostics.
type CheckResult struct {
    Parse   ParseResult
    Symbols *symbols.Table
    Types   *checker.Info
    Diags   []diag.Diagnostic
}

// Parse parses ST source into an AST. Always succeeds (partial AST on errors).
func Parse(src []byte, filename string) ParseResult { ... }

// Check parses and type-checks ST source.
func Check(src []byte, filename string) CheckResult { ... }

// Emit generates output from a checked program.
func Emit(result CheckResult, backend EmitBackend) ([]byte, error) { ... }
```

### Pattern 2: Error-Tolerant Pipeline (T, []Error Pattern)

**What:** Every pipeline stage returns `(result, diagnostics)` rather than `(result, error)`. The parser always produces an AST, even from broken code. Error nodes are valid AST nodes. This is the pattern rust-analyzer uses: "produces (T, Vec<Error>) rather than Result<T, Error>."
**When to use:** Every pipeline stage. Critical for LSP (code is usually broken while being edited).
**Trade-offs:** More complex node types (need error/missing variants), but enables LSP and incremental analysis.

**Example:**
```go
// AST always produced, diagnostics collected separately
type ParseResult struct {
    AST   *ast.File       // Always non-nil, may contain ErrorNode
    Diags []diag.Diagnostic // Errors, warnings, etc.
}

// ErrorNode represents unparseable source
type ErrorNode struct {
    Span  source.Span
    Token token.Token  // The problematic token
}
```

### Pattern 3: AST Visitor for Backends

**What:** All output backends (interpreter, C++ codegen, vendor ST emitters) implement a visitor interface over the AST. The AST is the central data structure; backends are pluggable consumers.
**When to use:** For all output generation. Adding a new vendor emitter means implementing the visitor, not modifying the pipeline.
**Trade-offs:** Visitor pattern can be verbose in Go (no sum types). Use interfaces + type switches instead of classic double-dispatch.

**Example:**
```go
// pkg/ast/visitor.go
type Visitor interface {
    VisitProgram(*ProgramDecl) error
    VisitFunctionBlock(*FunctionBlockDecl) error
    VisitFunction(*FunctionDecl) error
    VisitAssignment(*AssignStmt) error
    VisitIfStmt(*IfStmt) error
    // ... one method per node type
}

// Each backend implements Visitor
type CppEmitter struct { ... }
type BeckhoffEmitter struct { ... }
type Interpreter struct { ... }
```

### Pattern 4: Deterministic Scan Cycle Engine

**What:** The test runner and interpreter execute programs inside a simulated PLC scan cycle: Read Inputs -> Execute Logic -> Write Outputs -> Advance Time. Time is fully deterministic (virtual clock, not wall clock). This preserves PLC semantics on the host.
**When to use:** All test execution and simulation.
**Trade-offs:** Programs run slower than native PLC execution, but correctness and reproducibility matter more than speed for a development tool.

**Example:**
```go
// pkg/interp/scan.go
type ScanCycleEngine struct {
    Interp    *Interpreter
    Inputs    *IOTable       // Injected sensor values
    Outputs   *IOTable       // Captured actuator values
    Clock     *VirtualClock  // Deterministic time
    CycleTime time.Duration  // Configurable scan cycle period
}

func (e *ScanCycleEngine) RunCycle() {
    e.Interp.ReadInputs(e.Inputs)
    e.Interp.Execute()
    e.Interp.WriteOutputs(e.Outputs)
    e.Clock.Advance(e.CycleTime)
}
```

### Pattern 5: Incremental Document Model (for LSP)

**What:** The LSP server maintains a document store where each open file has its most recent parse result cached. On edit, only the changed file is re-parsed. Cross-file analysis (go-to-definition) uses the indexer. The key invariant from rust-analyzer: "typing inside a function body never invalidates global derived data."
**When to use:** LSP server document management.
**Trade-offs:** Requires careful cache invalidation design. Start simple (re-parse full file on each edit, index lazily) and optimize incrementally.

## Data Flow

### Compilation Pipeline Flow

```
Source File (.st)
    |
    v
[Preprocessor] -- handles {pragma}, file includes, conditional compilation
    |
    v
Token Stream
    |
    v
[Lexer] -- ST keywords, identifiers, literals, operators
    |
    v
Token Stream (with positions)
    |
    v
[Parser] -- recursive descent, error recovery, always produces AST
    |
    v
AST (may contain ErrorNodes) + Parse Diagnostics
    |
    v
[Name Resolver] -- populates symbol table, resolves references
    |
    v
AST + Symbol Table + Resolution Diagnostics
    |
    v
[Type Checker] -- validates types, infers coercions, checks assignments
    |
    v
Typed AST + Type Info + Type Diagnostics
    |
    v
[Backend Selection]
    |
    +---> [Interpreter] --> Execution (for test runner / simulation)
    +---> [C++ Emitter] --> C++17 source --> g++/clang++ --> native binary
    +---> [Vendor ST Emitter] --> Beckhoff/CODESYS-flavored .st files
    +---> [PLCopen XML Export] --> .xml for vendor IDE import
```

### LSP Integration Flow

```
VS Code Extension
    |  (JSON-RPC over stdio)
    v
[LSP Server]
    |
    +-- textDocument/didOpen --> Parse file, cache AST, run diagnostics
    +-- textDocument/didChange --> Re-parse changed file, update cache
    +-- textDocument/completion --> Query symbol table + type info
    +-- textDocument/definition --> Query indexer for declaration location
    +-- textDocument/hover --> Look up type info at position
    +-- textDocument/diagnostic --> Return cached diagnostics
    +-- textDocument/rename --> Query indexer for all references
    |
    v
[Compiler API] -- Parse(), Check() reused from compilation pipeline
    |
    v
[Cached Results] -- ParseResult, CheckResult per open file
```

### Test Runner Flow

```
Test File (.st with TEST_PROGRAM annotations)
    |
    v
[Test Discovery] -- finds TEST_PROGRAMs, extracts test metadata
    |
    v
[Compiler API] -- Parse + Check each test file
    |
    v
[Scan Cycle Engine]
    |
    +-- Setup: Initialize variables, configure mocks
    +-- Loop: Read Inputs -> Execute -> Write Outputs -> Advance Clock
    +-- Assert: Check output values against expected
    +-- Teardown: Report pass/fail
    |
    v
[JUnit XML Writer] -- produces test-results.xml for CI
    |
    v
[Console Reporter] -- human-readable test output
```

### Key Data Flows

1. **Source to diagnostics:** The primary flow for both CLI (`stc check`) and LSP. Source goes through preprocess -> lex -> parse -> resolve -> typecheck, collecting diagnostics at every stage. All diagnostics carry source spans for precise error reporting.

2. **Source to vendor ST:** Parse -> resolve -> typecheck -> vendor emit. The emitter walks the typed AST and produces vendor-flavored ST text. This is the "write once, deploy to Beckhoff/CODESYS" workflow.

3. **Source to test results:** Parse -> resolve -> typecheck -> interpret inside scan cycle engine -> assertions -> JUnit XML. The interpreter needs the full typed AST and symbol table to execute correctly.

4. **LSP incremental updates:** On file change, re-parse only that file. If the file's exports changed (new/removed/renamed declarations), update the cross-file index. Type checking re-runs on the changed file. Other files' cached results remain valid unless their imports changed.

## Build Order (Component Dependencies)

The dependency graph determines what must be built first:

```
Phase 1: Foundation
    source, diag, ast (node types only)
    No dependencies on each other beyond ast -> source for spans

Phase 2: Frontend Pipeline
    lexer -> source, ast
    parser -> lexer, ast, diag
    (Parser depends on lexer, produces AST nodes with diagnostics)

Phase 3: Semantic Analysis
    symbols -> ast, source
    resolve -> ast, symbols, diag
    checker -> ast, symbols, diag
    (Resolver depends on symbols; checker depends on resolver output)

Phase 4: Compiler API Facade
    compiler -> parser, resolve, checker, diag
    (Facade orchestrates the pipeline stages)

Phase 5: Backends (parallel, independent of each other)
    interp -> ast, symbols, checker (tree-walking interpreter)
    emit/cpp -> ast, symbols, checker (C++ code generation)
    emit/st -> ast, symbols, checker (vendor ST re-emission)
    emit/plcopen -> ast (PLCopen XML I/O)

Phase 6: Test Infrastructure
    interp/scan -> interp (scan cycle engine wraps interpreter)
    testrun -> compiler, interp/scan (test runner uses compiler + scan engine)

Phase 7: Consumers (parallel, independent of each other)
    CLI -> compiler, testrun, emit/* (thin wrapper)
    lsp -> compiler, index (LSP server)
    mcp -> CLI commands (MCP wraps CLI)

Phase 8: VS Code Extension (TypeScript, separate build)
    vscode-extension -> lsp (communicates via stdio)
```

**Critical path:** source -> ast -> lexer -> parser -> symbols -> resolver -> checker -> compiler facade. Everything else fans out from the compiler facade.

**What this means for roadmap phasing:**
1. Build the parser first (Phases 1-2). It unblocks everything else.
2. Semantic analysis (Phase 3) is the second critical milestone. Without it, no type checking, no meaningful LSP, no correct code generation.
3. Backends (Phase 5) can be built in parallel once the checker works. Start with the interpreter (enables test runner soonest).
4. LSP and CLI are consumers that wrap the compiler API. They can be built incrementally as compiler features land.
5. MCP and Skills are thin wrappers over CLI. Build last.

## Anti-Patterns

### Anti-Pattern 1: Compiler Logic in the CLI

**What people do:** Put parsing, type checking, or code generation logic directly in CLI command handlers.
**Why it's wrong:** LSP and MCP cannot reuse the logic. Testing requires spawning CLI processes. Adding new consumers requires duplicating code.
**Do this instead:** All compiler logic in `pkg/compiler/`. CLI is a thin wrapper that calls `compiler.Check()`, `compiler.Emit()`, etc.

### Anti-Pattern 2: Failing on First Error

**What people do:** Parser returns `error` on first syntax error and produces no AST.
**Why it's wrong:** LSP requires partial ASTs from broken code. Users want to see all errors, not just the first one. This is the most common mistake in hobby compiler projects.
**Do this instead:** Parser always produces an AST. Errors become `ErrorNode` entries in the AST and diagnostics in the result. Use the `(T, []Diagnostic)` pattern from rust-analyzer.

### Anti-Pattern 3: Monolithic AST Walker

**What people do:** One giant function that walks the AST and does resolution, type checking, and code generation in a single pass.
**Why it's wrong:** Cannot reuse individual phases. Cannot run just the parser for LSP syntax highlighting. Cannot type-check without also generating code.
**Do this instead:** Each semantic phase is a separate pass over the AST with its own input/output types. MATIEC does this with explicit stages (pre3, stage 3, stage 4).

### Anti-Pattern 4: Wall-Clock Time in Tests

**What people do:** Use `time.Now()` or `time.Sleep()` for timer function blocks (TON, TOF, TP) in the test runner.
**Why it's wrong:** Tests become non-deterministic, slow, and flaky. A TON with PT=T#5s should not take 5 real seconds to test.
**Do this instead:** Virtual clock that advances by the scan cycle time on each cycle. All timer FBs read from the virtual clock. Tests run at full speed.

### Anti-Pattern 5: Tight Coupling Between Parser and Type System

**What people do:** Parser tries to resolve types during parsing (e.g., looking up whether `MyType` is a struct or enum to decide how to parse).
**Why it's wrong:** Creates circular dependencies. Makes the parser depend on the symbol table, which may not be fully populated yet (forward declarations, cross-file references).
**Do this instead:** Parser produces unresolved AST. Name resolution is a separate pass after all files are parsed. This is what the Go compiler does (parsing is purely syntactic).

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| VS Code | LSP over stdio | TypeScript extension launches `stc-lsp` binary |
| LLM Agents | MCP over stdio | `stc-mcp` exposes compiler tools |
| CI Systems | CLI with `--format json` and JUnit XML | `stc test --junit results.xml` |
| Vendor IDEs | File-based (emit .st files, PLCopen XML) | No direct API; engineers paste/import output |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| CLI -> Compiler | Direct Go function calls | Same process, `pkg/compiler` API |
| LSP -> Compiler | Direct Go function calls | Same process, `pkg/compiler` API with caching layer |
| MCP -> CLI | Wraps CLI commands as MCP tools | Could also call compiler API directly |
| Compiler -> Backends | Visitor pattern over typed AST | Backends receive `CheckResult`, produce output |
| Test Runner -> Interpreter | Direct Go calls | Test runner orchestrates scan cycle engine |
| VS Code -> LSP | JSON-RPC over stdio | Standard LSP protocol |

## Comparison with Prior Art

| Aspect | MATIEC | STruCpp | RuSTy | STC (recommended) |
|--------|--------|---------|-------|---------------------|
| Language | C | TypeScript | Rust | Go |
| Parser | flex/bison (LALR) | Chevrotain (PEG-like) | logos + custom | Hand-written recursive descent |
| Error recovery | Limited | Unknown | Unknown | Full (partial ASTs, error nodes) |
| Output | ANSI C | C++17 | LLVM IR | Interpreter + C++ + Vendor ST |
| LSP | None | VS Code extension | None | Full LSP server |
| Test runner | None | Built-in | None | Built-in with scan cycle sim |
| IEC edition | Ed. 2 | Ed. 3 + CODESYS ext | Partial Ed. 3 | Ed. 3 + CODESYS ext |
| License | LGPL | GPL-3.0 | LGPL-3.0 | MIT |

**Key lessons from prior art:**
- **MATIEC** validates the 4-stage pipeline (lex -> parse -> semantic -> codegen) but its flex/bison approach limits error recovery and makes LSP integration impractical.
- **STruCpp** proves that ST-to-C++ transpilation works well and that 1400+ tests are achievable. Its Chevrotain parser is a reasonable approach but TypeScript limits deployment flexibility.
- **RuSTy** shows that a modern language + LLVM backend is viable, but LLVM is heavy for a development toolchain where the interpreter matters more.
- **rust-analyzer** provides the gold standard for LSP architecture: compiler-as-library, incremental analysis, error-tolerant parsing, layered crate design.
- **Go compiler** demonstrates clean phase separation in a Go codebase: syntax -> types2 -> ir -> ssa -> codegen. The package-per-phase pattern maps directly to STC's needs.

## Sources

- [MATIEC compiler stages and architecture](https://github.com/beremiz/matiec) - Stage 1-4 pipeline documentation
- [STruCpp GitHub repository](https://github.com/Autonomy-Logic/STruCpp) - TypeScript ST-to-C++17 compiler
- [RuSTy architecture documentation](https://plc-lang.github.io/rusty/arch/architecture.html) - Parser -> Indexer -> Linker -> Validation -> Codegen pipeline
- [Go compiler README](https://go.dev/src/cmd/compile/README) - 8-phase pipeline architecture
- [rust-analyzer architecture](https://rust-analyzer.github.io/book/contributing/architecture.html) - LSP integration patterns, incremental analysis
- [RuSTy PLC-lang GitHub](https://github.com/PLC-lang/rusty) - Rust-based ST compiler with LLVM backend
- [IEC 61131-3 Pragma directives](https://www.fernhillsoftware.com/help/iec-61131/common-elements/pragma.html) - Preprocessor directive specification

---
*Architecture research for: IEC 61131-3 Structured Text Compiler Toolchain*
*Researched: 2026-03-26*
