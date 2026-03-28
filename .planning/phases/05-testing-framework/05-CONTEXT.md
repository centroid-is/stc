# Phase 5: Testing Framework - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can write and run unit tests for ST code on their development machine with no PLC hardware. Tests use a simple DSL in *_test.st files with TcUnit-style assertions, deterministic time advancement, and I/O mocking. Test runner outputs JUnit XML for CI and JSON for agent consumption.

</domain>

<decisions>
## Implementation Decisions

### Test DSL Design
- `TEST_CASE 'name' ... END_TEST_CASE` blocks in .st files — new AST node types added to parser
- TcUnit-style assertions: `ASSERT_TRUE(expr)`, `ASSERT_FALSE(expr)`, `ASSERT_EQ(a, b)`, `ASSERT_NEAR(a, b, eps)`
- `ADVANCE_TIME(T#100ms)` function for explicit deterministic time advancement
- Inline I/O mocking: TEST_CASE declares VAR_INPUT/VAR_OUTPUT, sets inputs before assertions, reads outputs after

### Test Runner Architecture
- `stc test <dir>` recursively discovers `*_test.st` files (convention over config)
- JUnit XML output with testsuite/testcase elements, stdout capture for assertions
- Failure messages reference original ST file:line:col using debug source maps (DBUG-01, DBUG-02)
- Exit 0 on all pass, exit 1 on any failure — standard CI convention
- JSON output via `--format json`

### Claude's Discretion
- Internal test execution order (sequential is fine — PLC tests are deterministic)
- JUnit XML schema details
- Assertion message formatting
- Test timeout implementation

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/interp/` — Full interpreter with scan cycle engine, stdlib
- `pkg/parser/` — Parser to extend with TEST_CASE nodes
- `pkg/ast/` — AST to extend with test-related nodes
- `pkg/analyzer/` — Analyzer facade for pre-test type checking
- `cmd/stc/stubs.go` — Test stub to replace

### Established Patterns
- `*_test.go` convention in Go; analogous `*_test.st` for ST tests
- CLI follows Cobra pattern with --format flag
- Table-driven tests in Go for the test runner itself

### Integration Points
- Parser must learn TEST_CASE, ASSERT_*, ADVANCE_TIME
- Interpreter must handle assertion execution and time advancement
- CLI `stc test` wires discovery → parse → analyze → interpret → report

</code_context>

<specifics>
## Specific Ideas

- Source debug mapping (DBUG-01, DBUG-02) critical for useful test failure messages
- This is the killer differentiator — no open-source ST tool does host-based testing

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
