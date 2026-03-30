---
phase: 13-vendor-stub-loading
verified: 2026-03-30T00:00:00Z
status: passed
score: 9/9 must-haves verified
re_verification: false
---

# Phase 13: Vendor Stub Loading Verification Report

**Phase Goal:** Type-check and navigate code referencing vendor FBs via .st stub files
**Verified:** 2026-03-30
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Parser can parse .st stub files containing FUNCTION_BLOCK declarations without bodies | VERIFIED | `LoadLibraries` calls `pipeline.Parse` on stub files; `TestLoadLibraries_StubFBWithNoBody` passes |
| 2 | Vendor loader reads library_paths from config and returns parsed ASTs | VERIFIED | `LoadLibraries` iterates `cfg.Build.LibraryPaths`, globs `*.st`, returns `[]*ast.SourceFile` |
| 3 | Resolver registers library symbols with IsLibrary=true before user code | VERIFIED | `CollectDeclarations` calls `collectFileDeclarations(libFile, true)` before user files; `sym.IsLibrary = isLibrary` set in all resolve* methods |
| 4 | User code referencing vendor FB types resolves without errors | VERIFIED | `TestIntegrationStubLoadingValid` passes: `MC_MoveAbsolute` resolves with 0 errors |
| 5 | Parameter validation catches wrong input/output names against stubs | VERIFIED | `TestIntegrationStubLoadingWrongParam` passes: typo `Execut` produces errors |
| 6 | stc check resolves vendor FB types from stubs configured in stc.toml library_paths | VERIFIED | `cmd/stc/check.go` calls `vendor.LoadLibraries` before `analyzer.Analyze`; passes via `AnalyzeOpts` |
| 7 | stc check validates input/output parameter usage against stub signatures | VERIFIED | Flows through `AnalyzeOpts.LibraryFiles` -> `ResolveOpts.LibraryFiles` -> `FunctionBlockType.Inputs/Outputs` |
| 8 | LSP provides completion, hover, and go-to-definition for vendor FB members | VERIFIED | `DocumentStore.analyzeDocument` passes `AnalyzeOpts{LibraryFiles: s.libraryFiles}`; library symbols enter symbol table used by all LSP handlers |
| 9 | When project targets vendor X but loads stubs from vendor Y directory, a warning is emitted | VERIFIED | `checkCrossVendorLibraries` emits `VEND010`; `TestIntegrationCrossVendorWarning` and `TestAnalyze_CrossVendorWarning` pass |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/vendor/loader.go` | Library stub file loading from config paths; exports `LoadLibraries` | VERIFIED | 64 lines, substantive; iterates LibraryPaths, stats dirs, globs .st, calls pipeline.Parse |
| `pkg/vendor/loader_test.go` | Tests for stub loading | VERIFIED | 7 tests covering empty config, single/multiple stubs, nonexistent paths, no-body FBs, IsLibrary default |
| `pkg/symbols/symbol.go` | IsLibrary flag on Symbol struct | VERIFIED | `IsLibrary bool` field present at line 59 |
| `pkg/checker/resolve.go` | Library-aware declaration collection; contains `LibraryFiles` | VERIFIED | `ResolveOpts.LibraryFiles`, `collectFileDeclarations(file, isLibrary bool)`, IsLibrary set in all 5 resolve* methods |
| `pkg/analyzer/analyzer.go` | Library-aware Analyze function; contains `LibraryFiles` | VERIFIED | `AnalyzeOpts.LibraryFiles`, passed to `checker.ResolveOpts`, cross-vendor enforcement via `checkCrossVendorLibraries` |
| `pkg/lsp/document.go` | Library file loading on workspace init; contains `libraryFiles` | VERIFIED | `libraryFiles []*ast.SourceFile` field; `SetLibraryFiles` method; `analyzeDocument` passes `AnalyzeOpts{LibraryFiles: s.libraryFiles}` |
| `cmd/stc/check.go` | Check command with library loading; contains `vendor.LoadLibraries` | VERIFIED | `vendor.LoadLibraries` called at line 73; `AnalyzeOpts{LibraryFiles: libFiles}` passed at line 94 |
| `pkg/checker/diag_codes.go` | VEND010 diagnostic code | VERIFIED | `CodeCrossVendorLib = "VEND010"` at line 38 |
| `pkg/vendor/integration_test.go` | End-to-end integration tests | VERIFIED | 3 integration tests: valid stub loading, wrong param, cross-vendor warning |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/vendor/loader.go` | `pkg/project/config.go` | reads `Config.Build.LibraryPaths` | WIRED | `cfg.Build.LibraryPaths` iterated at line 28 |
| `pkg/checker/resolve.go` | `pkg/symbols/symbol.go` | sets `IsLibrary=true` on library symbols | WIRED | `sym.IsLibrary = isLibrary` in `resolveProgram`, `resolveFunctionBlock`, `resolveFunction`, `resolveTypeDecl`, `resolveInterface` |
| `pkg/analyzer/analyzer.go` | `pkg/checker/resolve.go` | passes `LibraryFiles` in `ResolveOpts` | WIRED | `resolveOpts.LibraryFiles = opts[0].LibraryFiles`; `resolver.CollectDeclarations(files, resolveOpts)` |
| `pkg/lsp/document.go` | `pkg/vendor/loader.go` | loads library files for cross-file analysis | WIRED | `analyzeDocument` passes `analyzer.AnalyzeOpts{LibraryFiles: s.libraryFiles}` — library files set via `SetLibraryFiles` called from server.go |
| `cmd/stc/check.go` | `pkg/vendor/loader.go` | loads libraries before analysis | WIRED | `vendor.LoadLibraries(cfg, projectDir)` at line 73 |
| `pkg/lsp/server.go` | `pkg/vendor/loader.go` | Initialize handler loads stubs | WIRED | `vendor.LoadLibraries(cfg, projectDir)` at line 119 in the wrapped Initialize handler |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `pkg/lsp/document.go` | `s.libraryFiles` | Set by `store.SetLibraryFiles(libFiles, cfg)` in `server.go:120` | Yes — populated from `vendor.LoadLibraries` which reads real .st files | FLOWING |
| `cmd/stc/check.go` | `libFiles` | `vendor.LoadLibraries(cfg, projectDir)` at line 73 | Yes — reads actual .st stub files from disk | FLOWING |
| `pkg/analyzer/analyzer.go` | `resolveOpts.LibraryFiles` | Passed in from caller via `opts[0].LibraryFiles` | Yes — flows from LoadLibraries through AnalyzeOpts | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `pkg/vendor` tests pass | `go test ./pkg/vendor/... -count=1` | ok (7 unit + 3 integration tests) | PASS |
| `pkg/checker` tests pass | `go test ./pkg/checker/... -count=1` | ok | PASS |
| `pkg/analyzer` tests pass | `go test ./pkg/analyzer/... -count=1` | ok | PASS |
| `pkg/lsp` tests pass | `go test ./pkg/lsp/... -count=1` | ok | PASS |
| `cmd/stc` tests pass | `go test ./cmd/stc/... -count=1` | ok | PASS |
| Full regression — all 25 packages | `go test ./... -count=1` | ok — all 25 packages | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| VLIB-01 | 13-01 | User can declare vendor FBs in .st stub files without bodies and reference them from production code | SATISFIED | `LoadLibraries` + `TestLoadLibraries_StubFBWithNoBody` + `TestIntegrationStubLoadingValid` |
| VLIB-02 | 13-01 | User configures library paths via `[build.library_paths]` in stc.toml | SATISFIED | `LoadLibraries` reads `cfg.Build.LibraryPaths` from TOML config; integration test uses full stc.toml |
| VLIB-03 | 13-01, 13-02 | `stc check` resolves vendor FB types from stubs and validates input/output parameter usage | SATISFIED | `check.go` calls `LoadLibraries` + `AnalyzeOpts`; wrong-param test produces errors |
| VLIB-04 | 13-02 | LSP provides completion, hover, and go-to-definition for vendor FB inputs and outputs | SATISFIED | Library symbols loaded into symbol table on Initialize; all LSP handlers query the same table |
| VLIB-05 | 13-02 | Single-vendor enforcement — stubs from other vendors produce warnings | SATISFIED | `VEND010` code emitted by `checkCrossVendorLibraries`; `TestIntegrationCrossVendorWarning` passes |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `pkg/checker/resolve.go` | 359 | Comment: "Forward reference -- create a placeholder FunctionBlockType" | Info | This is a legitimate comment describing a forward-reference resolution strategy, not a stub. The code creates a real `FunctionBlockType` for later resolution — not an empty placeholder that blocks the goal. |

No blockers or warnings found. The single "placeholder" match in resolve.go is a code comment about forward-reference semantics, not a stub implementation.

### Human Verification Required

The following aspects were verified by code wiring analysis but cannot be verified by automated grep:

**1. LSP Completion/Hover/Go-To-Definition for Library FBs**

**Test:** Open a project with `stc.toml` containing `library_paths`. Open a `.st` file that instantiates a library FB (e.g., `MC_MoveAbsolute`). Trigger completion on the FB instance variable, hover over it, and press go-to-definition.

**Expected:** Completion shows FB members (Axis, Position, Execute, Done, Busy, Error). Hover shows the FB type signature. Go-to-definition navigates to the declaration in the stub `.st` file.

**Why human:** LSP handler wiring is verified by code analysis — library symbols enter the same symbol table queried by completion/hover/definition handlers. Runtime LSP protocol behavior requires a running VS Code session to confirm the end-user experience.

### Gaps Summary

No gaps found. All 9 observable truths verified, all 9 required artifacts exist and are substantive and wired, all 5 requirements satisfied, full test suite passes with zero regressions.

---

_Verified: 2026-03-30_
_Verifier: Claude (gsd-verifier)_
