---
phase: 03-semantic-analysis
plan: 05
subsystem: compiler
tags: [analyzer-facade, cli-check, cross-file-resolution, vendor-diagnostics, iec-61131-3]

# Dependency graph
requires:
  - phase: 03-03
    provides: "Two-pass type checker (Resolver, Checker, CollectDeclarations, CheckBodies)"
  - phase: 03-04
    provides: "Vendor compatibility checker (CheckVendorCompat, LookupVendor) and usage analysis (CheckUsage)"
provides:
  - "Analyzer facade: Analyze() orchestrates parse -> resolve -> check -> usage -> vendor"
  - "AnalyzeFiles() convenience for disk-based analysis with source root discovery"
  - "stc check CLI command with text and JSON output formats"
  - "Cross-file symbol resolution via two-pass analysis (SEMA-05)"
affects: [04-standard-library, 05-interpreter, 09-lsp]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Analyzer facade pattern: single Analyze() entry point sequencing all checker passes"
    - "CLI check command follows same pattern as parse command (text/JSON, exit codes)"
    - "Diagnostics array JSON output for machine consumption"

key-files:
  created:
    - pkg/analyzer/analyzer.go
    - pkg/analyzer/analyzer_test.go
    - pkg/analyzer/testdata/multi_file_a.st
    - pkg/analyzer/testdata/multi_file_b.st
    - pkg/analyzer/testdata/vendor_test.st
    - cmd/stc/check.go
    - cmd/stc/check_test.go
  modified:
    - cmd/stc/stubs.go
    - cmd/stc/main_test.go

key-decisions:
  - "Analyzer sequences passes in fixed order: resolve -> check -> usage -> vendor (vendor only if config present)"
  - "AnalyzeFiles combines parse diagnostics and semantic diagnostics into single list"
  - "CLI check exits 1 on errors only; warnings alone produce exit 0"
  - "Text output uses Diagnostic.String() format (file:line:col: severity: message) to stderr"

patterns-established:
  - "pkg/analyzer as public facade wrapping pkg/checker internals"
  - "CLI commands use analyzer.AnalyzeFiles for end-to-end file processing"
  - "--vendor flag overrides stc.toml config's VendorTarget"

requirements-completed: [SEMA-05, SEMA-06]

# Metrics
duration: 4min
completed: 2026-03-27
---

# Phase 03 Plan 05: Analyzer Facade and CLI Check Command Summary

**Analyzer facade orchestrating all checker passes with stc check CLI command supporting text/JSON output and vendor-aware diagnostics**

## Performance

- **Duration:** 4 min (236s)
- **Started:** 2026-03-27T14:41:04Z
- **Completed:** 2026-03-27T14:45:00Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Analyzer facade correctly sequences all semantic analysis passes (resolve, check, usage, vendor) into a single Analyze() call
- Cross-file symbol resolution works: FB_Motor declared in file A, used as variable type in file B, no undeclared errors (SEMA-05)
- stc check command replaces stub with full implementation: text diagnostics to stderr, JSON array to stdout, --vendor flag, correct exit codes (SEMA-06)
- Full test suite (all 12 packages) passes with 12 new tests across analyzer and CLI

## Task Commits

Each task was committed atomically:

1. **Task 1: Analyzer facade orchestrating all checker passes** - `d95c2c2` (feat)
2. **Task 2: CLI check command replacing stub** - `0e3a129` (feat)

## Files Created/Modified
- `pkg/analyzer/analyzer.go` - Public Analyze() and AnalyzeFiles() facade with source root discovery
- `pkg/analyzer/analyzer_test.go` - 7 tests: single-file, cross-file, type mismatch, vendor, nil config, parse errors, nonexistent
- `pkg/analyzer/testdata/multi_file_a.st` - FB_Motor declaration for cross-file testing
- `pkg/analyzer/testdata/multi_file_b.st` - Program using FB_Motor from file A
- `pkg/analyzer/testdata/vendor_test.st` - FB with METHOD for vendor OOP checking
- `cmd/stc/check.go` - Full stc check implementation with --vendor and --format flags
- `cmd/stc/check_test.go` - 5 integration tests: valid, type error, JSON, vendor, no-files
- `cmd/stc/stubs.go` - Removed newCheckCmd stub (replaced by check.go)
- `cmd/stc/main_test.go` - Updated stub tests to exclude check (no longer a stub)

## Decisions Made
- Analyzer sequences passes in fixed order (resolve -> check -> usage -> vendor) matching RESEARCH.md specification
- AnalyzeFiles combines parse and semantic diagnostics into a single flat list for simplicity
- CLI check uses os.Exit(1) for error exit (same pattern as parse command) with SilenceErrors/SilenceUsage
- Cross-file test uses variable type resolution (motor : FB_Motor) rather than FB call syntax due to known parser limitation with CallStmt

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all analyzer and check command functionality is fully implemented per plan scope.

## Next Phase Readiness
- Analyzer facade ready for standard library expansion (Phase 4) - just add builtins to types.BuiltinFunctions
- stc check command ready for user-facing workflows and CI integration
- LSP (Phase 9) can call analyzer.Analyze() directly for diagnostics

## Self-Check: PASSED

All 9 files verified present. Both commit hashes verified in git log.

---
*Phase: 03-semantic-analysis*
*Completed: 2026-03-27*
