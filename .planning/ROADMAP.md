# Roadmap: STC -- Structured Text Compiler Toolchain

## Milestones

- [x] **v1.0 MVP** - Phases 1-11 (shipped 2026-03-28)
- [x] **v1.1 Vendor Libraries & I/O** - Phases 12-18 (shipped 2026-03-30)
- [ ] **v1.2 TwinCAT Import, EtherCAT Simulation & OPC UA** - Phases 19-29 (in progress)

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

<details>
<summary>v1.1 Vendor Libraries & I/O (Phases 12-18) - SHIPPED 2026-03-30</summary>

- [x] **Phase 12: I/O Address Parser & Table** - AT address syntax in parser, mock I/O table in interpreter, scan-cycle I/O sync, overlap detection
- [x] **Phase 13: Vendor Stub Loading** - .st stub file loading, library_paths config, type resolution from stubs, LSP support for vendor FBs, single-vendor enforcement
- [x] **Phase 14: Mock Framework** - ST mock FBs overriding stubs, mock_paths config, auto-generated zero-value instances, signature validation, test I/O injection
- [x] **Phase 15: Shipped Stubs -- Beckhoff** - Tc2_MC2, Tc2_System, Tc2_Utilities, Tc3_EventLogger stubs, common types, EtherCAT I/O examples
- [x] **Phase 16: Shipped Stubs -- Schneider & Allen Bradley** - Schneider motion/comm/system stubs, AB type-check profile, AB timers, AB common instructions
- [x] **Phase 17: Behavioral Mocks** - Shipped behavioral mocks for MC_MoveAbsolute, MC_Power, MC_Home, MC_Stop, ADSREAD with simulated behavior
- [x] **Phase 18: Auto-Defines & TcPOU Extractor** - STC_TEST/STC_SIM auto-define, stc vendor extract command for TwinCAT project files

</details>

### v1.2 TwinCAT Import, EtherCAT Simulation & OPC UA (In Progress)

**Milestone goal:** Run sildarvinnsla ST301 + SVNCoreComponents on the host unmodified, with simulated EtherCAT terminals and drives behind its `TcLinkTo` links and a TF6100-compatible OPC UA server that the existing Flutter HMI connects to.

**Acceptance oracle:** `/Users/jonb/Projects/sildarvinnsla` (ST301 + SVNCoreComponents; Baader for serial). Flattened probe sources: `/Users/jonb/Projects/beckhoff-docs/stc-probes/`.

- [ ] **Phase 19: TwinCAT Declaration Syntax** - Attribute pragmas kept in the AST, GVL files, `AT %I*` in structs/FBs, empty call arguments, ACTIONs
- [ ] **Phase 20: TwinCAT Expression Semantics** - Bit access, named function arguments, qualified/based enums, `REF=`/`THIS^`/`SUPER^`, checker knows standard FBs; ST301 parses with zero errors
- [ ] **Phase 21: TwinCAT Project Import & Library Stubs** - `stc vendor import` of tsproj/plcproj with library resolution, Tc2_EtherCAT/Tc2_System/ModbusSrv/Tc3_Module/SerialCom stubs, fixed `vendor extract`
- [ ] **Phase 22: Symbol Tree & Value Semantics** - Dotted-path symbol tree with Get/Set, integer wrap and literal typing, array/struct initialisers
- [ ] **Phase 23: Project Execution Runtime** - GVLs + PROGRAMs per task with cycle time, free-running mode, PERSISTENT/RETAIN state file, AT typing by declared type, sim/serve run whole projects
- [ ] **Phase 24: EtherCAT Topology & Link Binding** - Load EtherCATConfig exports, resolve `TcLinkTo`, `stc ecat validate`, per-master process images, InfoData/WcState/master state
- [ ] **Phase 25: EtherCAT Terminal Models** - Digital, analog, EL9222, PSU, diagnostic-only and EL6001 serial models selected by vendor/product code
- [ ] **Phase 26: ATV320 Drive & EtherCAT Master Services** - CiA402 drive model with CoE object dictionary and behavioural Tc2_EtherCAT mocks so FB_ATV320 and FB_EcDeviceDiag run unmodified
- [ ] **Phase 27: Plant Scenarios & Simulation CLI** - TOML scenarios and ST test built-ins for fault injection, `stc sim --project --io --scenario`
- [ ] **Phase 28: OPC UA Address Space** - awcullen/opcua server with Beckhoff namespace layout, TF6100 exposure rules, access levels, struct DataTypeDefinitions, type mapping
- [ ] **Phase 29: Live HMI & Agent Integration** - Writes and subscriptions against the running scan, CI browse diff against real TF6100, Flutter HMI acceptance, MCP sim/OPC UA tools, docs

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

