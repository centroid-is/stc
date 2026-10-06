---
phase: 20-twincat-expression-semantics
plan: 05
subsystem: interp
tags: [interp, runtime, bit-access, enums, to_string, stdlib, tc2_standard, testing-runner]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 02
    provides: "BitAccessExpr on the AST; constant-identifier index stays MemberAccessExpr (ruling A3)"
  - phase: 20-twincat-expression-semantics
    plan: 03
    provides: "EnumType.BaseType, ast.EnumOrdinals previous+1 numbering, TypeDecl.Attributes"
provides:
  - "Runtime bit read/write (literal and constant index) through ident, reference, struct, array, GVL and FB output targets"
  - "EnumDef registry: RegisterEnumDecl, Value.Enum tag, base IEC type on enum values"
  - "Qualified E.v in evalMemberAccess with variable/GVL shadowing; bare and E#v read the same registry"
  - "Inline VAR enums registered as <POU>.<var> for PROGRAMs, FB instances and TEST_CASEs"
  - "TO_STRING: enum name for to_string enums, kind-based text otherwise"
  - "CTU/CTD/CTUD/SR/RS accept RESET, LOAD, SET1, SET, RESET1 next to the IEC names"
  - "Runner enum numbering via RegisterEnumDecl (fixes nst=2 and 16#0006=160006)"
affects: [20-06, 20-07, 20-08, 20-09, 22]

tech-stack:
  added: []
  patterns:
    - "Bit writes are read-modify-write through assignToTarget, so every lvalue kind reuses its existing write path"
    - "assignBitAt takes the already-evaluated target value, so constant-index writes evaluate the object once"
    - "Enum-aware TO_STRING is answered in evalCall before the global stdlib function, which has no interpreter access"

key-files:
  created:
    - pkg/interp/bits.go
    - pkg/interp/enum.go
    - pkg/interp/bit_access_test.go
    - pkg/interp/enum_runtime_test.go
    - pkg/interp/stdlib_alias_test.go
    - pkg/testing/enum_register_test.go
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/value.go
    - pkg/interp/assertions.go
    - pkg/interp/fb_instance.go
    - pkg/interp/scan.go
    - pkg/interp/stdlib_convert.go
    - pkg/interp/stdlib_counters.go
    - pkg/interp/stdlib_bistable.go
    - pkg/testing/runner.go

key-decisions:
  - "Runtime constant-index bit access accepts any integer symbol in scope as the index; the CONSTANT requirement is the checker's job. It only applies when the object is an integer value, which has no members, so struct and FB member access is never shadowed"
  - "A bit write keeps the target's IECType, masks unsigned kinds to their width and sign-extends signed kinds (INT bit 15 gives -32768); an integer with an unknown IECType is treated as 64 bits"
  - "The legacy RegisterEnumType also fills EnumDefs with base DINT, so old callers keep their value types and every lookup reads one registry"
  - "An enum's zero value is its first declared value, typed by the base type and tagged with the enum name when declared through a named TYPE"
  - "Inline enum registration also runs for PROGRAMs in the scan engine, not only FB instances and the test runner (Rule 2: programs are the main use of inline enums)"

patterns-established:
  - "Interpreter tests that need enums register TYPE declarations through RegisterEnumDecl (enumEngine helper in enum_runtime_test.go)"
  - "ST identifiers are case-insensitive: a variable s shadows enum S, so tests name enum variables differently from their type"

requirements-completed: [RUNT-08]  # RUNT-08 was marked by 20-04; this plan adds the runtime half. DIAL-04 and DIAL-07 still need checker work in 20-06/20-09.

duration: 9min
completed: 2026-10-06
---

# Phase 20 Plan 05: Interpreter value semantics (bit access, enums, TO_STRING, Tc2 FB aliases) Summary

**Host tests can now read and write single bits, use qualified, inline and based enums with IEC numbering, print enum names with TO_STRING, and drive CTU/CTD/CTUD/SR/RS with TwinCAT input names.**

## Performance

- **Duration:** about 9 min
- **Started:** 2026-10-06T02:24Z
- **Completed:** 2026-10-06T02:33Z
- **Tasks:** 3 of 3
- **Files created:** 6, modified: 9

## Accomplishments

- `x := ECT.dev.q_wDigitalInputs.0;` reads bit 0 through a GVL, and `arr[1].3 := TRUE;` sets one bit of an array element without touching the others. Ident, reference, struct member, array element, GVL member and FB output targets all work.
- `b := w.cBit;` and `w.cBit := TRUE;` treat a constant integer `cBit` as the bit index (ruling A3). The write path runs inside execAssignMember and evaluates the object once. `s.cBit := 5` on a struct with a member `cBit` is still a member write.
- Bit access on REAL, STRING, BOOL or struct values, an index past the width, a negative index and a non-integer index all give a RuntimeError. The width check runs before any shift.
- `TYPE E : (tun := 0, rdy := 2, nst) UINT;` gives nst = 3 with IEC type UINT. `(a := 16#0006, b)` gives 6 and 7. The test runner used to give nst = 2 and a = 160006.
- `E.v` evaluates anywhere an expression is allowed, including CASE labels, label lists and ranges. A variable or GVL named E shadows the enum.
- Inline enums like `eStep : (E_IDLE, E_RUN) := E_RUN;` work in PROGRAMs, FB instances and TEST_CASEs.
- `TO_STRING(e)` returns 'rdy' for a `to_string` enum and '2' otherwise. INT, BOOL, REAL, STRING and other kinds get their natural text.
- `c(CU := x, RESET := r, PV := 3);` now resets. Before this plan the RESET input was silently ignored. LOAD, SET1, SET and RESET1 work too.

