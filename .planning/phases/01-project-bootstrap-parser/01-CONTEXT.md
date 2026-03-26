# Phase 1: Project Bootstrap & Parser - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can parse any IEC 61131-3 Ed.3 ST source file (including CODESYS OOP extensions) and get a structured AST or actionable error messages via a single CLI binary. This phase delivers the foundation: Go module, lexer, parser, AST types, CLI skeleton, and CI pipeline.

</domain>

<decisions>
## Implementation Decisions

### Project Structure
- Go module path: `github.com/centroid-is/stc`
- Package layout: `cmd/stc/`, `pkg/lexer/`, `pkg/parser/`, `pkg/ast/` — following the agent plan structure
- Build system: Makefile with `build`, `test`, `lint`, `install` targets
- Go version: 1.22+ (latest stable)

### Parser Architecture
- CST-first approach: preserve all tokens (whitespace, comments) in the tree — needed for formatter (Phase 8) and LSP, avoids painful retrofit. Based on rust-analyzer precedent.
- Error recovery: panic-mode with synchronization at `;`, `END_*`, `VAR` boundaries — proven in Go/Rust compilers
- CODESYS OOP: parse all OOP syntax (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS) but defer type-checking to Phase 3
- Hand-written lexer in Go with keyword table lookup

### CI & GitHub Actions
- CI matrix: macOS + Windows + Linux with Go 1.22, run on every PR
- Linting: golangci-lint with default + govet + errcheck + staticcheck
- Branch strategy: feature branches → PRs to main, no direct pushes
- Test coverage: `go test -coverprofile` with coverage badge in README

### Claude's Discretion
- Specific CST node type naming conventions
- Internal error representation details
- Test fixture organization
- Makefile target naming beyond the core four

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield project

### Established Patterns
- Reference: `st_compiler_requirements_v2.md` — comprehensive requirements doc
- Reference: `stc_agent_plan.md` — detailed agent plan with milestone breakdown

### Integration Points
- GitHub repo: `centroid-is/stc` — PRs, Actions, releases
- Future phases depend on AST types and parser API designed here

</code_context>

<specifics>
## Specific Ideas

- User's agent plan specifies exact package layout and milestone structure
- Interpreter-only execution model (no C++ transpiler, ever)
- No PLCopen XML (vendor interop through preprocessor ifdefs and ST re-emission)
- Agent-friendly CLI from day one: `--format json` on every command

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>
