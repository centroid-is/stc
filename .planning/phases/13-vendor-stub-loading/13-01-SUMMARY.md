---
phase: 13-vendor-stub-loading
plan: 01
subsystem: compiler
tags: [vendor, stubs, library, symbols, resolver]

# Dependency graph
requires:
  - phase: none
    provides: existing parser, symbol table, and resolver infrastructure
provides:
  - pkg/vendor/loader.go with LoadLibraries function for stub loading
  - Symbol.IsLibrary flag for vendor library symbol tracking
  - ResolveOpts with LibraryFiles for library-aware declaration collection
  - Scope.Delete and Table.RemovePOU for library symbol replacement
affects: [13-02, 14-mock-framework, 15-io-mapping, 16-shipped-stubs]

# Tech tracking
tech-stack:
  added: []
  patterns: [library-before-user registration ordering, variadic opts for backward compatibility, first-library-wins deduplication]

key-files:
  created:
    - pkg/vendor/loader.go
    - pkg/vendor/loader_test.go
  modified:
    - pkg/symbols/symbol.go
    - pkg/symbols/scope.go
    - pkg/symbols/table.go
    - pkg/checker/resolve.go
    - pkg/checker/resolve_test.go
    - pkg/checker/check_coverage_test.go

key-decisions:
  - "Variadic ResolveOpts pattern preserves backward compatibility for all existing CollectDeclarations callers"
  - "First-library-wins deduplication when multiple libraries declare same FB"
  - "User code silently replaces library symbols rather than erroring on redeclaration"

patterns-established:
  - "Library-before-user: library files registered first, user files second, enabling clean override semantics"
  - "IsLibrary flag: symbols from vendor stubs marked for later mock override support"

requirements-completed: [VLIB-01, VLIB-02, VLIB-03]

# Metrics
duration: 5min
completed: 2026-03-30
---

# Phase 13 Plan 01: Vendor Stub Loading Summary

**Vendor stub loader parsing .st files from configured library paths with library-aware resolver supporting IsLibrary symbol flag and user override semantics**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-30T11:33:22Z
- **Completed:** 2026-03-30T11:38:19Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- LoadLibraries function reads and parses .st stub files from configured library paths (relative/absolute, non-recursive glob)
- IsLibrary flag on Symbol struct enables downstream mock framework to identify vendor-provided symbols
- Resolver accepts LibraryFiles via variadic ResolveOpts, registers them before user code, marks with IsLibrary=true
- User code can override library symbols without redeclaration errors; duplicate library FBs silently ignored (first wins)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create pkg/vendor/loader.go and add Symbol.IsLibrary flag** - `ceb177f` (feat)
2. **Task 2: Modify Resolver to accept library files and mark library symbols** - `e83bacd` (feat)

## Files Created/Modified
- `pkg/vendor/loader.go` - LoadLibraries function: reads config library paths, globs .st files, parses with pipeline.Parse
- `pkg/vendor/loader_test.go` - 7 tests covering empty config, single/multiple stubs, nonexistent paths, no-body FBs
- `pkg/symbols/symbol.go` - Added IsLibrary bool field to Symbol struct
- `pkg/symbols/scope.go` - Added Scope.Delete method for library symbol replacement
- `pkg/symbols/table.go` - Added Table.RemovePOU method for clean library override
- `pkg/checker/resolve.go` - ResolveOpts struct, collectFileDeclarations helper, library-aware redeclaration logic
- `pkg/checker/resolve_test.go` - 5 new tests for library registration, IsLibrary flag, user override, parameter typing, dedup
- `pkg/checker/check_coverage_test.go` - Updated resolve* calls to include isLibrary parameter

## Decisions Made
- Variadic ResolveOpts pattern chosen over signature change to preserve backward compatibility
- First-library-wins semantics for duplicate library FB names (silent ignore, no error)
- User code replaces library symbols by deleting existing then re-registering (clean POU scope replacement)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated internal resolve* method callers in coverage tests**
- **Found during:** Task 2 (Resolver modification)
- **Issue:** check_coverage_test.go called resolveProgram, resolveFunctionBlock, etc. directly without the new isLibrary parameter
- **Fix:** Added `false` parameter to all 5 direct calls in check_coverage_test.go
- **Files modified:** pkg/checker/check_coverage_test.go
- **Verification:** All existing tests pass
- **Committed in:** e83bacd (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary for compilation. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Vendor stub loading infrastructure complete
- Ready for Plan 02: pipeline integration to wire LoadLibraries into the analysis pipeline
- ResolveOpts API ready for pipeline to pass library files to CollectDeclarations

---
*Phase: 13-vendor-stub-loading*
*Completed: 2026-03-30*