<details>
<summary>v1.1 Vendor Libraries & I/O Phase Details (Phases 12-18)</summary>

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
**Plans**: 2 plans

Plans:
- [x] 13-01-PLAN.md -- Vendor loader, Symbol IsLibrary flag, resolver library support
- [x] 13-02-PLAN.md -- Analyzer/CLI/LSP integration, single-vendor enforcement

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
**Plans**: 2 plans

Plans:
- [x] 14-01-PLAN.md -- Mock infrastructure: config, loader, resolver MockFiles, signature validation
- [x] 14-02-PLAN.md -- Test runner integration, auto-stub warnings, I/O injection, CLI wiring



### Phase 15: Shipped Stubs -- Beckhoff
**Goal**: Users targeting Beckhoff TwinCAT can immediately type-check code using common Beckhoff libraries without writing their own stubs
**Depends on**: Phase 13
**Requirements**: STUB-01, STUB-02, STUB-03, STUB-04, STUB-05, STUB-06
**Success Criteria** (what must be TRUE):
  1. User can reference MC_Power, MC_MoveAbsolute, MC_MoveRelative, MC_Stop, MC_Home, and other Tc2_MC2 FBs and `stc check` validates parameter usage
  2. User can reference ADSREAD, ADSWRITE, FB_FileOpen, FB_FileClose, and other Tc2_System FBs with correct type checking
  3. Common types (AXIS_REF, MC_Direction, T_AmsNetId, T_AmsPort, E_OpenPath) resolve correctly when used as FB parameters
  4. Example GVL stubs for EtherCAT terminal I/O patterns are documented and usable as templates for users' own I/O declarations
**Plans**: 2 plans

Plans:
- [x] 15-01-PLAN.md -- Beckhoff Tc2_MC2, Tc2_System, Tc2_Utilities, Tc3_EventLogger stubs, common types, EtherCAT I/O docs

### Phase 16: Shipped Stubs -- Schneider & Allen Bradley
**Goal**: Users targeting Schneider or Allen Bradley can type-check code using common vendor FBs without writing their own stubs
**Depends on**: Phase 13
**Requirements**: STUB-07, STUB-08, STUB-09, STUB-10, STUB-11, STUB-12
**Success Criteria** (what must be TRUE):
  1. User can reference Schneider motion FBs (MC_Power, MC_MoveAbsolute, MC_Stop with Schneider-specific parameters) and `stc check` validates usage
  2. User can reference Schneider communication FBs (READ_VAR, WRITE_VAR, SEND_REQ, RCV_REQ) and system FBs (GetBit, SetBit, RTC)
  3. AB type-check profile restricts code to AB-compatible subset -- no OOP, no POINTER TO, no REFERENCE TO, tag-based I/O patterns
  4. AB timer stubs (TONR, TOFR, RTO) and common instruction stubs (ADD, SUB, MUL, DIV, MOV, CMP, EQU, NEQ, GRT, LES, GEQ, LEQ) type-check correctly
**Plans**: 2 plans

Plans:
- [x] 16-01-PLAN.md -- Schneider motion/comm/system stubs and AB profile/timers/instructions stubs

### Phase 17: Behavioral Mocks
**Goal**: Users can run realistic simulations of motion control code using shipped behavioral mocks that simulate multi-cycle FB execution
**Depends on**: Phase 14, Phase 15
**Requirements**: BMOCK-01, BMOCK-02, BMOCK-03, BMOCK-04, BMOCK-05
**Success Criteria** (what must be TRUE):
  1. User can test motion code with MC_MoveAbsolute mock that simulates position changes over multiple scan cycles and sets Done when target is reached
  2. MC_Power mock simulates enable/disable with Status output, MC_Home simulates homing sequence, MC_Stop simulates deceleration
  3. ADSREAD mock returns configurable response data so users can test communication handling logic
  4. All behavioral mocks are pure ST files that users can inspect, modify, or use as templates for their own behavioral mocks
