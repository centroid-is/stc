---
phase: 20-twincat-expression-semantics
plan: 06
subsystem: checker
tags: [checker, resolver, enums, bit-access, named-arguments, sema035, sema036, twincat]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 04
    provides: "Pointer-stable enum shells, inherited EXTENDS scope, reserved SEMA035/SEMA036 codes"
  - phase: 20-twincat-expression-semantics
    plan: 01
    provides: "ast.BitAccessExpr, CallExpr.NamedArgs, ast.EnumOrdinals, EnumType.BaseType"
provides:
  - "EnumType.Ordinals, Qualified, Strict, ToString; BaseType from the AST; SEMA036 for bad base types and out-of-range ordinals"
  - "qualified_only enum values are not inserted bare into the global scope"
  - "Inline VAR enums named <POU>.<var>, values registered in the POU scope"
  - "FunctionType.Outputs for FUNCTIONs and METHODs"
  - "Bit access typing (SEMA035) for literal and CONSTANT indices, reads and writes, through REFERENCE TO"
  - "checkCallArgs: named, positional, mixed and output arguments for FUNCTION and unqualified METHOD calls"
  - "FB call statements bind name := value to VAR_IN_OUT parameters"
  - "Symbol.ConstInt/HasConstInt and ast.IntLiteralValue"
affects: [20-07, 20-08, 20-09, 21, 22]

tech-stack:
  added: []
  patterns:
    - "Resolver.enums caches one EnumType per *ast.EnumType, so the scope and parameter passes share a type and report once"
    - "A call's argument list is bound in source order; positional entries bind by slot index in the full list"
    - "assignRoot strips bit access from an assignment target before the constant-write check"

key-files:
  created:
    - pkg/checker/check_bits.go
    - pkg/checker/check_calls.go
    - pkg/checker/enum_resolve_test.go
    - pkg/checker/function_outputs_test.go
    - pkg/checker/bit_access_test.go
    - pkg/checker/named_args_test.go
  modified:
    - pkg/types/types.go
    - pkg/checker/resolve.go
    - pkg/checker/check.go
    - pkg/symbols/symbol.go
    - pkg/ast/enum_ordinals.go
    - pkg/ast/enum_ordinals_test.go

key-decisions:
  - "Constant bit index recognises a member that resolves to a VAR_GLOBAL CONSTANT, or to a VAR CONSTANT with an integer literal initialiser, of integer or bit-string type, and only when the object is an integer or bit-string value"
  - "An all-positional call must supply every parameter; once any argument is named, omitted inputs take their defaults"
  - "Non-lvalue and wrong-type output bindings are SEMA021; duplicates and unknown names are SEMA024"
  - "A REFERENCE TO T input binds a value of exactly T (or a reference to T) without widening"
  - "A METHOD or ACTION in the POU scope chain binds before a built-in function of the same name; global user functions still lose to built-ins"
  - "Integer and real literal arguments fit any integer or real parameter on function calls, matching the FB call rule"

patterns-established:
  - "New expression nodes get a checkExpr case plus handling in markRootUsed and checkConstantTarget"

requirements-completed: [DIAL-04]  # DIAL-06 needs the 20-07 interpreter half; DIAL-07 needs the 20-09 enum rules

duration: 18min
completed: 2026-10-06
---

# Phase 20 Plan 06: Checker metadata, bit access and named arguments Summary

**The checker now knows enum base types, ordinals and attributes, resolves inline VAR enums per POU, types `w.3` and `w.cBit` as BOOL with SEMA035 range errors, and binds named, mixed and output arguments on function and method calls.**

## Performance

- **Duration:** about 18 min
- **Started:** 2026-10-06T02:34Z
- **Completed:** 2026-10-06T02:52Z
- **Tasks:** 3 of 3
- **Files created:** 6, modified: 6

## Accomplishments

- `TYPE E : (a := 0, b := 1) UINT;` gives BaseType UINT and Ordinals [0 1]. `(tun := 0, rdy := 2, nst)` gives [0 2 3]. An ordinal outside the base range, such as `(a := 300) USINT`, reports SEMA036.
- `qualified_only`, `strict` and `to_string` set Qualified, Strict and ToString. Values of a qualified_only enum stay out of the global scope, so a GVL variable may reuse a value name.
- `eStep : (E_IDLE, E_RUN);` names the enum `P.eStep` and registers E_IDLE and E_RUN in that POU scope. Two POUs may reuse value names.
- FUNCTIONs and METHODs expose VAR_OUTPUT parameters as `FunctionType.Outputs`.
- `b := w.3;` checks as BOOL on every integer and bit-string type, through struct members, array elements and REFERENCE TO. `w.16`, `w.99999999999999999999` and `r.0` report SEMA035.
- `b := w.cBit;` and `w.cBit := TRUE;` check clean when cBit is a CONSTANT, and the constant's value is range-checked. `wc.3 := TRUE;` on a GVL constant reports SEMA034.
- `n := F_X(a := 1, b := 2);`, `F_X(1, b := 2)` and `F_X(a := 1, 2)` all check with the return type. Unknown names, duplicate bindings, wrong types, bad output targets and too many arguments are each reported.
- `fb(parameter := x)` with `parameter` in VAR_IN_OUT no longer reports SEMA024. This removes the 15 FB_Parameter errors in svncore that 20-04 deferred.

