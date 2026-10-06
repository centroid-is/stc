# STC — Structured Text Compiler Toolchain

## What This Is

A Go-based IEC 61131-3 Structured Text compiler toolchain that makes ST development feel like modern software development — with an LSP, unit testing on host, CI integration, multi-vendor compilation, and first-class LLM agent support. Built for humans and AI agents to quickly produce, validate, and deploy structured text code across PLC vendors.

## Core Value

Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor — no hardware required for development and testing.

## Requirements

### Validated

- IEC 61131-3 Ed.3 parser with CODESYS OOP extensions — v1.0
- CLI binary with JSON output for all commands — v1.0
- Error-recovering parser producing partial ASTs — v1.0
- Preprocessor with conditional compilation — v1.0
- Two-pass semantic analysis with vendor-aware diagnostics — v1.0
- Tree-walking interpreter with scan-cycle semantics — v1.0
- Full IEC standard library (timers, counters, edge, bistable, math, string, conversion) — v1.0
- Host-based unit testing with TEST_CASE syntax and JUnit XML — v1.0
- Closed-loop simulation with plant models — v1.0
- Multi-vendor ST emission (Beckhoff, Schneider, Portable) — v1.0
- Formatter and linter with PLCopen rules — v1.0
- Full LSP with VS Code extension — v1.0
- Incremental compilation with file caching — v1.0
- MCP server with 6 tool handlers — v1.0
- Claude Code skills for ST workflows — v1.0
- Vendor library stub loading from .st declaration files — v1.1
- ST-based mock framework with auto-generated zero-value stubs and behavioural mocks — v1.1
- I/O address parser and mock I/O table (%I/%Q/%M, explicit addresses) — v1.1
- Shipped starter stubs for Beckhoff, Schneider, Allen Bradley — v1.1
- Auto-defined STC_TEST/STC_SIM preprocessor symbols and TcPOU stub extractor — v1.1
- TwinCAT declaration syntax: attribute pragmas in the AST, GVL files with qualified_only, wildcard AT in structs/FBs, empty call arguments, ACTIONs (DIAL-01/02/03/05/08) — v1.2 Phase 19
- TwinCAT expression semantics: bit access, named call arguments, qualified/based enums with strict/to_string, REF=/THIS^/SUPER^, checker knows standard FBs and rejects undeclared types (DIAL-04/06/07/09/10, RUNT-08) — v1.2 Phase 20

## Current Milestone: v1.2 TwinCAT Import, EtherCAT Simulation & OPC UA

**Goal:** Run a real TwinCAT 3 project (sildarvinnsla ST301 + SVNCoreComponents) on the host unmodified, with simulated EtherCAT terminals and drives behind its `TcLinkTo` links and a TF6100-compatible OPC UA server that the existing Flutter HMI can connect to.

**Target features:**
- TwinCAT dialect parity: pragmas retained in the AST, GVL files, `AT %I*` in structs/FBs, bit access, empty call arguments, named function arguments, qualified enums, ACTIONs, `REF=`/`THIS^`
- `stc vendor import` of `.tsproj`/`.plcproj` (TcPOU, TcGVL, TcDUT, methods, actions, library refs) and Tc2_EtherCAT / Tc2_System / Tc2_ModbusSrv stubs
- Runtime model: symbol tree with dotted-path read/write, multi-POU tasks with configured cycle time, PERSISTENT state, correct integer wrap and initialisers
- EtherCAT process-image simulator loaded from TwinCAT `EtherCATConfig` exports, resolving `TcLinkTo` pragmas, with device models (EL1008/EL2008/EP2338, EL3054, EL9222-5500, PS2001, ATV320 with CiA402 + CoE, EL6001 serial), InfoData/WcState/master state, Tc2_EtherCAT behavioural mocks, fault injection
- OPC UA server (awcullen/opcua) honouring `OPC.UA.DA`, `.Access`, `.StructuredType`, `.Description` with Beckhoff namespace/NodeId layout, struct DataTypeDefinitions and enum strings
- `stc serve` and test/MCP built-ins to drive the simulated plant

**Research:** `.planning/research/v1.2/`

## Current State

**v1.2 in progress** — Phases 19-20 complete (2026-10-06): sildarvinnsla ST301 and SVNCoreComponents both parse with 0 diagnostics (from 2711 / 1430); `stc check` residuals are missing-library (Phase 21) and literal-typing (Phase 22) buckets; coverage total 96.0%.

**v1.0 shipped** — 2026-03-28

21 Go packages, ~31,000 LOC, 320 files, 21 test suites all passing. Full compiler toolchain from parse to emit, with host testing, simulation, LSP, and AI integration.