**Plans**: 2 plans

Plans:
- [x] 17-01-PLAN.md -- Behavioral mocks for MC_MoveAbsolute, MC_Power, MC_Home, MC_Stop, ADSREAD

### Phase 18: Auto-Defines & TcPOU Extractor
**Goal**: Users get automatic preprocessor symbols during test/sim and can extract FB stubs from existing TwinCAT projects
**Depends on**: Phase 5 (test runner), Phase 6 (sim runner)
**Requirements**: TEST-08, TEST-09, TOOL-01
**Success Criteria** (what must be TRUE):
  1. When running `stc test`, the STC_TEST preprocessor symbol is automatically defined so users can conditionally compile test-only code paths
  2. When running `stc sim`, the STC_SIM preprocessor symbol is automatically defined so users can conditionally compile simulation-only code paths
  3. User can run `stc vendor extract <path.plcproj>` on a TwinCAT project file and get .st stub files extracted from TcPOU XML declarations
**Plans**: 2 plans

Plans:
- [x] 18-01-PLAN.md -- STC_TEST/STC_SIM auto-defines and stc vendor extract command

</details>

### Phase 19: TwinCAT Declaration Syntax
**Goal**: TwinCAT declaration-level constructs (attribute pragmas, GVL files, wildcard AT bindings in structs and FBs, empty call arguments, ACTIONs) parse into the AST and survive tooling round-trips
**Depends on**: Phase 18 (v1.1 complete)
**Requirements**: DIAL-01, DIAL-02, DIAL-03, DIAL-05, DIAL-08
**Success Criteria** (what must be TRUE):
  1. `stc parse --format json` on an ST301 GVL such as `ECT` shows each `{attribute ...}` (`OPC.UA.DA`, `TcLinkTo`, `qualified_only`, single- or double-quoted, with `''` escapes and blank lines before the declaration) attached to the right VarDecl, StructMember, EnumValue, TypeDecl or POU, and `stc fmt` / `stc emit` reproduce them
  2. A file containing only `VAR_GLOBAL PERSISTENT RETAIN ... END_VAR` parses as a GVL named from the file or `--gvl-name`, and unqualified access to a `qualified_only` GVL variable is a checker error
  3. `ST_EL1008` (struct members `I1 AT %I* : BOOL`) and FBs with `AT %I*`/`%Q*` in `VAR`/`VAR_INPUT`/`VAR_OUTPUT` check with no warnings, while an explicit `%IX0.0` inside an FB still reports SEMA031
  4. `t(IN := b, PT := , Q => , ET => );` parses and runs as if the empty arguments were omitted
  5. A MAIN that is a chain of action calls (CODESYS `ACTION ... END_ACTION` text form) parses, checks and executes each action against the owning POU's variables
**Plans:** 10/10 plans complete

Plans:
- [x] 19-01-PLAN.md — Interp coverage lift, local coverage-gate script, committed TwinCAT probe fixtures, oracle baseline
- [x] 19-02-PLAN.md — AST contracts: Attribute, PragmaNode kind, GVLDecl, struct AT, POU actions, JSON
- [x] 19-03-PLAN.md — Attribute/pragma parsing, attachment, trivia order, fmt/emit round-trip (DIAL-01)
- [x] 19-04-PLAN.md — Wildcard AT on struct members and FBs, empty call arguments (DIAL-03, DIAL-05)
- [x] 19-05-PLAN.md — GVL parsing/printing, checker qualified_only SEMA033 and constants SEMA034, unused-var exemption, deferred GVL resolution (DIAL-02)
- [x] 19-06-PLAN.md — Interpreter GVL env layer, test runner and sim wiring, twincat_dialect ST suite (DIAL-02)
- [x] 19-07-PLAN.md — ACTION parsing (inside and after POU), printing, checker resolution (DIAL-08)
- [x] 19-08-PLAN.md — ACTION execution in owner env with recursion guard; action and empty-arg ST suites (DIAL-08, DIAL-05)
- [x] 19-09-PLAN.md — Phase gate: fixture/oracle tests, LSP robustness, coverage gate, validation sign-off
- [x] 19-10-PLAN.md — --gvl-name flag on parse/check/fmt/emit and ECT JSON CLI acceptance test (DIAL-02)

