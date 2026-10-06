---
phase: 20-twincat-expression-semantics
plan: 04
subsystem: checker
tags: [checker, resolver, forward-references, stdlib, extends, sema037, twincat]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 03
    provides: "NamedType.Namespace, EnumType.BaseType on the AST, zero parse diagnostics on both oracle files"
provides:
  - "Pointer-stable two-pass type registration: a shell per FB, PROGRAM, FUNCTION, INTERFACE, STRUCT and enum, filled in place"
  - "Alias TYPE fixpoint sweep (reverse-order chains, ARRAY OF later struct, self and mutual recursion terminate)"
  - "Ten IEC standard FBs registered as library symbols with POU scopes, Tc2_Standard names plus IEC aliases"
  - "Inherited EXTENDS scope: derived POU scope re-parented onto the base, base params prepended, shadowing reported"
  - "SEMA037 undeclared type, once per reference, no cascade; Lib.Type qualified-then-bare fallback"
  - "Reserved codes SEMA035 (bit access), SEMA036 (enum rules), SEMA038 (REF=/THIS/SUPER)"
  - "LTIME, LDATE, LTIME_OF_DAY, LTOD, LDATE_AND_TIME, LDT and VOID as elementary names"
affects: [20-05, 20-06, 20-07, 20-08, 20-09, 21, 22]

tech-stack:
  added: []
  patterns:
    - "Resolver pass 0 (preRegister) allocates type objects; resolveX fill them in place"
    - "Resolver.forward maps a name to the owning declaration's type: first user/mock, else first library"
    - "Inheritance by scope re-parenting, not symbol copying"
    - "Unknown names inside library declarations are never reported"

key-files:
  created:
    - pkg/checker/stdlib_fbs.go
    - pkg/checker/stdlib_fb_test.go
    - pkg/checker/inherited_scope_test.go
    - pkg/checker/undeclared_type_test.go
    - pkg/checker/testdata/forward_ref_members.st
  modified:
    - pkg/checker/resolve.go
    - pkg/checker/check.go
    - pkg/checker/diag_codes.go
    - pkg/checker/resolve_test.go
    - pkg/types/builtin.go

key-decisions:
  - "resolveTypeSpec checks the forward map before the global scope, so earlier library-file uses also get a later user override"
  - "EXTENDS re-parents the derived POU scope onto the base scope instead of copying symbols, so unused-variable checks and LSP symbol lists see each symbol once"
  - "Standard FBs have a zero source position; a library stub or user declaration of the same name replaces them silently"
  - "Unknown type names inside library declarations are not reported (stubs may name types from libraries that are not loaded)"
  - "Aliases left after the fixpoint sweep report SEMA037 at their declaration; later uses get the alias type silently"
  - "LTIME and the long date/time names map to the short kinds; VOID maps to TypeVOID"

patterns-established:
  - "One SEMA037 per *ast.NamedType node (reportedTypes map)"
  - "A call on an Invalid callee still checks argument values, so their variables count as used"

requirements-completed: [RUNT-08]  # DIAL-09 and DIAL-10 still need parser/interp/probe-gate work in 20-05..20-09

duration: 17min
completed: 2026-10-06
---

# Phase 20 Plan 04: Checker resolver (forward refs, standard FBs, EXTENDS, SEMA037) Summary

**The resolver now registers every type before resolving any of them, knows the ten IEC standard FBs, gives derived FBs their inherited members, and reports undeclared types as one SEMA037 each. Together these remove most of the false checker errors on the two oracle files.**

## Performance

- **Duration:** about 17 min
- **Started:** 2026-10-06T02:02Z
- **Completed:** 2026-10-06T02:19Z
- **Tasks:** 3 of 3
- **Files created:** 5, modified: 5

## Accomplishments

