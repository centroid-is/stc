# Phase 2: Preprocessor - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

Users can write vendor-portable ST using conditional compilation directives and get vendor-specific output with accurate source mapping. Implements {IF defined()}, {ELSIF}, {ELSE}, {END_IF}, {DEFINE}, {ERROR} directives with source maps from preprocessed output back to original file:line:col.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — infrastructure phase. Use ROADMAP phase goal, success criteria, and codebase conventions established in Phase 1 to guide decisions.

Key constraints from Phase 1:
- Preprocessor runs before parser: `stc parse` should run preprocessor first
- Source maps must preserve original file:line:col for downstream diagnostics
- CLI command: `stc pp <file> --define VENDOR_BECKHOFF`
- Use existing pkg/source and pkg/diag packages for positions and diagnostics
- Follow established Go patterns from Phase 1 (table-driven tests, etc.)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/source/source.go` — Pos, Span, SourceFile types
- `pkg/diag/diagnostic.go` — Diagnostic type with file:line:col formatting
- `pkg/lexer/` — Token types and keyword table (preprocessor may need to recognize directives)

### Established Patterns
- Table-driven tests with testdata/ directories
- `--format json` on all CLI commands
- Hand-written processing (no external parser generators)

### Integration Points
- Preprocessor output feeds into the lexer/parser pipeline
- `stc parse` should integrate preprocessor automatically when defines are present

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase. Refer to ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
