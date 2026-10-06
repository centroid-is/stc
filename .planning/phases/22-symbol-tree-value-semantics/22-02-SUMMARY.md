---
phase: 22-symbol-tree-value-semantics
plan: 02
subsystem: checker
tags: [checker, untyped-constants, literal-typing, range-check, bitwise]

requires:
  - phase: 22-01
    provides: "Runtime typed binary results; untyped operands adopt the typed side's kind"
provides:
  - "untypedConst / untypedAssignable / intRange (pkg/checker/untyped.go)"
  - "untypedStore: one untyped-constant rule for assignment, FB/function args, enum args, CASE labels, initialisers"
  - "adoptUntyped in checkBinaryExpr (arithmetic and comparisons)"
  - "Bitwise AND/OR/XOR on BYTE..LWORD in checker and interpreter"
  - "TwinCAT dialect gate with no allow-list"
affects: [22-05, checker, interp]

tech-stack:
  added: []
  patterns:
    - "Untyped constants are a checker-private AST predicate; no types.TypeKind"
    - "Range errors: constant <n> out of range for <TYPE>, SEMA001 at assignment/initialiser/CASE, SEMA021 at arguments"

key-files:
  created:
    - pkg/checker/untyped.go
    - pkg/checker/untyped_test.go
    - pkg/interp/bitwise_test.go
  modified:
    - pkg/checker/check.go
    - pkg/checker/check_calls.go
    - pkg/checker/check_enum.go
    - pkg/checker/check_init.go
    - pkg/checker/stdlib_fb_test.go
    - pkg/interp/interpreter.go
    - tests/twincat_dialect_check_test.go

key-decisions:
  - "Bitwise AND/OR/XOR accepted for BYTE..LWORD only (IEC ANY_BIT); ANY_INT bitwise stays out of scope"
  - "Typed literals (INT#5) no longer narrow implicitly between integer kinds; they follow the ordinary widening rules"
  - "Untyped constants above 2^63-1 or with overflowing folds are accepted without a range check"

requirements-completed: [RUNT-05]

duration: 9min
completed: 2026-10-06
---

# Phase 22 Plan 02: Untyped literal adoption in the checker Summary

**Untyped numeric constants adopt their context type at binary operators, assignments, call arguments, CASE labels and initialisers, with `constant <n> out of range for <TYPE>` errors, so `a := a + 1` on INT checks clean and all 85 literal-class oracle errors are gone.**

## Performance

- **Duration:** about 9 min
- **Started:** 2026-10-06T06:48:00Z
- **Completed:** 2026-10-06T06:57:00Z
- **Tasks:** 3
- **Files modified:** 10

## Accomplishments
- pkg/checker/untyped.go adds `untypedConst`, which folds `+ - * / MOD` exactly with math/big. Overflow and division by zero leave the value unknown and never panic (T-22-03).
- `untypedAssignable` accepts integer constants for ANY_INT, BYTE..LWORD and ANY_REAL, and real constants for ANY_REAL. Known values are range-checked.
- `untypedStore` applies that rule at assignment, FB-call args, function args, enum args, CASE labels and initialisers. `r := 0` and `r : REAL := 0` now follow one rule, which closes the 20-09 deferred item.
- `checkBinaryExpr` calls `adoptUntyped` after `checkEnumBinary`. Typed mixed operands keep the CommonType lattice and two untyped operands keep DINT/LREAL.
- `isLiteralCompatible` was deleted. `isLiteralExpr` stays because the initialiser gate and an existing coverage test use it.
- The dialect gate in tests/twincat_dialect_check_test.go has no allow-list.

## Task Commits

1. **Task 1: untypedConst, untypedAssignable, intRange** - `c6b6f86` (test, RED), `af00f88` (feat)
2. **Task 2: adoption at all sites** - `0f3178c` (feat, tests included)
3. **Task 3: remove allow-list, corpora diff, oracle measurement** - `e1a80e8` (test)

## Oracle measurement (flattened files, error counts by message)

| File | Literal class before | After | Total errors before | After |
|------|---------------------:|------:|--------------------:|------:|
| svncorecomponents.st | 80 | 0 | 170 | 86 |
| st301.st | 5 | 0 | 1782 | 1777 |

No new message bucket appeared in either file. Four bitwise errors on BYTE operands also disappeared in svncore: `AND ... BYTE and DINT` 3 and `BYTE and BYTE` 1.

Note: the plan's verify grep cannot match the two argument messages because the JSON has escaped quotes around the parameter name (`parameter \"nIndex\"`). The counts above come from bucketing the parsed JSON, and the diff confirms all 30 argument errors are gone.

