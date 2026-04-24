# stc -- Structured Text Compiler Toolchain

A Go-based IEC 61131-3 Structured Text compiler toolchain that brings modern software development practices to PLC programming -- parsing, type-checking, unit testing, simulation, formatting, linting, and multi-vendor emission, all from a single CLI binary with no hardware required.

[![CI](https://github.com/centroid-is/stc/actions/workflows/ci.yml/badge.svg)](https://github.com/centroid-is/stc/actions/workflows/ci.yml)
[![Coverage](https://github.com/centroid-is/stc/actions/workflows/coverage.yml/badge.svg)](https://github.com/centroid-is/stc/actions/workflows/coverage.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/centroid-is/stc)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Features

| Feature | Description |
|---------|-------------|
| **Parser** | Hand-written recursive descent parser for IEC 61131-3 Ed.3 with CODESYS OOP extensions, error recovery, and partial AST output |
| **Preprocessor** | Conditional compilation (`{IF}`, `{ELSIF}`, `{ELSE}`, `{END_IF}`, `{DEFINE}`) with source maps |
| **Type Checker** | Two-pass semantic analysis with vendor-aware diagnostics, unused variable detection, and unreachable code warnings |
| **Interpreter** | Tree-walking interpreter with PLC scan-cycle semantics and deterministic time |
| **Standard Library** | Full IEC stdlib: timers (TON, TOF, TP), counters (CTU, CTD), edge detectors (R_TRIG, F_TRIG), bistables (SR, RS), math, string, and conversion functions |
| **Unit Testing** | ST-native `TEST_CASE` syntax with `ASSERT_TRUE`, `ASSERT_FALSE`, `ASSERT_EQ`, `ASSERT_NEAR`, `ADVANCE_TIME`, I/O mocking, and JUnit XML output |
| **Simulation** | Closed-loop simulation with waveform injection (step, ramp, sine, square) and plant models (motor, valve, cylinder) |
| **Multi-Vendor Emission** | Emit vendor-flavored ST for Beckhoff, Schneider, or portable targets |
| **Formatter** | Auto-format with configurable indentation and keyword casing; comments preserved |
| **Linter** | PLCopen coding guidelines: magic numbers, nesting depth, POU length, naming conventions |
| **LSP Server** | Full Language Server Protocol: diagnostics, go-to-definition, hover, completion, rename, references, semantic tokens |
| **Incremental Compilation** | File-level dependency tracking with cached symbol tables -- only re-analyzes changed files |
| **Vendor Libraries** | Load vendor FB stubs from `.st` declaration files; shipped stubs for Beckhoff, Schneider, and Allen Bradley |
| **Mock Framework** | ST-based mock FBs for testing; auto-generated zero-value stubs; signature validation against vendor declarations |
| **MCP Server** | Model Context Protocol server exposing all tools for LLM agent integration |
| **Claude Code Skills** | Purpose-built skills for ST generation, validation, testing, emission, and code review |

## Quick Start

### Install from Source

```bash
go install github.com/centroid-is/stc/cmd/stc@latest
```

### Build from Repository

```bash
git clone https://github.com/centroid-is/stc.git
cd stc
make build
# Binary: ./stc
```

### First Parse

```bash
stc parse myfile.st
# Parsed 3 declaration(s), 0 diagnostic(s) in myfile.st

stc parse myfile.st --format json
# Full JSON AST output
```

### First Type Check

```bash
stc check src/*.st
# 0 error(s), 0 warning(s)

stc check src/*.st --vendor beckhoff
# Vendor-aware warnings for Beckhoff compatibility
```

### First Test

Create `motor_test.st`:

```iec
TEST_CASE 'Timer fires after preset'
VAR
    t : TON;
END_VAR
    t(IN := TRUE, PT := T#100ms);
    ASSERT_FALSE(t.Q);

    ADVANCE_TIME(T#110ms);
    t(IN := TRUE, PT := T#100ms);
    ASSERT_TRUE(t.Q);
END_TEST_CASE
```

Run it:

```bash
stc test .
# --- PASS: Timer fires after preset (0.000s)
# ok
# 1 tests, 1 passed, 0 failed
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `stc parse <file...>` | Parse ST source files and output AST |
| `stc check <file...>` | Type-check ST source files with semantic analysis |
| `stc test [dir]` | Discover and run `*_test.st` unit tests |
| `stc sim <file>` | Run closed-loop simulation with waveform injection |
| `stc emit <file...>` | Emit vendor-specific ST (Beckhoff, Schneider, portable) |
| `stc fmt <file...>` | Format ST source files with consistent style |
| `stc lint <file...>` | Lint ST source files against PLCopen guidelines |
| `stc pp <file...>` | Preprocess ST files (evaluate conditional directives) |
| `stc lsp` | Start the Language Server Protocol server on stdio |
| `stc vendor extract <.plcproj>` | Extract FB stubs from TwinCAT project files |

All commands support `--format json` for machine-readable output. See [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) for full details.

## MCP Server Setup

The MCP server exposes all stc tools for LLM agents over stdio transport.

Build:

```bash
go build -o stc-mcp ./cmd/stc-mcp
```

Add to your Claude configuration (e.g., `~/.claude/settings.json` or MCP client config):

```json
{
  "mcpServers": {
    "stc": {
      "command": "/path/to/stc-mcp",
      "args": []
    }
  }
}
```

Available MCP tools: `stc_parse`, `stc_check`, `stc_test`, `stc_emit`, `stc_lint`, `stc_format`.

## VS Code Extension

The VS Code extension provides syntax highlighting and LSP integration.

1. Build the extension:
   ```bash
   cd editors/vscode
   npm install
   npm run compile
   ```

2. Copy or symlink `editors/vscode` into `~/.vscode/extensions/stc-st`.

3. Set the `stc.lsp.path` setting to the path of your `stc` binary (default: `stc` on PATH).

The extension activates for `.st` files and provides real-time diagnostics, go-to-definition, hover, completion, rename, references, and semantic tokens for preprocessor blocks.

## Claude Code Skills

When working in the stc repository, Claude Code skills auto-invoke for `.st` files:

| Skill | Description |
|-------|-------------|
| Generate | Generate IEC 61131-3 ST code from natural language descriptions |
| Validate | Validate ST code through parse + check + lint pipeline |
| Test | Write and run ST unit tests with assertions and time simulation |
| Emit | Emit vendor-specific ST for Beckhoff, Schneider, or portable targets |
| Review | Review ST code against IEC 61131-3 best practices and PLCopen guidelines |

Skills are defined in `.claude/skills/` and chain CLI commands automatically.

## Vendor Library Support

stc ships with stub declarations for common vendor function blocks:

- **Beckhoff**: Tc2_MC2, Tc2_System, Tc2_Utilities, Tc3_EventLogger, common types
- **Schneider**: Motion, Communication, System function blocks
- **Allen Bradley**: Timers (TONR, TOFR, RTO), common instructions, AB-restricted profile

Configure in `stc.toml`:

```toml
[build]
vendor_target = "beckhoff"

[build.library_paths]
tc2_mc2 = "vendor/beckhoff/tc2_mc2.st"

[test]
mock_paths = ["mocks/"]
```

You can also extract stubs from existing TwinCAT projects:

```bash
stc vendor extract MyProject.plcproj --output vendor/custom/
```

See [docs/VENDOR_LIBRARIES.md](docs/VENDOR_LIBRARIES.md) for the full design and usage guide.

## Project Structure

```
cmd/
  stc/              CLI binary (parse, check, test, sim, emit, fmt, lint, pp, lsp, vendor)
  stc-mcp/          MCP server binary (6 tool handlers)
pkg/
  analyzer/         Cross-file semantic analysis orchestrator
  ast/              AST/CST node types with trivia for lossless round-tripping
  checker/          Two-pass type checker with vendor profiles
  diag/             Diagnostic types (errors, warnings with file:line:col)
  emit/             Multi-vendor ST emitter (Beckhoff, Schneider, portable)
  format/           Code formatter with configurable style
  incremental/      File-level dependency tracking and caching
  interp/           Tree-walking interpreter with scan-cycle engine and stdlib
  iomap/            I/O address parsing and mock I/O table (%I, %Q, %M)
  lexer/            Tokenizer with full IEC keyword table and trivia
  lint/             PLCopen coding guidelines linter
  lsp/              Language Server Protocol implementation
  parser/           Recursive descent parser with Pratt expressions and error recovery
  pipeline/         Parse pipeline (preprocess + parse in one step)
  preprocess/       Conditional compilation preprocessor with source maps
  project/          stc.toml configuration loading
  sim/              Simulation engine with waveforms and plant models
  source/           Source file and position tracking
  symbols/          Symbol table with hierarchical scoping
  testing/          Test runner (discovery, execution, JUnit XML, JSON output)
  types/            IEC type system (type lattice, widening, built-in functions)
  vendor/           Vendor library stub loading, mock loading, TcPOU extraction
  version/          Version info (injected at build time)
stdlib/
  vendor/           Shipped vendor FB stub declarations
  mocks/            Shipped behavioral mock implementations
editors/
  vscode/           VS Code extension (TextMate grammar + LSP client)
tests/              ST test suites and parse corpus
.claude/
  skills/           Claude Code skill definitions for ST workflows
```

## Contributing

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for development setup, code organization, and PR workflow.

## License

MIT
