# Roadmap: STC — Structured Text Compiler Toolchain

## Overview

STC delivers a complete IEC 61131-3 Structured Text development toolchain in Go. The build order follows hard dependencies: the parser unlocks everything, semantic analysis enables all backends and tools, the interpreter and standard library enable host-based testing (the killer feature), and tooling layers (emission, formatter, linter, LSP) build on the proven compiler core. MCP and Claude Code skills wrap the stable CLI last. Eleven phases, derived from 86 v1 requirements across 16 categories, with fine granularity to preserve natural delivery boundaries.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Project Bootstrap & Parser** - Go project scaffold, hand-written lexer and recursive descent parser with error recovery, CLI foundation, JSON AST output
- [ ] **Phase 2: Preprocessor** - Conditional compilation directives, vendor defines, source maps for preprocessed output
- [ ] **Phase 3: Semantic Analysis** - Name resolution, two-pass type checking, symbol tables, cross-file analysis, vendor-aware diagnostics
- [ ] **Phase 4: Standard Library & Interpreter** - IEC standard FBs with deterministic time, tree-walking interpreter with scan cycle semantics
- [ ] **Phase 5: Testing Framework** - ST-native test syntax, test discovery and runner, JUnit XML output, I/O mocking, source debug mapping
- [ ] **Phase 6: Simulation** - Closed-loop simulation with sensor injection, simple plant models, deterministic replay
- [ ] **Phase 7: Multi-Vendor Emission** - Beckhoff and Schneider ST emitters, vendor profiles, round-trip stability, portable normalized output
- [ ] **Phase 8: Formatter & Linter** - Auto-formatting with configurable style, PLCopen coding guidelines linter, JSON diagnostics
- [ ] **Phase 9: LSP & VS Code Extension** - Full language server (diagnostics, go-to-def, hover, completion, rename, references), VS Code extension with syntax highlighting
- [ ] **Phase 10: Incremental Compilation** - File-level dependency tracking, cached symbol tables, re-analyze only changed files
- [ ] **Phase 11: MCP Server & Claude Code Skills** - MCP tool wrappers for all CLI commands, Claude Code skills for ST development workflows

## Phase Details

### Phase 1: Project Bootstrap & Parser
**Goal**: Users can parse any IEC 61131-3 Ed.3 ST source file (including CODESYS OOP extensions) and get a structured AST or actionable error messages via a single CLI binary
**Depends on**: Nothing (first phase)
**Requirements**: PARS-01, PARS-02, PARS-03, PARS-04, PARS-05, PARS-06, PARS-07, PARS-08, PARS-09, PARS-10, CLI-01, CLI-02, CLI-03, CLI-04, CLI-05
**Success Criteria** (what must be TRUE):
  1. User can run `stc parse <file>` on a ST file containing PROGRAM, FUNCTION_BLOCK, FUNCTION, TYPE, INTERFACE, METHOD, and PROPERTY declarations and get a JSON AST back
  2. User can run `stc parse` on broken/incomplete ST code and get a partial AST with error nodes plus actionable diagnostics showing file:line:col
  3. User can run `stc --version` and every subcommand supports `--format json`
  4. User can create an `stc.toml` project manifest defining source roots and vendor target
  5. Parser correctly handles CODESYS extensions (OOP, POINTER TO, REFERENCE TO, 64-bit types), all control structures, all VAR sections, arrays, structs, enums, pragmas
**Plans**: 5 plans

Plans:
- [x] 01-01-PLAN.md — Project bootstrap: Go module, CI, Makefile, foundation packages (source, diag, project)
- [x] 01-02-PLAN.md — AST node types: all CST nodes, JSON marshaling, visitor pattern, trivia support
- [x] 01-03-PLAN.md — Lexer: tokenizer with full keyword table, trivia, typed literals, nested comments
- [x] 01-04-PLAN.md — Parser: recursive descent with Pratt expressions, error recovery, all declarations/statements
- [x] 01-05-PLAN.md — CLI: Cobra binary with parse command, version, stubs, integration tests

### Phase 2: Preprocessor
**Goal**: Users can write vendor-portable ST using conditional compilation directives and get vendor-specific output with accurate source mapping
**Depends on**: Phase 1
**Requirements**: PREP-01, PREP-02, PREP-03, PREP-04, PREP-05
**Success Criteria** (what must be TRUE):
  1. User can use `{IF defined(VENDOR_BECKHOFF)}` / `{ELSIF}` / `{ELSE}` / `{END_IF}` directives in ST source to conditionally include vendor-specific code
  2. User can run `stc pp <file> --define VENDOR_BECKHOFF` and get preprocessed output with only the Beckhoff-specific paths included
  3. Preprocessor emits source maps so that downstream diagnostics reference original file:line:col, not preprocessed positions
**Plans**: 2 plans

Plans:
- [x] 02-01-PLAN.md — Preprocessor core: directive parser, condition evaluator, source map, Preprocess function
- [x] 02-02-PLAN.md — CLI pp command: --define flag, text/JSON output, integration tests

