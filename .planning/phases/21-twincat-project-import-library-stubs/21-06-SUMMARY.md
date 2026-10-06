---
phase: 21-twincat-project-import-library-stubs
plan: 06
subsystem: testing
tags: [twincat, oracle, phase-gate, docs, validation]

requires:
  - phase: 21-05
    provides: stc vendor import, check/test/sim project modes, extract JSON
  - phase: 21-04
    provides: twincat.Import, analyzer.AnalyzeProject, vendor.ExtractProject
provides:
  - STC_SILD_DIR oracle (TestSildarvinnslaImport, TestSildarvinnslaExtract, TestClassifySild)
  - Data-driven owner allowlists (phase22, deferred, genuine, phase24)
  - TwinCAT import docs in VENDOR_LIBRARIES.md and CLI_REFERENCE.md
  - Signed-off 21-VALIDATION.md
affects: [22, 23, 24]

tech-stack:
  added: []
  patterns:
    - "Oracle allowlist rules are (bucket, code, anchored regex, optional project, optional file); unmatched errors fail"
    - "SEMA022, SEMA033, VEND020 and parse errors are always Phase 21 owned; unmatched SEMA037/SEMA010 default to Phase 21"

key-files:
  created:
    - tests/twincat_import_test.go
  modified:
    - docs/VENDOR_LIBRARIES.md
    - docs/CLI_REFERENCE.md
    - .planning/phases/21-twincat-project-import-library-stubs/deferred-items.md
    - .planning/phases/21-twincat-project-import-library-stubs/21-VALIDATION.md

key-decisions:
  - "Three pre-existing checker bugs found by the oracle are allowlisted in a separate deferred bucket, not fixed here: method call statements with arguments report no member, external read of an FB internal VAR, LEN typed as STRING"
  - "Pointer comparison with 0 and POINTER/STRING indexing are Phase 22 (pointer semantics with PVOID)"
  - "Unresolved libraries fail the oracle through Model.Libraries, since VEND020 is a warning"

requirements-completed: [IMPT-01, IMPT-02, IMPT-03, IMPT-04, IMPT-05]

duration: 30min
completed: 2026-10-06
---

# Phase 21 Plan 06: Phase gate on the sildarvinnsla projects Summary

**All five real TwinCAT projects import and check with zero Phase-21-owned errors. Every residual error lands in a named owner bucket, and vendor extract renders all 65 SVNCoreComponents objects as parsing stubs.**

## Performance

- **Duration:** about 30 min
- **Tasks:** 3/3
- **Files:** 1 created, 4 modified

## Oracle results

Command, run locally against sildarvinnsla HEAD:

```
STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run 'TestSildarvinnsla' -count=1 -v
```

| Project | PLC | Cycle | Libraries | Errors | phase21 | phase22 | deferred | genuine | phase24 |
|---------|-----|-------|-----------|--------|---------|---------|----------|---------|---------|
| ST301 | ST301, port 851 | 1 ms, MAIN | SVNCore sibling; Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_System, Tc3_Module stub; Tc2_Standard builtin | 37 | 0 | 34 | 0 | 3 | 0 |
| ST101 | ST101, port 851 | 1 ms, MAIN | same as ST301 | 43 | 0 | 43 | 0 | 0 | 0 |
| ST201 | ST201, port 851 | 1 ms, MAIN | same as ST301 | 33 | 0 | 31 | 0 | 2 | 0 |
| Baader | gagnasofnun, port 851 | 20 ms, MAIN | SVNCore sibling; Tc2_System, Tc3_IPCDiag, Tc3_Module stub; Tc2_Standard builtin | 270 | 0 | 204 | 14 | 0 | 52 |
| SVNCoreComponents | SVNCoreComponents | 10 ms default (plcproj only) | Tc2_EtherCAT, Tc2_System stub; Tc2_Standard builtin | 124 | 0 | 121 | 0 | 3 | 0 |

Errors by code:

