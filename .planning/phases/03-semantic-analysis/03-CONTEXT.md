# Phase 3: Semantic Analysis - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Users get type errors, undeclared variable warnings, and vendor-aware diagnostics with actionable messages before ever touching a PLC. Implements full type checking, cross-file symbol resolution, and vendor profile awareness.

</domain>

<decisions>
## Implementation Decisions

### Type System Scope
- IEC-defined implicit widening only (INT→DINT→LINT, REAL→LREAL) — strict, catches real bugs
- Two-pass candidate/narrow approach for ANY type hierarchy overloaded standard functions (proven by MATIEC)
- Parse OOP fully but defer inheritance/interface type checking to v1.x — check method signatures and FB instance calls only
- Parse and represent POINTER TO/REFERENCE TO in symbol table, skip dereferencing validation in v1

### Cross-File Resolution
- Use stc.toml `source_roots` to find .st files, then build dependency graph from POU references
- Hierarchical symbol table: global scope → POU scope → method scope → block scope, each with parent reference for name lookup
- Two-pass analysis: first pass collects all POU declarations and type signatures, second pass type-checks bodies (handles forward references)

### Vendor-Aware Diagnostics
- Go structs with feature flags (SupportsOOP, SupportsPointerTo, MaxStringLen, etc.)
- Vendor diagnostics are warnings, not errors — code may be valid for current vendor, just not portable
- Three built-in profiles: `beckhoff`, `schneider`, `portable` (intersection). User sets via stc.toml or --vendor flag

### Claude's Discretion
- Internal error representation details
- Specific diagnostic message wording
- Symbol table caching strategy
- Test fixture organization

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/ast/` — Full CST node hierarchy with visitor pattern
- `pkg/source/source.go` — Pos, Span types
- `pkg/diag/` — Diagnostic type with file:line:col formatting, collector
- `pkg/parser/` — Parser produces `(AST, []Diagnostic)` tuples
- `pkg/project/config.go` — stc.toml Config with VendorTarget field
- `pkg/preprocess/` — Preprocessor with source maps

### Established Patterns
- Table-driven tests with testdata/ directories
- `--format json` on all CLI commands
- Two-result pattern: `(result, diagnostics)` — never fail, always return both

### Integration Points
- `stc check <files...> --format json` — new CLI command
- Symbol table will be consumed by LSP (Phase 9) and interpreter (Phase 4)
- Vendor profiles will be consumed by emitters (Phase 7)

</code_context>

<specifics>
## Specific Ideas

- Research warns this is the hardest technical problem — budget extra time for ANY type hierarchy
- MATIEC's two-pass approach is the proven reference implementation
- Must handle forward references (FB declared after use)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
