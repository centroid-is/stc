# Roadmap: STC -- Structured Text Compiler Toolchain

## Milestones

- [x] **v1.0 MVP** - Phases 1-11 (shipped 2026-03-28)
- [ ] **v1.1 Vendor Libraries & I/O** - Phases 12-18 (in progress)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>v1.0 MVP (Phases 1-11) - SHIPPED 2026-03-28</summary>

- [x] **Phase 1: Project Bootstrap & Parser** - Go project scaffold, hand-written lexer and recursive descent parser with error recovery, CLI foundation, JSON AST output
- [x] **Phase 2: Preprocessor** - Conditional compilation directives, vendor defines, source maps for preprocessed output
- [x] **Phase 3: Semantic Analysis** - Name resolution, two-pass type checking, symbol tables, cross-file analysis, vendor-aware diagnostics
- [x] **Phase 4: Standard Library & Interpreter** - IEC standard FBs with deterministic time, tree-walking interpreter with scan cycle semantics
- [x] **Phase 5: Testing Framework** - ST-native test syntax, test discovery and runner, JUnit XML output, I/O mocking, source debug mapping
- [x] **Phase 6: Simulation** - Closed-loop simulation with sensor injection, simple plant models, deterministic replay
- [x] **Phase 7: Multi-Vendor Emission** - Beckhoff and Schneider ST emitters, vendor profiles, round-trip stability, portable normalized output
- [x] **Phase 8: Formatter & Linter** - Auto-formatting with configurable style, PLCopen coding guidelines linter, JSON diagnostics
- [x] **Phase 9: LSP & VS Code Extension** - Full language server (diagnostics, go-to-def, hover, completion, rename, references), VS Code extension with syntax highlighting
- [x] **Phase 10: Incremental Compilation** - File-level dependency tracking, cached symbol tables, re-analyze only changed files
- [x] **Phase 11: MCP Server & Claude Code Skills** - MCP tool wrappers for all CLI commands, Claude Code skills for ST development workflows

</details>

### v1.1 Vendor Libraries & I/O

- [ ] **Phase 12: I/O Address Parser & Table** - AT address syntax in parser, mock I/O table in interpreter, scan-cycle I/O sync, overlap detection
- [ ] **Phase 13: Vendor Stub Loading** - .st stub file loading, library_paths config, type resolution from stubs, LSP support for vendor FBs, single-vendor enforcement
- [ ] **Phase 14: Mock Framework** - ST mock FBs overriding stubs, mock_paths config, auto-generated zero-value instances, signature validation, test I/O injection
- [ ] **Phase 15: Shipped Stubs -- Beckhoff** - Tc2_MC2, Tc2_System, Tc2_Utilities, Tc3_EventLogger stubs, common types, EtherCAT I/O examples
- [ ] **Phase 16: Shipped Stubs -- Schneider & Allen Bradley** - Schneider motion/comm/system stubs, AB type-check profile, AB timers, AB common instructions
- [ ] **Phase 17: Behavioral Mocks** - Shipped behavioral mocks for MC_MoveAbsolute, MC_Power, MC_Home, MC_Stop, ADSREAD with simulated behavior
- [ ] **Phase 18: Auto-Defines & TcPOU Extractor** - STC_TEST/STC_SIM auto-define, stc vendor extract command for TwinCAT project files

## Phase Details

