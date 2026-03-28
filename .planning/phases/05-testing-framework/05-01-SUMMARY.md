---
phase: 05-testing-framework
plan: 01
subsystem: testing
tags: [test-case, assertions, advance-time, ast, parser, lexer, interpreter]

# Dependency graph
requires:
  - phase: 04-interpreter-engine
    provides: Interpreter with evalCall, StdlibFunctions, ScanCycleEngine, Value types
provides:
  - TestCaseDecl AST node for TEST_CASE blocks
  - KwTestCase and KwEndTestCase lexer keywords
  - parseTestCase parser method
  - AssertionCollector for gathering pass/fail results
  - ASSERT_TRUE, ASSERT_FALSE, ASSERT_EQ, ASSERT_NEAR interpreter functions
  - ADVANCE_TIME interpreter function
  - LocalFunctions dispatch on Interpreter (per-instance override over StdlibFunctions)
affects: [05-02-test-runner, 06-simulation]

# Tech tracking
tech-stack:
  added: []
  patterns: [LocalFunctions per-interpreter dispatch, position-aware function signature, AssertionCollector]

key-files:
  created:
    - pkg/ast/test_nodes.go
    - pkg/interp/assertions.go
    - pkg/parser/test_parse_test.go
    - pkg/interp/assertions_test.go
  modified:
    - pkg/lexer/token.go
    - pkg/lexer/keywords.go
    - pkg/ast/node.go
    - pkg/ast/json.go
    - pkg/parser/decl.go
    - pkg/interp/interpreter.go

key-decisions:
  - "LocalFunctions map[string]func(args []Value, pos ast.Pos) on Interpreter for test-specific functions, avoiding global StdlibFunctions mutation"
  - "TestCaseDecl.Name is string (not *Ident) because test names come from string literals"
  - "Assertions return BoolValue(true) always, recording failures on collector instead of aborting execution"
  - "Position-aware LocalFunctions signature passes CallExpr source pos for assertion error messages"

patterns-established:
  - "LocalFunctions: Per-interpreter instance function overrides checked before global StdlibFunctions"
  - "AssertionCollector: Accumulate test results without aborting, filter failures after execution"
  - "TestCaseDecl: Testing POU pattern following ProgramDecl structure with string name"

requirements-completed: [TEST-01, TEST-05, TEST-06, DBUG-01, DBUG-02]

# Metrics
duration: 4min
completed: 2026-03-28
---

# Phase 05 Plan 01: TEST_CASE Language Support Summary

**TEST_CASE/END_TEST_CASE parsing with assertion functions (ASSERT_TRUE/FALSE/EQ/NEAR), ADVANCE_TIME, and per-interpreter LocalFunctions dispatch**

## Performance

- **Duration:** 4 min (278s)
- **Started:** 2026-03-28T07:43:29Z
- **Completed:** 2026-03-28T07:48:07Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- TEST_CASE blocks parse into TestCaseDecl AST nodes with string name, VarBlocks, and Body
- ASSERT_TRUE, ASSERT_FALSE, ASSERT_EQ, ASSERT_NEAR execute as interpreter LocalFunctions collecting pass/fail with source positions
- ADVANCE_TIME advances virtual clock via callback, enabling deterministic time control
- LocalFunctions per-interpreter dispatch ensures test isolation (no global state mutation)

## Task Commits

Each task was committed atomically:

1. **Task 1: Lexer, AST, and parser extensions for TEST_CASE** - `02e9360` (test: RED), `8911949` (feat: GREEN)
2. **Task 2: Assertion functions, ADVANCE_TIME, and source position tracking** - `5ee6591` (test: RED), `8ae0a0b` (feat: GREEN)

_Note: TDD tasks have two commits each (test then implementation)_

## Files Created/Modified
- `pkg/ast/test_nodes.go` - TestCaseDecl struct (Name, VarBlocks, Body, Children, declNode)
- `pkg/interp/assertions.go` - AssertionCollector, RegisterAssertions, RegisterAdvanceTime
- `pkg/parser/test_parse_test.go` - 6 parser tests for TEST_CASE parsing
- `pkg/interp/assertions_test.go` - 14 tests for assertions, ADVANCE_TIME, collector
- `pkg/lexer/token.go` - KwTestCase, KwEndTestCase token kinds
- `pkg/lexer/keywords.go` - TEST_CASE, END_TEST_CASE keyword entries
- `pkg/ast/node.go` - KindTestCaseDecl node kind
- `pkg/ast/json.go` - TestCaseDecl JSON marshaling case
- `pkg/parser/decl.go` - parseTestCase method, KwTestCase dispatch in parseDeclaration
- `pkg/interp/interpreter.go` - LocalFunctions and Collector fields, LocalFunctions check in evalCall

## Decisions Made
- Used LocalFunctions with position-aware signature `func(args []Value, pos ast.Pos) (Value, error)` instead of adding assertions to global StdlibFunctions. This avoids test isolation issues (Pitfall 4 from research).
- TestCaseDecl.Name is `string` not `*Ident` because test names are string literals, not identifiers.
- Assertions always return `BoolValue(true)` to keep execution flowing after failures.
- Both single-quoted and double-quoted (WStringLiteral) test names supported in parser.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- TestCaseDecl AST node ready for test runner discovery (Plan 02)
- AssertionCollector ready for test runner result aggregation
- ADVANCE_TIME ready for deterministic time control in test execution
- LocalFunctions pattern ready for per-test-case isolation in runner

## Self-Check: PASSED

All 4 created files verified. All 4 commit hashes verified.

---
*Phase: 05-testing-framework*
*Completed: 2026-03-28*
