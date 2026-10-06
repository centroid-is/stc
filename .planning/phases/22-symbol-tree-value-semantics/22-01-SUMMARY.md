---
phase: 22-symbol-tree-value-semantics
plan: 01
subsystem: interp
tags: [interpreter, integer-wrap, iec-types, ulint, lword, storeAs]

requires: []
provides:
  - "storeAs(dst, v): single typed-store choke point (wrap to slot width, INT->REAL for real slots, enum tag)"
  - "wrapInt(n, kind) for all 12 IEC integer kinds"
  - "Typed integer binary results: resultIntKind + wrapping evalBinaryInt"
  - "ULINT/LWORD uint64 divide/MOD/compare/print; literals up to 2^64-1"
  - "Initialised scalars keep declared IECType (w : WORD := 16#9 is WORD)"
affects: [22-02, 22-04, interp, checker-literal-adoption]

tech-stack:
  added: []
  patterns:
    - "Every typed store goes through storeAs(currentSlotValue, newValue)"
    - "KindInvalid result kind marks untyped constant expressions (no wrap, DINT type)"

key-files:
  created:
    - pkg/interp/store.go
    - pkg/interp/store_test.go
    - pkg/interp/wrap_test.go
    - tests/type_system/wrap_test.st
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/enum.go
    - pkg/interp/bits.go
    - pkg/interp/ref_path.go
    - pkg/interp/scan.go
    - pkg/interp/fb_instance.go
    - pkg/interp/gvl.go
    - pkg/interp/call_args.go
    - pkg/interp/value.go
    - pkg/interp/stdlib_convert.go
    - pkg/interp/interp_coverage2_test.go

key-decisions:
  - "Typed binary results wrap at the operand kind (CONTEXT); CODESYS register-width evaluation (research A1) accepted as a known difference"
  - "Mixed typed operands: CommonType when it is an integer kind, else the wider kind, unsigned on a width tie (research A3)"
  - "Two untyped literal operands are not wrapped and stay DINT-typed, preserving constant-expression behaviour"
  - "storeAs leaves v's type alone when the slot has no known IECType"

patterns-established:
  - "storeAs(dst, v) at every store site; adoptEnumTag removed"

requirements-completed: []

duration: 7min
completed: 2026-10-06
---

# Phase 22 Plan 01: Integer value semantics in the interpreter Summary

**One typed-store choke point (storeAs) wraps integers to their IEC width, typed binary results wrap at the operand kind, and ULINT/LWORD behave as unsigned 64-bit, so INT 32767+1 = -32768 and UINT 0-1 = 65535 at runtime.**

## Performance

- **Duration:** about 7 min
- **Started:** 2026-10-06T06:41:10Z
- **Completed:** 2026-10-06T06:47:37Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments
- `storeAs`/`wrapInt` in pkg/interp/store.go replace `adoptEnumTag` at all 7 call sites and keep its enum-tag rule.
- Initialisers (program, GVL and FB VARs), `writeRef` (REF= paths), bit write-back and FOR counters now store through `storeAs`. `w : WORD := 16#9` is WORD-typed and `w.16` is a runtime error, which closes the 20-05 deferred item.
- `evalBinary` computes a result kind with `resultIntKind`. Untyped literals (also when parenthesised or negated) adopt the typed side's kind. Arithmetic results wrap and carry that kind.
- ULINT/LWORD use uint64 for `/`, `MOD`, `<`, `<=`, `>`, `>=`. They also print as unsigned in `Value.String`, JSON and TO_STRING. Division by zero is still a runtime error on both paths (T-22-01).
- ST suite tests/type_system/wrap_test.st has 4 wrap cases using typed literals, all passing.

## Task Commits

1. **Task 1: storeAs + wrapInt choke point** - `dafe6f8` (test, RED), `7efbf78` (feat)
2. **Task 2: route bypassing store sites through storeAs** - `1cf254f` (feat, tests included)
3. **Task 3: typed binary results, uint64 paths, ST wrap suite** - `2876e14` (test, RED), `33009f7` (feat)

