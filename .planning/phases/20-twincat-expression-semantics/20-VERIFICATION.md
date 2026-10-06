---
phase: 20-twincat-expression-semantics
verified: 2026-10-06T05:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 20: TwinCAT Expression Semantics Verification Report

**Phase Goal:** Expression- and call-level TwinCAT constructs parse, type-check and execute, so the flattened sildarvinnsla sources parse cleanly and `stc check` reports only genuine problems.
**Status:** passed
**Re-verification:** No, initial verification

## Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Bit read/write at runtime; `w.16` on WORD is a checker error | VERIFIED | `stc check` on a WORD `w.16` read prints "bit index 16 out of range for WORD (0..15)". `tests/twincat_dialect/bit_access_test.st` has 8 passing test cases covering struct, array and GVL bit paths. |
| 2 | `n := F_X(a := 1, b := 2)` type-checks and equals positional call | VERIFIED | Scratch file checks with 0 errors. `named_args_test.st` has 12 passing test cases. |
| 3 | Qualified enum CASE labels, enum base type with attributes, `REF=`, `THIS^`, `SUPER^` execute | VERIFIED | `enum_test.st` (12) and `ref_this_super_test.st` (10) pass inside `stc test tests/`, 237 of 237. |
| 4 | Zero parse errors on flattened ST301 and SVNCoreComponents | VERIFIED | Oracle run: 0 parse diagnostics and 0 P001 for both files. |
| 5 | Standard FBs accepted without stubs; unknown type rejected | VERIFIED | Ten standard FBs check with no errors. `FB_DoesNotExist` gives "undeclared type" (SEMA037). Only unused-variable warnings otherwise. |

**Score:** 5/5

## Gate runs

| Check | Result |
|-------|--------|
| `go build ./cmd/stc` | pass |
| `go test ./... -count=1` | all packages ok |
| `stc test tests/` | 237 tests, 237 passed |
| Oracle with `STC_PROBES_DIR` | PASS, 0 parse diagnostics per file |
| Debt markers (TBD, FIXME, XXX) in changed Go and ST files | none |

## Oracle remaining errors (hand-offs, bucketing holds)

Combined run reports 1727 errors in 21 buckets. The top buckets match the 20-08 summary: SEMA010 undeclared identifiers (library absent, Phase 21), SEMA037 missing types (Phase 21), SEMA033 from flattened GVLs (Phase 21), and LREAL and DINT literal-typing, bitwise-operator and built-in gaps (Phase 22). Three SEMA024 errors in svncore are recorded as genuine problems. All are listed in `deferred-items.md`.

## Requirements Coverage

| Requirement | Status |
|-------------|--------|
| DIAL-04 | SATISFIED (truth 1) |
| DIAL-06 | SATISFIED (truth 2) |
| DIAL-07 | SATISFIED (truth 3) |
| DIAL-09 | SATISFIED (truth 3) |
| DIAL-10 | SATISFIED (truth 4) |
| RUNT-08 | SATISFIED (truth 5) |

No orphaned requirements. REQUIREMENTS.md maps exactly these six IDs to Phase 20.

## Anti-Patterns and Warnings

- Non-blocking: `stc check` on the WORD `w.16` read emits a second error, "cannot assign BOOL to DWORD", which is correct for my scratch assignment and not a defect.
- Non-blocking, documented: METHOD bodies and member-callee calls are not checked (deferred to Phase 22/23). Initialised integer variables carry DINT at runtime, so runtime `w.16` on such a WORD is not a runtime error, though the checker still catches literal indices (Phase 22).

## Human Verification Required

None.

_Verifier: Claude (gsd-verifier)_
