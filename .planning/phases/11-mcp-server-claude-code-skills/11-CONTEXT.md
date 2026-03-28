# Phase 11: MCP Server & Claude Code Skills - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

LLM agents can parse, check, test, lint, format, and emit ST code through MCP tools. Claude Code users get purpose-built skills for ST development workflows (generate, validate, test, emit, review).

</domain>

<decisions>
## Implementation Decisions

### MCP Server
- Use official MCP Go SDK (`modelcontextprotocol/go-sdk`) per stack research
- Binary: `cmd/stc-mcp/` — separate from main `stc` CLI
- Tools exposed: stc_parse, stc_check, stc_test, stc_emit, stc_lint, stc_format
- All tool descriptions under 100 tokens each for minimal agent context
- JSON schemas for all inputs/outputs
- Stdin/stdout transport (standard MCP)

### Claude Code Skills
- Skills as markdown files in `.claude/skills/` directory
- Full workflow skills: generate, validate, test, emit, review
- Skills auto-invoke when working with .st files
- Each skill chains CLI commands (e.g., validate = parse + check + lint)

### Claude's Discretion
- MCP tool parameter naming
- Skill markdown structure
- Error handling patterns
- Which MCP SDK version to use

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- All CLI commands: parse, check, test, emit, lint, fmt, pp, sim, lsp
- `pkg/parser/`, `pkg/analyzer/`, `pkg/interp/`, `pkg/testing/`, `pkg/emit/`, `pkg/lint/`, `pkg/format/`
- Every command already supports `--format json`

### Integration Points
- MCP server wraps CLI tool functions directly (not shelling out)
- Skills reference `stc` binary commands
- Skills may reference MCP tools as alternative

</code_context>

<specifics>
## Specific Ideas

- MCP is entirely novel in the ST domain — no other tool has this
- Skills should make Claude Code a first-class ST development environment

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
