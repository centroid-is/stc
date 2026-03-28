# Phase 8: Formatter & Linter - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

Users auto-format ST code with `stc fmt` (like gofmt) and lint it with `stc lint` against PLCopen coding guidelines and naming conventions. Both tools are configurable and produce JSON output for CI/agent integration.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices at Claude's discretion — infrastructure phase. Key constraints:
- Formatter uses CST-first AST (trivia preservation) for comment-correct formatting
- Configurable indentation style and keyword casing
- PLCopen coding guidelines as baseline lint rules
- Naming convention checks (configurable)
- JSON output on all CLI commands
- Round-trip: `fmt(fmt(code)) == fmt(code)` (idempotent)

Use existing pkg/ast/ (CST with trivia), pkg/emit/ (emitter pattern), cmd/stc/stubs.go.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/ast/` — CST with trivia preservation
- `pkg/ast/trivia.go` — Trivia types for whitespace/comments
- `pkg/emit/emit.go` — Emitter pattern (type-switch AST walk)
- `pkg/checker/` — Diagnostic pattern
- `pkg/diag/` — Diagnostic infrastructure

### Integration Points
- `stc fmt` and `stc lint` CLI commands replacing stubs
- Formatter may share code with emitter (both produce ST text from AST)
- Linter consumes typed AST from analyzer

</code_context>

<specifics>
## Specific Ideas

- Formatter can reuse/extend the emitter with formatting options
- LSP (Phase 9) will consume formatter for format-on-save

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
