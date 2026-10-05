---
phase: 19-twincat-declaration-syntax
plan: 04
subsystem: parser, checker, interp
tags: [parser, emit, format, checker, interp, twincat, at-address, call-args]

requires:
  - phase: 19-02
    provides: StructMember.AtAddress field
  - phase: 19-03
    provides: emitStructMember helpers, attribute attachment on struct members
provides:
  - parseStructMember accepts `name AT <addr> : TYPE [:= init];`
  - parseCallArg keeps `name :=` / `name =>` followed by `,` or `)` with a nil Value
  - emit and format print struct member AT and empty args (`PT :=,`)
  - checkStructATAddresses (SEMA030 invalid, SEMA031 explicit, wildcard silent)
  - SEMA031 in FB/FUNCTION only for explicit addresses; "GVL" already exempt for 19-05
  - execCallStmt skips nil-valued input args
affects: [19-05, 20, 22, 24]

tech-stack:
  added: []
  patterns:
    - "Empty call arguments are CallArg{Name, Value: nil}; every consumer checks Value == nil"
    - "checkATAddresses(varBlocks, pouType) treats PROGRAM and GVL as AT-capable scopes"

key-files:
  created:
    - pkg/parser/struct_at_test.go
    - pkg/parser/callarg_test.go
    - pkg/emit/struct_at_test.go
    - pkg/format/struct_at_test.go
    - pkg/checker/at_wildcard_test.go
    - pkg/interp/empty_arg_test.go
  modified:
    - pkg/parser/types.go
    - pkg/parser/stmt.go
    - pkg/emit/emit.go
    - pkg/format/format.go
    - pkg/checker/check.go
    - pkg/interp/interpreter.go

key-decisions:
  - "Empty args print as `name :=` / `name =>` with no trailing space"
  - "`AT` with no address on a struct member is a parse error (VarDecl silently accepts it; left unchanged)"
  - "Struct AT check runs only for TYPE ... STRUCT declarations, not inline STRUCT var types"

requirements-completed: [DIAL-03, DIAL-05]

duration: 25min
completed: 2026-10-05
---

# Phase 19 Plan 04: Wildcard AT on structs and FBs, empty call arguments Summary

**`AT %I*` on STRUCT members and FB variables now parses, prints and checks with no warnings, explicit addresses in FBs, FUNCTIONs and STRUCTs still warn with SEMA031, and TwinCAT's `t(IN := b, PT := , Q => , ET => );` parses, round-trips through fmt and emit, and runs exactly like `t(IN := b);`.**

## Performance

- **Duration:** about 25 min
- **Started:** 2026-10-05T22:50:00Z
- **Completed:** 2026-10-05T23:15:00Z
- **Tasks:** 4 of 4
- **Files:** 6 created, 6 modified

## Accomplishments

- `parseStructMember` reads an optional `AT <addr>` after the member name. structat.st and fbat.st parse with zero diagnostics, and the member keeps its attribute.
- `parseCallArg` returns a `CallArg` with an untyped nil `Value` when `:=` or `=>` is followed by `,` or `)`. The span ends at the operator.
- Both printers print ` AT <addr>` on struct members and `name :=` / `name =>` for empty args. Output is idempotent and re-parses to the same shape.
- SEMA031 fires only when the address is explicit and the POU is not PROGRAM or GVL. fbat.st went from 2 warnings to 0.
- A new struct pass reports SEMA030 for an invalid member address and a SEMA031 warning for an explicit one. The warning says all instances share the address and suggests `%I*` / `%Q*`.
- The interpreter skips empty input arguments. The in-out and output loops already skipped nil values. A twin-engine test confirms identical `Q` and `ET` across 8 ticks.
- Every other `CallArg.Value` consumer (checker, lint walker, JSON) was confirmed nil-safe.

## Task Commits

