# Requirements: STC — Structured Text Compiler Toolchain

**Defined:** 2026-03-26
**Core Value:** Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor — no hardware required for development and testing.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Parsing

- [x] **PARS-01**: User can parse IEC 61131-3 Ed.3 ST source files (PROGRAM, FUNCTION_BLOCK, FUNCTION, TYPE declarations)
- [x] **PARS-02**: Parser handles CODESYS OOP extensions (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS)
- [x] **PARS-03**: Parser handles CODESYS pointer/reference types (POINTER TO, REFERENCE TO)
- [x] **PARS-04**: Parser handles 64-bit types (LINT, LREAL, LWORD, ULINT)
- [x] **PARS-05**: Parser produces partial ASTs from broken code with error nodes (error recovery for LSP)
- [x] **PARS-06**: Parser outputs AST as JSON via `stc parse <file> --format json`
- [x] **PARS-07**: Parser handles all ST control structures (IF/CASE/FOR/WHILE/REPEAT) with full operator precedence
- [x] **PARS-08**: Parser handles VAR sections (VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_TEMP, VAR_GLOBAL)
- [x] **PARS-09**: Parser handles arrays, structs, enums, subranges, typed literals
- [x] **PARS-10**: Parser handles {attribute '...'} pragmas (Beckhoff-style)

### Preprocessor

- [x] **PREP-01**: User can use {IF defined(NAME)}, {ELSIF}, {ELSE}, {END_IF} for conditional compilation
- [x] **PREP-02**: User can use {DEFINE NAME} for file-local definitions
- [x] **PREP-03**: User can use {ERROR "message"} to emit compile errors for unsupported vendor paths
- [x] **PREP-04**: Preprocessor emits source maps (original line:col → preprocessed line:col)
- [x] **PREP-05**: CLI command `stc pp <file> --define VENDOR_BECKHOFF` emits vendor-specific output

### Type Checking & Semantic Analysis

- [x] **SEMA-01**: User gets type mismatch errors with file:line:col and actionable messages
- [x] **SEMA-02**: Type checker resolves all IEC primitive types (BOOL, BYTE, WORD, DWORD, SINT, INT, DINT, LINT, USINT, UINT, UDINT, ULINT, REAL, LREAL, STRING, WSTRING, TIME, DATE, DT, TOD)
- [x] **SEMA-03**: Type checker handles arrays, structs, enums, FB instances, method calls
- [x] **SEMA-04**: Type checker detects undeclared variables, unused variables, unreachable code
- [x] **SEMA-05**: Type checker handles cross-file symbol resolution (multi-file projects)
- [x] **SEMA-06**: CLI command `stc check <files...> --format json` outputs diagnostics
- [x] **SEMA-07**: Vendor-aware diagnostics warn when using constructs unsupported by target vendor

### Incremental Compilation

- [ ] **INCR-01**: Only re-analyze changed files and their dependents on subsequent runs
- [ ] **INCR-02**: File-level dependency tracking with cached symbol tables

### Standard Library

- [x] **STLB-01**: Timers implemented with correct IEC semantics (TON, TOF, TP)
- [x] **STLB-02**: Counters implemented with correct IEC semantics (CTU, CTD, CTUD)
- [x] **STLB-03**: Edge detection implemented (R_TRIG, F_TRIG)
- [x] **STLB-04**: Bistable FBs implemented (SR, RS)
- [x] **STLB-05**: Standard math functions implemented (ABS, SQRT, MIN, MAX, SEL, MUX, LIMIT, etc.)
- [x] **STLB-06**: Standard string functions implemented (LEN, LEFT, RIGHT, MID, CONCAT, FIND, etc.)
- [x] **STLB-07**: Standard type conversion functions implemented (INT_TO_REAL, BOOL_TO_INT, etc.)
- [x] **STLB-08**: All standard library FBs support deterministic time injection for testing

### Interpreter & Host Execution

- [x] **INTP-01**: Interpreter executes typed AST with scan-cycle semantics (read inputs → execute → write outputs)
- [x] **INTP-02**: Interpreter supports deterministic time advancement (no wall-clock dependency)
- [x] **INTP-03**: Interpreter can set/get inputs and outputs programmatically for testing
- [x] **INTP-04**: Interpreter handles all ST control structures, expressions, and FB instance calls

### Testing

- [x] **TEST-01**: User can write tests in ST using TEST_CASE / ASSERT_EQ / ASSERT_TRUE / ASSERT_NEAR / ASSERT_FALSE
- [x] **TEST-02**: CLI command `stc test <dir>` discovers and runs test files
- [x] **TEST-03**: Test runner outputs JUnit XML for CI integration
- [x] **TEST-04**: Test runner supports JSON output (`--format json`)
- [x] **TEST-05**: Tests support I/O mocking (inject input values, read output values)
- [x] **TEST-06**: Tests support deterministic time advancement (ADVANCE_TIME or equivalent)
- [x] **TEST-07**: Test runner returns non-zero exit code on failure