## Decisions Made
- Result-kind rules follow research Pattern 2 and A3. The width-tie rule prefers unsigned, so INT + UINT gives UINT, because CommonType(INT, UINT) is REAL and therefore rejected.
- Two untyped constant operands keep the old behaviour, so `100000 * 100000` stays 10000000000 with DINT type. The checker plan (22-02) owns constant typing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Integer literals above 2^63 failed to parse**
- **Found during:** Task 3
- **Issue:** `parseLitInt` used `strconv.ParseInt`, so `16#FFFF_FFFF_FFFF_FFFF` was an "invalid integer literal". TestUnsigned64 could not be written without this fix.
- **Fix:** `parseInt64Bits` falls back to `ParseUint` and keeps the int64 bit pattern.
- **Files modified:** pkg/interp/interpreter.go
- **Commit:** 33009f7

**2. [Rule 2 - Missing functionality] ULINT/LWORD printed as negative**
- **Found during:** Task 3, because the must-have says "print as unsigned"
- **Fix:** `formatInt` is used by `Value.String`, `MarshalJSON` and `anyToString` (TO_STRING).
- **Files modified:** pkg/interp/store.go, value.go, stdlib_convert.go
- **Commit:** 33009f7

**3. [Rule 1 - Consistency] Unary minus dropped the operand type**
- **Issue:** `-i` on an INT returned a DINT value, so `-i + 1` lost INT semantics.
- **Fix:** A typed operand keeps its kind and wraps, so `-(-32768)` is -32768 for INT. Untyped literals stay DINT.
- **Commit:** 33009f7

**4. [Simplification] bits.go masking replaced by storeAs**
- `assignBitAt` now builds the new value and stores it through `storeAs(cur, next)`. `wrapInt` does the same masking and sign extension as before. `isUnsignedBits` is reused by `resultIntKind`.

**5. FOR start value** also goes through `storeAs` against an existing counter, not only the increment.

**Pinned DINT tests:** The plan expected about 13 existing tests that pin KindDINT to need updates. None did, because all of them use untyped constant operands, which still produce DINT.

### TDD gate note
Task 2 has no separate RED commit. Its tests were written together with the implementation in 1cf254f. Tasks 1 and 3 follow test-then-feat order.

## Deferred / Known Differences
- **Research A1 (accepted):** CODESYS may evaluate INT arithmetic at register width and truncate only on store. stc wraps typed intermediates, so `i + 1 > i` with i = 32767 is FALSE here. Store results are identical. A TwinCAT oracle check is still needed.
- **Research A3:** Mixed-signedness result kinds (wider kind, unsigned on a tie) are not verified against TwinCAT.
- **FOR with a narrow counter:** A FOR loop whose bound is the counter type's maximum, such as `FOR c := 0 TO 255` with USINT c, now wraps and runs until MaxLoopIterations. That matches PLC behaviour.
- A constant sub-expression such as `(1 + 2)` is not an "untyped literal" for `resultIntKind`, so `i + (1 + 2)` takes CommonType(INT, DINT) = DINT. Constant folding for typing is plan 22-02 Pattern 3.
- RUNT-05 is not marked complete because the checker half is plan 22-02.

## Verification
- `go test ./... -count=1`: all packages pass.
- `go run ./cmd/stc test tests/`: 241 passed, 0 failed.
- `bash scripts/coverage-gate.sh`: pkg/interp is at 98.20% against a 95% gate. Every new function is fully covered except `execFor`, whose remaining gaps are older error paths.

## Next Phase Readiness
- Plan 22-04 can build initialisers on `storeAs(zero, iv)`, which is the pattern already used in scan.go and fb_instance.go.
- Plan 22-02 checker literal adoption should agree with `resultIntKind` on the untyped-literal rule.

## Self-Check: PASSED