### Active

- [ ] Multi-vendor ST compatibility (Beckhoff TwinCAT 3, Schneider CODESYS-derived; Allen Bradley later)
- [ ] Runnable host artifacts that preserve PLC scan-cycle semantics
- [ ] Unit testing on host with mocking, deterministic time, JUnit XML output
- [ ] Closed-loop simulation with sensor injection and plant modeling
- [ ] LSP and VS Code extension with diagnostics, go-to-def, hover, completion, rename
- [ ] LLM agent friendliness — clear CLI, JSON output, MCP server, minimal token footprint
- [ ] No Java runtime dependency (build-time ANTLR generation acceptable)
- [ ] Machine-verifiable phase gates for every development milestone
- [ ] Vendor ST re-emission (parse → analyze → emit vendor-flavored ST)
- [ ] Incremental adoption alongside existing vendor IDEs
- [ ] PLCopen XML import/export for vendor interop
- [ ] IEC standard library (TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, SR, RS, standard functions)
- [ ] CODESYS extension compatibility (POINTER TO, REFERENCE TO, OOP, 64-bit types)
- [ ] Source-level debugging info (ST line → generated code mapping)
- [ ] Deterministic cross-platform behavior for arithmetic and timing
- [ ] Claude Code skills for ST workflows (generate, validate, test, emit, full development lifecycle)

### Out of Scope

- Allen Bradley support in v1 — restricted ST dialect, no OOP, different tag model; defer to v2
- LLVM backend — transpile to C++ first, LLVM later when interpreter and test runner are proven
- Real-time PLC execution — this is a development/testing toolchain, not a runtime (v1.2 adds a host-paced free-running mode for HMI development, not a deterministic real-time runtime)
- Frame-level EtherCAT emulation (ESC registers, mailbox protocol, DC) — simulate at process-image/PDO level only; frame level is TE1111 / acontis territory
- Vendor IDE replacement — augments existing workflows, engineers still paste into TwinCAT/Unity Pro/Studio 5000
- GUI — CLI-first with MCP and skills for AI integration

## Context

- **Language**: Go — single static binary, fast compilation, excellent for LLM agent iteration, cross-compiles trivially
- **License**: MIT
- **Audience**: Internal team first, open-source release for the ST community
- **Prior art**: STruC++ (TypeScript, ST→C++17, 1400+ tests, GPL-3.0) validates the architecture but is GPL and TypeScript. MATIEC (C, LGPL) is proven but Ed.2 only. ControlForge and Serhioromano VS Code extensions provide existing syntax highlighting and basic LSP.
- **Test corpus**: Real-world production ST files available, plus plan to pull from open-source ST projects for broader coverage
- **Vendor landscape**: Beckhoff and Schneider are both CODESYS-derived with similar ST dialects. Allen Bradley is the hardest target (no OOP, different tag model, most restrictive dialect). The portable subset should be designed around AB's limitations even though AB support is deferred.
- **Key insight from requirements**: The fastest path to value is a series of small CLI tools (lint, format, preprocess, check, test) that compose together and that LLM agents can call

## Constraints

- **No Java runtime**: Parser and compiler must not require Java at runtime
- **Go ecosystem**: All compiler core in Go; VS Code extension in TypeScript (mandatory for VS Code)
- **Vendor compatibility**: Must handle CODESYS extensions (OOP, pointers, 64-bit types) to parse production code
- **Error recovery**: Parser must produce partial ASTs from broken code (essential for LSP)
- **Determinism**: All test execution must be deterministic — no wall-clock dependencies
- **Machine-readable output**: Every CLI command supports `--format json`

## Development Workflow

- **GitHub-first delivery**: All work delivered via PRs to `centroid-is/stc` — do not commit directly to main
- **Multi-platform CI**: GitHub Actions workflows must test on macOS, Windows, and Linux
- **Agent PR reviews**: Use other agents to review PRs before merging
- **Open repository**: Public repo, free use of all GitHub features (Actions, Issues, PRs, Releases)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over C++ | Single binary, fast builds, agents write Go well, cross-compiles trivially | — Pending |
| MIT license | Maximum permissiveness for open-source adoption | — Pending |
| Beckhoff + Schneider first | Both CODESYS-derived, similar dialects, covers primary vendor needs | — Pending |
| Transpile to C++ before LLVM | Gets tests running months earlier, LLVM deferred until proven | — Pending |
| Hand-written recursive descent parser | Full control over error recovery, critical for LSP partial ASTs | — Pending |
| Claude skills for ST workflows | Full workflow skills for generating, validating, testing, and deploying ST code | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-10-06 after Phase 20 completion*
