# Stack Research

**Domain:** IEC 61131-3 Structured Text Compiler Toolchain
**Researched:** 2026-03-26
**Confidence:** HIGH (core stack), MEDIUM (LSP library choice)

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.23+ | Implementation language | Single static binary, fast compilation, excellent for LLM agent iteration, cross-compiles trivially. The Go compiler itself uses hand-written recursive descent -- a proven pattern for this exact use case. |
| Hand-written recursive descent parser | N/A | ST parsing with error recovery | Go, Clang, Rust, and TypeScript all use hand-written parsers for their compilers. Full control over error recovery is essential for LSP partial ASTs from broken code. Participle and ANTLR both impose constraints that fight the error-recovery requirement. |
| C++17 | Standard | Code generation target | STruC++ validates this target: function blocks become classes, interfaces use virtual methods, generics use templates. C++17 is the sweet spot -- universally supported by g++/clang++/MSVC without requiring bleeding-edge compiler features. |
| `text/template` | stdlib | C++ code emission | Go's built-in template engine is sufficient for emitting C++. No external dependency needed. Templates keep generated code readable and the emission logic maintainable. |

### Parser & Compiler Infrastructure

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Hand-written lexer | N/A | Tokenization | Write a custom lexer that emits ERROR tokens for invalid sequences and keeps scanning. Critical for LSP -- the lexer must never crash on malformed input. Model after Go's `scanner` package. |
| Hand-written Pratt parser | N/A | Expression parsing | Use Pratt parsing (operator-precedence) for the expression sub-grammar. Recursive descent for statements and declarations, Pratt for expressions. This is the standard approach for languages with complex operator precedence. |
| `text/template` | stdlib | C++ code templates | Emit generated C++ through templates. One template per major construct (function block, program, function). Keep templates in embedded files via `embed`. |

### CLI Framework

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `spf13/cobra` | v1.10.x | CLI command framework | The standard for Go CLIs. Used by kubectl, docker, gh, hugo. Provides subcommands, flags, help generation, shell completion. Use for all `stc` subcommands (parse, check, test, emit, lint, format, lsp, mcp). |
| `spf13/viper` | v1.19.x | Configuration management | Pairs with Cobra for config files, env vars, and flag binding. Use for project-level `.stc.yaml` configuration. |

### LSP Server

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `tliron/glsp` | latest (March 2024) | LSP protocol SDK | Best available Go LSP SDK. Supports LSP 3.16/3.17, stdio/TCP/WebSocket transports, provides ready-to-run JSON-RPC 2.0 server with all message structures pre-built. Use as the foundation, but expect to extend or patch for newer LSP features. |

### MCP Server

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `modelcontextprotocol/go-sdk` | v1.4.1 | MCP server implementation | The official Go SDK, maintained in collaboration with Google. Supports MCP spec 2025-11-25 with backward compatibility. Use this over `mark3labs/mcp-go` -- it is now the canonical implementation. |

### Testing

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` | stdlib | Test framework | Use Go's built-in testing for all unit and integration tests. No external framework needed. Go convention is table-driven tests, which map perfectly to parser test cases. |
| `stretchr/testify` | v1.10.x | Assertions and mocking | Use `assert` and `require` packages for cleaner test assertions. Use `mock` package sparingly -- prefer real implementations where possible. |
| `encoding/json` | stdlib | JSON output verification | For testing `--format json` CLI output and MCP protocol messages. |
| `os/exec` | stdlib | CLI integration tests | For end-to-end tests that invoke the `stc` binary and verify output. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/xml` | stdlib | PLCopen XML import/export | Parse and generate PLCopen XML format for vendor interop. |
| `embed` | stdlib | Embedded templates and test fixtures | Embed C++ templates and standard library definitions into the binary. |
| `go.uber.org/zap` | v1.27.x | Structured logging | Fast structured logging for compiler diagnostics, LSP events, MCP communication. |
| `github.com/google/go-cmp` | v0.6.x | Test comparison | Deep equality comparison for AST nodes in tests. Better diff output than `reflect.DeepEqual`. |
| `github.com/jstemmer/go-junit-report` | v2.1.x | JUnit XML output | Convert `go test` output to JUnit XML for CI integration. Required by project spec. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `golangci-lint` | Linting | Run the standard suite: `govet`, `staticcheck`, `errcheck`, `ineffassign`. Configure via `.golangci.yml`. |
| `gofumpt` | Formatting | Stricter than `gofmt`. Enforces consistent style across codebase. |
| `goreleaser` | Release builds | Cross-compile for Linux/macOS/Windows from CI. Single binary distribution. |
| `clang-format` | C++ output formatting | Format generated C++ code. Ship a `.clang-format` config targeting C++17 style. |
| `g++` / `clang++` | C++ compilation | For compiling generated C++ in host test mode. Require C++17 support. |

## Installation

