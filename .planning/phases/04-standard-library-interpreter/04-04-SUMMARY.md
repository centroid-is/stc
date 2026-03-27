---
phase: 04-standard-library-interpreter
plan: 04
subsystem: interpreter
tags: [timers, counters, edge-detection, bistable, function-blocks, iec-61131-3, scan-cycle]

# Dependency graph
requires:
  - phase: 04-standard-library-interpreter/04-02
    provides: "StandardFB interface, FBInstance, ScanCycleEngine, StdlibFBFactory map"
  - phase: 04-standard-library-interpreter/04-03
    provides: "StdlibFunctions map with math/string/conversion functions"
provides:
  - "TON, TOF, TP timer function blocks with deterministic time"
  - "CTU, CTD, CTUD counter function blocks with rising edge detection"
  - "R_TRIG, F_TRIG edge detection function blocks"
  - "SR, RS bistable function blocks"
  - "CallExpr dispatch to StdlibFunctions"
  - "FB instance creation from StdlibFBFactory during env init"
  - "End-to-end parse->interpret pipeline"
affects: [test-runner, simulation, lsp]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "StandardFB implementation pattern: struct with Execute/SetInput/GetOutput/GetInput"
    - "init() registration in StdlibFBFactory for FB discovery"
    - "Named-arg FB call lookahead in parser postfix"

key-files:
  created:
    - pkg/interp/stdlib_timers.go
    - pkg/interp/stdlib_timers_test.go
    - pkg/interp/stdlib_counters.go
    - pkg/interp/stdlib_counters_test.go
    - pkg/interp/stdlib_edge.go
    - pkg/interp/stdlib_edge_test.go
    - pkg/interp/stdlib_bistable.go
    - pkg/interp/stdlib_bistable_test.go
    - pkg/interp/integration_test.go
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/scan.go
    - pkg/interp/fb_instance.go
    - pkg/interp/fb_instance_test.go
    - pkg/parser/expr.go

key-decisions:
  - "Parser lookahead for named-arg FB calls: isNamedArgCall checks ident := or ident => after LParen to distinguish FB CallStmt from expression CallExpr"

patterns-established:
  - "StandardFB pattern: Go struct with exported input/output fields, case-insensitive SetInput/GetOutput using strings.ToUpper"
  - "Timer FBs accept time via Execute(dt), never wall clock"
  - "Edge detection via prevCLK comparison within Execute"

requirements-completed: [STLB-01, STLB-02, STLB-03, STLB-04, STLB-08, INTP-04]

# Metrics
duration: 6min
completed: 2026-03-27
---

# Phase 04 Plan 04: Standard Library FBs and Integration Summary

**All 10 IEC standard library FBs (TON/TOF/TP timers, CTU/CTD/CTUD counters, R_TRIG/F_TRIG edge, SR/RS bistable) with deterministic time and end-to-end parse-to-interpret pipeline**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-27T22:05:41Z
- **Completed:** 2026-03-27T22:12:33Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments
- All 10 standard library function blocks implementing IEC 61131-3 semantics with deterministic time
- 28 unit tests covering timer edge cases (ET capping, re-triggering, pulse timing), counter rising edge detection, bistable dominance
- CallExpr dispatch to StdlibFunctions enabling expression-position function calls
- FB instance creation from StdlibFBFactory during ScanCycleEngine env initialization
- 5 end-to-end integration tests: ST source string parsed, interpreted, outputs verified
- Parser fix for named-argument FB call syntax disambiguation

## Task Commits

Each task was committed atomically:

1. **Task 1: Standard library FBs (timers, counters, edge, bistable)** - `33611ae` (feat)
2. **Task 2: Integration wiring and end-to-end tests** - `6cf1fd6` (feat)

## Files Created/Modified
- `pkg/interp/stdlib_timers.go` - TON, TOF, TP timer implementations with deterministic time
- `pkg/interp/stdlib_timers_test.go` - 9 timer tests covering on-delay, off-delay, pulse timing
- `pkg/interp/stdlib_counters.go` - CTU, CTD, CTUD counter implementations with rising edge detection
- `pkg/interp/stdlib_counters_test.go` - 8 counter tests including edge detection, reset priority
- `pkg/interp/stdlib_edge.go` - R_TRIG, F_TRIG edge detection (one-scan pulse output)
- `pkg/interp/stdlib_edge_test.go` - 5 edge detection tests for transition behavior
- `pkg/interp/stdlib_bistable.go` - SR (set-dominant), RS (reset-dominant) bistable FBs
- `pkg/interp/stdlib_bistable_test.go` - 6 bistable tests including dominance and memory behavior
- `pkg/interp/integration_test.go` - 5 end-to-end tests: arithmetic, TON timer, CTU counter, string function, FOR loop
- `pkg/interp/interpreter.go` - Added evalCall for StdlibFunctions dispatch
- `pkg/interp/scan.go` - FB instance creation from StdlibFBFactory during initializeEnv
- `pkg/interp/fb_instance.go` - Added typeNameFromSpec helper
- `pkg/interp/fb_instance_test.go` - Updated TestStdlibFBFactory to verify all 10 registrations
- `pkg/parser/expr.go` - Added isNamedArgCall lookahead to distinguish FB calls from expression calls

## Decisions Made
- Parser lookahead for named-arg FB calls: `isNamedArgCall` checks if `(` is followed by `ident :=` or `ident =>` to prevent the expression parser from greedily consuming FB call syntax that should be handled as a CallStmt

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated TestStdlibFBFactory for populated factory**
- **Found during:** Task 1
- **Issue:** Pre-existing test expected StdlibFBFactory to be empty (placeholder from plan 02)
- **Fix:** Updated test to verify all 10 FB constructors are registered
- **Files modified:** pkg/interp/fb_instance_test.go
- **Verification:** go test passes
- **Committed in:** 33611ae (Task 1 commit)

**2. [Rule 3 - Blocking] Parser named-arg FB call disambiguation**
- **Found during:** Task 2
- **Issue:** Expression parser greedily consumed `(` after identifier, treating `myTimer(IN := val)` as a CallExpr. The `:=` inside parens caused parse errors.
- **Fix:** Added `isNamedArgCall()` lookahead in parser postfix to detect named-arg pattern and return early, letting the statement parser handle it as CallStmt
- **Files modified:** pkg/parser/expr.go
- **Verification:** All parser tests pass (no regressions), integration tests pass
- **Committed in:** 6cf1fd6 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations.

## Known Stubs
None - all FBs are fully implemented with correct IEC semantics.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 04 (standard-library-interpreter) is complete: all plans executed
- Full interpreter with expression evaluation, control flow, standard functions, and function blocks
- Ready for Phase 05 (test runner, simulation, or LSP)

---
*Phase: 04-standard-library-interpreter*
*Completed: 2026-03-27*
