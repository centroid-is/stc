# Phase 13: Vendor Stub Loading - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

Users can declare vendor FBs in .st stub files (declarations without bodies), configure library paths in stc.toml, and get type-checking + LSP support for vendor FB usage. Single-vendor enforcement warns on cross-vendor stubs.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
Key constraints from research:
- Library stubs are plain .st files with FB/FUNCTION declarations, no body
- Load via [build.library_paths] in stc.toml (already in config schema)
- Resolver loads library files BEFORE user code in Pass 1
- Add IsLibrary flag on symbols to allow mock override later (Phase 14)
- LSP loads library files on workspace init for completion/hover
- Single-vendor: warn if stubs from vendor X loaded but project targets vendor Y
- Existing pkg/project/config.go has LibraryPaths map

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- pkg/project/config.go — LibraryPaths in stc.toml config
- pkg/checker/resolve.go — CollectDeclarations two-pass resolver
- pkg/symbols/table.go — Symbol table with scope chain
- pkg/analyzer/analyzer.go — Analyzer facade
- pkg/lsp/document.go — Document store for LSP

### Integration Points
- Resolver needs to accept library source files before user code
- Analyzer needs to load library files from config
- LSP needs to load libraries on workspace init
- Checker vendor profile needs to validate against loaded library vendor

</code_context>

<specifics>
## Specific Ideas

None beyond requirements.

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