### Simulation

- [x] **SIM-01**: User can run closed-loop simulations injecting sensor waveforms
- [x] **SIM-02**: User can define simple plant models (motor, valve, cylinder behavior)
- [ ] **SIM-03**: Simulations are deterministic and replayable for regression testing

### Developer Experience — LSP

- [ ] **LSP-01**: LSP server provides real-time diagnostics (errors from parser + type checker)
- [ ] **LSP-02**: LSP server provides go-to-definition for variables, FBs, methods
- [ ] **LSP-03**: LSP server provides hover showing type information
- [ ] **LSP-04**: LSP server provides completion for keywords, types, declared variables, FB members
- [ ] **LSP-05**: LSP server provides find-references
- [ ] **LSP-06**: LSP server provides rename refactoring
- [ ] **LSP-07**: LSP server grays out inactive preprocessor blocks via semantic tokens
- [ ] **LSP-08**: VS Code extension launches stc-lsp binary and provides syntax highlighting

### Developer Experience — Formatter & Linter

- [ ] **FMT-01**: CLI command `stc fmt <file>` auto-formats ST code (indentation, keyword casing, spacing)
- [ ] **FMT-02**: Formatter is configurable (indent style, casing conventions)
- [ ] **FMT-03**: Formatter preserves comments correctly
- [ ] **LINT-01**: CLI command `stc lint <files...>` reports coding standard violations
- [ ] **LINT-02**: Linter checks PLCopen coding guidelines
- [ ] **LINT-03**: Linter checks naming conventions (configurable)
- [ ] **LINT-04**: Linter reports with JSON output (`--format json`)

### Multi-Vendor Emission

- [ ] **EMIT-01**: CLI command `stc emit <file> --target beckhoff` produces Beckhoff-flavored ST
- [ ] **EMIT-02**: CLI command `stc emit <file> --target schneider` produces Schneider-flavored ST
- [ ] **EMIT-03**: Emitters handle pragma/attribute differences between vendors
- [ ] **EMIT-04**: Round-trip stability: parse → emit → parse → emit produces identical output
- [ ] **EMIT-05**: CLI command `stc emit <file> --target portable` produces clean normalized ST

### CLI & Project

- [x] **CLI-01**: Single binary `stc` with subcommands: parse, check, test, emit, lint, fmt, pp
- [x] **CLI-02**: Every subcommand supports `--format json` for machine-readable output
- [x] **CLI-03**: `stc --version` outputs version information
- [x] **CLI-04**: Project manifest (stc.toml) defines source roots, vendor target, library paths
- [x] **CLI-05**: All diagnostics include file:line:col with actionable error messages

### MCP Server

- [ ] **MCP-01**: MCP server exposes stc_parse tool (parse ST, return AST or diagnostics)
- [ ] **MCP-02**: MCP server exposes stc_check tool (type check, return diagnostics)
- [ ] **MCP-03**: MCP server exposes stc_test tool (run tests, return results)
- [ ] **MCP-04**: MCP server exposes stc_emit tool (emit vendor ST)
- [ ] **MCP-05**: MCP server exposes stc_lint tool (lint, return suggestions)
- [ ] **MCP-06**: MCP server exposes stc_format tool (format ST code)
- [ ] **MCP-07**: All MCP tool descriptions are under 100 tokens each for minimal agent context

### Claude Code Skills

- [ ] **SKIL-01**: Skill for generating ST code from natural language description
- [ ] **SKIL-02**: Skill for validating ST code (parse + check + lint pipeline)
- [ ] **SKIL-03**: Skill for writing and running ST unit tests
- [ ] **SKIL-04**: Skill for emitting vendor-specific ST from portable source
- [ ] **SKIL-05**: Skill for reviewing ST code against IEC best practices
- [ ] **SKIL-06**: Skills auto-invoke when working with .st files in the stc project

### Source Debug Mapping

- [x] **DBUG-01**: Source maps from original ST lines to interpreter execution points
- [x] **DBUG-02**: Test failure messages reference original ST file:line, not internal representation

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Allen Bradley

- **AB-01**: CLI command `stc emit <file> --target allen_bradley` produces AB-compatible ST
- **AB-02**: AB vendor profile restricts to AB-supported ST subset (no OOP, no pointers)
- **AB-03**: Tag-based variable model mapping for AB output

### Advanced Execution

- **EXEC-01**: LLVM backend for native compilation (if interpreter proves insufficient)

## Out of Scope

