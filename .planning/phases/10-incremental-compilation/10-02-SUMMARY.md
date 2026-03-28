---
phase: 10-incremental-compilation
plan: 02
subsystem: incremental
tags: [incremental-analysis, content-hashing, cross-file-analysis, cli, lsp]
dependency_graph:
  requires:
    - phase: 10-01
      provides: DepGraph, FileCache, PurgeFile
  provides:
    - IncrementalAnalyzer facade for incremental parsing
    - CLI stc check with incremental file skipping
    - LSP cross-file analysis across open documents
  affects: [cli, lsp, analyzer]
tech_stack:
  added: []
  patterns: [incremental parse with caller-driven semantic analysis, cross-file LSP analysis]
key_files:
  created:
    - pkg/incremental/analyzer.go
    - pkg/incremental/analyzer_test.go
  modified:
    - cmd/stc/check.go
    - pkg/lsp/document.go
decisions:
  - "IncrementalAnalyzer.Parse returns files+stats; caller runs semantic analysis (avoids import cycle between incremental and analyzer packages)"
  - "CLI reports re-parsed count in stderr for user visibility of incremental benefit"
  - "LSP analyzes all open documents together for cross-file symbol resolution"
patterns_established:
  - "Incremental parse layer separate from semantic analysis to avoid circular imports"
  - "Cross-file LSP analysis with shared AnalysisResult reference on all open documents"
requirements_completed: [INCR-01, INCR-02]
metrics:
  duration: 287s
  completed: "2026-03-28T21:13:10Z"
---

# Phase 10 Plan 02: Incremental Analysis Integration Summary

IncrementalAnalyzer facade with content-hash parse skipping wired into CLI stc check and LSP cross-file analysis across open documents.

## Performance

- **Duration:** 287s (~5 min)
- **Started:** 2026-03-28T21:08:23Z
- **Completed:** 2026-03-28T21:13:10Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- IncrementalAnalyzer correctly skips parsing for unchanged files via SHA-256 content hashing
- CLI `stc check` reports re-parsed file count (e.g., "0/3 files re-parsed") on second run with no changes
- LSP performs cross-file analysis across all open documents for multi-file symbol resolution
- 5 integration tests verify first run, no-change run, single-file change, diagnostics equivalence, and file deletion

## Task Commits

Each task was committed atomically:

1. **Task 1: Incremental analyzer facade (RED)** - `c01011c` (test)
2. **Task 1: Incremental analyzer facade (GREEN)** - `c2a4b46` (feat)
3. **Task 2: Wire incremental into CLI and LSP** - `1fe7b93` (feat)

## Files Created/Modified
- `pkg/incremental/analyzer.go` - IncrementalAnalyzer facade with Parse method returning files, diags, and stats
- `pkg/incremental/analyzer_test.go` - 5 integration tests for incremental behavior
- `cmd/stc/check.go` - Uses IncrementalAnalyzer for incremental parse, reports re-parsed count
- `pkg/lsp/document.go` - Cross-file analysis across all open documents with shared AnalysisResult

## Decisions Made
- **Parse/Analyze split:** IncrementalAnalyzer.Parse returns `IncrResult{Files, Diags, Stats}` and the caller runs `analyzer.Analyze` separately. This avoids an import cycle (incremental -> analyzer -> incremental) that would occur if IncrementalAnalyzer called analyzer.Analyze directly.
- **AnalyzeFilesIncremental in check.go not analyzer.go:** The plan called for a wrapper in `pkg/analyzer/analyzer.go`, but the import cycle prevents this. The incremental logic is wired directly in `cmd/stc/check.go` instead.
- **LSP cross-file analysis:** All open documents analyzed together with shared AnalysisResult reference for cross-file go-to-definition and hover.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Import cycle between incremental and analyzer packages**
- **Found during:** Task 2 (wiring AnalyzeFilesIncremental)
- **Issue:** Plan specified adding `AnalyzeFilesIncremental` to `pkg/analyzer/analyzer.go`, but `pkg/incremental/analyzer.go` already imports `pkg/analyzer` for `analyzer.Analyze()`. Adding the reverse import creates a cycle.
- **Fix:** Restructured IncrementalAnalyzer to return parsed files+stats via `Parse()` method (no analyzer dependency). Caller (check.go) runs semantic analysis. Removed `AnalyzeFilesIncremental` from `pkg/analyzer/analyzer.go`.
- **Files modified:** pkg/incremental/analyzer.go, cmd/stc/check.go
- **Verification:** `go build ./cmd/stc/` succeeds, all tests pass
- **Committed in:** 1fe7b93 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Architecture change necessary to avoid Go import cycle. Same functionality delivered, just different package boundary.

## Issues Encountered
None beyond the import cycle addressed above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Incremental compilation fully wired into CLI and LSP
- Second run on unchanged files skips all parsing
- Cross-file LSP analysis enables go-to-definition across open documents
- Ready for future optimization: semantic analysis could also be incremental using the dependency graph's dirty set

## Known Stubs
None - all functionality is fully wired and tested.

---
*Phase: 10-incremental-compilation*
*Completed: 2026-03-28*
