---
phase: 20-twincat-expression-semantics
plan: 08
subsystem: testing
tags: [twincat, st-tests, oracle, probes, validation, phase-gate]

requires:
  - phase: 20-twincat-expression-semantics
    provides: "Bit access, named arguments, enums, REF=/THIS^/SUPER^, initialisers, standard FBs and SEMA037 from plans 20-01..20-07 and 20-09"
  - phase: 19-twincat-declaration-syntax
    provides: "Probe fixture gate, owned-line classifier, oracle harness and the hand-off tests"
provides:
  - "Four TwinCAT dialect ST suites (21 TEST_CASEs) that CI runs on three OSes through st-tests.yml"
  - "Probe gate with no per-line allowances over 15 fixtures"
  - "Zero-parse-error oracle through parser.Parse and analyzer.Analyze, plus templated stc check buckets"
  - "TestTwinCATDialectCheck: analyzer gate over dialect suites, action_inside.st, SEMA037 and the ten standard FBs"
  - "Signed-off 20-VALIDATION.md and the Phase 21/22 deferred list"
affects: [21-library-resolution, 22-stdlib-parity-literal-typing]

tech-stack:
  added: []
  patterns:
    - "Oracle messages are templated before logging: quoted names, identifiers with case, digits, underscores or dots, and numbers become placeholders"
    - "The only tolerated dialect-check error is one named allowlist variable pointing at Phase 22"

key-files:
  created:
    - tests/twincat_dialect/bit_access_test.st
    - tests/twincat_dialect/named_args_test.st
    - tests/twincat_dialect/enum_test.st
    - tests/twincat_dialect/ref_this_super_test.st
    - tests/twincat_dialect_check_test.go
    - tests/twincat_probes/case.st
    - tests/twincat_probes/enum.st
    - tests/twincat_probes/fcall.st
    - tests/twincat_probes/ptr.st
  modified:
    - tests/twincat_probes_test.go
    - tests/twincat_probes/README.md
    - pkg/checker/action_test.go
    - pkg/interp/action_test.go
    - .planning/phases/20-twincat-expression-semantics/20-VALIDATION.md
    - .planning/phases/20-twincat-expression-semantics/deferred-items.md

key-decisions:
  - "The oracle asserts zero parse diagnostics per file through parser.Parse and zero P001 through analyzer.Analyze; semantic errors are logged, never asserted"
  - "SEMA033 in st301 is an input artifact of flattening 17 GVL files into one GVL named st301, bucketed with Phase 21 multi-file loading"
  - "The Phase 22 allowlist holds the single message 'cannot assign DINT to INT'"

patterns-established:
  - "Dialect ST suites use DINT counters and step, never by, as a parameter name"

requirements-completed: [DIAL-04, DIAL-06, DIAL-07, DIAL-09, DIAL-10, RUNT-08]

duration: 11min
completed: 2026-10-06
---

# Phase 20 Plan 08: Phase gate Summary

**Both oracle files now parse with zero diagnostics through `stc check`, every committed probe parses clean with no allowances, and 21 new ST test cases cover bit access, named arguments, enums and REF=/THIS^/SUPER^ in CI.**

## Performance

- **Duration:** about 11 min
- **Started:** 2026-10-06T03:37:00Z
- **Completed:** 2026-10-06T03:48:00Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments

- Four ST suites under `tests/twincat_dialect/` pass under `stc test` and check clean under `stc check`. CI's existing "TwinCAT dialect" step runs them on macOS, Windows and Linux.
- The probe gate's allowance map is deleted. Any diagnostic on any of the 15 probes fails, and every probe round-trips through fmt.
- The oracle reports 0 parse diagnostics for st301.st and svncorecomponents.st. It checks both `parser.Parse` and `analyzer.Analyze`.
- The Phase 19 hand-off checks pass with timers and edge triggers. `action_inside.st` and every dialect suite report only the Phase 22 literal-typing message.
- `FB_DoesNotExist` gives exactly one SEMA037. All ten standard FBs check clean with both IEC and Tc2_Standard input names and no stubs.

## Task Commits

