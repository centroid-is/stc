---
phase: 05-testing-framework
plan: 02
subsystem: testing
tags: [test-runner, junit-xml, json-output, cli, discovery, isolation, advance-time]

# Dependency graph
requires:
  - phase: 05-01
    provides: TestCaseDecl AST node, AssertionCollector, assertion LocalFunctions, ADVANCE_TIME
  - phase: 04-interpreter-engine
    provides: Interpreter with evalCall, ScanCycleEngine, Value types, StdlibFBFactory
provides:
  - pkg/testing package with Run(), DiscoverTestFiles(), FormatJUnit(), FormatJSON()
  - stc test CLI command with text/junit/json output formats
  - Exported interpreter methods: ExecStatements, EvalExpr, SetDt, ZeroFromTypeSpec, MakeFBInstanceValue
  - Expression-statement support in interpreter (nil-Value AssignStmt)
affects: [06-simulation, 09-lsp]

# Tech tracking
tech-stack:
  added: []
  patterns: [encoding/xml JUnit output, test file discovery via filepath.Walk, per-test-case isolation]

key-files:
  created:
    - pkg/testing/runner.go
    - pkg/testing/result.go
    - pkg/testing/junit.go
    - pkg/testing/json_output.go
    - pkg/testing/runner_test.go
    - pkg/testing/testdata/passing_test.st
    - pkg/testing/testdata/failing_test.st
    - pkg/testing/testdata/timer_test.st
    - pkg/testing/testdata/multi_assert_test.st
    - cmd/stc/test_cmd.go
    - cmd/stc/test_cmd_test.go
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/fb_instance.go
    - cmd/stc/stubs.go
    - cmd/stc/main_test.go

key-decisions:
  - "Import testing package as stctesting to avoid collision with Go standard testing package"
  - "Expression-statement support: nil Value in AssignStmt evaluates Target for side effects (assertion calls)"
  - "Export thin wrappers (ExecStatements, EvalExpr, SetDt, ZeroFromTypeSpec, MakeFBInstanceValue) rather than making internal methods public"
  - "JUnit failure type set to AssertionFailure with concatenated assertion messages"

patterns-established:
  - "Test isolation: each TEST_CASE gets fresh Interpreter + Env + AssertionCollector"
  - "ADVANCE_TIME sets interpreter.dt via SetDt so subsequent FB calls see correct delta"
  - "CLI test command: exit 0 all pass, exit 1 any failure, supports --format text/json/junit"

requirements-completed: [TEST-02, TEST-03, TEST-04, TEST-07]

# Metrics
duration: 6min
completed: 2026-03-28
---

# Phase 05 Plan 02: Test Runner and CLI Summary

**Test runner discovering *_test.st files with isolated TEST_CASE execution, JUnit XML and JSON output formatters, and stc test CLI command**

## Performance

- **Duration:** 6 min (391s)
- **Started:** 2026-03-28T07:50:30Z
- **Completed:** 2026-03-28T07:57:01Z
- **Tasks:** 2
- **Files modified:** 15

## Accomplishments
- Test runner discovers *_test.st files recursively and executes each TEST_CASE in isolation with its own interpreter, environment, and assertion collector
- ADVANCE_TIME works with TON timer FBs in test context for deterministic timer testing
- JUnit XML output valid for CI integration with testsuites/testsuite/testcase structure
- JSON output with structured results including assertion details and file:line:col positions
- stc test CLI command replaces stub with text/junit/json output format support
- Expression-statement support added to interpreter enabling bare function call statements

## Task Commits

Each task was committed atomically:

1. **Task 1: Test runner package with discovery, execution, and output formatters** - `ed88e8f` (test: RED), `9055553` (feat: GREEN)
2. **Task 2: CLI stc test command replacing stub** - `c8ef49e` (feat)

_Note: TDD tasks have two commits each (test then implementation)_

## Files Created/Modified
- `pkg/testing/runner.go` - Test discovery (filepath.Walk) and execution orchestration with per-test isolation
- `pkg/testing/result.go` - TestResult, SuiteResult, RunResult types with JSON tags
- `pkg/testing/junit.go` - JUnit XML output using encoding/xml with proper hierarchy
- `pkg/testing/json_output.go` - JSON output using json.MarshalIndent
- `pkg/testing/runner_test.go` - 8 Go tests covering passing, failing, timer, multi-assert, empty dir, isolation, JUnit, JSON
- `pkg/testing/testdata/*.st` - 4 test fixture files
- `cmd/stc/test_cmd.go` - Real stc test CLI command with text/junit/json output
- `cmd/stc/test_cmd_test.go` - 6 integration tests for CLI command
- `cmd/stc/stubs.go` - Removed newTestCmd stub
- `cmd/stc/main_test.go` - Updated stub tests to exclude test from stub list
- `pkg/interp/interpreter.go` - Added ExecStatements, EvalExpr, SetDt exports; expression-statement handling
- `pkg/interp/fb_instance.go` - Added ZeroFromTypeSpec, MakeFBInstanceValue exports

## Decisions Made
- Used `stctesting` import alias for `pkg/testing` to avoid collision with Go's `testing` package
- Expression statements (bare function calls as statements) handled by checking nil Value in AssignStmt and evaluating Target for side effects -- this is a general-purpose fix that enables assertion calls like `ASSERT_EQ(x, y);`
- Exported thin wrapper methods on Interpreter rather than making internal methods public, maintaining clean API boundary
- JUnit failure messages concatenate all failed assertion messages with file:line:col positions

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Expression-statement support for assertion calls**
- **Found during:** Task 1 (test runner implementation)
- **Issue:** Parser creates AssignStmt with nil Value for expression statements (e.g., `ASSERT_EQ(x, y);`). Interpreter crashes with "unsupported expression type: <nil>" when evaluating nil Value.
- **Fix:** Added nil-Value check in execAssign to evaluate Target for side effects instead of trying to evaluate nil Value
- **Files modified:** pkg/interp/interpreter.go
- **Verification:** All test cases now execute assertions correctly
- **Committed in:** 9055553 (part of Task 1 GREEN commit)

**2. [Rule 1 - Bug] Updated existing stub tests after removing test stub**
- **Found during:** Task 2 (CLI command implementation)
- **Issue:** TestCLI_StubCommands and TestCLI_StubCommandsJSON still tested "test" as a stub, but it's now a real command
- **Fix:** Removed "test" from stub test list, changed StubCommandsJSON to test "emit" instead
- **Files modified:** cmd/stc/main_test.go
- **Verification:** All existing CLI tests pass
- **Committed in:** c8ef49e (part of Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both auto-fixes necessary for correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- Test runner and CLI complete, ready for use in CI pipelines
- JUnit XML output compatible with standard CI test result parsers
- Foundation ready for Phase 06 simulation work (shares interpreter isolation pattern)

## Self-Check: PASSED

All created files verified. All commit hashes verified.

---
*Phase: 05-testing-framework*
*Completed: 2026-03-28*