### Phase 20: TwinCAT Expression Semantics
**Goal**: Expression- and call-level TwinCAT constructs parse, type-check and execute, so the flattened sildarvinnsla sources parse cleanly and `stc check` reports only genuine problems
**Depends on**: Phase 19
**Requirements**: DIAL-04, DIAL-06, DIAL-07, DIAL-09, DIAL-10, RUNT-08
**Success Criteria** (what must be TRUE):
  1. `ECT.X.q_wDigitalInputs.0` and `Modbus.arr[0].3 := TRUE` read and write single bits at runtime, and `w.16` on a WORD is a checker error
  2. `n := F_X(a := 1, b := 2)` type-checks and returns the same value as the positional call
  3. `CASE e OF lft_e.eef1: ... E.a, E.b: ...` and an enum declared `(a := 0, b := 1) UINT` with `strict` / `to_string` attributes check and execute; `REF=`, `THIS^` and `SUPER^` execute with the expected results in host tests
  4. `stc check` on the flattened ST301 + SVNCoreComponents probe sources reports zero parse errors
  5. `stc check` accepts TON/TOF/TP/CTU/CTD/CTUD/R_TRIG/F_TRIG/SR/RS calls without stubs and reports an error for an undeclared type name such as `FB_DoesNotExist`
**Plans**: 9 plans

Plans:
- [x] 20-01-PLAN.md -- AST contracts: bit access, named call args, REF=, THIS/SUPER, initialisers, namespace types, enum base/default; JSON kinds, fmt/emit/lint, shared enum numbering
- [x] 20-02-PLAN.md -- Parser expressions/statements: bit access, named args in expression calls, trailing comma, qualified CASE labels, REF=, THIS^/SUPER^
- [x] 20-03-PLAN.md -- Lexer and declaration parsing: typed based literals, enum base type and TYPE default, namespace-qualified types, struct/array initialisers, stray semicolons
- [x] 20-04-PLAN.md -- Checker resolver: pointer-stable two-pass registration, ten standard FBs with aliases, inherited EXTENDS scope, SEMA037 with fixture audit
- [x] 20-05-PLAN.md -- Interpreter values: bit read/write, enum numbering/qualified/inline values, TO_STRING, standard FB input aliases
- [x] 20-06-PLAN.md -- Checker metadata and calls: enum metadata and inline enums, FUNCTION outputs, bit access SEMA035 (read and write), named-arg binding
- [x] 20-07-PLAN.md -- Interpreter calls and references: shared arg binder, FUNCTIONs in pkg/interp, unqualified methods, THIS^/SUPER^, path-based REF=
- [x] 20-09-PLAN.md -- Checker semantics: enum rules SEMA036 and TO_STRING, REF=/THIS/SUPER SEMA038 with reference auto-deref, initialiser checks
- [x] 20-08-PLAN.md -- Acceptance gate: ST dialect suites, probe gate without allowances, zero-parse-error oracle, hand-off re-run, validation sign-off

