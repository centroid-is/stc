---
phase: 19-twincat-declaration-syntax
plan: 09
subsystem: testing
tags: [phase-gate, oracle, twincat, lsp, coverage, validation]

requires:
  - phase: 19-twincat-declaration-syntax
    provides: attributes/pragmas (19-03), struct AT and empty args (19-04), GVLs (19-05, 19-06, 19-10), ACTIONs (19-07, 19-08), probe fixtures and coverage gate (19-01)
provides:
  - tests/twincat_probes_test.go, a CI-safe fixture gate plus an STC_PROBES_DIR-gated oracle classification test
  - pkg/lsp/twincat_test.go, LSP robustness on GVLs, attributes and actions
  - LSP hover/definition on variables of qualified_only GVLs
  - signed-off 19-VALIDATION.md (nyquist_compliant true)
  - Phase 20 hand-off with residual oracle diagnostics bucketed by construct
affects: [20]

tech-stack:
  added: []
  patterns:
    - "Phase acceptance oracle: committed fixtures always run; customer sources only via an env var, logging counts and line numbers only"
    - "Owned-token line classification separates this phase's residual errors from the next phase's"

key-files:
  created:
    - tests/twincat_probes_test.go
    - pkg/lsp/twincat_test.go
  modified:
    - pkg/lsp/navigate.go
    - .planning/phases/19-twincat-declaration-syntax/19-VALIDATION.md

key-decisions:
  - "gofmt is not enforced by CI (lint job runs go vet and a best-effort golangci-lint without a formatter), so the 29 pre-existing non-gofmt files were left untouched"
  - "LSP resolves a qualified_only GVL variable through the GVL struct type, preferring the GVL in the cursor's own file, then GVL name order"

patterns-established:
  - "Oracle tests read external sources only through STC_PROBES_DIR and t.Skip when it is unset or the files are missing"

requirements-completed: [DIAL-01, DIAL-02, DIAL-03, DIAL-05, DIAL-08]  # already marked complete by earlier plans; not re-marked

duration: 35min
completed: 2026-10-06
---

# Phase 19 Plan 09: Phase gate, oracle classification and validation sign-off Summary

**Automated Phase 19 acceptance gate: all 11 TwinCAT probes parse clean except the three known Phase 20 lines, the oracle files dropped from 2711/1430 to 917/615 diagnostics with none on a Phase 19 line, the coverage gate passes at 94.30% total, and the LSP now resolves qualified_only GVL variables.**

## Performance

- **Duration:** about 35 min
- **Completed:** 2026-10-06
- **Tasks:** 3 of 3
- **Files:** 2 created, 2 modified

## Accomplishments

- `TestTwinCATProbeFixtures` walks `tests/twincat_probes/*.st`. Diagnostics are allowed only on prog.st line 9 and link.st lines 9 and 10. Every clean fixture round-trips through fmt, re-parses with zero diagnostics, and is a fixed point on the second format. `TestOwnedLine` pins the classifier.
- `TestTwinCATProbeOracle` skips without `STC_PROBES_DIR`. With it set, it fails on any diagnostic on an owned-token line and logs the counts.
- `pkg/lsp/twincat_test.go` covers GVL hover, definition and references with ECT.st, action references in action_inside.st, and semantic tokens over seven attribute fixtures. It also covers member lookup across several qualified_only GVLs.
- 19-VALIDATION.md has a row for each of the 27 tasks in plans 19-01 to 19-10. Every automated command was re-run and passed on this branch.

## Oracle results

Classification check used, run on this branch:

```
STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests/ -run TestTwinCATProbeOracle -count=1 -v
```

It passed. No remaining diagnostic on either file lands on a line containing `{attribute`, `{warning`, `VAR_GLOBAL`, `ACTION`, `END_ACTION`, ` AT %`, `:= ,`, `:=,`, `:= )`, `:=)`, `=> ,`, `=>,`, `=> )` or `=>)`. A separate script over the `stc parse` stderr output reached the same result.

