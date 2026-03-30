# Phase 14: Mock Framework - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

Users can write ST mock FBs with full bodies that override vendor stubs by name during testing. FBs without mocks auto-generate zero-value instances. Mock signatures validated against stubs. Tests can inject I/O values via mock I/O table.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
Key constraints from research:
- Mock paths from [test.mock_paths] in stc.toml
- Mock loading priority: user mock > stdlib FB > auto-stub
- Mock declarations override library stubs (IsLibrary flag allows redeclaration)
- Auto-stubs: FBs with empty body accept inputs, return zeros (already works via NewUserFBInstance)
- Zero-value auto-stubs emit fidelity warnings in test output
- Mock signatures validated: parameter count and types must match stub
- Tests inject I/O via IOTable.Set before execution

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- pkg/vendor/loader.go — LoadLibraries loads .st files
- pkg/checker/resolve.go — Library-aware resolver with IsLibrary flag
- pkg/symbols/symbol.go — Symbol.IsLibrary for override control
- pkg/interp/scan.go — ScanCycleEngine with IOTable
- pkg/interp/fb_instance.go — NewUserFBInstance (empty body = auto-stub)
- pkg/testing/runner.go — Test runner
- pkg/project/config.go — Config with LibraryPaths

### Integration Points
- Test runner needs to load mock files from mock_paths
- Resolver needs to allow mock override of library symbols
- Test output needs fidelity warnings for auto-stubs
- IOTable needs test injection API (IO-04)

</code_context>

<specifics>
## Specific Ideas

None beyond requirements.

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
