---
phase: 20-twincat-expression-semantics
plan: 09
subsystem: checker
tags: [checker, enums, strict, qualified_only, to_string, ref-assign, this, super, auto-deref, initialisers, sema036, sema038, twincat]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 06
    provides: "EnumType Ordinals/Qualified/Strict/ToString, derefRef, checkCallArgs, FunctionType.Outputs"
  - phase: 20-twincat-expression-semantics
    plan: 04
    provides: "SEMA036/SEMA037/SEMA038 codes, inherited EXTENDS scope"
  - phase: 20-twincat-expression-semantics
    plan: 07
    provides: "Runtime THIS/SUPER/REF= semantics and error texts mirrored by the checker"
provides:
  - "SEMA036 for bare qualified_only enum values, strict-enum arithmetic, implicit strict-enum/integer conversion and strict cross-type comparison"
  - "Non-strict enums convert to and from their base integer in assignments, arguments, outputs, operators and CASE labels (ruling A4)"
  - "Conversion builtins (<X>_TO_<Y>, TO_<Y>) take strict enums explicitly; TO_STRING and TO_<int|bits|real> in checker and interpreter"
  - "Checker knows the interpreter's width/sign and *_TO_REAL conversions (UINT_TO_INT, UDINT_TO_REAL, TIME_TO_REAL, ...)"
  - "SEMA038 for REF= misuse, THIS outside an FB, SUPER outside an FB or without EXTENDS"
  - "THIS^.m and SUPER^.m resolve through the FB scope chain; THIS is POINTER TO the FB"
  - "Reference auto-deref in index, member, binary, unary and assignment typing"
  - "Initialiser checks: struct field names (struct members, FB inputs/outputs), element counts with saturating repetition counts, literal values"
  - "types.ArrayDimension.Known for literal array bounds"
affects: [20-08, 21, 22]

tech-stack:
  added: []
  patterns:
    - "Enum rules live in check_enum.go; callers test involvesEnum(a, b) and delegate to checkEnumArg with a mismatch callback for their own SEMA code"
    - "Initialisers: only isLiteralExpr values are type-checked; every value is still walked by checkExpr"
    - "THIS^/SUPER^ member access looks the member up in the POU scope chain, not in FunctionBlockType"

key-files:
  created:
    - pkg/checker/check_enum.go
    - pkg/checker/check_ref.go
    - pkg/checker/check_init.go
    - pkg/checker/enum_attr_test.go
    - pkg/checker/ref_this_test.go
    - pkg/checker/initializer_check_test.go
  modified:
    - pkg/checker/check.go
    - pkg/checker/check_calls.go
    - pkg/checker/resolve.go
    - pkg/types/builtin.go
    - pkg/types/types.go
    - pkg/types/types_coverage_test.go
    - pkg/interp/stdlib_convert.go
    - pkg/interp/stdlib_convert_test.go

key-decisions:
  - "Two different enum types never assign to each other (SEMA001, or SEMA036 when either is strict); comparing them is allowed only when both are non-strict"
  - "A CASE label of a different enum type is SEMA001; an integer label on a strict enum selector is SEMA036"
  - "A strict enum passed to a typed or generic parameter of a non-conversion builtin (LIMIT) is SEMA036; ANY parameters (SEL, MOVE) take it unchanged"
  - "TO_<type> conversions take ANY argument and return the named type; the interpreter rounds reals half to even, maps BOOL to 0/1, TIME to milliseconds and parses strings"
  - "THIS^.M(args) as a statement now binds its arguments against the method signature; member-callee expressions stay unchecked apart from the THIS/SUPER root"
  - "Initialiser literal rule = assignment literal rule plus integer literals into BYTE..LWORD and REAL/LREAL and string literals into CHAR/WCHAR"
  - "A plain literal initialising a pointer, reference, array, struct or FB is not checked (left to Phase 22)"
  - "FB instance initialisers accept VAR_INPUT and VAR_OUTPUT names; anything else is SEMA024"
  - "Too many initialisers is SEMA001; it is reported only when every bound is a literal (ArrayDimension.Known) and every repetition count is a literal"

patterns-established:
  - "Defensive branches in checker helpers get direct unit tests with built ASTs (TestInitializerCheckDefensive)"

requirements-completed: [DIAL-07, DIAL-09]  # DIAL-10 is the gate plan's (20-08)

duration: 20min
completed: 2026-10-06
---

# Phase 20 Plan 09: Enum rules, REF=/THIS/SUPER and initialiser checks Summary

**The checker now enforces qualified_only and strict enums with SEMA036 while letting non-strict enums act as their base integer, checks `REF=`, `THIS^` and `SUPER^` with SEMA038, dereferences references implicitly, and type-checks struct and array initialisers without new false errors on the oracle files.**

## Performance

- **Duration:** about 20 min
- **Started:** 2026-10-06T03:15Z
- **Completed:** 2026-10-06T03:35Z
- **Tasks:** 3 of 3
- **Files created:** 6, modified: 8

## Accomplishments