### Phase 3: Semantic Analysis
**Goal**: Users get type errors, undeclared variable warnings, and vendor-aware diagnostics with actionable messages before ever touching a PLC
**Depends on**: Phase 2
**Requirements**: SEMA-01, SEMA-02, SEMA-03, SEMA-04, SEMA-05, SEMA-06, SEMA-07
**Success Criteria** (what must be TRUE):
  1. User can run `stc check <files...>` and get type mismatch errors with file:line:col and clear fix suggestions
  2. Type checker correctly resolves all IEC primitive types, arrays, structs, enums, FB instances, and method calls across multiple files
  3. User gets warnings for undeclared variables, unused variables, and unreachable code
  4. User can pass `--vendor beckhoff` or `--vendor schneider` and get warnings when using constructs unsupported by that vendor
  5. `stc check --format json` outputs machine-readable diagnostics for CI integration
**Plans**: 5 plans

Plans:
- [x] 03-01-PLAN.md — Type system: IEC type lattice, widening rules, built-in type constants and function signatures
- [x] 03-02-PLAN.md — Symbol table: hierarchical scope chain, case-insensitive lookup, POU registry
- [x] 03-03-PLAN.md — Two-pass checker: declaration resolution (pass 1) and expression/statement type checking (pass 2)
- [x] 03-04-PLAN.md — Vendor profiles and usage analysis: vendor-aware warnings, unused variables, unreachable code
- [x] 03-05-PLAN.md — Analyzer facade and CLI check command: cross-file orchestration, text/JSON output

### Phase 4: Standard Library & Interpreter
**Goal**: Users can execute ST programs on their development machine with correct PLC scan-cycle semantics and IEC standard library support, no hardware required
**Depends on**: Phase 3
**Requirements**: STLB-01, STLB-02, STLB-03, STLB-04, STLB-05, STLB-06, STLB-07, STLB-08, INTP-01, INTP-02, INTP-03, INTP-04
**Success Criteria** (what must be TRUE):
  1. User can execute a ST program containing TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, SR, RS function blocks with correct IEC semantics
  2. Interpreter runs programs with scan-cycle semantics (read inputs, execute, write outputs) and deterministic time advancement (no wall-clock dependency)
  3. User can programmatically set inputs and read outputs on the interpreter for testing scenarios
  4. All standard math, string, and type conversion functions work correctly (ABS, SQRT, LEN, CONCAT, INT_TO_REAL, etc.)
  5. Standard library FBs accept injected time for deterministic test execution
**Plans**: 4 plans