```bash
# Initialize Go module
go mod init github.com/jonb/stc

# Core dependencies
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/tliron/glsp@latest
go get github.com/modelcontextprotocol/go-sdk@latest
go get go.uber.org/zap@latest

# Test dependencies
go get github.com/stretchr/testify@latest
go get github.com/google/go-cmp@latest

# Dev tools (install globally)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install mvdan.cc/gofumpt@latest
go install github.com/goreleaser/goreleaser@latest
go install github.com/jstemmer/go-junit-report/v2@latest
```

## Alternatives Considered

### Parser Approach

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| Hand-written recursive descent | `alecthomas/participle` v2.1.4 | Participle uses struct tags to define grammars -- elegant for simple DSLs but fundamentally wrong for a compiler-grade parser. It is LL(k) only (no left recursion), provides no control over error recovery, and cannot produce partial ASTs from malformed input. The LSP requirement kills this option. |
| Hand-written recursive descent | ANTLR4 Go target (`antlr4-go/antlr` v4.13.1) | ANTLR generates parsers from `.g4` grammars. Existing IEC 61131-3 grammars exist (vlsi/iec61131-parser, TUM-AIS). However: (1) ANTLR requires Java at build time (acceptable per constraints but adds friction), (2) generated Go code is verbose and hard to debug, (3) error recovery is possible but requires custom error strategies that fight the generated code, (4) the Go target is the least mature ANTLR target. The project constraint "hand-written recursive descent parser" in KEY DECISIONS already settles this. |

### LSP Library

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| `tliron/glsp` | `go.lsp.dev/protocol` v0.12.0 | Last published March 2022. Implements LSP 3.15.3 (two major versions behind current 3.17). Low-level protocol types only -- you must build the entire server yourself. Not actively maintained. |
| `tliron/glsp` | `TobiasYin/go-lsp` | Smaller community, less feature coverage. GLSP provides more complete transport support (stdio, TCP, WebSocket, Node IPC). |
| `tliron/glsp` | Hand-written LSP from scratch | Possible (gopls does this internally) but enormous effort. LSP spec has hundreds of message types. GLSP gives you the message structures and JSON-RPC plumbing; you focus on language features. |

### MCP Library

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| `modelcontextprotocol/go-sdk` | `mark3labs/mcp-go` | mcp-go was the community standard before the official SDK. Now that the official SDK exists (maintained with Google collaboration), use the official one. It tracks the spec more closely and will be better maintained long-term. |

### CLI Framework

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| `spf13/cobra` | `urfave/cli` v2 | Both are mature. Cobra wins on ecosystem: more examples, better documentation, used by more Go CLI tools. The `stc` tool will have many subcommands (parse, check, test, emit, lint, format, lsp, mcp) -- Cobra handles this cleanly. |
| `spf13/cobra` | `alecthomas/kong` | Kong is newer and cleaner for simple CLIs. But Cobra's extensive documentation and community examples matter when LLM agents need to modify CLI code. |

### Testing

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| stdlib `testing` + `testify` | `onsi/ginkgo` + `gomega` | BDD-style is unnecessary complexity for a compiler test suite. Table-driven tests with `testify/assert` are the Go standard pattern. Ginkgo adds a test runner that replaces `go test`, which complicates CI. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `alecthomas/participle` | Cannot produce partial ASTs from broken code. LL(k) limitation prevents natural expression of ST grammar. Struct-tag grammars become unmaintainable at compiler scale. | Hand-written recursive descent with Pratt expression parsing |
| ANTLR4 Go target at runtime | Requires Java toolchain for grammar compilation. Generated Go code is verbose, hard to debug, and the Go target is ANTLR's weakest. Error recovery fights the generated structure. | Hand-written recursive descent. Reference existing ANTLR `.g4` grammars (vlsi/iec61131-parser) for grammar specification only. |
| `go.lsp.dev/protocol` | Stale (2022), implements LSP 3.15.3 (obsolete). No server infrastructure -- just types. | `tliron/glsp` for LSP 3.17 with full server support |
| `mark3labs/mcp-go` | Community library superseded by official SDK. Will diverge from spec over time. | `modelcontextprotocol/go-sdk` (official, Google-maintained) |
| `onsi/ginkgo` | BDD overhead adds complexity without value for compiler testing. Replaces `go test` runner. | stdlib `testing` with `testify/assert` |
| CGo for C++ integration | Couples the Go binary to a C toolchain at build time. Breaks cross-compilation. Makes the binary non-static. | Shell out to `g++`/`clang++` to compile generated C++. The compiler emits `.cpp` files; a separate step compiles them. |
| LLVM Go bindings | Premature optimization. LLVM adds massive complexity and binary size. C++ transpilation gets tests running months earlier. | Transpile to C++17 first. LLVM is a future optimization, not a starting point. |

## Stack Patterns by Variant

**For parser testing (most common test type):**
- Table-driven tests with `testify/assert`
- One test case per ST construct
- Golden file tests for C++ output comparison
- Use `go-cmp` for AST deep comparison with readable diffs

**For C++ code generation:**
- `text/template` with `embed` for template files
- Templates organized per construct: `function_block.cpp.tmpl`, `program.cpp.tmpl`, etc.
- `clang-format` as post-processing step for readable output
- Golden file tests comparing expected vs actual C++ output

