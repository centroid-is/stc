---
phase: 21-twincat-project-import-library-stubs
plan: 05
subsystem: cli
tags: [twincat, cli, vendor-import, check, test-runner, sim]

requires:
  - phase: 21-04
    provides: twincat.Import, ParseModel, analyzer.AnalyzeProject, vendor.ExtractProject
provides:
  - stc vendor import (text, JSON, --out writer)
  - stc vendor extract --format json
  - stc check project mode (.tsproj/.plcproj)
  - stc test --project via RunOpts.ProjectFiles
  - stc sim <project> with task program and cycle time
  - twincat.WriteOut, twincat.ParseForTest, twincat.IsSiblingSource
affects: [21-06, 22, 23, 28]

tech-stack:
  added: []
  patterns:
    - "All --out targets validated (absolute, drive, escape) before any write"
    - "Project declarations merged through the test-file declaration path, test file wins by name"
    - "Sibling library projects are real code (ProjectFiles); embedded and library_paths stubs stay LibraryFiles"

key-files:
  created:
    - pkg/twincat/out.go
    - pkg/twincat/out_test.go
    - cmd/stc/vendor_import_test.go
    - cmd/stc/check_project_test.go
    - cmd/stc/project_mode_test.go
    - pkg/testing/project_files_test.go
    - pkg/twincat/testdata/sln/tests/demo_test.st
  modified:
    - cmd/stc/vendor_cmd.go
    - cmd/stc/check.go
    - cmd/stc/test_cmd.go
    - cmd/stc/sim_cmd.go
    - pkg/testing/runner.go
    - pkg/twincat/import.go
    - pkg/twincat/import_test.go
    - pkg/vendor/extract.go

key-decisions:
  - "Sibling library sources run as ProjectFiles under stc test --project; only embedded and library_paths stubs are auto-stubbed"
  - "--out writes libraries flat under libs/<Library>/ because vendor.LoadLibraries reads directories non-recursively; GVL files are named <Name>.st"
  - "Generated stc.toml sets vendor_target = beckhoff and library_paths for libs/<Library> and libs/stubs"
  - "stc check project mode bypasses the incremental cache; text mode prints a source count line instead of the re-parse line"
  - "stc check JSON prints [] instead of null when there are no diagnostics"

requirements-completed: [IMPT-01, IMPT-02, IMPT-03, IMPT-05]

duration: 25min
completed: 2026-10-06
---

# Phase 21 Plan 05: CLI for TwinCAT project import Summary

**`stc vendor import`, `stc check`, `stc test --project` and `stc sim` now take a .tsproj or .plcproj directly. `--out` writes a self-contained stc project that checks with the same result as the on-the-fly import.**

## Performance

- **Duration:** about 25 min
- **Tasks:** 3/3
- **Files:** 7 created, 8 modified

## Accomplishments

- `stc vendor import` prints the PLC, task, source counts and library resolution in text. JSON is the model plus a `diagnostics` array that carries codes.
- `--out` writes compact .st at the plcproj paths, libraries under libs/, and an stc.toml. Every target is validated before the first write, so a traversal Include leaves nothing on disk.
- `stc check <project>` reports Demo with zero errors and the broken fixture with SEMA010 at MAIN.TcPOU line 11.
- `stc test --project` runs the Demo tests against FB_Motor (method and property), F_Add, ST_Point and GVL_Main. No project or sibling FB is auto-stubbed.
- `stc sim <project>` runs the first task's PouCall program, falls back to the first PROGRAM, and defaults `--dt` to the task cycle time.
- `stc vendor extract --format json` prints `{"stubs": [...], "diagnostics": [...]}` in plcproj order.
- The local coverage gate passes with a 96.46% total.

## Real project smoke check (ST301, read-only)

| Command | Result |
|---------|--------|
| `stc vendor import` | Exit 0. PLC ST301, AMS port 851, PlcTask 1ms calling MAIN. |
| Sources | 3 POUs, 17 GVLs, 3 DUTs, 71 library source files |
| Libraries | SVNCoreComponents from a sibling, 4 from stubs, Tc2_Standard built in |
| Import diagnostics | One VEND022 info for the skipped Line3.xti TwinSAFE project |
| `stc check` | Exit 1 with 37 errors and 272 warnings, matching 21-04 |
| `stc check` on the `--out` copy | 37 errors and 272 warnings, identical to the direct check |

Errors by code: SEMA021 16, SEMA001 9, SEMA010 9, SEMA024 2, SEMA037 1. Output went to the session scratchpad and no customer files were written or committed.

## Task Commits

1. **Task 1: vendor import, --out writer, extract JSON:** `6bf114b` (test), `eaa8a90` (feat)
2. **Task 2: stc check project mode:** `fa6a286` (test), `c74a24f` (feat)
3. **Task 3: test --project and sim <project>:** `b8c51f2` (test), `628ecc0` (test), `d577fc7` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Paths follow the 21-04 package move**
- **Found during:** Task 1
- **Issue:** The plan named pkg/vendor/twincat/out.go. The package now lives at pkg/twincat.
- **Fix:** WriteOut lives in pkg/twincat/out.go. The demo test lives at pkg/twincat/testdata/sln/tests/demo_test.st.
- **Commit:** eaa8a90

**2. [Rule 1 - Bug] Sibling library FBs were auto-stubbed under stc test --project**
- **Found during:** Task 3
- **Issue:** Passing every ParseModel library file as LibraryFiles reported FB_LibThing from the sibling DemoLib project as an auto-stub with zero outputs. That code is a real implementation.
- **Fix:** `twincat.ParseForTest` puts sibling library sources first in ProjectFiles. Embedded stubs and library_paths files stay in LibraryFiles.
- **Files modified:** pkg/twincat/import.go, pkg/twincat/import_test.go, cmd/stc/test_cmd.go
- **Commit:** d577fc7

### Other adjustments

- The extract JSON is an object with `stubs` and `diagnostics`. The plan's bare array had no room for diagnostics.
- A test file cannot override a project GVL by name, because a test file's GVL name comes from its `_test.st` basename. The override logic exists, but only the FUNCTION override is tested.
- PROGRAM declarations from ProjectFiles are not registered, matching how the runner treats a test file's own PROGRAMs.
- In text mode, import diagnostics from `vendor import` end with the code in brackets, because `Diagnostic.String()` omits it.
- `--out` with `--format json` prints its "Wrote" line to stderr so stdout stays valid JSON.
- Existing helpers `runStcIn` and `slashPath` were reused instead of being duplicated.

## Notes for 21-06

- `isProjectPath`, `hasErrors`, `exitWithFailure`, `printJSON` and `nonNilDiags` in cmd/stc are shared helpers.
- `stc sim` on Demo fails until Phase 23, because MAIN instantiates user FBs from other files.
- `stc check` on a project ignores `.stc-cache`. Diagnostics point at absolute TcPOU paths.

## TDD Gate Compliance

Each task has `test(21-05)` commits followed by a `feat(21-05)` commit.

## Known Stubs

None.

## Threat Flags

None. T-21-11 is mitigated and unit-tested in TestWriteOutRejectsTraversal. That test covers a backslash traversal, an absolute path, a drive path and a library name that escapes. T-21-12 is covered because every import diagnostic prints in both text and JSON.

## Self-Check: PASSED