Plans:
- [x] 04-01-PLAN.md — Interpreter core: Value type, Env scoping, expression/statement evaluation engine
- [x] 04-02-PLAN.md — Scan cycle engine: FB instance management, Tick(dt), I/O table, deterministic clock
- [x] 04-03-PLAN.md — Standard library functions: math, string (1-based indexing), type conversion (banker's rounding)
- [x] 04-04-PLAN.md — Standard library FBs (timers, counters, edge, bistable) and integration wiring

### Phase 5: Testing Framework
**Goal**: Users can write unit tests for ST code in ST syntax, run them on their machine, and integrate results into CI pipelines
**Depends on**: Phase 4
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06, TEST-07, DBUG-01, DBUG-02
**Success Criteria** (what must be TRUE):
  1. User can write tests using TEST_CASE / ASSERT_EQ / ASSERT_TRUE / ASSERT_NEAR / ASSERT_FALSE in ST files
  2. User can run `stc test <dir>` and all test files are discovered and executed automatically
  3. Test failures reference original ST file:line, not internal representation, with clear assertion messages
  4. Test runner outputs JUnit XML (`--format junit`) and JSON (`--format json`) for CI integration and returns non-zero exit code on failure
  5. Tests support I/O mocking (inject inputs, read outputs) and deterministic time advancement (ADVANCE_TIME)
**Plans**: 2 plans

Plans:
- [ ] 05-01-PLAN.md — Lexer/parser/AST extensions for TEST_CASE, assertion functions, ADVANCE_TIME, source position tracking
- [ ] 05-02-PLAN.md — Test runner package (discovery, execution, JUnit XML, JSON output) and CLI stc test command

### Phase 6: Simulation
**Goal**: Users can run closed-loop simulations of their ST programs with simulated sensors and actuators for integration testing without hardware
**Depends on**: Phase 5
**Requirements**: SIM-01, SIM-02, SIM-03
**Success Criteria** (what must be TRUE):
  1. User can define a simulation with sensor waveforms (ramp, sine, step) injected into program inputs and observe program behavior over time
  2. User can define simple plant models (motor with inertia, valve with flow dynamics, cylinder with position) that respond to program outputs
  3. Simulations are fully deterministic and replayable — running the same simulation twice produces identical results for regression testing
**Plans**: TBD

Plans:
- [ ] 06-01: TBD

### Phase 7: Multi-Vendor Emission
**Goal**: Users can write ST once and emit vendor-flavored output for Beckhoff TwinCAT and Schneider/CODESYS targets, ready to paste into vendor IDEs
**Depends on**: Phase 3
**Requirements**: EMIT-01, EMIT-02, EMIT-03, EMIT-04, EMIT-05
**Success Criteria** (what must be TRUE):
  1. User can run `stc emit <file> --target beckhoff` and get Beckhoff-flavored ST with correct pragma/attribute syntax
  2. User can run `stc emit <file> --target schneider` and get Schneider/CODESYS-flavored ST output
  3. Round-trip stability: parse then emit then parse then emit produces identical output
  4. User can run `stc emit <file> --target portable` to get clean normalized ST stripped of vendor-specific constructs
**Plans**: TBD

Plans:
- [ ] 07-01: TBD
- [ ] 07-02: TBD

### Phase 8: Formatter & Linter
**Goal**: Users can auto-format ST code to a consistent style and check it against coding standards, with no commercial tool dependency
**Depends on**: Phase 3
**Requirements**: FMT-01, FMT-02, FMT-03, LINT-01, LINT-02, LINT-03, LINT-04
**Success Criteria** (what must be TRUE):
  1. User can run `stc fmt <file>` and get consistently formatted ST code (indentation, keyword casing, spacing) with comments preserved
  2. Formatter style is configurable (indent style, casing conventions) via stc.toml or command-line flags
  3. User can run `stc lint <files...>` and get coding standard violations (PLCopen guidelines, naming conventions) with JSON output
  4. Linter naming conventions are configurable per project
**Plans**: TBD

Plans:
- [ ] 08-01: TBD
- [ ] 08-02: TBD

### Phase 9: LSP & VS Code Extension
**Goal**: Users get a modern IDE experience for ST development in VS Code with real-time diagnostics, navigation, and refactoring
**Depends on**: Phase 3, Phase 8
**Requirements**: LSP-01, LSP-02, LSP-03, LSP-04, LSP-05, LSP-06, LSP-07, LSP-08
**Success Criteria** (what must be TRUE):
  1. User sees real-time diagnostics (parser errors + type errors) in VS Code as they type, without saving
  2. User can go-to-definition on any variable, FB, or method and jump to its declaration
  3. User can hover over any symbol and see its type information; completions suggest keywords, types, declared variables, and FB members
  4. User can rename a symbol and all references update across files; find-references shows all usages
  5. Inactive preprocessor blocks are grayed out via semantic tokens in the editor
**Plans**: TBD
**UI hint**: yes

Plans:
- [ ] 09-01: TBD
- [ ] 09-02: TBD
- [ ] 09-03: TBD

### Phase 10: Incremental Compilation
**Goal**: Users experience fast re-analysis on large multi-file ST projects because only changed files and their dependents are re-processed
**Depends on**: Phase 3
**Requirements**: INCR-01, INCR-02
**Success Criteria** (what must be TRUE):
  1. After changing one file in a multi-file project, `stc check` only re-analyzes that file and its dependents, not the entire project
  2. File-level dependency graph and cached symbol tables persist between invocations, reducing repeated work
**Plans**: TBD

Plans:
- [ ] 10-01: TBD

### Phase 11: MCP Server & Claude Code Skills
**Goal**: LLM agents can parse, check, test, lint, format, and emit ST code through MCP tools, and Claude Code users get purpose-built skills for ST development workflows
**Depends on**: Phase 1 through Phase 8 (all CLI commands stable)
**Requirements**: MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, MCP-06, MCP-07, SKIL-01, SKIL-02, SKIL-03, SKIL-04, SKIL-05, SKIL-06
**Success Criteria** (what must be TRUE):
  1. LLM agent can call MCP tools (stc_parse, stc_check, stc_test, stc_emit, stc_lint, stc_format) and get structured JSON responses
  2. All MCP tool descriptions are under 100 tokens each for minimal agent context consumption
  3. Claude Code skills auto-invoke when working with .st files and cover the full lifecycle: generate, validate, test, emit, and review ST code
  4. Skills chain CLI commands correctly (e.g., validate skill runs parse + check + lint pipeline)
**Plans**: TBD

Plans:
- [ ] 11-01: TBD
- [ ] 11-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8 -> 9 -> 10 -> 11

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Project Bootstrap & Parser | 5/5 | Complete | 2026-03-26 |
| 2. Preprocessor | 2/2 | Complete | 2026-03-26 |
| 3. Semantic Analysis | 0/5 | Planning complete | - |
| 4. Standard Library & Interpreter | 0/4 | Planning complete | - |
| 5. Testing Framework | 0/2 | Planning complete | - |
| 6. Simulation | 0/1 | Not started | - |
| 7. Multi-Vendor Emission | 0/2 | Not started | - |
| 8. Formatter & Linter | 0/2 | Not started | - |
| 9. LSP & VS Code Extension | 0/3 | Not started | - |
| 10. Incremental Compilation | 0/1 | Not started | - |
| 11. MCP Server & Claude Code Skills | 0/2 | Not started | - |
