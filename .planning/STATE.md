---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
stopped_at: Completed 04-02-PLAN.md
last_updated: "2026-03-27T22:04:39.419Z"
last_activity: 2026-03-27
progress:
  total_phases: 11
  completed_phases: 3
  total_plans: 16
  completed_plans: 15
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-26)

**Core value:** Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor — no hardware required for development and testing.
**Current focus:** Phase 04 — standard-library-interpreter

## Current Position

Phase: 04 (standard-library-interpreter) — EXECUTING
Plan: 4 of 4
Status: Ready to execute
Last activity: 2026-03-27

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: -
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
| Phase 01 P01 | 2min | 2 tasks | 14 files |
| Phase 01 P02 | 5min | 2 tasks | 10 files |
| Phase 01 P03 | 271s | 2 tasks | 9 files |
| Phase 01 P04 | 7min | 2 tasks | 17 files |
| Phase 01 P05 | 2min | 2 tasks | 11 files |
| Phase 02-01 P01 | 3min | 2 tasks | 5 files |
| Phase 02-preprocessor P02 | 2min | 2 tasks | 7 files |
| Phase 03 P02 | 219s | 2 tasks | 5 files |
| Phase 03 P01 | 4min | 2 tasks | 6 files |
| Phase 03 P04 | 260s | 2 tasks | 7 files |
| Phase 03 P03 | 634s | 2 tasks | 12 files |
| Phase 03 P05 | 236s | 2 tasks | 9 files |
| Phase 04 P01 | 5min | 2 tasks | 7 files |
| Phase 04 P03 | 361s | 2 tasks | 6 files |
| Phase 04 P02 | 6min | 2 tasks | 6 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Init]: Interpreter only — no C++ transpiler, ever
- [Init]: No PLCopen XML — vendor interop through preprocessor ifdefs and ST re-emission
- [Init]: Go language, MIT license
- [Init]: Beckhoff + Schneider first, Allen Bradley deferred to v2
- [Init]: GitHub-first — all work via PRs, CI on macOS/Windows/Linux, agent PR reviews
- [Phase 01]: Go 1.22 minimum in go.mod; Pos uses 1-based line/col; Diagnostic format file:line:col: severity: message
- [Phase 01]: Local Pos/Span types in ast package to avoid circular imports with future source package
- [Phase 01]: JSON marshaling via centralized nodeToMap dispatch with kind discriminator on every node
- [Phase 01]: Lexer-local Pos/Span types mirroring ast.Pos/Span to avoid circular imports
- [Phase 01]: Time/date and typed literal prefixes scanned as compound tokens with # and value
- [Phase 01]: ErrorNode implements all marker interfaces for universal error recovery
- [Phase 01]: METHOD modifiers accepted both before and after keyword for CODESYS dialect compatibility
- [Phase 01]: Pratt parser with 8 IEC 61131-3 precedence levels including right-associative ** operator
- [Phase 01]: Cobra CLI with persistent --format flag; stub subcommands return exit 0; binary integration tests via TestMain
- [Phase 02-01]: Line-based preprocessing with stack-based IF nesting for IEC 61131-3 directives
- [Phase 02-01]: Diagnostic codes: PP001 (ERROR), PP002 (unmatched), PP003 (unclosed IF)
- [Phase 02-01]: Source map per-line mappings sufficient for ST line-based directives
- [Phase 02-preprocessor]: StringSlice for --define flag supports multiple defines per invocation
- [Phase 02-preprocessor]: JSON output includes source_map array and diagnostics for tool integration
- [Phase 03]: Type stored as any in Symbol to avoid circular import between symbols and types packages
- [Phase 03]: Scope keys normalized with strings.ToUpper for IEC 61131-3 case-insensitive identifiers
- [Phase 03]: IEC-strict widening only: LINT->LREAL rejected (precision loss)
- [Phase 03]: GenericConstraint as func(TypeKind) bool on Parameter for ANY_* validation
- [Phase 03]: Vendor diagnostic codes shared via diag_codes.go, not duplicated per file
- [Phase 03]: Interface variables (VAR_INPUT/OUTPUT/IN_OUT/GLOBAL/EXTERNAL) excluded from unused warnings
- [Phase 03]: Integer literals (DINT) compatible with any integer target; real literals (LREAL) compatible with any real target
- [Phase 03]: Resolver uses POU scope directly (bypassing scope stack) for Pass 1 variable registration
- [Phase 03]: Analyzer facade sequences passes: resolve -> check -> usage -> vendor (vendor only if config present)
- [Phase 03]: CLI check exits 1 on errors only; warnings alone produce exit 0
- [Phase 04]: Tagged union Value with IECType tracks precise IEC type through runtime
- [Phase 04]: Control flow (RETURN/EXIT/CONTINUE) uses typed error values for stack unwinding
- [Phase 04]: Power (**) always returns real via math.Pow per IEC EXPT semantics
- [Phase 04]: StdlibFunctions as package-level map populated via init() for simple function registration
- [Phase 04]: IEC 1-based string indexing with goIdx = iecPos - 1 conversion; FIND returns 0 for not-found
- [Phase 04]: REAL_TO_INT uses math.RoundToEven for IEC-standard banker's rounding
- [Phase 04]: FBRef field changed from any to *FBInstance for type safety
- [Phase 04]: ScanCycleEngine lazy-initializes env on first Tick, not at construction
- [Phase 04]: GetMember resolves outputs first then inputs matching PLC convention

### Pending Todos

None yet.

### Blockers/Concerns

- [Research]: GLSP (`tliron/glsp`) last updated March 2024 — validate before Phase 9 commitment
- [Research]: Two-pass type inference for IEC ANY hierarchy is the hardest technical problem — budget extra time in Phase 3
- [Research]: TwinCAT `.TcPOU` file format not publicly documented — may need reverse engineering in Phase 7

## Session Continuity

Last session: 2026-03-27T22:04:39.417Z
Stopped at: Completed 04-02-PLAN.md
Resume file: None