1. **Task 1: Parse struct member AT and empty call arguments**: `984727e` (test), `abf3cef` (feat)
2. **Task 2: Print struct member AT and empty arguments**: `076b69b` (test), `c581237` (feat)
3. **Task 3: Checker SEMA031 wildcard policy and struct AT pass**: `2f39b8d` (test), `3f505ad` (feat)
4. **Task 4: Interpreter skips empty input arguments**: `1cd2107` (test), `95b5557` (feat)

## Verification

- `go test ./... -count=1` passes.
- `bash scripts/coverage-gate.sh` passes.

| package | coverage | gate |
|---------|----------|------|
| pkg/parser | 96.42% | 95% |
| pkg/checker | 94.85% | 94% |
| pkg/interp | 96.17% | 95% |
| pkg/emit | 97.38% | 95% |
| total | 93.43% | 85% |

Oracle diagnostics, before this plan at `fc7e87b` and after:

| file | parse diags before | after | check errors before | after | check warnings before | after |
|------|-----|-----|-----|-----|-----|-----|
| st301.st | 2727 | 1610 | 4727 | 3110 | 21 | 13 |
| svncorecomponents.st | 1406 | 628 | 2548 | 1770 | 37 | 25 |

Neither oracle file has any remaining "got KwAt" parse errors or "only valid in PROGRAM" warnings.

## Requirements

DIAL-03 and DIAL-05 are both marked complete. Each is handled end to end by the parser, both printers, the checker and the interpreter. Binding wildcard AT members to I/O at runtime is Phase 24 by design.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test correctness] Used `%IX0.9` for the struct SEMA030 test instead of `%Z9`**
- **Found during:** Task 3
- **Issue:** The lexer turns `%Z9` into an Illegal token, so parsing fails and the checker never sees an address.
- **Fix:** The SEMA030 test uses `%IX0.9`, which lexes but fails `iomap.ParseAddress`. A separate subtest asserts that `%Z9` is a parse error.
- **Files modified:** pkg/checker/at_wildcard_test.go
- **Commit:** 3f505ad

**2. [Scope] Dropped an expression-call empty-arg subtest from Task 1**
- **Found during:** Task 1
- **Issue:** `x := F(a := 1, b := );` fails because named function arguments in expressions are Phase 20 scope.
- **Fix:** Removed the subtest. Statement calls are covered.
- **Commit:** abf3cef

**3. [Rule 2] `AT` with no address on a struct member reports an error**
- **Found during:** Task 1
- **Issue:** Copying parseVarDecl exactly would have silently accepted `a AT : BOOL;`.
- **Fix:** parseStructMember reports "expected address after AT". VarDecl behavior is unchanged.
- **Commit:** abf3cef

## Issues Encountered

These were logged to deferred-items.md and are pre-existing or out of scope:

- `stc check` does not know standard FB signatures. `t(IN := b, PT := T#1S)` reports `TON has no input parameter "IN"`, and this reproduces on `main`. prog.st and action_inside.st therefore still show check errors on TON calls. Those errors are not caused by empty arguments.
- A trailing comma before `)` in a call is still a parse error. st301.st has 5 of these and svncorecomponents.st has 1. This is a different construct from empty arguments.
- Several files were already not gofmt-clean before this plan: pkg/parser/expr.go, pkg/emit/emit_final_coverage_test.go, pkg/checker/check_coverage_test.go and pkg/checker/vendor.go.

## Notes for Later Plans

- 19-05 can pass `"GVL"` to `checkATAddresses`. Explicit addresses there are already exempt from SEMA031.
- `execCallStmt` only accepts an `*ast.Ident` callee. Member callees like `SPB03.speedBatcher(...)` in st301 are not executed yet.
- `checkStructATAddresses` runs only on `TYPE ... STRUCT` declarations. Inline `STRUCT` var types are not checked.

## Self-Check: PASSED

- All 6 created test files and 6 modified source files exist.
- All 8 commits (984727e, abf3cef, 076b69b, c581237, 2f39b8d, 3f505ad, 1cd2107, 95b5557) are in `git log`.