### Phase 21: TwinCAT Project Import & Library Stubs
**Goal**: A TwinCAT solution on disk becomes one stc project model, with library references resolved and the Beckhoff libraries sildarvinnsla uses available as stubs
**Depends on**: Phase 20
**Requirements**: IMPT-01, IMPT-02, IMPT-03, IMPT-04, IMPT-05
**Success Criteria** (what must be TRUE):
  1. `stc vendor import ST301.tsproj` loads every TcPOU (declaration, implementation, methods, actions, properties), TcGVL and TcDUT listed in the plcproj, and `stc check` on the result reports zero Phase-21-owned errors for ST301 + SVNCoreComponents (remaining errors are explicitly allow-listed as Phase 22 literal typing or genuine sildarvinnsla drift)
  2. A reference to an SVNCoreComponents FB resolves from the sibling library plcproj, a Tc2_EtherCAT FB resolves from shipped stubs, and a deliberately missing library reference is reported as a diagnostic with its plcproj position
  3. The imported project model carries the `.tsproj` task cycle time (1 ms for ST301) and PLC project name, visible in `--format json` output
  4. ST101, ST201, ST301 and the Baader project import and type-check against the shipped Tc2_EtherCAT, Tc2_System, Tc2_ModbusSrv, Tc3_Module and Tc2_SerialCom stubs with only allow-listed residuals (Phase 22 literal typing, genuine drift, Baader Phase 24 I/O types)
  5. `stc vendor extract` output for the sildarvinnsla plcproj parses with `stc parse` and includes methods, GVLs and DUTs
**Plans**: 6 plans

Plans:
- [x] 21-01-PLAN.md -- Parser/checker fixes: PROPERTY access modifiers, statement-level `fb();` checked as FB call, double-quoted attributes ignored with SEMA039
- [x] 21-02-PLAN.md -- Embedded Beckhoff stubs (Tc2_EtherCAT, Tc2_System, Tc2_Utilities, Tc2_ModbusSrv, Tc2_SerialCom, Tc3_Module, Tc3_IPCDiag) with dependency closure and check-clean tests
- [x] 21-03-PLAN.md -- pkg/vendor/twincat model, tsproj/xti/plcproj/TcTTO readers, line-preserving TcPOU converter, synthetic fixtures
- [x] 21-04-PLAN.md -- Ordered library resolver (sibling, library_paths, stubs, VEND020), Import + analyzer.AnalyzeProject, vendor extract on the shared converter
- [x] 21-05-PLAN.md -- CLI: `stc vendor import [--out]`, `stc check <project>`, `stc test --project` (RunOpts.ProjectFiles), `stc sim <project>`, extract JSON
- [x] 21-06-PLAN.md -- STC_SILD_DIR oracle gate on ST301/ST101/ST201/Baader/SVNCore with owner buckets, docs, VALIDATION sign-off, coverage gate

### Phase 22: Symbol Tree & Value Semantics
**Goal**: Every live variable in a running project is addressable by dotted path with correct IEC value semantics, giving tests, servers and agents one shared view of PLC state
**Depends on**: Phase 21
**Requirements**: RUNT-01, RUNT-02, RUNT-05, RUNT-06
**Success Criteria** (what must be TRUE):
  1. After analysing ST301, the symbol tree lists `GVL.fb[2].HMI.p_stat_State` with its IEC type, enum strings and attributes, and every GVL, PROGRAM, FB instance, struct member and array element is reachable by path
  2. A Go test can `Set("GVL.x.p_cmd_Start", true)` and `Get(...)` it back on a live interpreter, with coercion from JSON-style values (number to INT/REAL, string to enum name)
  3. INT `32767 + 1` yields `-32768`, UINT `0 - 1` yields `65535`, and `a := a + 1` on an INT checks without a type error
  4. A struct-array initialiser `:= [(a := 1, s := 'x'), ...]` over `ARRAY[1..GVL.CONST]` is visible through `Get` at cycle 0
**Plans**: TBD

### Phase 23: Project Execution Runtime
**Goal**: A whole imported project runs on the host the way the PLC runs it: GVLs once, PROGRAMs per task at the configured cycle, retained state across restarts, wildcard I/O by declared type
**Depends on**: Phase 22
**Requirements**: RUNT-03, RUNT-04, RUNT-07, RUNT-09
**Success Criteria** (what must be TRUE):
  1. `stc sim --project ST301.tsproj --cycles 1000` runs MAIN with all its user FBs, functions, methods and actions and advances simulated time by exactly 1 s at the 1 ms task cycle, identically on every run
  2. In free-running mode the scan is paced against wall-clock so 10 s of real time is approximately 10 000 cycles of a 1 ms task
  3. A `p_cfg_*` PERSISTENT value written in one run is restored from the state file on the next run
  4. An `AT %I*` INT reads back as -5 after its slot is set to 0xFFFB, and REAL, enum and struct-with-`AT %I*`-member bindings round-trip by declared type
