# Phase 7: Multi-Vendor Emission - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

Users write ST once and emit vendor-specific output for Beckhoff or Schneider targets. A `portable` target produces clean normalized ST. Round-trip stability: parse → emit → parse → emit produces identical output.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices at Claude's discretion — infrastructure phase. Key constraints:
- No PLCopen XML — vendor interop through preprocessor ifdefs and ST re-emission only
- Beckhoff and Schneider targets first (Allen Bradley deferred to v2)
- `portable` target produces intersection of both vendor dialects
- Emitter handles pragma/attribute differences between vendors
- Round-trip stability is a hard requirement (EMIT-04)
- CST-first AST already preserves all tokens — emitter can reconstruct source faithfully

Use existing pkg/ast/, pkg/parser/, pkg/preprocess/, pkg/checker/vendor.go patterns.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/ast/` — Full CST with trivia preservation (whitespace, comments)
- `pkg/parser/` — Parser produces faithful CST
- `pkg/preprocess/` — Preprocessor for conditional compilation
- `pkg/checker/vendor.go` — Vendor profiles (Beckhoff/Schneider/Portable)

### Integration Points
- `stc emit <file> --target beckhoff|schneider|portable` CLI command
- Emitter consumes typed AST, produces vendor-flavored ST text
- Preprocessor may run before emission for ifdef resolution

</code_context>

<specifics>
## Specific Ideas

None beyond requirements.

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
