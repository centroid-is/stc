---
phase: 21-twincat-project-import-library-stubs
plan: 04
subsystem: twincat
tags: [twincat, import, library-resolution, analyzer, vendor-extract]

requires:
  - phase: 21-02
    provides: stdlib/beckhoff FS, Closure, IsBuiltin
  - phase: 21-03
    provides: ReadTsproj, ReadPlcproj, ReadTcTTO, mergeTasks, ConvertFile modes, VEND02x codes
provides:
  - pkg/twincat (moved from pkg/vendor/twincat) Import, Options, ParseModel
  - ordered library resolver (siblings, library_paths, stubs) with VEND020/VEND026
  - analyzer.AnalyzeProject(model, cfg, defines)
  - vendor.ExtractProject returning ordered []ExtractedStub plus diagnostics
  - deterministic vendor.LoadLibraries
  - checker support for reading and writing FB PROPERTY members from outside the FB
affects: [21-05, 21-06, 22, 23, 28]

tech-stack:
  added: []
  patterns:
    - "Library sources concatenated as siblings, then library_paths sorted by key, then stub closures; maps used only for lookup"
    - "Sibling search via os.ReadDir (case-insensitive, dot-directories skipped), no globbing"
    - "Import diagnostics sorted by file, line, column, code"

key-files:
  created:
    - pkg/twincat/libs.go
    - pkg/twincat/libs_test.go
    - pkg/twincat/import.go
    - pkg/twincat/import_test.go
    - pkg/twincat/testdata/broken/
    - pkg/analyzer/project.go
    - pkg/analyzer/project_test.go
    - pkg/checker/fb_property_test.go
  modified:
    - pkg/twincat/* (moved from pkg/vendor/twincat)
    - pkg/vendor/extract.go
    - pkg/vendor/extract_test.go
    - pkg/vendor/loader.go
    - pkg/vendor/loader_test.go
    - cmd/stc/vendor_cmd.go
    - cmd/stc/vendor_cmd_test.go
    - pkg/types/types.go
    - pkg/checker/resolve.go
    - pkg/checker/check.go

key-decisions:
  - "TwinCAT package lives at pkg/twincat: Go refuses imports of paths with a /vendor/ segment from outside that tree"
  - "library_paths sources are registered before stub closures so a user override of a stub library wins (orchestrator ruling over the plan's siblings+stubs+library_paths order)"
  - "A configured library_paths directory that does not exist emits a VEND020 warning and resolution falls through to builtin/stub"
  - "Unreadable or malformed items become VEND027 error diagnostics; Import only fails for an unsupported extension, unreadable tsproj/plcproj or broken stc.toml"
  - "FunctionBlockType gains Properties so fb.Prop type-checks from outside the FB"

requirements-completed: [IMPT-01, IMPT-02, IMPT-03, IMPT-05]

duration: 10min
completed: 2026-10-06
---

# Phase 21 Plan 04: Library resolution, Import and AnalyzeProject Summary

**One call turns a tsproj or plcproj into a model with resolved libraries, and `analyzer.AnalyzeProject` checks it with diagnostics at TcPOU XML positions. `vendor extract` now runs on the shared converter and emits ordered, parseable stubs for every object kind.**

## Performance

- **Duration:** about 10 min
- **Tasks:** 3/3
- **Files:** 8 created, 10 modified, the twincat package moved

## Accomplishments

- The Demo tsproj resolves DemoLib from its sibling plcproj and four Tc2/Tc3 libraries from stubs. Tc2_Standard is built in, and Tc2_Missing gets VEND020 at plcproj line 64.
- The `.hidden` decoy is never chosen. Two hits at one ancestor level produce VEND026 and the sorted-first path wins.
- Mutually referencing sibling projects terminate through the visited set.
- `AnalyzeProject` on Demo reports zero errors and one SEMA039 for GVL_Quoted.
- The broken fixture reports SEMA010 at the absolute MAIN.TcPOU path, line 11, column 6.
- `ExtractProject` returns 8 stubs in plcproj order and one VEND021 for Screen.TcVIS. Every stub parses with zero errors.
- Coverage: pkg/twincat 95.0%, pkg/analyzer 96.9%, pkg/checker 98.5%.

## Real project measurement (ST301, read-only)

`Import("/Users/jonb/Projects/sildarvinnsla/ST301/ST301 solution.tsproj")` loads 23 user sources and 71 library sources. SVNCoreComponents resolves as a sibling and the `.claude/worktrees` copy is skipped. Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_System and Tc3_Module resolve from stubs, and Tc2_Standard is built in.

AnalyzeProject errors by code (37 total, research expected about 42):

| Code | Count | Cause |
|------|-------|-------|
| SEMA021 | 16 | Phase 22 literal typing (DINT/LREAL literal into WORD/REAL/BYTE inputs) |
| SEMA001 | 9 | Phase 22 literal typing (LREAL/DINT literal assignments) |
| SEMA010 | 9 | Phase 22 built-ins: ADR 4, SIZEOF 4, UINT_TO_WORD 1 |
| SEMA024 | 2 | Genuine: ST_LineRecipe has no member stopDistanceFromEnd |
| SEMA037 | 1 | Genuine: FB_TwoWayConveyor exists only in the .claude worktree copy |
| SEMA033 | 0 | Downgraded to SEMA039 by 21-01 |

So 34 errors belong to Phase 22 and 3 are genuine source issues.

## Exported API for plan 05

| Name | Purpose |
|------|---------|
| `twincat.Import(path, twincat.Options{Defines}) (*Model, []diag.Diagnostic, error)` | tsproj or plcproj to Model |
| `twincat.ParseModel(m, defines) (user, libs []*ast.SourceFile, []diag.Diagnostic)` | parse with GVL names from Source.Name |
| `analyzer.AnalyzeProject(m, cfg, defines) AnalysisResult` | parse diagnostics, then analysis diagnostics |
| `vendor.ExtractProject(plcproj) ([]vendor.ExtractedStub, []diag.Diagnostic, error)` | `ExtractedStub{Name, RelPath, Kind, Text}` |

The import path is `github.com/centroid-is/stc/pkg/twincat`, not `pkg/vendor/twincat`.

## Task Commits

1. **Task 1: Library resolver:** `e9e8bdc` (test, RED), `7507467` (feat, GREEN), `e6efd46` (style)
2. **Task 2: Import and AnalyzeProject:** `89295d6` (test, RED), `63f2ed6` (refactor, package move), `b6dc624` (fix, FB properties), `1581905` (feat, GREEN)
3. **Task 3: vendor extract and LoadLibraries:** `26874bb` (test, RED), `d43b43a` (feat, GREEN)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Moved pkg/vendor/twincat to pkg/twincat**
- **Found during:** Task 2
- **Issue:** `pkg/analyzer` could not import the package. Go reports "must be imported as twincat" for any path with a `/vendor/` segment.
- **Fix:** `git mv pkg/vendor/twincat pkg/twincat`. Plans 05 and 06 must use the new path.
- **Commit:** 63f2ed6

**2. [Rule 1 - Bug] FB properties were not members for the checker**
- **Found during:** Task 2
- **Issue:** `spd := fbMotor.Speed` in the Demo MAIN failed with SEMA024 because the checker ignored PROPERTY declarations on member access.
- **Fix:** `types.FunctionBlockType.Properties` is filled from PROPERTY declarations and consulted in `checkMemberAccessExpr`. Inherited properties via EXTENDS are not covered, matching how inputs are looked up today.
- **Files modified:** pkg/types/types.go, pkg/checker/resolve.go, pkg/checker/check.go, pkg/checker/fb_property_test.go
- **Commit:** b6dc624

### Other adjustments

- Library order is siblings, then library_paths, then stubs, per the orchestrator ruling. The plan text listed stubs before library_paths.
- The stub import is `stdlib/beckhoff`, as 21-02 built it.
- Diagnostic.String does not print the code, so the CLI test checks for the item name instead of "VEND021". Plan 05 owns CLI output formats.
- The old `ParseTcPOU`, `ParsePlcProj`, `ExtractStub` and XML structs in pkg/vendor were deleted with their tests. Nothing else used them.

## TDD Gate Compliance

Each task has a `test(21-04)` commit followed by a `feat(21-04)` commit.

## Known Stubs

None.

## Threat Flags

None. The sibling search reads only shallow directory listings at most three ancestors up, as planned in T-21-08.

## Self-Check: PASSED

- Created files exist: pkg/twincat/libs.go, pkg/twincat/import.go, pkg/analyzer/project.go, pkg/checker/fb_property_test.go.
- Commits e9e8bdc, 7507467, e6efd46, 89295d6, 63f2ed6, b6dc624, 1581905, 26874bb and d43b43a are on main.
- `go test ./... -count=1` and `go vet ./...` pass.