**Plans**: TBD

### Phase 24: EtherCAT Topology & Link Binding
**Goal**: The project's EtherCAT I/O tree is loaded from TwinCAT exports and every `TcLinkTo` link is resolved and copied through per-master process images, so link errors are caught statically and I/O values flow at scan boundaries
**Depends on**: Phase 23
**Requirements**: ECAT-01, ECAT-02, ECAT-03, ECAT-07
**Success Criteria** (what must be TRUE):
  1. Loading ST301 `Device 1..4.xml` yields the same slaves, E-bus nesting, `Module N` segments and link paths that `generate_gvl.py` produces for the same files
  2. `stc ecat validate` on ST301 reports zero unresolved links, and a mistyped link path or a type-size mismatch is reported with the GVL file position
  3. Writing a byte in a master's input image makes the linked `AT %I*` struct member (e.g. `ECT.ST301_A1_03.I1`) change at the next scan, and a linked `%Q*` FB member appears in the output image after the scan
  4. `ECT_Diag` variables linked to slave `WcState`/`InfoData.State`/`InfoData.AdsAddr` and master `DevState`/`SlaveCount`/`Frm0State`/`Frm0WcState`/`InfoData.AmsNetId` read the Beckhoff values for a healthy network (OP = 0x8, WcState 0)
**Plans**: 4 plans

Plans:
- [x] 24-01-PLAN.md -- pkg/ecat loader, generate_gvl.py tree/link-path port, process image layout + pseudo-input slots, synthetic Demo Device fixtures
- [x] 24-02-PLAN.md -- TcLinkTo parsing, CollectLinks, Resolve (ECAT001-007), `stc ecat validate` CLI
- [x] 24-03-PLAN.md -- Network + Device registry + healthy pseudo-inputs/fault API, interp IOBinder with two scan.go hooks
- [x] 24-04-PLAN.md -- env-gated ST301 equivalence gate, docs, VALIDATION sign-off, coverage gate

### Phase 25: EtherCAT Terminal Models
**Goal**: Every non-drive terminal in the sildarvinnsla hardware list behaves like the real device at PDO level, selected automatically by vendor and product code
**Depends on**: Phase 24
**Requirements**: ECAT-04, ECAT-06
**Success Criteria** (what must be TRUE):
  1. Each slave in ST301's export gets a model by (VendorId, ProductCode), and unknown devices fall back to byte passthrough with a diagnostic
  2. Setting an EL3054 channel to 12 mA yields the scaled INT and a clean status word, and out-of-range values set the Underrange/Overrange bits
  3. Tripping an EL9222-5500 channel sets Tripped in its status, and the channel recovers only after the PLC pulses Reset
  4. A scripted byte-stream peer on an EL6001 answers `FB_BaaderSerial`'s `md`/`mt1` requests through the 22-byte serial PDO
**Plans**: TBD

### Phase 26: ATV320 Drive & EtherCAT Master Services
**Goal**: The unmodified FB_ATV320 configures and runs a simulated drive, and FB_EcDeviceDiag fills its diagnostics from the simulator, because the drive model and the Tc2_EtherCAT ADS services behave like the real ones
**Depends on**: Phase 24, Phase 21 (Tc2_EtherCAT stubs)
**Requirements**: ECAT-05, ECAT-08
**Success Criteria** (what must be TRUE):
  1. FB_ATV320 bound to an ATV320 model walks its configurator (PreOp to OP via `FB_EcSetSlaveState`, SDO parameter writes via `FB_EcCoESDoWrite`, EEPROM save at 0x2032:01) and reaches `cfgReady` without code changes
  2. Commanding a run makes ETA go through the CiA402 states to Operation enabled, RFR ramps to LFR at the configured ACC/DEC, and HMIS reports `run`
  3. Injecting a drive fault sets the CiA402 Fault state with the chosen LFT code, and FB_ATV320 surfaces it
  4. FB_EcDeviceDiag fills `ECT_Diag.Device_N_Diag` from `FB_EcGetAllSlaveStates`, `FB_EcGetMasterState` and the CRC FBs, with busy/done completing over several scans
