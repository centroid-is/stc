# Phase 4: Standard Library & Interpreter - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can execute ST programs on their development machine with correct PLC scan-cycle semantics and IEC standard library support, no hardware required. Delivers the interpreter engine and all standard FBs/functions needed for the test runner (Phase 5).

</domain>

<decisions>
## Implementation Decisions

### Interpreter Architecture
- Tree-walking AST interpreter in Go — simplest approach, matches "interpreter only" project decision
- Explicit `Tick(dt)` scan cycle: read inputs → execute program body → write outputs → advance time
- Each FB instance gets its own scope with persistent state across scan cycles (standard PLC behavior)
- Go `time.Duration` for all time values — deterministic, injected via `Tick(dt)`, no wall-clock dependency

### Standard Library Implementation
- All standard library FBs and functions implemented in Go — full control, deterministic, easier to test
- Microsecond-level timer precision (match PLC behavior, compare accumulated vs preset)
- Fixed-length strings (STRING[80] default per IEC) backed by Go strings with length validation at boundaries
- Full IEC standard library coverage per REQUIREMENTS.md:
  - Timers: TON, TOF, TP
  - Counters: CTU, CTD, CTUD
  - Edge detection: R_TRIG, F_TRIG
  - Bistable: SR, RS
  - Math: ABS, SQRT, MIN, MAX, SEL, MUX, LIMIT, etc.
  - String: LEN, LEFT, RIGHT, MID, CONCAT, FIND, etc.
  - Type conversion: INT_TO_REAL, BOOL_TO_INT, etc.
- All FBs accept injected time for deterministic testing

### Claude's Discretion
- Internal interpreter value representation
- Standard function parameter naming
- Test fixture organization
- Error message formatting for runtime errors

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/ast/` — CST node hierarchy with visitor/walker
- `pkg/types/` — IEC type system with widening rules and built-in function signatures
- `pkg/symbols/` — Hierarchical symbol table with scope chain
- `pkg/checker/` — Two-pass type checker (resolve + check)
- `pkg/analyzer/` — Analyzer facade orchestrating all passes
- `pkg/diag/` — Diagnostic types

### Established Patterns
- Table-driven tests with testdata/ directories
- Two-result pattern: `(result, diagnostics)`
- CLI follows Cobra command pattern

### Integration Points
- Interpreter will be consumed by test runner (Phase 5) and simulation (Phase 6)
- Standard library FBs must integrate with interpreter's time injection mechanism
- `stc run` or similar CLI command (or just used internally by test runner)

</code_context>

<specifics>
## Specific Ideas

- Standard library FBs should all support deterministic time injection for testing (STLB-08)
- Interpreter must handle all control structures, expressions, and FB instance calls (INTP-04)
- Scan-cycle semantics are critical for timer correctness

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