| Project | Codes |
|---------|-------|
| ST301 | SEMA021 16, SEMA001 9, SEMA010 9, SEMA024 2, SEMA037 1 |
| ST101 | SEMA010 15, SEMA001 14, SEMA021 14 |
| ST201 | SEMA021 16, SEMA010 9, SEMA001 4, SEMA024 1, SEMA037 1, P001 1, P002 1 |
| Baader | SEMA010 80, SEMA001 49, SEMA024 49, SEMA023 43, SEMA003 33, SEMA037 16 |
| SVNCoreComponents | SEMA021 50, SEMA001 46, SEMA010 16, SEMA003 9, SEMA024 2, SEMA037 1 |

- ST301 matches the research projection exactly: 34 Phase 22 errors and 3 genuine ones. No unresolved library, SEMA022, SEMA033, VEND020 or parse error appears in any project.
- E_EcSlaveState, EcDiagParam and FB_EcDeviceDiag resolve from SVNCoreComponents sources.
- Extract on the SVNCoreComponents plcproj returns 65 objects: 27 POUs, 36 DUTs and 2 GVLs. Every one parses, and FB_ATV320 keeps its methods.
- Phase 22 is landing concurrently. Remove its literal-typing and built-in rules from sildRules as it lands. Stale rules do not fail the test, so shrinking them is a manual step.

## Coverage

| Package | Coverage | Gate |
|---------|----------|------|
| pkg/parser | 98.35% | 95% |
| pkg/lexer | 97.60% | 95% |
| pkg/checker | 98.59% | 94% |
| pkg/interp | 98.10% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.32% | 95% |
| total | 96.46% | 85% |
| pkg/twincat | 94.8% | not gated |
| pkg/vendor | 88.3% | not gated |
| stdlib/beckhoff | 96.6% | not gated |

`go vet ./...`, `go test ./... -count=1` and `stc test tests/` (237 of 237) pass. `.testcoverage.yml` is unchanged.

## Task Commits

1. **Task 1: Oracle with owner buckets:** `bfaf2a0` (test)
2. **Task 2: Docs and deferred items:** `782c985` (docs)
3. **Task 3: Full suite, coverage gate, VALIDATION sign-off:** `32add3f` (docs)

## Deviations from Plan

### Auto-fixed Issues

None.

### Other adjustments

- **Deferred bucket.** The oracle surfaced three checker bugs that predate Phase 21. A method call used as a statement with arguments reports "no member" (10 Baader errors). An external read of an FB's internal VAR reports "no member" (3). LEN is typed as STRING by generic candidate resolution (1). They sit in a `deferred` bucket with repros in deferred-items.md instead of being fixed here, because Phase 22 is editing the checker concurrently.
- **Package paths.** The plan and earlier verify commands named `pkg/vendor/twincat` and `stdlib/vendor/beckhoff`. VALIDATION.md uses the current `pkg/twincat` and `stdlib/beckhoff` paths, and every row was re-run green.
- **VEND020 check.** VEND020 is a warning, so the oracle fails on any `unresolved` library in the model rather than on error diagnostics.
- **SVNCoreComponents task.** As a bare plcproj it has no tsproj, so it gets the 10 ms default task with no programs. The oracle asserts no model fields for it.

## Assumption awaiting the user

- **A2.** stc treats `{attribute "qualified_only"}` with double quotes as ignored by TwinCAT and warns with SEMA039. If TwinCAT honours it, the line projects have 23 real SEMA033 findings.

## Genuine sildarvinnsla findings for the user

- ST201 SPB02 and ST301 SPB03 still declare `FB_TwoWayConveyor`, which SVNCoreComponents renamed to `FB_BatchConveyor`.
- ST201 and ST301 MAIN, and SVNCoreComponents FB_Conveyor, use `ST_LineRecipe.stopDistanceFromEnd` and `drivePastForDelivery`, which no longer exist.
- SVNCoreComponents FB_Conveyor uses `ST_Batch`, which is declared nowhere.

## Threat mitigations

- T-21-13: the oracle reads only from STC_SILD_DIR, and nothing from sildarvinnsla is copied into the repo. Logs use templated messages and file basenames.
- T-21-14: allowlist rules are anchored regexes with owner comments. Unmatched errors fail the test. TestClassifySild pins the routing, including that a SEMA037 for an unknown type falls into Phase 21.

## Self-Check: PASSED

- FOUND: tests/twincat_import_test.go
- FOUND: commits bfaf2a0, 782c985, 32add3f