**For LSP development:**
- `tliron/glsp` provides the server skeleton
- Implement handlers incrementally: diagnostics first, then hover, then completion
- Test via VS Code extension development host
- Use stdio transport for development, add TCP/WebSocket later

**For MCP server:**
- `modelcontextprotocol/go-sdk` for the server
- Expose tools: `stc_parse`, `stc_check`, `stc_test`, `stc_emit`, `stc_lint`
- JSON schema for tool parameters auto-generated from Go structs
- Test with Claude Desktop or MCP Inspector

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| cobra v1.10.x | viper v1.19.x | Official pairing. Cobra uses viper for flag binding. |
| glsp (latest) | LSP 3.16-3.17 | Check for 3.18 updates as LSP evolves. |
| go-sdk v1.4.x | MCP spec 2025-11-25 | Backward compatible with 2025-06-18, 2025-03-26. |
| Go 1.23+ | All listed packages | Use latest stable Go. Generics available since 1.18. |
| C++17 | g++ 7+, clang++ 5+, MSVC 19.14+ | Universal C++17 support across all major compilers. |

## Key Architecture Decisions Driven by Stack

1. **Parser produces a Concrete Syntax Tree (CST) first, then lowered to AST.** The hand-written parser preserves all tokens (whitespace, comments) in the CST for formatting and LSP features. A second pass produces a clean AST for semantic analysis and code generation. This is the pattern used by `rust-analyzer` and newer `gopls`.

2. **C++ emission via templates, not string concatenation.** Using `text/template` keeps the generated C++ readable and the emission code maintainable. Templates can be tested independently by rendering them with test data.

3. **No CGo. Ever.** The `stc` binary must be a static single binary. All C++ compilation happens by shelling out to the system C++ compiler. This preserves cross-compilation and keeps the binary small.

4. **MCP and LSP share the same compiler core.** Both the LSP server and MCP server call the same `parse()`, `check()`, `emit()` functions. The compiler core is a library; CLI/LSP/MCP are thin wrappers.

## Reference Prior Art

| Project | Language | Relevance | License |
|---------|----------|-----------|---------|
| STruC++ | TypeScript | Validates ST-to-C++17 architecture. 1400+ tests. Function blocks as classes, virtual methods for interfaces. | GPL-3.0 (cannot reuse code) |
| MATIEC | C | Proven IEC 61131-3 compiler. Ed.2 only. | LGPL |
| blark | Python | Beckhoff TwinCAT ST parser using Lark (Earley). Good reference for TwinCAT dialect specifics. | BSD-2 |
| vlsi/iec61131-parser | ANTLR4/Java | ANTLR4 grammar for IEC 61131-3. Useful as grammar reference even though we hand-write the parser. | Apache-2.0 |
| TUM-AIS/IEC611313ANTLRParser | ANTLR4/Java | Parses both IEC 61131-3 ST and TIA Portal SCL. Good reference for vendor dialect differences. | GPL-3.0 |
| iec-checker | OCaml | Static analysis of IEC 61131-3 programs. Reference for analysis patterns. | LGPL-3.0 |

## Sources

- [Participle v2 on pkg.go.dev](https://pkg.go.dev/github.com/alecthomas/participle/v2) -- v2.1.4, published March 24, 2025 (MEDIUM confidence)
- [ANTLR4 Go target](https://pkg.go.dev/github.com/antlr4-go/antlr/v4) -- v4.13.1, published May 15, 2024 (HIGH confidence)
- [GLSP on GitHub](https://github.com/tliron/glsp) -- LSP 3.16/3.17, last updated March 2024 (MEDIUM confidence -- early release)
- [go.lsp.dev/protocol on pkg.go.dev](https://pkg.go.dev/go.lsp.dev/protocol) -- v0.12.0, published March 2022, LSP 3.15.3 (HIGH confidence -- verified stale)
- [MCP Go SDK releases](https://github.com/modelcontextprotocol/go-sdk/releases) -- v1.4.1, published March 2026 (HIGH confidence)
- [Cobra on pkg.go.dev](https://pkg.go.dev/github.com/spf13/cobra) -- v1.10.2+, published December 2025 (HIGH confidence)
- [STruC++ on GitHub](https://github.com/Autonomy-Logic/STruCpp) -- Architecture reference for ST-to-C++17 (HIGH confidence)
- [vlsi/iec61131-parser](https://github.com/vlsi/iec61131-parser) -- ANTLR4 grammar reference (MEDIUM confidence)
- [Go compiler uses hand-written recursive descent](https://news.ycombinator.com/item?id=19008781) -- Pattern validation (HIGH confidence)
- [Resilient recursive descent parsing](https://thunderseethe.dev/posts/parser-base/) -- Error recovery patterns (MEDIUM confidence)
- [Eli Bendersky on ungrammar and resilient parsing in Go](https://eli.thegreenplace.net/2023/ungrammar-in-go-and-resilient-parsing/) -- Implementation reference (HIGH confidence)

---
*Stack research for: IEC 61131-3 Structured Text Compiler Toolchain in Go*
*Researched: 2026-03-26*