- Bare `q1` of a qualified_only enum reports SEMA036 "enum value 'q1' of qualified_only enum E_Q must be written E_Q.q1". Names no enum declares stay SEMA010.
- On a strict enum, `e + 1`, `n := e`, `e := 1`, `F_Int(e)`, `fb(i := e)`, `e = 1` and `e = x` each report one SEMA036. `UINT_TO_INT(e)`, `TO_INT(e)`, `TO_DINT(e)` and `TO_STRING(e)` are clean.
- A non-strict enum assigns to and from its base integer, widens like it (`d := f` with base INT), compares and adds with integers, and passes to INT parameters (ruling A4). This removed the svncore `ecfg_e` to UINT in-out SEMA021.
- CASE labels `E.a:`, `E.b, E.c:` and `E.a..E.c:` check. A label of another enum is SEMA001.
- `r REF= x` checks the target is a REFERENCE TO T and the value is a variable path of type T. `r REF= y` (REAL), `r REF= 5`, `r REF= a + b`, `r REF= x.3` and `n REF= x` are SEMA038.
- `THIS` types as POINTER TO the FB in FB bodies and actions. `THIS^.x`, `THIS^.M(...)`, `SUPER^.M(...)`, `SUPER^()` and `SUPER^.x` resolve through the FB scope chain, inherited members included. THIS in a PROGRAM or FUNCTION and SUPER without EXTENDS are SEMA038, with the interpreter's wording.
- REFERENCE TO values index, member-access and take part in arithmetic as their base type. All 14 svncore SEMA023 "REFERENCE TO ARRAY ... does not support indexing" errors are gone.
- Initialisers in PROGRAM, FB and FUNCTION var blocks, GVLs (qualified_only included) and STRUCT members check field names, statically known element counts and literal values. `[1000000000(0)]` and `[9223372036854775807(0), ...]` are counted, never expanded.

## Task Commits

1. **Task 1: Enum rules and TO_STRING builtin**
   - `41b50f7` test(20-09): failing tests (RED, build failure on isConversionBuiltin)
   - `9e0dc0f` feat(20-09): implementation (GREEN)
2. **Task 2: REF=, THIS^, SUPER^ and reference auto-deref**
   - `6fdadd5` test(20-09): failing tests (RED)
   - `6df4a5c` feat(20-09): implementation (GREEN)
3. **Task 3: Initialiser checks**
   - `7f7a4bc` test(20-09): failing tests (RED, build failure on ArrayDimension.Known)
   - `17bb57e` feat(20-09): implementation (GREEN)

## Verification

- `go test ./... -count=1` passes after each task. `go run ./cmd/stc test tests/` reports 216 passed, 0 failed.
- `go test ./pkg/checker -run 'TestEnumAttr/(strict_explicit_conversion|nonstrict_int_ops)' -v` and `-run 'TestInitializerCheck/nonliteral_clean' -v` run and pass.
- `STC_PROBES_DIR=... go test ./tests -run TestTwinCATProbe` passes.
- `go vet ./...` is clean. Every block of check_enum.go, check_ref.go and check_init.go is covered.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/parser | 98.17% | 95% |
| pkg/lexer | 97.52% | 95% |
| pkg/checker | 98.77% | 94% |
| pkg/interp | 98.08% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.73% | 95% |
| total | 96.00% | 85% |

### Oracle: `stc check --format json` error counts

svncorecomponents.st:

| stage | errors | SEMA001 | SEMA021 | SEMA010 | SEMA037 | SEMA023 | SEMA022 | SEMA024 | SEMA003 |
|-------|--------|---------|---------|---------|---------|---------|---------|---------|---------|
| after 20-06 | 187 | 46 | 46 | 49 | 23 | 14 | 6 | 2 | 1 |
| after Task 1 | 183 | 59 | 45 | 33 | 23 | 14 | 6 | 2 | 1 |
| after Task 2 | 170 | 59 | 45 | 33 | 23 | 0 | 6 | 3 | 1 |
| after Task 3 | 170 | 59 | 45 | 33 | 23 | 0 | 6 | 3 | 1 |

st301.st stays at 1782 errors through all three tasks: SEMA010 1518, SEMA037 235, SEMA033 24, SEMA001 5.

- **Task 1:** 16 SEMA010 for `UDINT_TO_REAL`, `UINT_TO_REAL` and `TIME_TO_REAL` are gone. 13 of those lines now report SEMA001 "cannot assign LREAL to REAL", because `x / 5.0` types as LREAL. That is real-literal typing (Phase 22). The `ecfg_e` SEMA021 is gone (ruling A4).
- **Task 2:** the 14 SEMA023 are gone. One genuine SEMA024 appears: `batches REF= settings.p_stat_Batches;` names a member that `ST_Conveyor` does not have. REF= values were never checked before.
- **Task 3:** no new diagnostics on either file, and no "cannot assign" on an initialiser line. Seeding `zzz : INT := 'x';` into svncore gives "cannot initialise INT with STRING", so the checks run there. The large st301 initialisers target `ST_EcSlaveInfo`, which is undeclared (SEMA037) and therefore skipped.
- **No SEMA036 or SEMA038** appears on either oracle file.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] Conversion builtins the checker lacked**
- **Found during:** Task 1
- **Issue:** The behaviour requires `UINT_TO_INT(e)` and `TO_INT(e)` to check, but neither existed in `types.BuiltinFunctions`. The interpreter implemented 16 conversions the checker did not know.
- **Fix:** Added those 16 to the checker (width and sign changes, `*_TO_REAL`, `REAL_TO_LREAL`, `LREAL_TO_REAL`, `TIME_TO_REAL`). Added `TO_SINT` through `TO_LREAL` and `TO_STRING` as ANY-argument conversions.
- **Files modified:** pkg/types/builtin.go
- **Commit:** 9e0dc0f

