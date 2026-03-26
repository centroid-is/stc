---
phase: 02-preprocessor
plan: 02
subsystem: cli
tags: [cobra, preprocessor, cli, json, source-map]

# Dependency graph
requires:
  - phase: 02-preprocessor-01
    provides: "preprocess.Preprocess API, SourceMap, directive parser"
provides:
  - "stc pp subcommand with --define/-D flag and text/JSON output"
  - "8 integration tests for stc pp command"
  - "3 ST test fixtures for vendor-portable preprocessing"
affects: [03-type-checker, 09-lsp]

# Tech tracking
tech-stack:
  added: []
  patterns: ["CLI subcommand with --define StringSlice flag", "ppOutput JSON struct with source_map"]

key-files:
  created:
    - cmd/stc/pp.go
    - testdata/preprocess/vendor_portable.st
    - testdata/preprocess/define_local.st
    - testdata/preprocess/error_directive.st
  modified:
    - cmd/stc/stubs.go
    - cmd/stc/main_test.go
    - pkg/preprocess/sourcemap.go

key-decisions:
  - "StringSlice for --define flag supports multiple defines per invocation"
  - "JSON output includes source_map array and diagnostics for tool integration"
  - "Text mode prints preprocessed source to stdout, diagnostics to stderr"

patterns-established:
  - "pp command pattern: read file, call library, format output (mirrors parse.go)"
  - "Empty diagnostics array in JSON (never null) for consistent parsing"

requirements-completed: [PREP-05]

# Metrics
duration: 2min
completed: 2026-03-26
---

# Phase 02 Plan 02: CLI Integration Summary

**stc pp subcommand with --define flags, text/JSON output, source maps, and 8 integration tests**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-26T17:20:21Z
- **Completed:** 2026-03-26T17:22:53Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Working `stc pp` command replacing the stub, with --define/-D flag for vendor symbol definition
- Text and JSON output modes; JSON includes source_map, diagnostics, and has_errors
- 8 integration tests covering vendor selection, local defines, error directives, JSON output, and file-not-found
- Full project test suite passes (9 packages)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement stc pp command with --define flag and test fixtures** - `a321dee` (feat)
2. **Task 2: Integration tests for stc pp command** - `0c04df4` (test)

## Files Created/Modified
- `cmd/stc/pp.go` - New pp subcommand with newPpCmd and runPp functions
- `cmd/stc/stubs.go` - Removed newPpCmd stub (now real implementation)
- `cmd/stc/main_test.go` - 8 new integration tests, removed pp from stub test list
- `pkg/preprocess/sourcemap.go` - Added Mappings() accessor for JSON serialization
- `testdata/preprocess/vendor_portable.st` - Multi-vendor IF/ELSIF/ELSE fixture
- `testdata/preprocess/define_local.st` - Local DEFINE directive fixture
- `testdata/preprocess/error_directive.st` - ERROR directive fixture

## Decisions Made
- StringSlice for --define flag: supports `--define X --define Y` syntax naturally via cobra
- JSON output always returns `[]` for empty diagnostics (not null) for consistent downstream parsing
- Text mode prints preprocessed source to stdout and diagnostics to stderr, matching parse.go convention

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added Mappings() accessor to SourceMap**
- **Found during:** Task 1 (pp command implementation)
- **Issue:** SourceMap.mappings field is unexported; JSON output needs mapping data
- **Fix:** Added `Mappings() []Mapping` method returning a copy of internal mappings
- **Files modified:** pkg/preprocess/sourcemap.go
- **Verification:** JSON output includes source_map entries; go test passes
- **Committed in:** a321dee (Task 1 commit)

**2. [Rule 1 - Bug] Removed pp from stub command test list**
- **Found during:** Task 1 (stub replacement)
- **Issue:** TestCLI_StubCommands included "pp" which is no longer a stub
- **Fix:** Removed "pp" from stub test list in main_test.go
- **Files modified:** cmd/stc/main_test.go
- **Verification:** Full test suite passes
- **Committed in:** a321dee (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functionality is fully wired.

## Next Phase Readiness
- Preprocessor phase (02) is complete: library and CLI both functional
- Ready for Phase 03 (type checker) which can use preprocessed output
- Source map infrastructure ready for diagnostic remapping in downstream tools

---
*Phase: 02-preprocessor*
*Completed: 2026-03-26*