## Task Commits

1. **Task 1: Enum metadata, inline VAR enums, function outputs**
   - `972297b` test(20-06): failing tests (RED, build failure on the new fields)
   - `51cc0fc` feat(20-06): implementation (GREEN)
2. **Task 2: Bit access typing**
   - `60d86fb` test(20-06): failing tests (RED)
   - `a3ff3fd` feat(20-06): implementation (GREEN)
3. **Task 3: Named-argument binding**
   - `94a9d54` test(20-06): failing tests (RED)
   - `7bc2ff5` feat(20-06): implementation (GREEN)

## Verification

- `go test ./... -count=1` passes after each task. `go run ./cmd/stc test tests/` reports 216 passed, 0 failed.
- `STC_PROBES_DIR=... go test ./tests -run TestTwinCATProbe` passes.
- `stc check tests/twincat_probes/enum_attr.st` reports 0 errors. `stc check tests/twincat_probes/link.st` reports only the two undeclared `F_X` callees, and no named-argument errors.
- `go test ./pkg/checker -run 'TestBitAccess/const_index_write' -v` runs and passes its subtests.
- `go vet` is clean on pkg/checker, pkg/types, pkg/ast and pkg/symbols.
- Every block in check_bits.go, check_calls.go and the new resolver functions is covered. The uncovered lines left in check.go predate this plan.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/checker | 98.18% | 94% |
| pkg/types | 100.00% | 95% |
| pkg/parser | 98.17% | 95% |
| pkg/interp | 97.09% | 95% |
| total | 95.47% | 85% |

### Oracle: `stc check --format json` error counts

svncorecomponents.st:

| stage | errors | SEMA010 | SEMA001 | SEMA021 | SEMA037 | SEMA024 | SEMA023 | SEMA022 | SEMA020 | SEMA003 |
|-------|--------|---------|---------|---------|---------|---------|---------|---------|---------|---------|
| before 20-06 | 295 | 136 | 49 | 45 | 23 | 17 | 14 | 6 | 4 | 1 |
| after Task 1 | 205 | 49 | 46 | 45 | 23 | 17 | 14 | 6 | 4 | 1 |
| after Task 2 | 205 | 49 | 46 | 45 | 23 | 17 | 14 | 6 | 4 | 1 |
| after Task 3 | 187 | 49 | 46 | 46 | 23 | 2 | 14 | 6 | 0 | 1 |

st301.st:

| stage | errors | SEMA010 | SEMA037 | SEMA033 | SEMA001 | SEMA020 | SEMA021 |
|-------|--------|---------|---------|---------|---------|---------|---------|
| before 20-06 | 1726 | 1460 | 235 | 24 | 5 | 2 | 0 |
| after Task 1 | 1726 | 1460 | 235 | 24 | 5 | 2 | 0 |
| after Task 2 | 1784 | 1518 | 235 | 24 | 5 | 2 | 0 |
| after Task 3 | 1782 | 1518 | 235 | 24 | 5 | 0 | 0 |

- **svncore SEMA010 fell by 87** after Task 1, more than the 60 the plan estimated. These were inline enum state values such as E_IDLE, cfgReady, READ and ON. The SEMA010 left are missing conversion functions (UDINT_TO_REAL, BYTE_TO_UDINT), ADR, SIZEOF and SHL, plus a few names svncore never declares.
- **svncore SEMA024 fell from 17 to 2.** The 15 FB_Parameter VAR_IN_OUT errors are gone. The two left are real missing members of ST_LineRecipe.
- **svncore SEMA020 fell from 4 to 0** and **st301 SEMA020 from 2 to 0.** These were named-argument calls of `create_cmd`, `status_word_to_state` and `find_order`, previously counted as calls with zero arguments.
- **The st301 SEMA010 rise of 58 is all `ECT` (39) and `Modbus` (19).** Bit access targets were not visited before, so their undeclared roots went unreported. Both are svncore GVLs that st301 alone does not declare, so they go away once Phase 21 loads the library.
- **One new svncore SEMA021** passes an `ecfg_e` enum value to the UINT in-out `parameter`. It became visible because in-out binding now works. Whether a non-strict enum converts to its base integer is ruling A4, which 20-09 owns.
- **No SEMA035 or SEMA036** appears on either oracle file.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] REFERENCE TO inputs rejected their referenced type**
- **Found during:** Task 3, oracle review
- **Issue:** `find_order(prio := fetches)`, with `prio : REFERENCE TO ARRAY[1..10] OF UINT` and `fetches : ARRAY[1..10] OF UINT`, reported a false SEMA021 (3 errors in st301).
- **Fix:** `checkInputArg` compares a REFERENCE TO T parameter with `derefRef` of both sides and allows no widening.
- **Files modified:** pkg/checker/check_calls.go, pkg/checker/named_args_test.go
- **Commit:** 7bc2ff5