| file | before (19-01 baseline) | after (19-09) | lines with diagnostics |
|------|------|------|------|
| st301.st | 2711 | 917 | 202 |
| svncorecomponents.st | 1430 | 615 | 206 |

Probe copies in the oracle directory, counted from `stc parse` stderr:

| probe | after |
|-------|-------|
| gvl1.st | 0 |
| gvl2.st | 0 |
| structat.st | 0 |
| structpragma.st | 0 |
| action.st | 0 |
| prog.st | 3, all on line 9 (bit access) |
| link.st | 12, all on lines 9 and 10 (named function args) |

### Residual buckets (diagnostic count / distinct lines)

Each diagnostic line was bucketed by a regex over its source text, after string literals and comments were stripped. Continuation lines of multi-line initialisers count under the initialiser bucket. Cascading errors on one line count once per diagnostic. Source text is not reproduced here because these are customer files.

| Phase 20 construct | st301.st | svncorecomponents.st |
|--------------------|----------|----------------------|
| array/struct initialisers (incl. multi-line array-of-struct literals) | 700 / 130 | 17 / 1 |
| bit access (`w.3`, `arr[0].1`) | 192 / 64 | 177 / 26 |
| qualified enum CASE labels (`E.val:`) | 0 | 326 / 161 |
| named function args in expressions | 12 / 2 | 70 / 9 |
| trailing comma before `)` | 10 / 5 | 2 / 1 |
| enum base types (`) USINT;`) | 0 | 10 / 5 |
| namespace-qualified type (`Tc2_EtherCAT.ST_EcSlaveState`) | 0 | 7 / 1 |
| typed based literals (`BYTE#16#10`) | 0 | 5 / 1 |
| empty declaration (`REAL;;`) | 3 / 1 | 0 |
| REF= | 0 | 1 / 1 |
| THIS^ / SUPER^ | 0 | 0 |
| other | 0 | 0 |
| **total** | **917 / 202** | **615 / 206** |

## Coverage (scripts/coverage-gate.sh, final run)

| package | covered | percent | min | result |
|---------|---------|---------|-----|--------|
| pkg/parser | 1006/1039 | 96.82% | 95% | PASS |
| pkg/lexer | 228/234 | 97.44% | 95% | PASS |
| pkg/checker | 837/873 | 95.88% | 94% | PASS |
| pkg/interp | 1673/1732 | 96.59% | 95% | PASS |
| pkg/types | 124/124 | 100.00% | 95% | PASS |
| pkg/emit | 571/586 | 97.44% | 95% | PASS |
| total | 8009/8493 | 94.30% | 85% | PASS |

`STC_COVER_TOOL=1 bash scripts/coverage-gate.sh` also ran go-test-coverage v2.20.0 against .testcoverage.yml. It reported package and total thresholds satisfied, with total 94.3% (8009/8493). `git diff main -- .testcoverage.yml` is empty.

Other gates: `go vet ./...` is clean and `go test ./... -count=1` passes in all packages. `go run ./cmd/stc test tests/` passes 216 of 216 tests. `stc test tests/twincat_dialect` passes 12 of 12.

## Hand-off to Phase 20

- **Success criterion 5 is proven for checking only on standard-FB-free sources.** An action-chain MAIN parses, checks and executes. But `stc check` still rejects standard FB parameters until RUNT-08 lands in Phase 20. On action_inside.st it reports `TON has no input parameter "IN"` and `"PT"`, and `R_TRIG has no input parameter "CLK"`. The interpreter runs these calls correctly. Phase 20 must re-run these tests with timers and edge triggers inside actions once RUNT-08 is in:
  - `pkg/checker/action_test.go` TestAction (19-07). Its sources contain no TON or TOF.
  - `tests/twincat_dialect/action_test.st` (19-08). Its four TEST_CASEs run through `stc test`, which does not check. One of them uses R_TRIG.
  - `tests/twincat_dialect/empty_args_test.st` (19-08). It includes a timer case that is run but never checked.
  - `pkg/interp/action_test.go` TestAction and TestZeroArgFBCall (19-08).
  - `stc check tests/twincat_probes/action_inside.st`. It should drop to the integer-literal issue only, until that is also fixed.