- Forward references resolve to the final type object. Examples are an FB, a struct, an alias chain or an array alias used before its declaration in file order. svncore no longer reports "type ST_Drive_HMI has no member".
- TON, TOF, TP, CTU, CTD, CTUD, R_TRIG, F_TRIG, SR and RS check with both input-name sets. The Tc2_Standard names are RESET, LOAD, SET1, SET, RESET and RESET1. The IEC aliases R, LD, S1, S and R1 are appended after them. Wrong parameter names still report SEMA024, and wrong argument types report SEMA021.
- A derived FB body sees base variables, methods and actions. An instance of the derived FB accepts base inputs and exposes base outputs. A derived variable that redeclares a base variable is SEMA011. EXTENDS cycles terminate.
- `x : FB_DoesNotExist;` gives one SEMA037 "undeclared type 'FB_DoesNotExist'". Uses of x add no further errors. FB and FUNCTION inputs are deduplicated, and so are POINTER TO, REFERENCE TO and an undeclared EXTENDS base.
- `Tc2_EtherCAT.ST_EcSlaveState` resolves to a user or library ST_EcSlaveState. Without one it reports the qualified name.
- `tests/twincat_probes/action_inside.st` reports only the line-13 literal-typing error, which Phase 22 owns.

## Task Commits

1. **Task 1: Pointer-stable two-pass registration**
   - `63b0862` test(20-04): failing tests (RED)
   - `55ceb5a` feat(20-04): implementation (GREEN)
2. **Task 2: Standard FB signatures and inherited EXTENDS scope**
   - `ea563d2` test(20-04): failing tests (RED)
   - `e6201c4` feat(20-04): implementation (GREEN)
3. **Task 3: SEMA037, namespace fallback, fixture audit**
   - `aa0ccb0` test(20-04): failing tests (RED, build failure on the missing code)
   - `16a2c30` feat(20-04): implementation (GREEN)

## Verification

- `go test ./... -count=1` passes after each task. `go run ./cmd/stc test tests/` reports 216 passed, 0 failed after each task.
- `STC_PROBES_DIR=... go test ./tests -run TestTwinCATProbe` passes, with 0 parse diagnostics on both oracle files.
- `go vet` is clean on pkg/checker and pkg/types.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/checker | 97.66% | 94% |
| pkg/types | 100.00% | 95% |
| pkg/parser | 98.17% | 95% |
| pkg/interp | 96.72% | 95% |
| total | 95.09% | 85% |

Every new function in resolve.go and stdlib_fbs.go has full branch coverage. The only uncovered block in resolve.go is the fall-through `return types.Invalid` at the end of `resolveTypeSpec`, which predates this plan.

### Oracle: `stc check --format json` error counts

svncorecomponents.st:

| stage | errors | SEMA024 | SEMA010 | SEMA001 | SEMA021 | SEMA037 | "has no member" |
|-------|--------|---------|---------|---------|---------|---------|-----------------|
| before 20-04 | 1101 | 889 | 116 | 53 | 1 | 0 | 506 |
| after Task 1 | 673 | 438 | 116 | 49 | 45 | 0 | 148 |
| after Task 2 | 441 | 201 | 121 | 49 | 45 | 0 | 74 |
| after Task 3 | 295 | 17 | 136 | 49 | 45 | 23 | 2 |

st301.st:

| stage | errors | SEMA010 | SEMA024 | SEMA033 | SEMA037 | "has no member" |
|-------|--------|---------|---------|---------|---------|-----------------|
| before 20-04 | 1340 | 1173 | 138 | 22 | 0 | 58 |
| after Task 1 | 1308 | 1173 | 106 | 22 | 0 | 26 |
| after Task 2 | 1259 | 1190 | 40 | 22 | 0 | 7 |
| after Task 3 | 1726 | 1460 | 0 | 24 | 235 | 0 |

