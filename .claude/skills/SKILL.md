# STC - Structured Text Compiler Skills

Skills for IEC 61131-3 Structured Text development with the stc toolchain.

## Auto-Invoke

These skills activate automatically when working with `*.st` files in this project.

**Trigger pattern:** Any file matching `*.st` (Structured Text source files)

## Skills

| Skill | File | Description |
|-------|------|-------------|
| Generate | [st-generate.md](st-generate.md) | Generate IEC 61131-3 ST code from natural language descriptions |
| Validate | [st-validate.md](st-validate.md) | Validate ST code through parse + check + lint pipeline |
| Test | [st-test.md](st-test.md) | Write and run ST unit tests with assertions and time simulation |
| Emit | [st-emit.md](st-emit.md) | Emit vendor-specific ST for Beckhoff, Schneider, or portable targets |
| Review | [st-review.md](st-review.md) | Review ST code against IEC 61131-3 best practices and PLCopen guidelines |

## MCP Tools

When using stc via MCP server (`stc-mcp`), tools are available with these names:

| CLI Command | MCP Tool | Description |
|-------------|----------|-------------|
| `stc parse` | `stc_parse` | Parse ST, return AST or diagnostics |
| `stc check` | `stc_check` | Type check, return diagnostics |
| `stc test` | `stc_test` | Run tests, return results |
| `stc emit` | `stc_emit` | Emit vendor-specific ST |
| `stc lint` | `stc_lint` | Lint, return suggestions |
| `stc fmt` | `stc_format` | Format ST code |

Build MCP server: `go build -o stc-mcp ./cmd/stc-mcp`

## Prerequisites

- **stc binary** must be built and on PATH, or use `go run ./cmd/stc/...` from the project root
- Build: `go build -o stc ./cmd/stc` (produces the `stc` binary)
- Verify: `stc --help` should list available subcommands

## Key Conventions

- ST source files use the `.st` extension
- Test files use the `_test.st` suffix (e.g., `motor_control_test.st` for `motor_control.st`)
- Project configuration lives in `stc.toml`
- All stc commands support `--format json` for machine-readable output
- Keywords are UPPERCASE (`PROGRAM`, `FUNCTION_BLOCK`, `VAR`, `END_VAR`, etc.)
- POU names use PascalCase; variables use camelCase
- 4-space indentation throughout