## Task Commits

1. **Task 1: Bit access read and write**
   - `d1222a1` test(20-05): failing tests (RED, build failure on missing bitWidth)
   - `d423acf` feat(20-05): implementation (GREEN)
2. **Task 2: Enum runtime**
   - `2e31326` test(20-05): failing tests (RED)
   - `911770b` feat(20-05): implementation (GREEN)
3. **Task 3: Standard FB input aliases and wave gate**
   - `4860af5` test(20-05): failing tests (RED)
   - `da3637d` feat(20-05): implementation (GREEN) plus edge-case coverage tests

## Verification

- `go test ./... -count=1` passes after each task.
- `go run ./cmd/stc test tests/` reports 216 passed, 0 failed after each task.
- `go test ./pkg/interp -run 'TestBitAccess/const_index_write' -v` runs and passes 3 subtests.
- The acceptance greps match. `case *ast.BitAccessExpr` appears twice in interpreter.go. `assignBitAt` appears in execAssignMember. `RegisterEnumDecl` appears in runner.go and fb_instance.go.
- `go vet` is clean on pkg/interp and pkg/testing.
- Every function in bits.go and enum.go is at 100% statement coverage, and so are RegisterInlineEnums, anyToString, formatReal and registerEnumTypes.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/parser | 98.17% | 95% |
| pkg/lexer | 97.52% | 95% |
| pkg/checker | 97.66% | 94% |
| pkg/interp | 97.09% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.73% | 95% |
| total | 95.35% | 85% |

## Decisions Made

See `key-decisions` in the frontmatter. The most important one for later plans is about constant-index bit access. At runtime, any integer symbol in scope is accepted as the index, and only when the object is an integer. SEMA035 in the checker must enforce that the symbol is a CONSTANT.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] Inline enums in PROGRAMs run by the scan engine**
- **Found during:** Task 2
- **Issue:** The plan listed FB instances and the test runner. PROGRAMs run through ScanCycleEngine would still report "undefined variable: E_RUN".
- **Fix:** `initializeEnv` in scan.go calls RegisterInlineEnums with the program name before it initialises variables.
- **Files modified:** pkg/interp/scan.go
- **Commit:** 911770b

**2. [Rule 1 - Bug] Enum zero values were plain DINT 0**
- **Found during:** Task 2
- **Issue:** `zeroFromTypeSpecWith` had no EnumType case. An enum variable started as DINT 0 with no tag, which is wrong for `(a := 3)` and made TO_STRING on a fresh to_string enum print '0'.
- **Fix:** The zero value is now the first declared value with the base type. Named enum TYPEs also tag it with the enum name.
- **Files modified:** pkg/interp/fb_instance.go, pkg/interp/enum.go
- **Commit:** 911770b

**3. [Rule 3 - Blocking] TYPE attributes were not reachable from registerEnumTypes**
- **Found during:** Task 2
- **Issue:** fileContext and externalContext stored only the TypeSpec, so `{attribute 'to_string'}` was lost.
- **Fix:** Both contexts gained a typeAttrs map. It is filled wherever typeDecls is filled, including the merge from mock and library files.
- **Files modified:** pkg/testing/runner.go
- **Commit:** 911770b

The RegisterInlineEnums helper lives in fb_instance.go, next to the VAR-block initialisation it serves. This keeps the plan's `RegisterEnumDecl` grep on that file meaningful.

## Issues Encountered

None blocking. Three runtime limits were logged to deferred-items.md:

- Initialised integer variables carry IECType DINT. Bit width on `w : WORD := 16#9` is therefore 32. This belongs to Phase 22, RUNT-05/06.
- An enum tag is lost when a plain integer is assigned.
- Bare inline enum values are global at runtime.

## Notes for 20-06..20-09

- **20-06 and 20-09 (checker):** The interpreter does not check that a constant-identifier bit index is a CONSTANT. SEMA035 must do it. DIAL-04 and DIAL-07 stay open until the checker side lands.
- **20-07:** The `=>` output binding switch in execCallStmt still handles only Ident and Member. `isAssignable` already returns true for BitAccessExpr, so routing outputs through assignToTarget makes `fb(q => s.w.2)` work with no further bit code.
- **20-07:** The StdlibFunctions path in evalCall still reads only `e.Args`. The enum-aware TO_STRING hook sits just before `fn(args)`, so a shared bindArgs must keep that hook.
- **Value.Enum:** This is a new field on Value. Code that compares whole Values with `==` or `assert.Equal` will now see the enum tag on enum values.

## Self-Check: PASSED

All 6 created files exist and all 6 task commits are in git history.