- **svncore SEMA037** has 17 distinct names, all missing libraries or types the file never declares. They are ADSREAD, ADSWRITE, AMSADDR, ten FB_Ec* and FB_LocalSystemTime FBs, ST_EcSlaveState, its Tc2_EtherCAT-qualified form, T_AmsNetId and ST_Batch.
- **st301 SEMA037** has 33 distinct SVNCoreComponents types, matching the research count. Examples are FB_Sensor, FB_ATV320, ST_EL1008 and ST_MotorParams.
- **The st301 SEMA010 rise** comes from argument values to instances of undeclared FB types. Before, each named argument hit "has no input parameter" and its value was skipped. The values are now checked, which surfaces names that st301 alone does not declare, such as `Modbus` and `ECT`. These disappear once Phase 21 loads the library. The st301 total therefore rises, but it now consists of real missing-library findings instead of placeholder artefacts.
- **The new SEMA021 errors** (WORD to UINT, DINT to WORD and similar) appear because FB parameters are now known. They are checker strictness questions for Phase 22, not regressions.

### Fixture audit (placeholder fallback, before removal)

Method: a temporary log in the placeholder branch, run over `go test ./...`, `stc test tests/` and `stc check` on every `.st` file in the repository, one file at a time. The log was removed before commit.

| name | where found | class | resolution |
|------|-------------|-------|------------|
| T, V (self and mutual alias) | pkg/checker resolve_test.go (new Task 1 test) | intended | SEMA037 by design, asserted in TestUndeclaredType |
| (none) | every other `go test ./...` package, including pkg/pipeline, pkg/sim, pkg/testing, cmd/stc and tests | n/a | no hits |
| (none) | `stc test tests/` | n/a | no hits |
| FB_Motor | pkg/analyzer/testdata/multi_file_b.st alone | (c) multi-file companion | declared in multi_file_a.st, which the analyzer test loads together. No change |
| S_Point | pkg/checker/testdata/array_struct.st alone | (c) multi-file companion | check_test.go supplies S_Point in a second file. No change |
| T_AmsNetId, T_AmsPort, MC_Direction, AXIS_REF, E_OpenPath, T_MaxString | stdlib/vendor/beckhoff/tc2_*.st, tc3_eventlogger.st, stdlib/mocks/beckhoff/*.st alone | (c) stub companion | declared in stdlib/vendor/beckhoff/common_types.st, loaded with the stubs. Names inside library files are not reported anyway. No change |
| VOID | tests/corpus/structured-text-utilities UTILITIES_ARRAY/BYTE/STRING.st | (a) elementary | added `VOID` to the elementary table as TypeVOID |
| BINARY_ENCODING, FLOAT_ARRAY_STATISTICS, INTEGRAL_ARRAY_STATISTICS, UNSIGNED_INTEGRAL_ARRAY_STATISTICS, STRING_ENCODING, DAY_OF_WEEK_TYPE, WEEK_OF_YEAR_TYPE, DATE_AND_TIME_DISSECTED, TIME_DISSECTED, TIME_OF_DAY_DISSECTED, DATE_DISSECTED | tests/corpus/structured-text-utilities UTILITIES_BYTE/MATH/STRING/TIME.st | (c) parser gap | declared in multi-declaration TYPE blocks, which the parser does not fully handle (20-03 deferred item). These files already report about 200 parse errors each, and no test checks them. No change |
| LTIME and the long date/time names | not used in the repository | (a) elementary, from the plan | added LTIME, LDATE, LTIME_OF_DAY, LTOD, LDATE_AND_TIME and LDT, mapped to the short kinds |
| TON..RS | n/a | (b) standard FB | registered by Task 2 |

No fixture or expected-diagnostic file needed changes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] EXTENDS by scope re-parenting instead of copying symbols**
- **Found during:** Task 2
- **Issue:** Copying base symbols into the derived scope would make `checkUnusedVars` warn twice for one unused base variable. It would also duplicate entries in the LSP symbol collection.
- **Fix:** The derived POU scope's Parent becomes the base POU scope. Lookups walk derived, then base, then global. A cycle check refuses the edge that would close a loop. Shadowed variables are reported explicitly. Base params are prepended to the derived FunctionBlockType in place.
- **Files modified:** pkg/checker/resolve.go
- **Commit:** e6201c4

**2. [Rule 2 - Missing functionality] Library stubs may override a standard FB**
- **Found during:** Task 2
- **Issue:** "First library wins" would have kept the built-in TON over a vendor stub's TON. The plan requires stubs to be able to override.
- **Fix:** `isStdFB` exempts the standard entries from first-library-wins, for every POU kind and for TYPE.
- **Files modified:** pkg/checker/resolve.go, pkg/checker/stdlib_fbs.go
- **Commit:** e6201c4

**3. [Rule 1 - Bug] Standard FBs use a zero source position**
- **Found during:** Task 2
- **Issue:** A synthetic file name would make LSP go-to-definition emit a `file://<stdlib>` URI.
- **Fix:** Standard FBs use a zero position, which the LSP already treats as "no file".
- **Commit:** e6201c4

**4. [Rule 2 - Missing functionality] Calls on an Invalid callee check argument values**
- **Found during:** Task 3, during the cascade review the plan asks for
- **Issue:** `x(a := n)` on an undeclared type returned before checking `n`. That left `n` unused and produced an unused-variable warning cascade.
- **Fix:** `checkCallStmt` checks argument values before returning on an Invalid callee.
- **Files modified:** pkg/checker/check.go
- **Commit:** 16a2c30

**5. [Rule 2 - Missing functionality] No SEMA037 inside library declarations**
- **Found during:** Task 3
- **Issue:** Stubs name types from libraries that may not be loaded, such as T_NotInAnyStub. Reporting them would put unfixable errors on library files.
- **Fix:** `Resolver.inLibrary` suppresses SEMA037 while library groups resolve. Those names become Invalid.
- **Commit:** 16a2c30

**6. resolveTypeSpec order is forward map, then global scope.** The plan said "after the global-symbol lookups". Checking the forward map first returns the owning declaration's type, even when a library-file use resolves before the user override is processed. The two orders differ only in that case, and the plan's precedence rules favour the forward map.

## Issues Encountered

None blocking. Pre-existing gaps found while reading the oracle output are logged in deferred-items.md:
- Named VAR_IN_OUT arguments in FB calls report SEMA024. This gives 15 errors in svncore (FB_Parameter `parameter`).
- EXTENDS cycles are silent.
- Interface property reads report "has no member".

## Notes for 20-05..20-09

- **Diagnostic codes:** SEMA035 (`CodeBitAccess`), SEMA036 (`CodeEnumRule`) and SEMA038 (`CodeRefThisSuper`) exist in diag_codes.go and are unused so far.
- **RUNT-08** is complete on the checker side, matching the requirement text. The interpreter still accepts only the IEC names (R, LD, S1, R1, S). A TwinCAT `ctu(RESET := r)` now checks clean but is still ignored at runtime until 20-05 adds the aliases to `SetInput` and `GetInput`.
- **Standard FB types:** PV and CV are INT (ruling A2). Inputs list the canonical names first, then the aliases.
- **Enums:** `EnumType` shells are created with BaseType KindINT. 20-06 should set the base type from the AST (`ast.EnumType.BaseType`) in `typeDeclType`, where it fills the shell. Inline VAR enums still register no values.
- **Inherited scope:** the derived POU scope's Parent is the base POU scope. Any new scope walk should stop at `ScopeGlobal`, or use `lookupInPOUChain`.
- **Interfaces:** an interface type is an empty `FunctionBlockType` carrying the interface name.
- **Forward-referenced types:** a declared type resolves to the same pointer everywhere, so pointer identity (`assert.Same`) holds for forward references.

## Self-Check: PASSED

- All five created files exist. All six task commits are in `git log`.
- `grep -n "r.forward" pkg/checker/resolve.go` matches inside lookupTypeName, which resolveTypeSpec calls.
- `grep -n "registerStdFBs" pkg/checker/resolve.go` and `grep -n "SEMA037" pkg/checker/diag_codes.go` both match.
- `grep -n "FunctionBlockType{Name: name}" pkg/checker/resolve.go` has no match.
