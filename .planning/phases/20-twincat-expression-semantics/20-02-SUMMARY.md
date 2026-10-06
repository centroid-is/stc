---
phase: 20-twincat-expression-semantics
plan: 02
subsystem: parser
tags: [parser, twincat, bit-access, named-args, case, ref-assign, this, super, fmt]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 01
    provides: "BitAccessExpr, ThisExpr, SuperExpr, RefAssignStmt, CallExpr.NamedArgs, KindCallArg"
provides:
  - "Parser builds BitAccessExpr for `x.N` (Dot IntLiteral) at any postfix depth, read or write"
  - "THIS / SUPER primary expressions; both may start a statement"
  - "Expression calls accept named, output and mixed args; CallExpr.Args holds leading positional args, NamedArgs the rest in source order"
  - "Parser.stmtHead flag: `fb(name := ...)` stays a CallStmt only at statement head"
  - "Trailing comma in any call argument list is accepted and dropped"
  - "Every parsed CallArg has NodeKind KindCallArg"
  - "`lhs REF= rhs;` parses to RefAssignStmt (REF matched case-insensitively, followed by Eq)"
  - "CASE labels may be E.v, Lib.E.v, E#v, typed literals and negative integers, alone, in lists or ranges"
affects: [20-03, 20-05, 20-06, 20-07, 20-08, 20-09]

tech-stack:
  added: []
  patterns:
    - "Parser-context flag saved, cleared and restored around nested argument and index parsing"
    - "Lookahead helper Parser.kindAt(offset) returns EOF past the end"

key-files:
  created:
    - pkg/parser/bit_access_test.go
    - pkg/parser/named_args_expr_test.go
    - pkg/parser/ref_this_super_test.go
    - pkg/parser/case_qualified_test.go
    - pkg/format/phase20_expr_roundtrip_test.go
    - .planning/phases/20-twincat-expression-semantics/deferred-items.md
  modified:
    - pkg/parser/expr.go
    - pkg/parser/stmt.go
    - pkg/parser/parser.go

key-decisions:
  - "`w.3.1` lexes as Dot RealLiteral(\"3.1\"); the parser splits it into two nested BitAccessExprs and reports \"bit access on a bit\" at the inner dot"
  - "A constant-name bit index (`v.cEnable`) stays a MemberAccessExpr (ruling A3); the checker reinterprets it"
  - "`r REF= ;` reports an error and stores an ErrorNode value without consuming the semicolon, so the next statement still parses"
  - "Positional-first mixed calls at statement head (`fb(1, b := 2);`) become expression statements over a CallExpr, not CallStmts (deferred item)"

patterns-established:
  - "Phase 20 parser tests use bodyOf / parseBody helpers from bit_access_test.go"

requirements-completed: []  # parsing only; DIAL-04/06/07/09/10 complete once checker/interpreter plans land

duration: 8min
completed: 2026-10-06
---

# Phase 20 Plan 02: Expression and statement parsing Summary

**The parser now accepts bit access, named and mixed arguments in expression calls, trailing commas, qualified CASE labels, `REF=`, and `THIS^` / `SUPER^`, and fmt round-trips all of them.**

## Performance

- **Duration:** about 8 min
- **Started:** 2026-10-06T01:45Z
- **Completed:** 2026-10-06T01:53Z
- **Tasks:** 3 of 3
- **Files created:** 6, modified: 3

## Accomplishments

- `parsePostfix` builds a `BitAccessExpr` when an integer literal follows a dot. The target can be any chain: `ECT.X.q_wDigitalInputs.0`, `Modbus.arr[0].3`, or `create_cmd.8` on the left of `:=`.
- `THIS` and `SUPER` are primary expressions. `THIS^.x := 1;`, `THIS^.M();`, `SUPER^.M();`, `SUPER^();` and `p := THIS;` all parse.
- Outside statement head, a call parses all arguments through `parseCallArgs`. Leading positional arguments go to `CallExpr.Args`. Everything from the first named argument on goes to `NamedArgs`. A positional-only call keeps its old JSON shape.
- `fb(IN := x);` and `inst.M(a := 1);` at statement head still produce `CallStmt`. Calls nested in brackets, arguments or conditions are expression calls.
- `r REF= x;` produces a `RefAssignStmt`. A variable named `REF` can still be assigned with `REF := 1;`.
- The CASE label lookahead accepts qualified names, `E#v`, typed literals and negative integers, so `lft_e.no_fault:` no longer ends the previous branch body.

