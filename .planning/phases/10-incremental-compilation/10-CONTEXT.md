# Phase 10: Incremental Compilation - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

Fast re-analysis on large multi-file ST projects — only re-analyze changed files and their dependents. File-level dependency tracking with cached symbol tables.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All choices at Claude's discretion. Key constraints:
- File-level dependency graph (which POUs reference which)
- Symbol table caching between invocations
- Invalidation on file change (modified file + all dependents)
- Integration with analyzer facade and LSP server
- Use existing pkg/symbols/ and pkg/analyzer/ infrastructure

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/symbols/table.go` — Symbol table with file tracking
- `pkg/analyzer/analyzer.go` — AnalyzeFiles facade
- `pkg/checker/resolve.go` — Declaration resolver (builds dependency info)
- `pkg/lsp/document.go` — Document store (LSP integration point)

### Integration Points
- Analyzer needs to skip unchanged files
- LSP server should use incremental analysis for faster feedback
- CLI commands (check, lint) should benefit from caching

</code_context>

<specifics>
## Specific Ideas

None beyond requirements.

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
