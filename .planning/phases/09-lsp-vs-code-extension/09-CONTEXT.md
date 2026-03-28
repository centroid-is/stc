# Phase 9: LSP & VS Code Extension - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase — all success criteria are technical)

<domain>
## Phase Boundary

Users get a modern IDE experience for ST development in VS Code with real-time diagnostics, navigation, and refactoring. Delivers `stc-lsp` binary and VS Code extension.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices at Claude's discretion. Key constraints from research:
- GLSP (`tliron/glsp`) for LSP SDK (best available Go LSP library)
- LSP server as separate binary `stc-lsp` communicating via JSON-RPC over stdio
- VS Code extension in TypeScript — thin wrapper launching stc-lsp
- TextMate grammar for syntax highlighting (fork from existing open-source grammars)
- Diagnostics from parser + type checker piped to LSP publishDiagnostics
- Go-to-definition, hover, completion, references, rename from symbol table
- Preprocessor block graying via semantic tokens
- Full-file re-parse on edit initially (defer incremental to Phase 10)

Use existing pkg/parser/, pkg/analyzer/, pkg/symbols/, pkg/checker/, pkg/format/ infrastructure.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/parser/` — Error-recovering parser producing partial ASTs
- `pkg/analyzer/` — Full analysis pipeline (parse → resolve → check → usage → vendor)
- `pkg/symbols/` — Symbol table with scope chain, position tracking
- `pkg/checker/` — Type checker with diagnostics
- `pkg/format/` — Formatter for textDocument/formatting
- `pkg/diag/` — Diagnostic types with file:line:col

### Integration Points
- LSP server wraps analyzer facade
- textDocument/didOpen/didChange triggers re-analysis
- textDocument/completion, definition, hover, references read from symbol table
- textDocument/formatting delegates to pkg/format

</code_context>

<specifics>
## Specific Ideas

- No open-source ST tool has a real LSP — this is a major differentiator
- Research flagged GLSP as "early release" — may need workarounds

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
