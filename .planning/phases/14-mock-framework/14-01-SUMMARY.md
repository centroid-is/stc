---
phase: 14-mock-framework
plan: 01
subsystem: testing
tags: [mock, function-block, vendor-stubs, tdd, symbol-table]

# Dependency graph
requires:
  - phase: 13-vendor-stub-loading
    provides: "LoadLibraries, ResolveOpts.LibraryFiles, IsLibrary symbol flag"
provides:
  - "TestConfig with MockPaths field for [test.mock_paths] config"
  - "LoadMocks function parsing .st mock files from configured paths"
  - "ResolveOpts.MockFiles for mock-aware symbol resolution"
  - "ValidateMockSignatures checking mock/stub parameter compatibility"
affects: [14-mock-framework, 15-shipped-stubs, testing]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Mock override of library symbols via CollectDeclarations ordering", "Signature validation comparing FunctionBlockType parameters"]

key-files:
  created:
    - pkg/vendor/mock.go
    - pkg/vendor/mock_test.go
    - pkg/project/testdata/stc_with_test.toml
  modified:
    - pkg/project/config.go
    - pkg/project/config_test.go
    - pkg/checker/resolve.go
    - pkg/checker/resolve_test.go

key-decisions:
  - "Mock symbols registered with isLibrary=false so they have bodies and override library stubs"
  - "ValidateMockSignatures uses Type.String() equality for parameter type comparison"
  - "FBs not found in the symbol table are silently skipped during signature validation"

patterns-established:
  - "Mock files processed after user files in CollectDeclarations, leveraging existing IsLibrary override logic"
  - "extractFBParams builds FunctionBlockType from AST var blocks for comparison outside the resolver"

requirements-completed: [MOCK-01, MOCK-02, MOCK-03, MOCK-04]

# Metrics
duration: 4min
completed: 2026-03-30
---

# Phase 14 Plan 01: Mock Framework Infrastructure Summary

**Mock FB loader with config-driven mock_paths, resolver integration overriding library stubs, and signature validation against vendor stub parameters**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-30T11:54:31Z
- **Completed:** 2026-03-30T11:58:43Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- TestConfig struct with MockPaths field parses from stc.toml [test.mock_paths]
- LoadMocks function reads and parses .st mock files from configured directories
- MockFiles in ResolveOpts override library symbols without redeclaration errors
- ValidateMockSignatures catches parameter count/type mismatches between mocks and stubs
- Full test suite (26 packages) passes with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add TestConfig.MockPaths to config and create LoadMocks function** - `d22dcbe` (test+feat)
2. **Task 2: Add MockFiles to ResolveOpts and implement ValidateMockSignatures** - `10f649a` (feat)

## Files Created/Modified
- `pkg/project/config.go` - Added TestConfig struct with MockPaths and Test field on Config
- `pkg/vendor/mock.go` - LoadMocks function and ValidateMockSignatures with helper functions
- `pkg/vendor/mock_test.go` - Tests for LoadMocks and ValidateMockSignatures (8 tests)
- `pkg/project/config_test.go` - Tests for [test] section TOML parsing (2 tests)
- `pkg/project/testdata/stc_with_test.toml` - Test fixture with [test.mock_paths]
- `pkg/checker/resolve.go` - MockFiles field on ResolveOpts, mock registration in CollectDeclarations
- `pkg/checker/resolve_test.go` - Tests for mock override and user-code protection (2 tests)

## Decisions Made
- Mock symbols registered with isLibrary=false so they are treated as real implementations with bodies
- ValidateMockSignatures uses Type.String() equality for parameter type comparison (same approach as checker)
- FBs in mock files not found in the symbol table are silently skipped (they may be test-only FBs)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Mock framework infrastructure complete, ready for auto-stub support and integration testing
- LoadMocks and ValidateMockSignatures available for test runner integration
- ResolveOpts.MockFiles ready for use by analyzer and test commands

---
*Phase: 14-mock-framework*
*Completed: 2026-03-30*
