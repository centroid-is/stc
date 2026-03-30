---
phase: 14-mock-framework
plan: 02
subsystem: testing
tags: [mock, function-block, auto-stub, fidelity, io-injection, test-runner]

# Dependency graph
requires:
  - phase: 14-mock-framework
    provides: "LoadMocks, ValidateMockSignatures, ResolveOpts.MockFiles, TestConfig.MockPaths"
  - phase: 13-vendor-stub-loading
    provides: "LoadLibraries, library stub files"
  - phase: 12-io-table
    provides: "IOTable with SetBit/GetBit for I/O injection"
provides:
  - "RunOpts and RunWithOpts for mock-aware test execution"
  - "Auto-stub fidelity warnings for library FBs without mocks"
  - "SET_IO/GET_IO test functions for I/O table injection"
  - "CLI wiring of mock loading from stc.toml into test pipeline"
  - "Fidelity warning output in text and JSON formats"
affects: [15-shipped-stubs, testing]

# Tech tracking
tech-stack:
  added: []
  patterns: ["External FB context merged into test file context with mock-override-stub ordering", "IOTable created per test case for isolated I/O injection"]

key-files:
  created: []
  modified:
    - pkg/testing/runner.go
    - pkg/testing/result.go
    - pkg/testing/runner_test.go
    - cmd/stc/test_cmd.go
    - cmd/stc/test_cmd_test.go

key-decisions:
  - "Auto-stub tracking at file level aggregated to run level for deduplication"
  - "SET_IO/GET_IO use string area identifiers (I/Q/M) for simplicity in ST test syntax"
  - "IOTable created per test case for isolation, not shared across tests"
  - "FindConfig failure (no stc.toml) falls back silently to Run() behavior"

patterns-established:
  - "externalContext struct pattern separates library stubs from mock FBs for clear override semantics"
  - "registerIOFunctions pattern adds test-only functions to interpreter via RegisterFunction"

requirements-completed: [MOCK-05, IO-04]

# Metrics
duration: 5min
completed: 2026-03-30
---

# Phase 14 Plan 02: Mock Framework Integration Summary

**Mock-aware test runner with auto-stub fidelity warnings, SET_IO/GET_IO I/O injection, and CLI wiring from stc.toml mock_paths**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-30T12:00:33Z
- **Completed:** 2026-03-30T12:05:12Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- RunWithOpts accepts LibraryFiles and MockFiles, merging external FB declarations into test file context
- Auto-stubbed FBs (library stubs without mocks) return zero values and emit fidelity warnings
- SET_IO/GET_IO functions allow test cases to inject and read I/O table values per test case
- stc test CLI loads stc.toml, reads mock_paths and library_paths, passes files to RunWithOpts
- Fidelity warnings printed as [fidelity] prefixed lines in text output, included in JSON via struct tag
- Full test suite (26 packages) passes with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add RunOpts with mock/library support and auto-stub fidelity warnings** - `875d5b2` (test), `a85571a` (feat)
2. **Task 2: Wire mock loading into CLI test command and print fidelity warnings** - `de1b66c` (feat)

## Files Created/Modified
- `pkg/testing/runner.go` - RunOpts, RunWithOpts, externalContext, SET_IO/GET_IO registration, auto-stub tracking
- `pkg/testing/result.go` - Warnings field on RunResult
- `pkg/testing/runner_test.go` - 5 new tests for mock FB, auto-stub, I/O injection, backward compatibility
- `cmd/stc/test_cmd.go` - Config loading, LoadLibraries, LoadMocks wiring, fidelity warning output
- `cmd/stc/test_cmd_test.go` - 2 integration tests for mock FB execution and fidelity warnings
- `pkg/testing/testdata/mock_fb_test.st` - Test fixture for mock FB
- `pkg/testing/testdata/mock_mc_moveabsolute.st` - Mock FB fixture
- `pkg/testing/testdata/stub_mc_moveabsolute.st` - Library stub fixture
- `pkg/testing/testdata/autostub_test.st` - Auto-stub test fixture
- `pkg/testing/testdata/io_inject_test.st` - I/O injection test fixture

## Decisions Made
- Auto-stub tracking done at file level, aggregated at run level with deduplication via map
- SET_IO/GET_IO use string area identifiers ("I", "Q", "M") -- matches ST developer mental model
- IOTable created fresh per test case for isolation (no cross-test contamination)
- When no stc.toml found, CLI silently falls back to RunWithOpts with empty opts (backward compatible)
- Fidelity warnings sorted alphabetically for deterministic output

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Mock framework complete: infrastructure (Plan 01) + integration (Plan 02)
- Ready for Phase 15 (shipped stubs) to provide vendor-specific library stubs
- SET_IO/GET_IO ready for I/O-intensive test scenarios

---
*Phase: 14-mock-framework*
*Completed: 2026-03-30*
