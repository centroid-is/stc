# Milestones

## v1.1 Vendor Libraries & I/O (Shipped: 2026-10-05)

**Phases completed:** 18 phases, 38 plans, 73 tasks

**Key accomplishments:**

- Go module with CI pipeline, Makefile, and foundation packages (source positions, diagnostics, TOML config) providing types imported by all downstream compiler packages
- Complete IEC 61131-3 + CODESYS OOP AST/CST node types with trivia attachment, visitor pattern, and JSON marshaling
- Hand-written IEC 61131-3 lexer with ~80 case-insensitive keywords, nested block comments, typed/time/date literals, multi-base integers, pragmas, and trivia preservation for CST fidelity
- Recursive descent parser with Pratt expression parsing, error recovery, and full IEC 61131-3 + CODESYS OOP support across 7 source files and 11 passing tests
- Cobra-based stc CLI with parse subcommand (text/JSON), version info, stub subcommands, and 10 integration tests
- IEC 61131-3 conditional compilation preprocessor with IF/ELSIF/ELSE/END_IF, DEFINE, ERROR directives and line-level source map for position remapping
- stc pp subcommand with --define flags, text/JSON output, source maps, and 8 integration tests
- IEC 61131-3 type system with 23 elementary types, data-driven widening lattice, and 37 built-in function signatures
- Hierarchical symbol table with case-insensitive scope chains, redeclaration detection, and POU registry for IEC 61131-3 semantic analysis
- Two-pass type checker with expression/statement checking, forward reference support, candidate resolution for generic functions, and 20 diagnostic codes
- Vendor profile feature-flag system (beckhoff/schneider/portable) with VEND001-006 warnings, plus unused variable (SEMA012) and unreachable code (SEMA013) detection
- Analyzer facade orchestrating all checker passes with stc check CLI command supporting text/JSON output and vendor-aware diagnostics
- Tree-walking AST interpreter with tagged union Value type, scoped environment chain, and full expression/statement evaluation for all IEC control structures
- ScanCycleEngine with deterministic Tick(dt), StandardFB interface for stdlib FBs, and FBInstance dual-mode wrapper for persistent FB state across scan cycles
- IEC 61131-3 math, string, and type conversion functions with banker's rounding and 1-based indexing
- All 10 IEC standard library FBs (TON/TOF/TP timers, CTU/CTD/CTUD counters, R_TRIG/F_TRIG edge, SR/RS bistable) with deterministic time and end-to-end parse-to-interpret pipeline
- TEST_CASE/END_TEST_CASE parsing with assertion functions (ASSERT_TRUE/FALSE/EQ/NEAR), ADVANCE_TIME, and per-interpreter LocalFunctions dispatch
- 1. [Rule 1 - Bug] Expression-statement support for assertion calls
- Deterministic waveform generators (Step/Ramp/Sine/Square) and 3 plant models (Motor/Valve/Cylinder) for closed-loop simulation
- Closed-loop SimulationEngine wiring waveforms and plant models into scan cycle loop, with stc sim CLI outputting text tables or JSON
- AST-to-ST emitter with Beckhoff/Schneider/Portable vendor targets and round-trip stability
- Wire emit package into CLI as `stc emit <file> --target <vendor>` with text/JSON output
- ST code formatter with configurable 4-space/2-space indent, uppercase/lowercase keywords, and idempotent output via AST re-emission
- Rule-based ST linter with PLCopen coding guidelines, configurable naming conventions, and JSON output for CI integration
- Post-parse trivia attachment pass that maps lexer comment tokens to AST nodes, closing the parse->format comment preservation gap
- GLSP-based LSP server with document sync, real-time parse/analysis diagnostics, and full-document formatting via stc lsp command
- Position-based symbol lookup with go-to-definition, hover, completion, find-references, and rename handlers for full IDE navigation
- Semantic tokens graying inactive preprocessor blocks plus VS Code extension with TextMate ST grammar and stc-lsp client over stdio
- 1. [Rule 3 - Blocking] Import cycle between incremental and analyzer packages
- MCP server binary with 6 tools (parse, check, test, emit, lint, format) over stdio transport using modelcontextprotocol/go-sdk
- 5 ST workflow skills for Claude Code with auto-invoke on .st files covering generate, validate, test, emit, and review workflows
- IEC 61131-3 direct address parser (%IX0.0, %QW4, %MD48, wildcards) with flat byte-array IOTable and DirectAddr lexer token
- ScanCycleEngine reads/writes AT-addressed variables via IOTable with bidirectional memory sync, checker validates AT format and detects overlapping addresses
- Vendor stub loader parsing .st files from configured library paths with library-aware resolver supporting IsLibrary symbol flag and user override semantics
- Library-aware Analyze facade with CLI check, LSP workspace init, and cross-vendor enforcement wiring vendor stubs through the full analysis pipeline
- Mock FB loader with config-driven mock_paths, resolver integration overriding library stubs, and signature validation against vendor stub parameters
- Mock-aware test runner with auto-stub fidelity warnings, SET_IO/GET_IO I/O injection, and CLI wiring from stc.toml mock_paths

---
