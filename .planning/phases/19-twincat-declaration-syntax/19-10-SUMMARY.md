---
phase: 19-twincat-declaration-syntax
plan: 10
subsystem: cli
tags: [cli, gvl, twincat, json, incremental]

requires:
  - phase: 19-02
    provides: ast.SanitizeGVLName, ast.SetGVLName
  - phase: 19-05
    provides: top-level VAR_GLOBAL as *ast.GVLDecl named from the file basename; KindGVL symbols
provides:
  - "--gvl-name on stc parse, check, fmt and emit"
  - "cmd/stc/gvlname.go: addGVLNameFlag, validateGVLName, applyGVLName"
  - "TestGVLNameFlag and TestParseJSONECT exec tests (Phase 19 success criteria 1 and the --gvl-name part of 2)"
affects: [19-09, 20, 21]

tech-stack:
  added: []
  patterns:
    - "CLI-level AST overrides are applied after parsing, never inside the incremental parse path"
    - "Usage errors under --format json are a {\"error\": \"...\"} object on stdout with exit 1"

key-files:
  created:
    - cmd/stc/gvlname.go
    - cmd/stc/gvl_name_test.go
  modified:
    - cmd/stc/parse.go
    - cmd/stc/check.go
    - cmd/stc/fmt_cmd.go
    - cmd/stc/emit_cmd.go

key-decisions:
  - "--gvl-name with != 1 input file: text mode returns the error so cobra prints it with usage (exit 1); JSON mode prints {\"error\"} on stdout and exits 1"
  - "stc check applies --gvl-name to incrResult.Files after ia.Parse and before analyzer.Analyze"

requirements-completed: [DIAL-02]

duration: 15min
completed: 2026-10-05
---

# Phase 19 Plan 10: --gvl-name CLI override Summary

**`--gvl-name` on parse, check, fmt and emit, sanitised through ast.SanitizeGVLName, stable across incremental cache hits, plus an exec test pinning the ECT parse JSON shape.**

## Performance

- **Duration:** about 15 min
- **Completed:** 2026-10-05
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- One shared helper file registers, validates and applies the flag for all four commands.
- `stc check --gvl-name G` renames after the incremental parse, so a cache hit (`0/1 files re-parsed`) gives the same diagnostics as a cold run. The test asserts both the hit and the equality.
- More than one input file is rejected before any file is read. Under `--format json` stdout is `{"error":"--gvl-name requires exactly one input file"}`.
- `TestParseJSONECT` pins qualified_only (no value key) on GVLDecl ECT, the three X attributes in source order, the unescaped apostrophe on nLost, `%I*` on ST_EL1008.I1, and that `--gvl-name ECT2` changes only the name.
- gvlname.go is 100% covered by the exec tests.

## Task Commits

1. **Task 1 RED: failing --gvl-name tests** - `3fbe863` (test)
2. **Task 1 GREEN: --gvl-name on four commands** - `548cc9a` (feat)
3. **Task 2: ECT JSON acceptance test** - `bbfd15e` (test)

## Files Created/Modified

- `cmd/stc/gvlname.go` - flag registration, single-file validation, sanitised rename
- `cmd/stc/gvl_name_test.go` - TestGVLNameFlag (16 subtests) and TestParseJSONECT (6 subtests); reuses findNodes from parse_attr_test.go
- `cmd/stc/parse.go`, `fmt_cmd.go`, `emit_cmd.go` - register flag, validate first, rename after parsing
- `cmd/stc/check.go` - register flag, validate first, rename after ia.Parse

## Decisions Made

- Validation runs before the existing "no input files" checks, and only when the flag is explicitly set (`Flags().Changed`), so an empty `--gvl-name ""` still counts as set and gets sanitised to `GVL`.
- In text mode the validation error is returned to cobra, which prints `Error: ...` plus usage on stderr and main exits 1.

## Deviations from Plan

- **TDD order:** the implementation was drafted before the tests. RED was then verified by restoring the four original command files and removing gvlname.go: both new tests failed, then the work was committed as test then feat.
- **Task 2 has no RED phase.** The ECT JSON shape already existed from 19-03 to 19-05, so TestParseJSONECT is a contract test. Only its `--gvl-name ECT2` subtest depends on this plan, and it failed against the pre-plan code.
- **StructMember lookup** matches on `at_address` plus name rather than on kind, because StructMember still serialises with `"kind": "SourceFile"` (deferred item from 19-02).

Otherwise the plan was executed as written.

## Issues Encountered

None.

## Known Stubs

None.

## Next Phase Readiness

- DIAL-02 is complete.
- 19-09 Task 3 (phase coverage gate) can now run; the new cmd/stc code is fully covered.
- Pre-existing: cmd/stc/check_test.go and cmd/stc/main.go are not gofmt-clean; not touched here.

## Self-Check: PASSED

- FOUND: cmd/stc/gvlname.go, cmd/stc/gvl_name_test.go
- FOUND commits: 3fbe863, 548cc9a, bbfd15e
- `go test ./cmd/stc -count=1` passes