1. **Task 1: ST dialect suites run by CI** - `f0fb6c7` (test)
2. **Task 2: Probe gate, zero-parse-error oracle and hand-off re-run** - `54d73f0` (test)
3. **Task 3: Full gate, oracle numbers and validation sign-off** - `a358ea6` (docs)

## Oracle results

Parse diagnostics:

| File | Before Phase 19 | After Phase 19 | After Phase 20 |
|------|-----------------|----------------|----------------|
| st301.st | 2711 | 917 | 0 |
| svncorecomponents.st | 1430 | 615 | 0 |

`stc check` errors, each file checked alone:

| File | Before Phase 20 | After Phase 20 |
|------|-----------------|----------------|
| st301.st | 2384 | 1782 |
| svncorecomponents.st | 1745 | 170 |

Checking both files together gives 1727 errors, because svncore declares some names st301 uses.

### Remaining `stc check` buckets

Messages are templated. `<id>` stands for a name.

**svncorecomponents.st, 170 errors**

| Count | Code | Message template | Owner |
|-------|------|------------------|-------|
| 23 | SEMA037 | undeclared type `<id>` (Tc2_EtherCAT, Tc2_System, Tc2_Utilities types such as ADS and EtherCAT FBs) | Phase 21, missing library |
| 6 | SEMA022 | `<id>` is not callable (type Invalid), cascade from those SEMA037 | Phase 21, missing library |
| 3 | SEMA010 | undeclared identifier `<id>` (a parameter list and a Tc2 function) | Phase 21, missing library |
| 30 | SEMA010 | undeclared identifier: ADR, SIZEOF, BYTE_TO_UDINT, SHL, WORD_TO_UINT, SYSTEMTIME_TO_DT, UINT_TO_WORD, BOOL_TO_UINT | Phase 22, built-ins |
| 16 | SEMA001 | cannot assign LREAL to REAL (`x / 5.0`) | Phase 22, literal typing |
| 21 | SEMA001 | cannot assign LREAL to UDINT, UINT or USINT (unsigned plus untyped literal) | Phase 22, literal typing |
| 11 | SEMA001 | array index must be an integer type, got LREAL (same cause) | Phase 22, literal typing |
| 30 | SEMA021 | cannot pass DINT as input parameter `<id>` (expected BYTE or WORD) | Phase 22, literal typing |
| 15 | SEMA021 | cannot pass WORD as input parameter `<id>` (expected UINT) | Phase 22, implicit conversion |
| 1 | SEMA001 | cannot assign WORD to UINT | Phase 22, implicit conversion |
| 9 | SEMA001 | boolean operator AND/OR requires BOOL operands (integer operands) | Phase 22, vendor strictness |
| 1 | SEMA003 | cannot compare BYTE and DINT | Phase 22, literal typing |
| 1 | SEMA001 | cannot assign DINT to INT | Phase 22, literal typing |
| 3 | SEMA024 | type `<id>` has no member `<id>` | Genuine |

The three genuine SEMA024 are the `batches REF= settings.p_stat_Batches` mismatch that 20-09 found, and two reads of members that the struct does not declare.

**st301.st, 1782 errors**

| Count | Code | Message template | Owner |
|-------|------|------------------|-------|
| 1518 | SEMA010 | undeclared identifier `<id>` (SVNCore GVLs, FBs and functions absent from the single file) | Phase 21, missing library |
| 235 | SEMA037 | undeclared type `<id>` (SVNCore and EtherCAT terminal types) | Phase 21, missing library |
| 24 | SEMA033 | GVL `<id>` is qualified_only; use `<id>` | Phase 21, flattening artifact |
| 4 | SEMA001 | cannot assign LREAL to UINT, UDINT or REAL | Phase 22, literal typing |
| 1 | SEMA001 | cannot assign DINT to INT | Phase 22, literal typing |

The flattener merged 17 TwinCAT GVL files into one GVL named `st301`. Bare accesses to variables of the original GVLs therefore report SEMA033. Multi-file project loading removes this.

**Totals by owner**