<details>
<summary>v1.0 MVP Phase Details (Phases 1-11)</summary>

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
- [x] 01-01-PLAN.md -- Project bootstrap: Go module, CI, Makefile, foundation packages (source, diag, project)
- [x] 01-02-PLAN.md -- AST node types: all CST nodes, JSON marshaling, visitor pattern, trivia support
- [x] 01-03-PLAN.md -- Lexer: tokenizer with full keyword table, trivia, typed literals, nested comments
- [x] 01-04-PLAN.md -- Parser: recursive descent with Pratt expressions, error recovery, all declarations/statements
- [x] 01-05-PLAN.md -- CLI: Cobra binary with parse command, version, stubs, integration tests

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
- [x] 02-01-PLAN.md -- Preprocessor core: directive parser, condition evaluator, source map, Preprocess function
- [x] 02-02-PLAN.md -- CLI pp command: --define flag, text/JSON output, integration tests

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
- [x] 03-01-PLAN.md -- Type system: IEC type lattice, widening rules, built-in type constants and function signatures
- [x] 03-02-PLAN.md -- Symbol table: hierarchical scope chain, case-insensitive lookup, POU registry
- [x] 03-03-PLAN.md -- Two-pass checker: declaration resolution (pass 1) and expression/statement type checking (pass 2)
- [x] 03-04-PLAN.md -- Vendor profiles and usage analysis: vendor-aware warnings, unused variables, unreachable code
- [x] 03-05-PLAN.md -- Analyzer facade and CLI check command: cross-file orchestration, text/JSON output

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
- [x] 04-01-PLAN.md -- Interpreter core: Value type, Env scoping, expression/statement evaluation engine
- [x] 04-02-PLAN.md -- Scan cycle engine: FB instance management, Tick(dt), I/O table, deterministic clock
- [x] 04-03-PLAN.md -- Standard library functions: math, string (1-based indexing), type conversion (banker's rounding)
- [x] 04-04-PLAN.md -- Standard library FBs (timers, counters, edge, bistable) and integration wiring

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
- [x] 05-01-PLAN.md -- Lexer/parser/AST extensions for TEST_CASE, assertion functions, ADVANCE_TIME, source position tracking
- [x] 05-02-PLAN.md -- Test runner package (discovery, execution, JUnit XML, JSON output) and CLI stc test command

### Phase 6: Simulation
**Goal**: Users can run closed-loop simulations of their ST programs with simulated sensors and actuators for integration testing without hardware
**Depends on**: Phase 5
**Requirements**: SIM-01, SIM-02, SIM-03
**Success Criteria** (what must be TRUE):
  1. User can define a simulation with sensor waveforms (ramp, sine, step) injected into program inputs and observe program behavior over time
  2. User can define simple plant models (motor with inertia, valve with flow dynamics, cylinder with position) that respond to program outputs
  3. Simulations are fully deterministic and replayable -- running the same simulation twice produces identical results for regression testing
**Plans**: 2 plans

Plans:
- [x] 06-01-PLAN.md -- Waveform generators (Step, Ramp, Sine, Square) and plant models (Motor, Valve, Cylinder)
- [x] 06-02-PLAN.md -- Simulation engine with closed-loop feedback and CLI stc sim command

### Phase 7: Multi-Vendor Emission
**Goal**: Users can write ST once and emit vendor-flavored output for Beckhoff TwinCAT and Schneider/CODESYS targets, ready to paste into vendor IDEs
**Depends on**: Phase 3
**Requirements**: EMIT-01, EMIT-02, EMIT-03, EMIT-04, EMIT-05
**Success Criteria** (what must be TRUE):
  1. User can run `stc emit <file> --target beckhoff` and get Beckhoff-flavored ST with correct pragma/attribute syntax
  2. User can run `stc emit <file> --target schneider` and get Schneider/CODESYS-flavored ST output
  3. Round-trip stability: parse then emit then parse then emit produces identical output
  4. User can run `stc emit <file> --target portable` to get clean normalized ST stripped of vendor-specific constructs
**Plans**: 2 plans

Plans:
- [x] 07-01-PLAN.md -- Core emitter package: AST-to-ST printer with vendor profiles, round-trip tests
- [x] 07-02-PLAN.md -- CLI stc emit command: --target flag, text/JSON output, integration tests

### Phase 8: Formatter & Linter
**Goal**: Users can auto-format ST code to a consistent style and check it against coding standards, with no commercial tool dependency
**Depends on**: Phase 3
**Requirements**: FMT-01, FMT-02, FMT-03, LINT-01, LINT-02, LINT-03, LINT-04
**Success Criteria** (what must be TRUE):
  1. User can run `stc fmt <file>` and get consistently formatted ST code (indentation, keyword casing, spacing) with comments preserved
  2. Formatter style is configurable (indent style, casing conventions) via stc.toml or command-line flags
  3. User can run `stc lint <files...>` and get coding standard violations (PLCopen guidelines, naming conventions) with JSON output
  4. Linter naming conventions are configurable per project
**Plans**: 3 plans

Plans:
- [x] 08-01-PLAN.md -- Formatter package (pkg/format) with configurable style, comment preservation, idempotency, and CLI stc fmt command
- [x] 08-02-PLAN.md -- Linter package (pkg/lint) with PLCopen rules, naming conventions, and CLI stc lint command
- [x] 08-03-PLAN.md -- Gap closure: parser trivia attachment so parse->format round-trips preserve comments (FMT-03)

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
**Plans**: 3 plans
**UI hint**: yes

Plans:
- [x] 09-01-PLAN.md -- LSP server core: GLSP setup, document sync, real-time diagnostics, formatting, stc lsp CLI command
- [x] 09-02-PLAN.md -- Navigation and refactoring: go-to-definition, hover, completion, find-references, rename
- [x] 09-03-PLAN.md -- Semantic tokens for preprocessor blocks and VS Code extension with TextMate grammar

### Phase 10: Incremental Compilation
**Goal**: Users experience fast re-analysis on large multi-file ST projects because only changed files and their dependents are re-processed
**Depends on**: Phase 3
**Requirements**: INCR-01, INCR-02
**Success Criteria** (what must be TRUE):
  1. After changing one file in a multi-file project, `stc check` only re-analyzes that file and its dependents, not the entire project
  2. File-level dependency graph and cached symbol tables persist between invocations, reducing repeated work
**Plans**: 2 plans

Plans:
- [x] 10-01-PLAN.md -- Dependency graph, per-file symbol purge, and on-disk file cache infrastructure
- [x] 10-02-PLAN.md -- Incremental analyzer facade with CLI check and LSP integration

### Phase 11: MCP Server & Claude Code Skills
**Goal**: LLM agents can parse, check, test, lint, format, and emit ST code through MCP tools, and Claude Code users get purpose-built skills for ST development workflows
**Depends on**: Phase 1 through Phase 8 (all CLI commands stable)
**Requirements**: MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, MCP-06, MCP-07, SKIL-01, SKIL-02, SKIL-03, SKIL-04, SKIL-05, SKIL-06
**Success Criteria** (what must be TRUE):
  1. LLM agent can call MCP tools (stc_parse, stc_check, stc_test, stc_emit, stc_lint, stc_format) and get structured JSON responses
  2. All MCP tool descriptions are under 100 tokens each for minimal agent context consumption
  3. Claude Code skills auto-invoke when working with .st files and cover the full lifecycle: generate, validate, test, emit, and review ST code
  4. Skills chain CLI commands correctly (e.g., validate skill runs parse + check + lint pipeline)
**Plans**: 2 plans

Plans:
- [x] 11-01-PLAN.md -- MCP server binary (cmd/stc-mcp) with 6 tool handlers wrapping pkg/ functions directly
- [x] 11-02-PLAN.md -- Claude Code skills (.claude/skills/) for generate, validate, test, emit, and review workflows

</details>

### Phase 12: I/O Address Parser & Table
**Goal**: Users can declare AT-addressed variables in ST code and have them mapped to a mock I/O table that behaves like a real PLC I/O image during interpretation
**Depends on**: Phase 4 (interpreter with scan cycle engine)
**Requirements**: IO-01, IO-02, IO-03, IO-05
**Success Criteria** (what must be TRUE):
  1. User can declare variables with AT %IX0.0, %QX0.0, %IW0, %QW0, %MW0, %MD0 addresses in VAR blocks and they parse without errors
  2. Interpreter maintains three flat byte arrays (%I, %Q, %M) and AT-addressed variables read from and write to correct byte offsets
  3. I/O values sync at scan cycle boundaries -- inputs copied before execution, outputs copied after, matching real PLC behavior
  4. User gets a warning when AT addresses overlap (e.g., %IW0 and %IX0.3 referencing the same byte range)
**Plans**: 2 plans

Plans:
- [x] 12-01-PLAN.md -- IOMap package (address parser, IOTable data structure) and lexer DirectAddr token
- [x] 12-02-PLAN.md -- ScanCycleEngine IOTable integration, checker AT validation, overlap detection

### Phase 13: Vendor Stub Loading
**Goal**: Users can type-check and navigate production ST code that references vendor-specific function blocks by loading .st stub files with declarations
**Depends on**: Phase 12
**Requirements**: VLIB-01, VLIB-02, VLIB-03, VLIB-04, VLIB-05
**Success Criteria** (what must be TRUE):
  1. User can create .st stub files containing FUNCTION_BLOCK declarations without bodies and have them recognized as valid type definitions
  2. User can configure `[build.library_paths]` in stc.toml to point at vendor stub directories and `stc check` resolves FB types from those stubs
  3. `stc check` validates input/output parameter usage against stub signatures -- wrong parameter names or types produce errors
  4. LSP provides completion, hover, and go-to-definition for vendor FB inputs and outputs loaded from stubs
  5. When project targets one vendor, stubs from other vendors produce warnings about cross-vendor usage
**Plans**: TBD

### Phase 14: Mock Framework
**Goal**: Users can test ST code that depends on vendor FBs by writing ST mock implementations or relying on auto-generated zero-value stubs
**Depends on**: Phase 12, Phase 13
**Requirements**: MOCK-01, MOCK-02, MOCK-03, MOCK-04, MOCK-05, IO-04
**Success Criteria** (what must be TRUE):
  1. User can write a FUNCTION_BLOCK in a mock directory with the same name as a vendor stub and it overrides the stub during test execution
  2. User configures `[test.mock_paths]` in stc.toml and mock FBs are loaded from those paths during `stc test`
  3. Vendor FBs without explicit mocks auto-generate zero-value instances that accept inputs and return zeros, with fidelity warnings in test output
  4. Mock FB signatures are validated against stub signatures -- parameter count or type mismatches produce errors before test execution
  5. Tests can inject I/O values into the mock I/O table before assertions to simulate sensor inputs and verify actuator outputs
**Plans**: TBD

### Phase 15: Shipped Stubs -- Beckhoff
**Goal**: Users targeting Beckhoff TwinCAT can immediately type-check code using common Beckhoff libraries without writing their own stubs
**Depends on**: Phase 13
**Requirements**: STUB-01, STUB-02, STUB-03, STUB-04, STUB-05, STUB-06
**Success Criteria** (what must be TRUE):
  1. User can reference MC_Power, MC_MoveAbsolute, MC_MoveRelative, MC_Stop, MC_Home, and other Tc2_MC2 FBs and `stc check` validates parameter usage
  2. User can reference ADSREAD, ADSWRITE, FB_FileOpen, FB_FileClose, and other Tc2_System FBs with correct type checking
  3. Common types (AXIS_REF, MC_Direction, T_AmsNetId, T_AmsPort, E_OpenPath) resolve correctly when used as FB parameters
  4. Example GVL stubs for EtherCAT terminal I/O patterns are documented and usable as templates for users' own I/O declarations
**Plans**: TBD

### Phase 16: Shipped Stubs -- Schneider & Allen Bradley
**Goal**: Users targeting Schneider or Allen Bradley can type-check code using common vendor FBs without writing their own stubs
**Depends on**: Phase 13
**Requirements**: STUB-07, STUB-08, STUB-09, STUB-10, STUB-11, STUB-12
**Success Criteria** (what must be TRUE):
  1. User can reference Schneider motion FBs (MC_Power, MC_MoveAbsolute, MC_Stop with Schneider-specific parameters) and `stc check` validates usage
  2. User can reference Schneider communication FBs (READ_VAR, WRITE_VAR, SEND_REQ, RCV_REQ) and system FBs (GetBit, SetBit, RTC)
  3. AB type-check profile restricts code to AB-compatible subset -- no OOP, no POINTER TO, no REFERENCE TO, tag-based I/O patterns
  4. AB timer stubs (TONR, TOFR, RTO) and common instruction stubs (ADD, SUB, MUL, DIV, MOV, CMP, EQU, NEQ, GRT, LES, GEQ, LEQ) type-check correctly
**Plans**: TBD

### Phase 17: Behavioral Mocks
**Goal**: Users can run realistic simulations of motion control code using shipped behavioral mocks that simulate multi-cycle FB execution
**Depends on**: Phase 14, Phase 15
**Requirements**: BMOCK-01, BMOCK-02, BMOCK-03, BMOCK-04, BMOCK-05
**Success Criteria** (what must be TRUE):
  1. User can test motion code with MC_MoveAbsolute mock that simulates position changes over multiple scan cycles and sets Done when target is reached
  2. MC_Power mock simulates enable/disable with Status output, MC_Home simulates homing sequence, MC_Stop simulates deceleration
  3. ADSREAD mock returns configurable response data so users can test communication handling logic
  4. All behavioral mocks are pure ST files that users can inspect, modify, or use as templates for their own behavioral mocks
**Plans**: TBD

### Phase 18: Auto-Defines & TcPOU Extractor
**Goal**: Users get automatic preprocessor symbols during test/sim and can extract FB stubs from existing TwinCAT projects
**Depends on**: Phase 5 (test runner), Phase 6 (sim runner)
**Requirements**: TEST-08, TEST-09, TOOL-01
**Success Criteria** (what must be TRUE):
  1. When running `stc test`, the STC_TEST preprocessor symbol is automatically defined so users can conditionally compile test-only code paths
  2. When running `stc sim`, the STC_SIM preprocessor symbol is automatically defined so users can conditionally compile simulation-only code paths
  3. User can run `stc vendor extract <path.plcproj>` on a TwinCAT project file and get .st stub files extracted from TcPOU XML declarations
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 12 -> 13 -> 14 -> 15/16 (parallel) -> 17 -> 18

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Project Bootstrap & Parser | v1.0 | 5/5 | Complete | 2026-03-26 |
| 2. Preprocessor | v1.0 | 2/2 | Complete | 2026-03-26 |
| 3. Semantic Analysis | v1.0 | 5/5 | Complete | 2026-03-27 |
| 4. Standard Library & Interpreter | v1.0 | 4/4 | Complete | 2026-03-27 |
| 5. Testing Framework | v1.0 | 2/2 | Complete | 2026-03-27 |
| 6. Simulation | v1.0 | 2/2 | Complete | 2026-03-27 |
| 7. Multi-Vendor Emission | v1.0 | 2/2 | Complete | 2026-03-27 |
| 8. Formatter & Linter | v1.0 | 3/3 | Complete | 2026-03-28 |
| 9. LSP & VS Code Extension | v1.0 | 3/3 | Complete | 2026-03-28 |
| 10. Incremental Compilation | v1.0 | 2/2 | Complete | 2026-03-28 |
| 11. MCP Server & Claude Code Skills | v1.0 | 2/2 | Complete | 2026-03-28 |
| 12. I/O Address Parser & Table | v1.1 | 2/2 | Complete    | 2026-03-30 |
| 13. Vendor Stub Loading | v1.1 | 0/0 | Not started | - |
| 14. Mock Framework | v1.1 | 0/0 | Not started | - |
| 15. Shipped Stubs -- Beckhoff | v1.1 | 0/0 | Not started | - |
| 16. Shipped Stubs -- Schneider & AB | v1.1 | 0/0 | Not started | - |
| 17. Behavioral Mocks | v1.1 | 0/0 | Not started | - |
| 18. Auto-Defines & TcPOU Extractor | v1.1 | 0/0 | Not started | - |