**Remaining non-literal buckets in svncorecomponents.st** (out of scope per research Open Question 1):

| Count | Code | Message |
|------:|------|---------|
| 15 | SEMA021 | cannot pass WORD as input parameter "nSlaveAddr" (expected UINT) |
| 1 | SEMA001 | cannot assign WORD to UINT |
| 3 | SEMA001 | boolean operator AND requires BOOL operands, got UINT and UINT |
| 2 | SEMA001 | boolean operator OR requires BOOL operands, got UINT and UINT |
| 10 | SEMA010 | undeclared identifier "ADR" |
| 5 | SEMA010 | undeclared identifier "SIZEOF" |
| 3 | SEMA010 | undeclared identifier "SHL" |
| 12 | SEMA010 | undeclared conversions (BYTE_TO_UDINT 6, WORD_TO_UINT 2, SYSTEMTIME_TO_DT 2, BOOL_TO_UINT 1, UINT_TO_WORD 1) |
| 23 | SEMA037 | undeclared type (flattening) |
| 12 | various | undeclared names, missing members and not-callable instances from flattening |

st301.st remains dominated by flattening errors: undeclared identifiers, SEMA037 235 and SEMA033 qualified_only 24.

## Corpora diff (tests/**/*.st, 78 files)

- Errors went from 7555 to 7519.
- **Gone:** `cannot assign DINT to INT` 20, `DINT to WORD` 6, `DINT to BYTE` 5, `LREAL to REAL` 3, `cannot compare BYTE and DINT` 2, `LREAL to INT` 2.
- **New:** none at a new location. In tests/corpus/structured-text-utilities/UTILITIES_MATH.st, lines 505 and 551 changed message from `cannot assign LREAL to INT` to `cannot assign REAL to INT`. The real expression now adopts REAL from its typed operand, and the error is still genuine.
- No out-of-range constant appears in any corpus file.

## Decisions Made
- Bitwise AND/OR/XOR is accepted on BYTE..LWORD operands, giving CommonType. The interpreter computes it bitwise on two integer values and wraps at `resultIntKind`. See the deviation below.
- Enum argument and assignment checks use `untypedAssignable` against the enum base type. An out-of-range constant there reports the existing enum mismatch message, not the range message.
- A CASE label that is an untyped constant adopts the selector's base type. An out-of-range label is SEMA001 with the range message.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] Bitwise AND/OR/XOR on bit strings**
- **Found during:** Task 2
- **Issue:** The must-have `CASE b AND 16#0F OF` checks clean could not pass because the checker rejected AND on non-BOOL operands. The interpreter also evaluated AND only as a logical operator, so accepting it in the checker alone would have computed wrong values at runtime.
- **Fix:** The checker accepts AND/OR/XOR when both operands are BYTE..LWORD, after literal adoption. The interpreter computes the bitwise result for two integer values. ANY_INT bitwise such as `UINT AND UINT` stays an error, as research Open Question 1 requires.
- **Files modified:** pkg/checker/check.go, pkg/checker/untyped.go, pkg/interp/interpreter.go, pkg/interp/bitwise_test.go
- **Commit:** 0f3178c

**2. [Rule 1 - Test update] Pinned Phase 22 error in TestStdFB**
- The `action_inside probe` subtest required exactly one Phase 22 literal-typing error. It now asserts no errors. Commit 0f3178c.

**3. Helper indirection**
- Call sites use `c.untypedStore(...)`, a thin wrapper that reports the range diagnostic. It calls `untypedAssignable`, so a literal grep for `untypedAssignable(` finds check_enum.go and untyped.go but not check.go, check_calls.go or check_init.go.

**4. Typed-literal narrowing**
- `isLiteralCompatible` used to let typed literals such as `s : SINT := INT#5` narrow. They now follow normal widening. The corpora diff showed no file affected.

## Deferred Issues
See deferred-items.md. Method-call arguments are not type-checked at all, which is pre-existing, so the planned `fb.M(w := 16#10000)` range-error row was dropped. ANY_INT bitwise is also deferred.

## Notes for 22-03..22-05
- 22-05 should add the durable oracle assertion by bucketing parsed JSON messages, not by grepping raw JSON. The argument messages contain escaped quotes.
- `untypedConst` is reusable for constant array bounds. Unlike `ast.ConstIntValue`, it also folds `/` and `MOD`.

## Self-Check: PASSED
- Found: pkg/checker/untyped.go, pkg/checker/untyped_test.go, pkg/interp/bitwise_test.go
- Found commits: c6b6f86, af00f88, 0f3178c, e1a80e8
- `go test ./... -count=1` green; checker coverage 98.5%
