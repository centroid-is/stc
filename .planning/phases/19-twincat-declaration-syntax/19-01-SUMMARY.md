---
phase: 19-twincat-declaration-syntax
plan: 01
subsystem: testing
tags: [coverage, ci-gate, fixtures, twincat, interp]

requires: []
provides:
  - pkg/interp coverage headroom (96.10% under -coverpkg=./..., gate 95%)
  - scripts/coverage-gate.sh, a local replica of the CI coverage gate with per-package PASS/FAIL
  - tests/twincat_probes/ with 11 one-construct TwinCAT fixtures and README
  - pre-phase oracle baseline diagnostic counts
affects: [19-02, 19-03, 19-04, 19-05, 19-06, 19-07, 19-08, 19-09, 19-10]

tech-stack:
  added: []
  patterns:
    - "Coverage is judged by scripts/coverage-gate.sh (merged unit + exec profile, block covered if any duplicate entry > 0)"
    - "TwinCAT probe fixtures live in tests/twincat_probes/ without _test suffix so stc test ignores them"

key-files:
  created:
    - pkg/interp/stdlib_errors_test.go
    - scripts/coverage-gate.sh
    - tests/twincat_probes/README.md
    - tests/twincat_probes/action_inside.st
    - tests/twincat_probes/ECT.st
    - tests/twincat_probes/enum_attr.st
    - tests/twincat_probes/{gvl1,gvl2,structat,structpragma,prog,action,link,fbat}.st
  modified: []

key-decisions:
  - "coverage-gate.sh hard-codes the .testcoverage.yml thresholds (95/95/94/95/95/95, total 85) rather than parsing YAML, keeping it stdlib/awk only"
  - "Per-plan coverage verification uses scripts/coverage-gate.sh; go-test-coverage is opt-in via STC_COVER_TOOL=1 because it needs network and pulls go1.27"

patterns-established:
  - "Interp coverage tests call StdlibFunctions entries directly with a table of {fn, args, errSub|want}"

requirements-completed: []  # enabling work only; DIAL-01/02/03/05/08 are delivered by plans 19-02..19-10

duration: 25min
completed: 2026-10-05
---

# Phase 19 Plan 01: Coverage headroom, local gate and TwinCAT probe fixtures Summary

**pkg/interp lifted from 94.02% to 96.10% with test-only stdlib and scan I/O size tests, a one-command local replica of the CI coverage gate, 11 committed TwinCAT probe fixtures, and the pre-parser-change oracle baseline (st301 2711, svncorecomponents 1430 diagnostics).**

## Performance

- **Duration:** about 25 min
- **Completed:** 2026-10-05
- **Tasks:** 3 of 3
- **Files created:** 14

## Accomplishments

- `pkg/interp/stdlib_errors_test.go` covers 33 previously uncovered statements: UPPER_BOUND error and happy paths, conversion arg-count errors, INSERT/DELETE/REPLACE edge positions, MIN/MAX/LIMIT branches, nil-aggregate Clone, and `%IB`/`%ID`/`%QB`/`%QD`/`%MD` scan bindings parsed from real ST.
- `scripts/coverage-gate.sh` builds exactly the CI merged profile and prints a per-package table; exit 1 on any FAIL. Its total (7219/7760, 93.0%) matches go-test-coverage v2.20.0 run on the same profile, and the tool reported all overrides satisfied.
- `tests/twincat_probes/` holds the eight verbatim probes plus three synthetic shapes; `go test ./tests/` and `stc test tests/` (204/204) are unaffected.

## Coverage table (scripts/coverage-gate.sh, after Task 1)

| package | covered | percent | min | result |
|---------|---------|---------|-----|--------|
| pkg/parser | 838/874 | 95.88% | 95% | PASS |
| pkg/lexer | 228/234 | 97.44% | 95% | PASS |
| pkg/checker | 688/726 | 94.77% | 94% | PASS |
| pkg/interp | 1527/1589 | 96.10% | 95% | PASS |
| pkg/types | 124/124 | 100.00% | 95% | PASS |
| pkg/emit | 515/539 | 95.55% | 95% | PASS |
| total | 7219/7760 | 93.03% | 85% | PASS |

pkg/interp before Task 1: 1494/1589 (94.02%), matching research. Interp now has 32 statements of slack over 95% at the current size (1510 needed of 1589); slack shrinks as new statements are added, so later plans must cover their own code.

Note the thin margins elsewhere: pkg/parser has about 7 statements of slack, pkg/checker about 6, pkg/emit about 3. Plans 19-02+ touching those packages must test new code fully.

## Oracle baseline (pre-parser-change, binary built at d43606f, no parser changes yet)

`stc parse` stderr diagnostic lines, run in /Users/jonb/Projects/beckhoff-docs/stc-probes:

| file | diagnostics |
|------|-------------|
| st301.st | 2711 |
| svncorecomponents.st | 1430 |

Per committed fixture (`stc parse tests/twincat_probes/<f>`):

| fixture | diagnostics |
|---------|-------------|
| ECT.st | 28 |
| action.st | 4 |
| action_inside.st | 10 |
| enum_attr.st | 5 |
| fbat.st | 0 |
| gvl1.st | 2 |
| gvl2.st | 2 |
| link.st | 12 |
| prog.st | 10 |
| structat.st | 12 |
| structpragma.st | 0 |

fbat.st and structpragma.st already parse cleanly; for those the Phase 19 work is in the checker/interpreter, not the parser.

## Task Commits

1. **Task 1: Lift pkg/interp coverage** - `2f1cd8b` (test)
2. **Task 2: scripts/coverage-gate.sh** - `d43606f` (chore)
3. **Task 3: TwinCAT probe fixtures + baseline** - `7722d29` (test)

## Decisions Made

- Thresholds are hard-coded in the script, mirroring .testcoverage.yml, to avoid a YAML parser dependency. If .testcoverage.yml changes, update the awk BEGIN block.
- The script writes into a `mktemp -d` directory under `$TMPDIR` and prints the merged profile path last.

## Deviations from Plan

- Requirements DIAL-01, DIAL-02, DIAL-03, DIAL-05 and DIAL-08 were NOT marked complete. This plan only builds test assets for them; `requirements.mark-complete` would have checked them off prematurely, so REQUIREMENTS.md was left unchanged.
- Task 1 additionally covers two `Value.Clone` nil-aggregate branches in value.go and the LIMIT arg-count error; these are test-only additions within the task's intent.

## Issues Encountered

- `go run github.com/vladopajic/go-test-coverage/v2@latest` resolves v2.20.0, which requires go >= 1.27 and auto-downloads go1.27.1. It works locally but is slow on first run; hence opt-in.

## Known Stubs

None.

## Next Phase Readiness

Every later Phase 19 plan can verify with `bash scripts/coverage-gate.sh` and the fixtures in tests/twincat_probes/. The baseline above is the comparison point for the parser residual classification.

## Self-Check: PASSED

- FOUND: pkg/interp/stdlib_errors_test.go, scripts/coverage-gate.sh (executable), tests/twincat_probes/ (11 .st + README.md)
- FOUND commits: 2f1cd8b, d43606f, 7722d29