**2. [Rule 1 - Bug] Named VAR_IN_OUT arguments on FB calls (deferred from 20-04)**
- **Found during:** Task 3
- **Issue:** `checkCallStmt` searched only `fbType.Inputs` for `name := value`, which gave 15 false SEMA024 in svncore. The orchestrator put this in scope for the named-argument task. `checkCallArgs` covers function calls only, but the fix belongs to the same binding rule.
- **Fix:** The FB path also searches `fbType.InOuts`.
- **Files modified:** pkg/checker/check.go
- **Commit:** 7bc2ff5

**3. [Rule 2 - Missing functionality] Constant values on symbols**
- **Found during:** Task 2
- **Issue:** The checker had no way to know the value of a constant bit index. Local VAR CONSTANT symbols are also not marked `IsConstant`.
- **Fix:** Added `Symbol.ConstInt` and `HasConstInt`, set for CONSTANT variables whose initialiser is an integer literal. The evaluator is shared with the enum numbering through the new `ast.IntLiteralValue`.
- **Files modified:** pkg/symbols/symbol.go, pkg/ast/enum_ordinals.go, pkg/checker/resolve.go
- **Commit:** a3ff3fd

**4. [Rule 2 - Missing functionality] One EnumType per enum spec**
- **Found during:** Task 1
- **Issue:** FB and FUNCTION var declarations resolve twice, once for the scope and once for the parameters. An inline enum would get two types and report SEMA036 twice.
- **Fix:** `Resolver.enums` caches the resolved type per `*ast.EnumType`. TYPE enums map to their 20-04 shell, which `typeDeclType` now fills through `resolveEnumSpec`.
- **Commit:** 51cc0fc

**5. Non-integer enum base type tested on a built AST.** The parser accepts only integer and bit-string keywords after an enum, so `) REAL;` is already a parse error from source. The SEMA036 base-type rule is tested by setting a REAL base on a parsed AST. It is logged in deferred-items.md.

**6. Unqualified callee lookup.** The plan asked to resolve unqualified callees through the current scope first. Only the POU scope chain (own and inherited methods and actions) is checked before the built-ins. Global user functions and vendor stubs keep losing to a built-in of the same name, as before, so stubs that redeclare a built-in do not change behaviour.

## Issues Encountered

None blocking. New out-of-scope items are logged in deferred-items.md:
- Named arguments on built-in functions (`LIMIT(MN := 0, ...)`) still report SEMA020.
- An enum value passed to an integer in-out reports SEMA021 (ruling A4, 20-09).
- Local VAR CONSTANT symbols are not `IsConstant`, so writes to them are not reported.
- Non-integer enum base types are parse errors before the checker sees them.

## Notes for 20-07..20-09

- **Enum metadata:** `EnumType.Ordinals[i]` matches `Values[i]`. Qualified, Strict and ToString are set only on the enum's own TYPE, never through an alias. Bare use of a qualified_only value now reports SEMA010 "undeclared identifier". 20-09 should give it the SEMA036 message. `qualifiedOnlyGVLs` in check.go shows the same pattern for GVLs.
- **Inline enums:** the type is named `<POU>.<var>` and its values are KindEnumValue symbols in the POU scope, typed by that enum.
- **derefRef** lives in check_bits.go. It is applied to bit access targets, constant-index objects and REFERENCE TO inputs. Index, member, binary operand and assignment-target deref is still 20-09's (Pitfall 10). svncore's 14 SEMA023 "REFERENCE TO ARRAY ... does not support indexing" are those.
- **Function outputs:** `FunctionType.Outputs` is filled. 20-07 can use it to write `=>` targets at runtime.
- **Binding order:** positional arguments after a named one bind by their index in the full list. This matches ruling A1 and should match the interpreter in 20-07.
- **Member callees** (`inst.M(...)`, `THIS^.M(...)`) are still unchecked, arguments included (Pitfall 11).
- **DIAL-04** is marked complete. DIAL-06 waits for the interpreter half in 20-07, and DIAL-07 waits for the 20-09 enum rules.

## Self-Check: PASSED

- All six created files exist. All six task commits are in `git log`.
- `grep -n "Outputs" pkg/types/types.go` matches inside FunctionType.
- `grep -n "CodeBitAccess" pkg/checker/check_bits.go`, `grep -n "case \*ast.BitAccessExpr" pkg/checker/check.go` and `grep -n "checkCallArgs" pkg/checker/check.go` all match.