## Task Commits

1. **Task 1: Bit access and THIS/SUPER**
   - `860540a` test(20-02): failing tests (RED)
   - `c214682` feat(20-02): implementation (GREEN)
2. **Task 2: Named args in expression calls and trailing comma**
   - `0935b9a` test(20-02): failing tests (RED)
   - `db301e4` feat(20-02): implementation (GREEN)
3. **Task 3: REF=, qualified CASE labels, fmt round-trip**
   - `386e9d1` test(20-02): failing tests (RED)
   - `e279915` feat(20-02): implementation (GREEN), plus edge unit tests

## Verification

- `go vet ./pkg/parser/...` is clean, and `go test ./... -count=1` passes.
- `stc parse` reports no diagnostics for tests/twincat_probes/prog.st and link.st.
- `stc test tests/twincat_dialect` passes 12 of 12 tests.
- `go test ./tests -run 'TestTwinCATProbeFixtures|TestCorpus'` passes. The Phase 20 line allowances are still present, as the plan requires.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/parser | 98.04% | 95% |
| pkg/lexer | 97.44% | 95% |
| pkg/checker | 95.92% | 94% |
| pkg/interp | 96.72% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.73% | 95% |
| total | 94.79% | 85% |

Every new parser branch is covered. The blocks that remain uncovered in expr.go and stmt.go predate this plan.

Oracle parse error counts from `stc parse`:

| file | before | after |
|------|--------|-------|
| st301.st | 917 | 703 |
| svncorecomponents.st | 615 | 39 |

The remaining st301 errors are mostly struct and array initialisers in VAR blocks, for example `(p_stat_sName := '...', ...)`, which belong to 20-03.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `w.3.1` lexes as a real literal**
- **Found during:** Task 1
- **Issue:** The lexer turns `3.1` after a dot into one `RealLiteral`, so the planned check for a second Dot IntLiteral would never fire. The parser would report "expected identifier" instead of "bit access on a bit".
- **Fix:** `splitBitPair` splits a digits-dot-digits real literal into two bit indexes with correct positions. The second one reports "bit access on a bit".
- **Files modified:** pkg/parser/expr.go
- **Commit:** c214682

**2. [Rule 2 - Missing critical] `REF=` with no value keeps recovery intact**
- **Found during:** Task 3
- **Issue:** Calling `parseExpr` at `;` would consume the semicolon as a bad token and merge the next statement into the error.
- **Fix:** At `;` or end of file, the parser reports "expected expression after REF=" and stores an `ErrorNode` value without consuming the token.
- **Files modified:** pkg/parser/stmt.go
- **Commit:** e279915

The subtest for `SendString := THIS^.SendBytes(...)` lives in TestNamedArgsExpr rather than TestThisSuper, so it was added with Task 2 as the plan intended.

## Issues Encountered

None blocking. Two out-of-scope findings are in deferred-items.md:

- `INT#5:` without a space lexes the colon into the typed literal. The test uses `INT#5 :`.
- `fb(1, b := 2);` at statement head becomes an expression statement over a `CallExpr`, not a `CallStmt`.

## Notes for 20-03..20-09

- `CallExpr.NamedArgs` is now populated by the parser. Checker and interpreter call paths that read only `Args` will silently ignore named arguments until 20-05 and 20-06 handle them.
- A `NamedArgs` entry with `Name == nil` is a positional argument after a named one. A named entry may have a nil `Value` when the source has an empty argument such as `a :=`.
- `THIS^.M()` and `SUPER^()` are expression statements, an `AssignStmt` with a nil Value and a `CallExpr` Target. `SUPER^.M(a := 1);` is a `CallStmt` whose callee is a `MemberAccessExpr` over `DerefExpr{SuperExpr}`.
- `RefAssignStmt.Value` may be an `ErrorNode` in broken source.
- The probe allowances in tests/twincat_probes_test.go are untouched and now unused for prog.st and link.st. 20-08 removes them.

## Self-Check: PASSED