**Plans**: 4 plans

Plans:
- [x] 26-01-PLAN.md -- ATV320 model: entry layout hook, CiA402 state machine, ramps, HMIS/LFT, DI/OL1R, stimulus API
- [x] 26-02-PLAN.md -- Network service API, ATV320 CoE object dictionary with parameter defaults and EEPROM save, PreOp start
- [x] 26-03-PLAN.md -- Tc2_EtherCAT StandardFB mocks (state, master, CRC, physical write, CoE SDO) with 2-scan async latency
- [ ] 26-04-PLAN.md -- End-to-end: trimmed CI flow, env-gated real FB_ATV320 + FB_EcDeviceDiag gate, docs, VALIDATION, coverage gate

### Phase 27: Plant Scenarios & Simulation CLI
**Goal**: Engineers and CI can drive the simulated plant through deterministic scripted scenarios, either from a TOML file on the CLI or from ST unit tests
**Depends on**: Phase 25, Phase 26
**Requirements**: ECAT-09, ECAT-10, DEVX-01
**Success Criteria** (what must be TRUE):
  1. `stc sim --project ST301.tsproj --io "Device*.xml" --scenario jam.toml --cycles N --format json` runs the project against the simulator and reports outputs and diagnostics, with identical output on repeated runs
  2. A scenario can set an input by variable path or link path, set an analog value, trip an EL9222 channel, pull a slave (not present / link error), raise an ATV320 fault with an LFT code and ramp a value over time, each at a chosen scan
  3. A `*_test.st` using `SET`, `GET`, `SIM_SET_LINK`, `SIM_TRIP`, `SIM_SLAVE_STATE` and `RUN_CYCLES` passes under `stc test` with the project and I/O config loaded, e.g. a removed slave shows up in `ECT_Diag` after N cycles
**Plans**: TBD

### Phase 28: OPC UA Address Space
**Goal**: `stc serve` publishes the same OPC UA address space TF6100 would for the project: Beckhoff namespace and NodeIds, exposure decided by `OPC.UA.DA` attributes, correct access levels, data types and struct definitions
**Depends on**: Phase 23
**Requirements**: OPCUA-01, OPCUA-02, OPCUA-03, OPCUA-04, OPCUA-05, OPCUA-06
**Success Criteria** (what must be TRUE):
  1. First plan is a spike proving awcullen/opcua can serve a StructureDefinition DataTypeDefinition for a custom struct (fallback: NodeSet2 import) before the address-space builder is written
  2. `stc serve --project ST301.tsproj --opcua :4840` accepts an anonymous SecurityPolicy None client and, with the flag set, a Basic256Sha256 client using self-signed certificates; `ns=0;i=2259` reads Running
  3. Browsing shows `urn:BeckhoffAutomation:Ua:PLC1` at namespace index 4, `ns=4;s=GVL.fb[2].HMI.p_stat_State` exists with declared case, PROGRAM MAIN publishes nothing, and `OPC.UA.DA := '0'` / `'2'` prune or flatten subtrees as TF6100 does
  4. `p_stat_*` nodes are read-only and `p_cmd_*` / `p_cfg_*` writable per `OPC.UA.DA.Access`, Description attributes carry the pragma text, and each IEC type reads with its PLCopen OPC 30000 UA type (TIME as Int64 ms, enums as Int32 with EnumStrings)
  5. A StructuredType struct such as `ST_Sensor_HMI` reads as one ExtensionObject decodable by field name from its served DataTypeDefinition, while its members stay individually readable
**Plans**: 4 plans

Plans:
- [x] 28-01-PLAN.md -- Server core: pin awcullen/opcua v1.4.0, SymbolNode/NodeSource contract + MapSource, certs, Config/Start/Stop, namespace index 4, anonymous writes, i=2259, None + Basic256Sha256 client tests
- [x] 28-02-PLAN.md -- Value layer: PLCopen type mapping, toUA/fromUA, enum + StructuredType DataTypes (StructureDefinition, reflect.StructOf, nested), Space/Publish with access levels, descriptions, arrays
- [x] 28-03-PLAN.md -- Builder: TF6100 exposure rules over SymbolNode, DeviceSet/PLC1, diagnostics, ST301-shaped fixture, golden browse snapshot
- [ ] 28-04-PLAN.md -- (after Phases 21-23 merge) symtree/Runtime adapters, `stc serve`, parsed-ST parity with golden, VALIDATION + coverage gate