| Feature | Reason |
|---------|--------|
| C++ transpiler | Interpreter-only execution model — decided by project owner |
| PLCopen XML import/export | Not needed — vendor interop through preprocessor ifdefs and ST re-emission |
| Full PLC runtime | Would compete with OpenPLC; test runner with simulated scan cycles instead |
| GUI / Visual IDE | Massive scope; CLI + LSP for VS Code; engineers keep vendor IDE for visual work |
| Ladder Diagram / FBD editing | Graphical languages need graphical editor; focus on ST as textual language |
| Cloud IDE / SaaS | Splits focus; industrial customers often have air-gapped networks |
| Automated PLC deployment | Safety-critical domain; human-in-the-loop for deployment is a feature |
| LLVM backend in v1 | Interpreter is sufficient; LLVM only if concrete performance need arises |
| Java runtime dependency | Parser and compiler must be pure Go — no Java at runtime |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PARS-01 | Phase 1 | Complete |
| PARS-02 | Phase 1 | Complete |
| PARS-03 | Phase 1 | Complete |
| PARS-04 | Phase 1 | Complete |
| PARS-05 | Phase 1 | Complete |
| PARS-06 | Phase 1 | Complete |
| PARS-07 | Phase 1 | Complete |
| PARS-08 | Phase 1 | Complete |
| PARS-09 | Phase 1 | Complete |
| PARS-10 | Phase 1 | Complete |
| CLI-01 | Phase 1 | Complete |
| CLI-02 | Phase 1 | Complete |
| CLI-03 | Phase 1 | Complete |
| CLI-04 | Phase 1 | Complete |
| CLI-05 | Phase 1 | Complete |
| PREP-01 | Phase 2 | Complete |
| PREP-02 | Phase 2 | Complete |
| PREP-03 | Phase 2 | Complete |
| PREP-04 | Phase 2 | Complete |
| PREP-05 | Phase 2 | Complete |
| SEMA-01 | Phase 3 | Complete |
| SEMA-02 | Phase 3 | Complete |
| SEMA-03 | Phase 3 | Complete |
| SEMA-04 | Phase 3 | Complete |
| SEMA-05 | Phase 3 | Complete |
| SEMA-06 | Phase 3 | Complete |
| SEMA-07 | Phase 3 | Complete |
| STLB-01 | Phase 4 | Complete |
| STLB-02 | Phase 4 | Complete |
| STLB-03 | Phase 4 | Complete |
| STLB-04 | Phase 4 | Complete |
| STLB-05 | Phase 4 | Complete |
| STLB-06 | Phase 4 | Complete |
| STLB-07 | Phase 4 | Complete |
| STLB-08 | Phase 4 | Complete |
| INTP-01 | Phase 4 | Complete |
| INTP-02 | Phase 4 | Complete |
| INTP-03 | Phase 4 | Complete |
| INTP-04 | Phase 4 | Complete |
| TEST-01 | Phase 5 | Complete |
| TEST-02 | Phase 5 | Complete |
| TEST-03 | Phase 5 | Complete |
| TEST-04 | Phase 5 | Complete |
| TEST-05 | Phase 5 | Complete |
| TEST-06 | Phase 5 | Complete |
| TEST-07 | Phase 5 | Complete |
| DBUG-01 | Phase 5 | Complete |
| DBUG-02 | Phase 5 | Complete |
| SIM-01 | Phase 6 | Complete |
| SIM-02 | Phase 6 | Complete |
| SIM-03 | Phase 6 | Pending |
| EMIT-01 | Phase 7 | Pending |
| EMIT-02 | Phase 7 | Pending |
| EMIT-03 | Phase 7 | Pending |
| EMIT-04 | Phase 7 | Pending |
| EMIT-05 | Phase 7 | Pending |
| FMT-01 | Phase 8 | Pending |
| FMT-02 | Phase 8 | Pending |
| FMT-03 | Phase 8 | Pending |
| LINT-01 | Phase 8 | Pending |
| LINT-02 | Phase 8 | Pending |
| LINT-03 | Phase 8 | Pending |
| LINT-04 | Phase 8 | Pending |
| LSP-01 | Phase 9 | Pending |
| LSP-02 | Phase 9 | Pending |
| LSP-03 | Phase 9 | Pending |
| LSP-04 | Phase 9 | Pending |
| LSP-05 | Phase 9 | Pending |
| LSP-06 | Phase 9 | Pending |
| LSP-07 | Phase 9 | Pending |
| LSP-08 | Phase 9 | Pending |
| INCR-01 | Phase 10 | Pending |
| INCR-02 | Phase 10 | Pending |
| MCP-01 | Phase 11 | Pending |
| MCP-02 | Phase 11 | Pending |
| MCP-03 | Phase 11 | Pending |
| MCP-04 | Phase 11 | Pending |
| MCP-05 | Phase 11 | Pending |
| MCP-06 | Phase 11 | Pending |
| MCP-07 | Phase 11 | Pending |
| SKIL-01 | Phase 11 | Pending |
| SKIL-02 | Phase 11 | Pending |
| SKIL-03 | Phase 11 | Pending |
| SKIL-04 | Phase 11 | Pending |
| SKIL-05 | Phase 11 | Pending |
| SKIL-06 | Phase 11 | Pending |

**Coverage:**
- v1 requirements: 86 total
- Mapped to phases: 86
- Unmapped: 0

---
*Requirements defined: 2026-03-26*
*Last updated: 2026-03-26 after roadmap creation*
