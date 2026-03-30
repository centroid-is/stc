---
phase: 12-io-address-parser-table
plan: 02
subsystem: interp, checker
tags: [iomap, iotable, scan-cycle, at-address, overlap-detection]

requires:
  - phase: 12-01
    provides: "IOTable byte-array storage and ParseAddress parser"
provides:
  - "ScanCycleEngine I/O sync via IOTable for AT-addressed variables"
  - "Checker AT address format validation and POU type restriction"
  - "Checker overlap detection for AT addresses in the same I/O area"
affects: [vendor-stubs, mock-framework, simulation]

tech-stack:
  added: []
  patterns:
    - "IOBinding pattern: AT addresses parsed during initializeEnv, synced in Tick"
    - "Bidirectional memory sync: %M* vars read at Tick start and write at Tick end"

key-files:
  created: []
  modified:
    - pkg/interp/scan.go
    - pkg/interp/scan_test.go
    - pkg/checker/check.go
    - pkg/checker/check_test.go
    - pkg/checker/diag_codes.go

key-decisions:
  - "AT-bound input vars synced before staged inputs, so SetInput can override IOTable values"
  - "Wildcard AT addresses (%I*) silently skipped (no binding created)"
  - "Overlap detection uses byte-span ranges; two bit addresses on same byte but different bits are not overlapping"

patterns-established:
  - "IOBinding type pairs variable names with parsed IOAddress for scan-cycle sync"
  - "Checker validates AT addresses in CheckBodies alongside body type-checking"

requirements-completed: [IO-02, IO-03, IO-05]

duration: 4min
completed: 2026-03-30
---

# Phase 12 Plan 02: IOTable Scan-Cycle Wiring & Checker AT Validation Summary

**ScanCycleEngine reads/writes AT-addressed variables via IOTable with bidirectional memory sync, checker validates AT format and detects overlapping addresses**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-30T10:54:35Z
- **Completed:** 2026-03-30T10:58:49Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- ScanCycleEngine reads AT-bound input/memory variables from IOTable before program execution
- ScanCycleEngine writes AT-bound output/memory variables to IOTable after execution
- IOTable() accessor allows external code to inject test I/O values
- Checker validates AT address format via iomap.ParseAddress, emits errors on malformed addresses
- Checker warns when AT addresses used in FUNCTION_BLOCK or FUNCTION (only valid in PROGRAM)
- Checker detects overlapping AT address byte ranges within the same I/O area

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire IOTable into ScanCycleEngine** - `4d9dc7c` (test) + `b073d5f` (feat)
2. **Task 2: Checker AT address validation and overlap detection** - `7be5b88` (test) + `9281537` (feat)

_Note: TDD tasks have two commits each (test then feat)_

## Files Created/Modified
- `pkg/interp/scan.go` - Added IOBinding type, ioTable/ioBindings fields, I/O sync in Tick, readIOValue/writeIOValue helpers
- `pkg/interp/scan_test.go` - 11 new tests for IOTable integration (input/output bit/word/dword, memory bidirectional, wildcards)
- `pkg/checker/check.go` - Added checkATAddresses method with format validation, POU type check, overlap detection
- `pkg/checker/check_test.go` - 9 new tests for AT validation (valid/invalid, FUNCTION_BLOCK restriction, overlap cases)
- `pkg/checker/diag_codes.go` - Added SEMA030 (InvalidATAddress), SEMA031 (ATNotAllowedHere), SEMA032 (ATOverlap)

## Decisions Made
- AT-bound input vars are synced before staged inputs in Tick, allowing SetInput to override IOTable values for testing flexibility
- Wildcard AT addresses (%I*) are silently skipped -- no I/O binding created since they have no concrete address
- Overlap detection operates on byte-span ranges; two bit addresses in the same byte but different bit offsets are NOT considered overlapping

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Changed invalid AT test case from %ZZ0 to %IX0.9**
- **Found during:** Task 2 (checker AT validation tests)
- **Issue:** %ZZ0 is rejected by the lexer as Illegal token and never reaches the checker; the test would never exercise ParseAddress validation
- **Fix:** Changed test to use %IX0.9 (bit offset 9, out of range 0-7) which passes lexer but fails ParseAddress
- **Files modified:** pkg/checker/check_test.go
- **Verification:** Test passes, confirms checker emits SEMA030 for out-of-range bit offset
- **Committed in:** 9281537 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Test correction necessary for accurate coverage. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Full I/O pipeline complete: lexer tokenizes AT addresses, parser stores them, checker validates them, interpreter syncs them with IOTable
- Ready for vendor stub loading and mock framework phases that will use AT-addressed variables
- IOTable is externally accessible for test input injection

---
*Phase: 12-io-address-parser-table*
*Completed: 2026-03-30*
