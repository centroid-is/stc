---
phase: 19-twincat-declaration-syntax
verified: 2026-10-06T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 19: TwinCAT Declaration Syntax Verification Report

**Phase Goal:** TwinCAT declaration-level constructs parse into the AST and survive tooling round-trips.
**Status:** passed. Re-verification: No.

## Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Attributes attach in AST JSON; fmt and emit reproduce them | VERIFIED | `stc parse --format json ECT.st` shows qualified_only, TcLinkTo, OPC.UA.DA on the right nodes. fmt and emit keep all 7 attribute lines, including the `''` escape. Output is normalised (quotes, tabs, PERSISTENT RETAIN order). |
| 2 | Bare VAR_GLOBAL file is a GVL named from file or `--gvl-name`; unqualified qualified_only access is an error | VERIFIED | `--gvl-name Foo` yields name Foo. `GV.st` plus `y := gx` gives "GVL 'GV' is qualified_only; use GV.gx". `GV.gx` is accepted. |
| 3 | Struct and FB `AT %I*`/`%Q*` check clean; explicit `%IX0.0` in FB warns | VERIFIED | structat.st and fbat.st: 0 errors, 0 warnings. An explicit `%IX0.0` in an FB gives the AT-address warning (SEMA031). Covered by pkg/checker/at_wildcard_test.go. |
| 4 | `t(IN := b, PT := , Q => , ET => );` parses and runs as omitted | VERIFIED | Parses with 0 diagnostics. Runtime proven by tests/twincat_dialect/empty_args_test.st, 2 tests pass. |
| 5 | Action-call chain parses, checks, executes in owner env | VERIFIED | action.st parses. Both action forms (inside and after POU) are covered by tests/twincat_dialect/action_test.st, all pass. `stc check` on action_inside.st reports only TON/R_TRIG signature errors, which are the Phase 20 hand-off (RUNT-08), recorded in SUMMARY 19-04, 19-07 and 19-09 and in deferred-items.md. |

**Score:** 5/5

## Behavioral Spot-Checks and Tests

| Check | Result |
|-------|--------|
| `go build ./cmd/stc` | PASS |
| `go test ./... -count=1` | PASS, all packages ok |
| `stc test tests/twincat_dialect` | PASS, 12 tests, 12 passed |
| Debt markers (TBD/FIXME/XXX) in changed non-test files | none found |

## Requirements Coverage

| ID | Status | Evidence |
|----|--------|----------|
| DIAL-01 | SATISFIED | Truth 1 |
| DIAL-02 | SATISFIED | Truth 2, gvl_test.st |
| DIAL-03 | SATISFIED | Truth 3 |
| DIAL-05 | SATISFIED | Truth 4 |
| DIAL-08 | SATISFIED | Truth 5 |

DIAL-04, 06, 07 are mapped to Phase 20 in REQUIREMENTS.md. No orphaned Phase 19 IDs.

## Notes (not gaps)

- prog.st and link.st fail to parse on `w.3` bit access and named function arguments. These are DIAL-04 and DIAL-06, Phase 20, and the probe README says so.
- `n := n + 1` with `n : INT` reports DINT to INT. This is pre-existing literal typing, logged in deferred-items.md, and it affects the action_inside.st check output.
- Other pre-existing issues are logged in deferred-items.md: trailing comma before `)`, zero-argument FB instance calls, and emit comment trivia.
- `--gvl-name` requires exactly one input file, as designed.

## Human Verification Required

None.

_Verifier: Claude (gsd-verifier)_
