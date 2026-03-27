---
phase: 04-standard-library-interpreter
plan: 02
subsystem: interpreter
tags: [scan-cycle, function-block, fb-instance, deterministic-time, plc-runtime]

requires:
  - phase: 04-01
    provides: "Interpreter core with Value types, Env scoping, expression eval, statement exec"
provides:
  - "StandardFB interface for all stdlib FB implementations"
  - "FBInstance wrapper for both stdlib and user-defined FBs"
  - "ScanCycleEngine with deterministic Tick(dt) and I/O table"
  - "CallStmt and MemberAccessExpr wiring for FB calls"
  - "StdlibFBFactory registration map for plan 04"
affects: [04-03, 04-04, 05-testing]

tech-stack:
  added: []
  patterns:
    - "StandardFB interface contract for all stdlib FBs"
    - "ScanCycleEngine read-inputs/execute/write-outputs/advance-clock cycle"
    - "Lazy environment initialization on first Tick"
    - "Case-insensitive I/O keys via strings.ToUpper"
    - "FBInstance dual-mode: StandardFB delegation vs user-defined Env+Decl"

key-files:
  created:
    - pkg/interp/fb_instance.go
    - pkg/interp/fb_instance_test.go
    - pkg/interp/scan.go
    - pkg/interp/scan_test.go
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/value.go

key-decisions:
  - "FBRef field changed from any to *FBInstance for type safety"
  - "GetMember resolves outputs first, then inputs, matching PLC convention"
  - "ScanCycleEngine lazy-initializes env on first Tick, not at construction"
  - "ErrReturn swallowed in FB/program execution (normal PLC termination)"

patterns-established:
  - "StandardFB interface: Execute(dt), SetInput, GetOutput, GetInput"
  - "FBInstance dual-mode wrapping for stdlib vs user-defined FBs"
  - "ScanCycleEngine Tick(dt) ordering: inputs -> execute -> outputs -> clock"
  - "zeroFromTypeSpec resolves NamedType to Zero(kind) via LookupElementaryType"

requirements-completed: [INTP-01, INTP-02, INTP-03]

duration: 6min
completed: 2026-03-27
---

# Phase 04 Plan 02: Scan Cycle Engine and FB Instance Management Summary

**ScanCycleEngine with deterministic Tick(dt), StandardFB interface for stdlib FBs, and FBInstance dual-mode wrapper for persistent FB state across scan cycles**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-27T21:57:25Z
- **Completed:** 2026-03-27T22:03:32Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- StandardFB interface defining the contract for all IEC standard library function blocks (TON, TOF, CTU, etc.)
- FBInstance wrapping both stdlib FBs (via StandardFB delegation) and user-defined FBs (via persistent Env + AST body execution)
- ScanCycleEngine implementing deterministic scan cycles: read inputs -> execute body -> write outputs -> advance virtual clock
- CallStmt and MemberAccessExpr wired in interpreter for FB call/access semantics
- Programmatic I/O API (SetInput/GetOutput) with case-insensitive keys for testing

## Task Commits

Each task was committed atomically:

1. **Task 1: StandardFB interface and FBInstance management** - `48b046f` (feat)
2. **Task 2: Scan cycle engine with deterministic time and I/O table** - `ebf1f09` (feat)

## Files Created/Modified
- `pkg/interp/fb_instance.go` - StandardFB interface, FBInstance struct, NewUserFBInstance, StdlibFBFactory
- `pkg/interp/fb_instance_test.go` - Tests for stdlib/user-defined FB instances, call semantics, member access
- `pkg/interp/scan.go` - ScanCycleEngine with Tick(dt), SetInput, GetOutput, Clock, lazy env init
- `pkg/interp/scan_test.go` - Tests for scan cycle, deterministic clock, I/O access, state persistence
- `pkg/interp/interpreter.go` - Added dt field, execCallStmt, evalMemberAccess, execAssignMember
- `pkg/interp/value.go` - Changed FBRef from any to *FBInstance

## Decisions Made
- FBRef field changed from `any` to `*FBInstance` for type safety and direct field access without type assertions
- GetMember checks outputs first, then inputs, matching PLC convention (fb.Q resolves to output Q)
- ScanCycleEngine lazy-initializes env on first Tick (not at construction) to allow SetInput before first cycle
- ErrReturn swallowed in both FB and program execution as normal termination per PLC semantics

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Pre-existing test failures in stdlib_string_test.go (TestLEN, TestLEFT, etc.) due to unregistered string functions from a prior plan. Not caused by these changes and out of scope.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- StandardFB interface ready for plan 03/04 to implement TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, SR, RS
- ScanCycleEngine ready for integration testing with real ST programs
- StdlibFBFactory map ready to receive factory registrations

---
*Phase: 04-standard-library-interpreter*
*Completed: 2026-03-27*