**2. [Rule 2 - Missing functionality] Runtime TO_<type> conversions**
- **Found during:** Task 1
- **Issue:** Code using `TO_INT(e)` would check but fail at runtime, since the interpreter had only `TO_STRING`.
- **Fix:** Added `TO_<int|bits|REAL|LREAL>` to the interpreter's stdlib, with tests.
- **Files modified:** pkg/interp/stdlib_convert.go, pkg/interp/stdlib_convert_test.go
- **Commit:** 9e0dc0f

**3. [Rule 1 - Bug] Array bounds were not reliably known**
- **Found during:** Task 3
- **Issue:** `evalConstInt` returns 0 for any bound that is not a plain decimal literal. It also drops the sign of `-2` and misreads `16#3`. The checker could not tell a known bound from `GVL.MAX`.
- **Fix:** `types.ArrayDimension` gained `Known`. The resolver uses `ast.IntLiteralValue` for literal bounds (signed and based) and falls back to the old path otherwise. Unkeyed `ArrayDimension{0, 9}` literals in types_coverage_test.go are now keyed.
- **Files modified:** pkg/types/types.go, pkg/checker/resolve.go, pkg/types/types_coverage_test.go
- **Commit:** 17bb57e

**4. [Rule 2 - Missing functionality] Enum rules on every argument path**
- **Found during:** Task 1
- **Issue:** The plan named `checkAssignStmt` and `checkCallArgs`. The svncore A4 case passes an enum to an FB VAR_IN_OUT, which goes through `checkCallStmt`. Output bindings (`o => n`) and builtin arguments also need the rule.
- **Fix:** FB call statements, function and method inputs, `=>` outputs and builtin arguments all use `checkEnumArg`.
- **Files modified:** pkg/checker/check.go, pkg/checker/check_calls.go
- **Commit:** 9e0dc0f

**5. Auto-deref also covers unary operands.** `NOT rb` and `-ri` on references check as their base type. This is one line beyond the four sites the plan listed.

## Issues Encountered

None blocking. New out-of-scope items are logged in deferred-items.md:
- Non-literal initialiser values are not type-checked (Phase 22, RUNT-05).
- Real-literal arithmetic types as LREAL, which gives 13 svncore SEMA001 (Phase 22).
- `[C(0)]` with a constant count parses as a call, so an initialiser walk would report SEMA022. Neither oracle file uses the form.
- METHOD bodies and their VAR initialisers are still not checked.
- The initialiser literal rule is more permissive than the assignment rule for BYTE..LWORD and REAL targets.

## Notes for 20-08

- **DIAL-07 and DIAL-09 are marked complete.** DIAL-10 stays with the gate plan.
- **Record in deferred-items.md** that non-literal initialiser value checks are deferred to Phase 22 with literal typing. The line has already been added under [20-09].
- **Oracle counts by code after 20-09:**
  - svncore has 170 errors: SEMA001 59, SEMA021 45, SEMA010 33, SEMA037 23, SEMA022 6, SEMA024 3, SEMA003 1.
  - st301 has 1782 errors: SEMA010 1518, SEMA037 235, SEMA033 24, SEMA001 5.
  - There is no SEMA023, SEMA035, SEMA036 or SEMA038 on either file.
- **svncore SEMA001 is now mostly literal typing:** `REAL := ... / 5.0` and INT targets of DINT literal arithmetic.
- **The remaining svncore SEMA010** are missing functions (`BYTE_TO_UDINT`, `ADR`, `SIZEOF`, `SHL`) and a few undeclared names.
- **THIS^ messages match the runtime:** "THIS used outside a function block", "SUPER used outside a function block", and "SUPER used in X, which does not EXTEND another function block".

## Self-Check: PASSED

- All six created files exist on disk.
- All six task commits are in `git log`: 41b50f7, 9e0dc0f, 6fdadd5, 6df4a5c, 7f7a4bc and 17bb57e.
- `grep CodeEnumRule pkg/checker/check_enum.go`, `grep isConversionBuiltin pkg/checker/check.go`, `grep '"TO_STRING"' pkg/types/builtin.go`, `grep RefAssignStmt pkg/checker/check.go` and `grep isLiteralCompatible pkg/checker/check_init.go` all match.
- `derefRef` appears 6 times in check.go.
