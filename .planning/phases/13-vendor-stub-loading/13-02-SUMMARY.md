---
phase: 13-vendor-stub-loading
plan: 02
subsystem: compiler
tags: [vendor, stubs, analyzer, lsp, cli, cross-vendor]

# Dependency graph
requires:
  - phase: 13-01
    provides: LoadLibraries function, ResolveOpts with LibraryFiles, Symbol.IsLibrary flag
provides:
  - AnalyzeOpts with LibraryFiles for library-aware analysis facade
  - CLI check command loading vendor stubs from stc.toml library_paths
  - LSP loading vendor stubs on workspace init for completion/hover/go-to-def
  - Cross-vendor enforcement warning (VEND010) when library key mismatches target vendor
affects: [14-mock-framework, 16-shipped-stubs]

# Tech tracking
tech-stack:
  added: []
  patterns: [variadic AnalyzeOpts for backward-compatible Analyze signature, cross-vendor heuristic matching on library path keys]

key-files:
  created:
    - pkg/vendor/integration_test.go
  modified:
    - pkg/analyzer/analyzer.go
    - pkg/analyzer/analyzer_test.go
    - pkg/checker/diag_codes.go
    - pkg/lsp/document.go
    - pkg/lsp/server.go
    - cmd/stc/check.go

key-decisions:
  - "Variadic AnalyzeOpts pattern preserves backward compatibility for all existing Analyze callers"
  - "Cross-vendor detection uses simple string-contains heuristic on library path keys against known vendor names"
  - "VEND010 diagnostic code for cross-vendor library warnings"

patterns-established:
  - "AnalyzeOpts: optional configuration for Analyze via variadic parameter, same pattern as ResolveOpts"
  - "Library loading in LSP Initialize handler: load once on workspace open, pass to all subsequent analyses"

requirements-completed: [VLIB-03, VLIB-04, VLIB-05]

# Metrics
duration: 5min
completed: 2026-03-30
---

# Phase 13 Plan 02: Pipeline Integration Summary

**Library-aware Analyze facade with CLI check, LSP workspace init, and cross-vendor enforcement wiring vendor stubs through the full analysis pipeline**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-30T11:40:21Z
- **Completed:** 2026-03-30T11:45:34Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- Analyze() accepts AnalyzeOpts with LibraryFiles (variadic, backward compatible with all 6+ existing callers)
- CLI `stc check` loads vendor stubs from stc.toml library_paths before analysis, passes via AnalyzeOpts
- LSP loads vendor stubs on workspace initialization (Initialize handler), stores on DocumentStore for all subsequent analyses
- Library FB symbols flow through existing LSP features: go-to-definition, hover, completion all work automatically
- Cross-vendor enforcement warns (VEND010) when library path key contains vendor name mismatching project target
- End-to-end integration tests verify full pipeline: config -> load libraries -> parse -> analyze

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for library-aware Analyze** - `c68be7c` (test)
2. **Task 1 GREEN: Wire library loading into Analyzer and CLI check** - `b3a276f` (feat)
3. **Task 2: Wire library loading into LSP** - `a1f3efd` (feat)
4. **Task 3: Integration tests for end-to-end stub loading** - `69416b5` (test)

## Files Created/Modified
- `pkg/analyzer/analyzer.go` - Added AnalyzeOpts struct, variadic opts parameter, cross-vendor enforcement logic
- `pkg/analyzer/analyzer_test.go` - 3 new tests: library FB resolution, wrong param error, cross-vendor warning
- `pkg/checker/diag_codes.go` - Added CodeCrossVendorLib (VEND010) diagnostic code
- `cmd/stc/check.go` - Wire vendor.LoadLibraries before analysis, pass AnalyzeOpts
- `pkg/lsp/document.go` - Added libraryFiles/libCfg fields, SetLibraryFiles method, pass to Analyze
- `pkg/lsp/server.go` - Load vendor libraries from stc.toml on Initialize, call store.SetLibraryFiles
- `pkg/vendor/integration_test.go` - 3 integration tests: valid stub loading, wrong param, cross-vendor warning

## Decisions Made
- Variadic AnalyzeOpts pattern matches the ResolveOpts pattern from Plan 01 for consistency
- Cross-vendor detection uses string-contains heuristic on library path keys -- pragmatic for v1.1
- VEND010 code chosen to leave gap after VEND006 for future vendor-specific diagnostics

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None - all data flows are fully wired.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Full pipeline integration complete: LoadLibraries -> Analyze -> LSP/CLI all wired
- Ready for Phase 14 (mock framework) and Phase 16 (shipped stubs)
- All 25 test suites pass with zero regressions

## Self-Check: PASSED

All 7 files verified present. All 4 commits verified in git log.

---
*Phase: 13-vendor-stub-loading*
*Completed: 2026-03-30*