| Owner | svncore | st301 |
|-------|---------|-------|
| Phase 21, missing library or input | 32 | 1777 |
| Phase 22, literal typing, conversions, built-ins, strictness | 135 | 5 |
| Genuine | 3 | 0 |

## Hand-off re-run

- `stc check tests/twincat_probes/action_inside.st` reports one error, "cannot assign DINT to INT" on line 13. That is the Phase 22 literal-typing message.
- `stc check` reports no errors on `empty_args_test.st` and `gvl_test.st`. `action_test.st` reports only the same Phase 22 message on line 25.
- `pkg/checker/action_test.go` has two new subtests. A PROGRAM action and an FB action drive TON and R_TRIG and read Q and ET, and both check clean.
- `pkg/interp/action_test.go` has a new subtest. A PROGRAM action drives TON and R_TRIG across 40 ms ticks. It verifies the one-scan edge, TON done after PT, and the reset when IN drops.

## Final gate

All of these pass: `go test ./... -count=1`, `go vet ./...`, `go run ./cmd/stc test tests/` with 237 of 237 passing, `bash scripts/coverage-gate.sh` and `STC_COVER_TOOL=1 bash scripts/coverage-gate.sh`.

| Package | Covered | Percent | Minimum |
|---------|---------|---------|---------|
| pkg/parser | 1183/1205 | 98.17% | 95% |
| pkg/lexer | 236/242 | 97.52% | 95% |
| pkg/checker | 1602/1622 | 98.77% | 94% |
| pkg/interp | 2151/2193 | 98.08% | 95% |
| pkg/types | 128/128 | 100.00% | 95% |
| pkg/emit | 647/662 | 97.73% | 95% |
| total | 9778/10185 | 96.00% | 85% |

## Decisions Made

- The oracle templater also keeps elementary type names, operator keywords and the attribute names `qualified_only`, `strict` and `to_string`. `TestTemplateMessage` pins this so the log stays readable without leaking names.
- `TestTwinCATDialectCheck` uses the same pipeline as `stc check`: `parser.Parse` diagnostics plus `analyzer.Analyze` with a nil config.

## Deviations from Plan

### Auto-fixed Issues

None. No implementation bugs surfaced. Every new TEST_CASE passed on its first run.

### Other deviations

**1. Validation map has 25 rows, not 23.** The plan expects 23 Phase 20 tasks. The nine plans contain 25 tasks: 2 + 3 + 2 + 3 + 3 + 3 + 3 + 3 for 20-08 + 3 for 20-09. Every task has a row, and every row was re-run green.

**2. No RED commits.** Tasks 1 and 2 are marked tdd but only add tests for behaviour that 20-01..20-09 already built, so there was no failing state to commit. Task 2 also adds `TestTemplateMessage` for the oracle's anonymiser.

**3. Combined-run log line.** The oracle logs the top 15 buckets of the combined run. The per-file buckets above came from local `stc check --format json` runs, templated before recording.

## Known Stubs

None.

## Issues Encountered

- `tests/twincat_probes/ptr.st` parses clean, but `stc check` reports `ADR` and `SIZEOF` as undeclared. The fixture gate checks parsing only, so it passes. This is in deferred-items.md for Phase 22.
- `{attribute "qualified_only"}` with double quotes appears once in each of st101, st201 and st301. Whether TwinCAT honours the double-quoted form was not verified. This is in deferred-items.md.
- st201 still has one parse diagnostic, the chained assignment that research deferred. It is not an oracle file.

## Next Phase Readiness

- Phase 20 is complete. DIAL-10 is satisfied by the zero-parse-error oracle assertion.
- Phase 21 owns 1777 st301 errors and 32 svncore errors: missing SVNCore and Tc2 libraries, plus the flattened GVLs.
- Phase 22 owns literal typing, conversions, ADR/SIZEOF/SHL, METHOD body checking and the RUNT-05 allowlist entry in `tests/twincat_dialect_check_test.go`.

## Self-Check: PASSED

All created files exist and commits f0fb6c7, 54d73f0, a358ea6 and 8347e0a are in the branch history. No tracked files were deleted.