### Phase 29: Live HMI & Agent Integration
**Goal**: The unmodified sildarvinnsla Flutter HMI and AI agents operate the simulated ST301 line live, with CI guarding fidelity against the real TF6100 server and docs describing the workflow
**Depends on**: Phase 27, Phase 28
**Requirements**: OPCUA-07, OPCUA-08, OPCUA-09, OPCUA-10, DEVX-02, DEVX-03
**Success Criteria** (what must be TRUE):
  1. An OPC UA client writing `p_cmd_Start := TRUE` while the scan runs sees the FB clear it again, and a subscription on a sensor `p_stat_*` delivers a data change when a scenario toggles the input
  2. CI diffs the emulated ST301 address space against the stored TF6100 browse fixture (node ids, data types, access levels, struct definitions) and fails on any divergence
  3. The Flutter HMI (tfc-hmi / open62541_dart) connects to `stc serve` running ST301 with the EtherCAT simulator and shows live sensor, conveyor and drive HMI structs with enum names like `rdy(2)`
  4. An agent can step the simulation, read and write variables and browse the OPC UA tree through the `stc_sim_step`, `stc_sim_read`, `stc_sim_write` and `stc_opcua_browse` MCP tools
  5. `docs/` has a TwinCAT import, EtherCAT simulation and OPC UA guide, and the stale claims in `TESTING_GUIDE.md`, `ST_LANGUAGE_SUPPORT.md` and `stdlib/vendor/beckhoff/ethercat_io.md` are corrected
**Plans**: TBD

## Progress

**Execution Order:**
v1.2 phases execute in numeric order: 19 -> 20 -> 21 -> 22 -> 23 -> 24 -> 25 -> 26 -> 27 -> 28 -> 29. Phase 28 depends only on Phase 23 and may run in parallel with Phases 24-27; Phases 25 and 26 both depend only on Phase 24 and may run in parallel.

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
| 13. Vendor Stub Loading | v1.1 | 2/2 | Complete    | 2026-03-30 |
| 14. Mock Framework | v1.1 | 2/2 | Complete    | 2026-03-30 |
| 15. Shipped Stubs -- Beckhoff | v1.1 | 1/1 | Complete | 2026-03-30 |
| 16. Shipped Stubs -- Schneider & AB | v1.1 | 1/1 | Complete | 2026-03-30 |
| 17. Behavioral Mocks | v1.1 | 1/1 | Complete | 2026-03-30 |
| 18. Auto-Defines & TcPOU Extractor | v1.1 | 1/1 | Complete | 2026-03-30 |
| 19. TwinCAT Declaration Syntax | v1.2 | 10/10 | Complete    | 2026-10-06 |
| 20. TwinCAT Expression Semantics | v1.2 | 9/9 | Complete    | 2026-10-06 |
| 21. TwinCAT Project Import & Library Stubs | v1.2 | 6/6 | Complete    | 2026-10-06 |
| 22. Symbol Tree & Value Semantics | v1.2 | 5/5 | Complete    | 2026-10-06 |
| 23. Project Execution Runtime | v1.2 | 0/TBD | Not started | - |
| 24. EtherCAT Topology & Link Binding | v1.2 | 4/4 | Complete    | 2026-10-06 |
| 25. EtherCAT Terminal Models | v1.2 | 0/TBD | Not started | - |
| 26. ATV320 Drive & EtherCAT Master Services | v1.2 | 3/4 | In Progress|  |
| 27. Plant Scenarios & Simulation CLI | v1.2 | 0/TBD | Not started | - |
| 28. OPC UA Address Space | v1.2 | 3/4 | In Progress|  |
| 29. Live HMI & Agent Integration | v1.2 | 0/TBD | Not started | - |