- **Integer literals type as DINT.** `n := n + 1` with `n : INT` reports `cannot assign DINT to INT` on action_inside.st line 13, as recorded in deferred-items for 19-07.
- **Residual oracle diagnostics are all Phase 20 constructs.** The buckets above are the Phase 20 worklist. Expected impact, in order: initialisers and bit access dominate st301, and qualified enum CASE labels dominate svncorecomponents.
- **Constructs not named in the Phase 20 list.** The empty declaration `REAL;;` appears once, in st301. The namespace-qualified type `Tc2_EtherCAT.ST_EcSlaveState` appears once, in svncorecomponents. Phase 20 planning should decide whether they belong there.
- **Trailing comma before `)`** is the 5 + 1 case recorded under 19-04 in deferred-items. It is classified as Phase 20 and was not implemented here.

## Task Commits

1. **Task 1: Committed fixture gate and env-gated oracle test** - `1dbd0c4` (test)
2. **Task 2: LSP robustness on TwinCAT constructs** - `05fc1b3` (test, includes the navigate.go fix)
3. **Task 3: Full gate run, oracle counts and validation sign-off** - `ac86346` (docs)

## Decisions Made

- gofmt was left alone. CI's lint job runs `go vet ./...` plus a best-effort golangci-lint with `continue-on-error` and no formatter enabled, so nothing enforces gofmt. `gofmt -l ./cmd ./pkg ./tests` lists 29 pre-existing files, none of them created or modified by this plan. The 19-09 files are gofmt-clean.
- The LSP fallback for qualified_only GVL variables returns a synthesized variable symbol whose position is the GVL's position. The resolver keeps no per-member position for these variables, so go-to-definition lands on the GVL rather than on the member line.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] LSP hover returned nothing on variables of a qualified_only GVL**
- **Found during:** Task 2
- **Issue:** The resolver registers qualified_only GVL variables only as members of the GVL symbol's struct type, never in a scope. `findSymbolAtPosition` searches only scopes, so hover on `X` in ECT.st returned nil.
- **Fix:** Added `findGVLMember`, which checks the GVLs that declare the name. The GVL in the cursor's own file wins, then GVL name order decides. Tests cover the same-file preference, the name tie-break, a missing name and a non-struct GVL type, giving the function 100% coverage.
- **Files modified:** pkg/lsp/navigate.go, pkg/lsp/twincat_test.go
- **Commit:** 05fc1b3

### TDD notes

- Tasks 1 and 2 are acceptance gates over code delivered in 19-02 to 19-10. Task 1's tests passed on their first run, which is expected for a gate over finished work. Task 2's GVL hover test did fail first, and that exposed the bug above. Both tasks were committed as `test(...)` commits.

## Issues Encountered

- The first oracle classifier matched string-literal contents such as `'ST301.A1.01'` as bit access. It also missed the continuation lines of multi-line array-of-struct initialisers. The script was fixed to strip string literals and comments first, and to give those continuation lines to the initialiser bucket. The table above uses the fixed script, which leaves nothing in "other".

## Known Stubs

None.

## Threat Flags

None. The oracle test reads only through STC_PROBES_DIR (T-19-18) and .testcoverage.yml is unchanged (T-19-19).

## Next Phase Readiness

Phase 19 is verified end to end and ready for a PR with CI on macOS, Windows and Linux. Phase 20 starts from the residual buckets and the hand-off list above.

## Self-Check: PASSED

- FOUND: tests/twincat_probes_test.go, pkg/lsp/twincat_test.go, pkg/lsp/navigate.go, .planning/phases/19-twincat-declaration-syntax/19-VALIDATION.md
- FOUND commits: 1dbd0c4, 05fc1b3, ac86346
