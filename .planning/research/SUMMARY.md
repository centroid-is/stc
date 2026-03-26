# Project Research Summary

**Project:** STC — IEC 61131-3 Structured Text Compiler Toolchain
**Domain:** Compiler / Language Toolchain (industrial PLC domain)
**Researched:** 2026-03-26
**Confidence:** HIGH

## Executive Summary

STC is a compiler toolchain for IEC 61131-3 Structured Text (ST) — the dominant textual language for industrial PLC programming. The right approach is a Go-based compiler-as-library architecture with a hand-written recursive descent parser, C++17 transpilation backend, and consumers (CLI, LSP server, MCP server) as thin wrappers over a shared compiler API. This is not a toy project: ST has a genuinely hard grammar (not LALR(1)-parseable as written in the spec), real-world code uses CODESYS OOP extensions heavily, and the LSP requires error-tolerant partial ASTs from the start. All three prior open-source ST compilers (MATIEC, STruC++, RuSTy) have critical limitations that STC is positioned to address.

The killer feature is host-based unit testing — the ability to run ST unit tests on a developer's machine without PLC hardware. No open-source tool does this. TcUnit and CfUnit both require a PLC runtime. STC's value proposition is: parse standard ST + CODESYS extensions, validate it, run tests, and emit vendor-flavored output for Beckhoff and Schneider — all from a single static binary usable in CI pipelines. A secondary differentiator is being the first ST tool with full LSP support for VS Code/Neovim and the first with LLM agent integration via MCP.

The dominant risks are (1) the grammar complexity — identifier ambiguity means the parser must consult a symbol table during parsing, not after it; (2) deferring OOP support, which MATIEC did and which made it irrelevant to modern codebases; and (3) getting scan cycle semantics wrong in the host test runner, which would break timer-dependent tests. All three risks must be addressed in the first two phases. The architecture research is clear: build the compiler as a library first, add consumers incrementally, and never let compiler logic leak into CLI handlers.

## Key Findings

### Recommended Stack

The entire toolchain is implemented in Go 1.23+ using a hand-written lexer and recursive descent parser (with Pratt parsing for expressions), `text/template` for C++17 code emission, and stdlib `testing` + `testify` for tests. The Go compiler itself uses this exact pattern — hand-written recursive descent with clean phase separation. No parser generators: both Participle (LL(k), no left recursion, no error recovery) and ANTLR4 Go target (requires Java, verbose generated code) are explicitly rejected.

For the LSP: `tliron/glsp` (LSP 3.16/3.17, stdio/TCP/WebSocket support). For the MCP server: the official `modelcontextprotocol/go-sdk` v1.4.1 (maintained with Google, supersedes `mark3labs/mcp-go`). For the CLI: `spf13/cobra` + `spf13/viper`. No CGo, ever — the binary must remain a single static executable cross-compilable for Linux/macOS/Windows.

**Core technologies:**
- **Go 1.23+**: Implementation language — single static binary, fast, cross-compiles trivially, LLM-friendly
- **Hand-written recursive descent + Pratt parser**: Required for error recovery and symbol-table-augmented parsing
- **C++17 via `text/template`**: Transpilation target — STruC++ validates this approach with 1400+ tests; no CGo
- **`tliron/glsp`**: LSP server SDK — best available Go option; LSP 3.16/3.17
- **`modelcontextprotocol/go-sdk` v1.4.1**: Official MCP SDK — canonical, Google-maintained
- **`spf13/cobra` + `spf13/viper`**: CLI framework — standard for complex Go CLIs

### Expected Features

The feature research is unusually clear about what matters and what to defer. The primary differentiators — host-based unit testing, LSP intelligence, multi-vendor ST emission — are all absent from every existing open-source ST tool. MIT licensing removes the last adoption barrier that plagues MATIEC (LGPL) and STruC++ (GPL-3.0).

**Must have (table stakes for v1.0):**
- IEC 61131-3 Ed.3 parser with CODESYS extensions (OOP, POINTER TO, REFERENCE TO) — cannot parse production code without
- Type checking and semantic analysis — the minimum value proposition
- IEC standard library (TON, CTU, R_TRIG, math/string/conversion) — required for type checking and test execution
- C++17 transpiler — enables host-based testing (the killer feature)
- Host-based unit test runner with JUnit XML output — the single biggest differentiator
- CLI with subcommands + JSON output — composable tool interface
- Multi-file project support — real projects have hundreds of POUs
- VS Code syntax highlighting — minimum editor experience

**Should have (v1.x, add after validation):**
- Vendor ST re-emission (Beckhoff first, then Schneider/CODESYS)
- LSP server (diagnostics, go-to-definition, hover, completions)
- Formatter (`stc fmt`) — no open-source ST formatter exists; STweep is commercial only
- Linter (`stc lint`) — CODESYS static analysis is paid; open equivalent has clear demand
- MCP server — thin wrapper over CLI; low effort once CLI is stable
- Claude Code skills for ST workflows
- PLCopen XML import

**Defer (v2+):**
- Closed-loop simulation with plant modeling — very high complexity; needs test runner proven first
- Allen Bradley emission — entirely different dialect; distinct research phase required
- PLCopen XML export — lower priority than import
- Incremental compilation — performance optimization for large projects
- LLVM backend — only if C++ transpilation proves insufficient

### Architecture Approach

The architecture is a compiler-as-library with a clean facade API (`pkg/compiler/`), multiple consumers (CLI, LSP server, MCP server) as thin wrappers, and pluggable output backends via the visitor pattern. Every pipeline stage uses the `(T, []Diagnostic)` pattern — never `(T, error)` — so the parser always produces an AST even from broken code. The critical path is: `source` → `ast` → `lexer` → `parser` → `symbols` → `resolver` → `checker` → `compiler` facade, after which backends (interpreter, C++ codegen, vendor ST emitter) and consumers (CLI, LSP, MCP) fan out independently.

**Major components:**
1. **Compiler Core** (`pkg/compiler/`) — facade orchestrating lexer, parser, name resolver, type checker; single entry point for all consumers
2. **Frontend Pipeline** (`pkg/lexer/`, `pkg/parser/`, `pkg/ast/`) — tokenization, recursive descent parsing, CST/AST production with error nodes and recovery
3. **Semantic Analysis** (`pkg/symbols/`, `pkg/checker/`) — hierarchical scope resolution, two-pass type inference (candidate enumeration then narrowing per MATIEC approach)
4. **Output Backends** (`pkg/emit/cpp/`, `pkg/emit/st/`, `pkg/interp/`) — C++17 codegen via templates, vendor ST emitters, tree-walking interpreter with scan cycle engine
5. **Test Runner** (`pkg/testrun/`) — discovers ST test programs, executes via scan cycle engine with virtual clock, produces JUnit XML
6. **LSP Server** (`pkg/lsp/`) — consumes compiler API with caching layer; incremental document model
7. **MCP Server** (`pkg/mcp/`) — wraps CLI tools as MCP tools; thin wrapper

### Critical Pitfalls

1. **Grammar is not LALR(1)-parseable** — The IEC spec grammar has pervasive identifier ambiguity (same token can be variable, type name, function, or enum value). The parser must consult a symbol table during parsing. MATIEC documents this explicitly. Hand-written recursive descent with interleaved symbol table building is the solution; this must be correct in Phase 1 or it requires a parser rewrite.

2. **Deferring OOP support kills parser relevance** — MATIEC's Edition 2-only support made it irrelevant to modern codebases. 60-80% of production TwinCAT/CODESYS code uses INTERFACE, METHOD, EXTENDS, IMPLEMENTS, PROPERTY. The AST, symbol tables, and parser must handle OOP from the first milestone, even if type-checking of OOP semantics is incremental.

3. **Wrong type inference approach leads to type checker rewrite** — IEC's ANY_NUM/ANY_INT type classes require two-pass type inference (MATIEC's `fill_candidate_datatypes` then `narrow_candidate_datatypes`). Single-pass type resolution rejects valid code. Budget 2x expected time for the type system.

4. **Error recovery designed as afterthought breaks LSP** — LSP requires partial ASTs from mid-edit broken code. Panic-mode recovery (skip to next semicolon) produces garbage ASTs. Error nodes must be first-class AST citizens from Phase 1. Recovery points: `;`, `END_IF`, `END_FOR`, `END_WHILE`, `END_FUNCTION`, `END_FUNCTION_BLOCK`, `END_PROGRAM`, `END_VAR`.

5. **Wall-clock time in scan cycle engine breaks timer tests** — TON/TOF/TP timer FBs must use a virtual clock advancing by one scan cycle per iteration, not wall-clock time. Tests that use `time.Now()` or `time.Sleep()` become flaky and non-deterministic. Design the virtual clock into the scan cycle engine from Phase 3 day one.

## Implications for Roadmap

Based on research, the build order is dictated by hard dependencies. The parser unblocks everything. Semantic analysis unblocks all backends and the LSP. Backends can be built in parallel. Consumers wrap the compiler API. The architecture research provides an explicit phase ordering.

### Phase 1: Parser Foundation

**Rationale:** Everything depends on the parser. The LSP requirement mandates error-tolerant partial ASTs from the start — this is not a feature to add later. CODESYS OOP extensions must be in the grammar from day one (Pitfall 2). The grammar ambiguity (Pitfall 1) requires symbol-table-augmented parsing baked into the recursive descent design. This phase is unusually high-risk because getting it wrong requires a rewrite.

**Delivers:** Hand-written lexer + recursive descent parser producing error-tolerant ASTs for IEC 61131-3 Ed.3 + CODESYS extensions (OOP, POINTER TO, REFERENCE TO, INTERFACE, METHOD, PROPERTY, EXTENDS, IMPLEMENTS). CLI `stc parse` command with JSON AST output. Syntax highlighting TextMate grammar for VS Code.

**Addresses:** IEC 61131-3 Ed.3 parser, CODESYS extension support, VS Code syntax highlighting, CLI interface scaffold, structured JSON error output

**Avoids:** Grammar ambiguity pitfall (Pitfall 1), OOP deferral trap (Pitfall 2), error recovery afterthought (Pitfall 4), agent-unfriendly error messages (Pitfall 8)

**Research flag:** Standard patterns — hand-written recursive descent for Go is well-documented; reference Go compiler `src/go/parser` and rust-analyzer architecture.

---

### Phase 2: Semantic Analysis and Type System

**Rationale:** Type checking is the core value proposition ("catch errors without a PLC"). MATIEC's two-pass type inference approach (Pitfall 3) is the single hardest technical problem in the project. Symbol table design for OOP hierarchies (interfaces, inheritance, method dispatch) is complex. This phase must be complete before any backend can produce correct output.

**Delivers:** Name resolver, hierarchical symbol tables (supports OOP scope chains), two-pass type checker with type lattice, IEC standard library type signatures (TON, CTU, etc.), semantic diagnostics with source spans, `stc check` CLI command, multi-file project support.

**Addresses:** Type checking, semantic analysis, IEC standard library (type signatures), multi-file project support, clear error messages, vendor-aware diagnostics (dialect mode flags)

**Avoids:** Single-pass type resolution failure (Pitfall 3), vendor dialect drift (Pitfall 5) — add vendor profile system here

**Research flag:** Needs research-phase — two-pass type inference for IEC ANY type hierarchy is complex; K-ST formal semantics paper is a key reference. MATIEC stage3 source code is the best implementation reference.

---

### Phase 3: Host Execution and Testing

**Rationale:** Host-based unit testing is the killer differentiator. This is what no existing open-source ST tool provides. The C++17 transpiler and tree-walking interpreter are parallel paths to host execution; start with the interpreter (faster to implement, no external C++ toolchain dependency for tests). Virtual clock and scan cycle semantics must be correct or tests are unreliable (Pitfall 6).

**Delivers:** Tree-walking interpreter with deterministic virtual clock, scan cycle engine (Read Inputs → Execute → Write Outputs → Advance Time), IEC standard library implementations (TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, math/string/conversion), test discovery and runner, JUnit XML output, `stc test` CLI command, I/O mocking framework.

**Addresses:** Host-based unit testing, IEC standard library (full implementation), C++17 transpiler (for host compilation path)

**Avoids:** Wall-clock time in scan cycle (Pitfall 6), standard library gaps (Pitfall 9)

**Research flag:** Needs research-phase — scan cycle simulation and virtual time are well-documented in STruC++ and the K-ST paper; review ADVANCE_TIME pattern from STruC++ test suite.

---

### Phase 4: Developer Experience (LSP + Formatter + Linter)

**Rationale:** Once the compiler core is solid, the LSP transforms STC from a batch tool into a real development environment. LSP performance (Pitfall 10) requires incremental document model design — debouncing semantic analysis, scope-level cache invalidation. Formatter and linter are high-value additions with no open-source equivalents.

**Delivers:** LSP server with diagnostics, go-to-definition, hover types, completions, rename; VS Code extension; formatter (`stc fmt`); linter (`stc lint`) with PLCopen coding guidelines.

**Addresses:** LSP server, formatter, linter, source-level debug mapping (partial via LSP)

**Avoids:** Full re-parse on every keystroke (Pitfall 10) — incremental document model is mandatory

**Research flag:** Standard patterns — rust-analyzer architecture is the gold standard reference; `tliron/glsp` provides the scaffolding.

---

### Phase 5: Multi-Vendor Emission and Interop

**Rationale:** Vendor ST re-emission ("write once, deploy to Beckhoff + Schneider") is the second major differentiator. Start with Beckhoff (TwinCAT) and Schneider/CODESYS since they are both CODESYS-derived and share the most surface area. PLCopen XML import enables onboarding users from existing vendor projects. Vendor dialect differences (Pitfall 5) must be handled via vendor profiles, not hardcoded logic.

**Delivers:** Beckhoff TwinCAT ST emitter, CODESYS/Schneider ST emitter, vendor profile system, vendor-aware diagnostics (`--vendor twincat`, `--vendor codesys`), PLCopen XML import, `stc emit` CLI command.

**Addresses:** Vendor ST re-emission, vendor-aware diagnostics, PLCopen XML import, preprocessor/conditional compilation

**Avoids:** Single-vendor assumptions baked in (Pitfall 5), PLCopen XML round-trip expectations (Pitfall 7)

**Research flag:** Needs research-phase — TwinCAT `.TcPOU`/`.tsproj` file format parsing is not well-documented; CODESYS project format is proprietary. May need direct file format reverse-engineering.

---

### Phase 6: LLM Integration (MCP Server + Claude Code Skills)

**Rationale:** The MCP server is a thin wrapper over the CLI; it is low-effort once the CLI surface is stable. Claude Code skills are markdown files chaining CLI commands. Both are novel in the ST domain and provide compounding value as LLM coding agents improve. Design the CLI for agents from day one (Pitfall 8) means this phase's foundation work starts in Phase 1.

**Delivers:** MCP server exposing `stc_parse`, `stc_check`, `stc_test`, `stc_emit`, `stc_lint` as MCP tools; Claude Code skills for `/st:generate`, `/st:validate`, `/st:test`, `/st:emit`, `/st:review`; `stc examples` command for canonical ST patterns.

**Addresses:** MCP server, Claude Code skills, agent-friendly tool surface

**Avoids:** Over-exposing CLI surface to agents (curate ~5 tools, not 30 commands)

**Research flag:** Standard patterns — `modelcontextprotocol/go-sdk` v1.4.1 is well-documented; Claude Code skills are markdown files.

---

### Phase Ordering Rationale

- **Parser before everything**: The LSP requirement forces error-tolerant partial ASTs. This is not a feature to bolt on — it is a parser architecture decision. Getting it right in Phase 1 prevents the highest-cost rewrite scenario.
- **OOP in Phase 1 grammar, not deferred**: MATIEC's failure pattern is precisely deferring OOP. 60-80% of production code uses it. The parser must handle it from the start even if semantic checking is incremental.
- **Type system before backends**: No backend can produce correct output from an untyped AST. The interpreter needs type info for value representation. C++ codegen needs type info for class hierarchies and virtual dispatch.
- **Interpreter before C++ codegen**: The interpreter path (parser → check → interpret) gets test execution running without requiring a C++ toolchain. It also validates the AST design before committing to the code generation visitor pattern.
- **LSP after type checker**: Useful LSP requires type information for hover types and completions. Diagnostics-only LSP is possible earlier but not high enough quality to be useful.
- **MCP last**: Thin wrapper; builds on a stable CLI. No point building it until CLI commands are stable.

### Research Flags

Phases needing deeper research during planning:
- **Phase 2 (Type System):** Two-pass type inference for IEC ANY type hierarchy is complex; no clean tutorials exist. MATIEC stage3 source code and K-ST paper are the primary references. Budget extra time.
- **Phase 5 (Vendor Emission):** TwinCAT `.TcPOU` file format and CODESYS project format are not well-documented in public sources. Direct inspection of exported files will be required.

Phases with standard patterns (can skip research-phase):
- **Phase 1 (Parser):** Hand-written recursive descent in Go is extremely well-documented; Go compiler and rust-analyzer are directly applicable references.
- **Phase 3 (Testing):** Scan cycle simulation pattern is validated by STruC++ test suite; virtual time implementation is straightforward.
- **Phase 4 (LSP):** rust-analyzer architecture is the definitive reference; `tliron/glsp` handles protocol scaffolding.
- **Phase 6 (MCP):** Official SDK is well-documented; this is a thin integration layer.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Go ecosystem choices verified against current package versions (March 2026). MCP SDK v1.4.1 confirmed. GLSP is MEDIUM — early release, may need patching for newer LSP features. |
| Features | HIGH | Competitor analysis is thorough (MATIEC, STruC++, RuSTy, CODESYS, TwinCAT). LLM integration features are MEDIUM — MCP and Claude Code skills are rapidly evolving. |
| Architecture | HIGH | Compiler pipeline architecture is well-established; validated against MATIEC, STruC++, RuSTy, Go compiler, and rust-analyzer. Phase ordering follows hard dependency graph. |
| Pitfalls | HIGH | All 10 pitfalls confirmed across multiple sources (MATIEC docs, K-ST paper, Agents4PLC paper, LSP study). Most pitfalls cite production evidence, not speculation. |

**Overall confidence:** HIGH

### Gaps to Address

- **GLSP maturity:** `tliron/glsp` was last updated March 2024. Monitor for activity; may need to patch or fork for LSP 3.18 features. Validate it handles the full set of LSP messages STC needs before committing.
- **TwinCAT `.TcPOU` format:** Not publicly documented. Will require hands-on inspection of exported project files during Phase 5 planning. Consider reaching out to Beckhoff developer community.
- **CODESYS project format:** Proprietary. PLCopen XML may be the only practical import path for CODESYS users without reverse-engineering the `.project` format.
- **Allen Bradley scoping:** AB emission is deferred to v2+, but understanding how much the portable subset must be constrained to be AB-compatible should be researched before finalizing the vendor profile system in Phase 5.
- **LSP performance baseline:** `tliron/glsp` performance on files >500 lines is unknown. Profile early in Phase 4 before committing to architecture.

## Sources

### Primary (HIGH confidence)
- [STruC++ GitHub](https://github.com/Autonomy-Logic/STruCpp) — ST-to-C++17 architecture validation, 1400+ test patterns, OOP support proof-of-concept
- [MATIEC Compiler](https://openplcproject.gitlab.io/matiec/) — two-pass type inference, grammar ambiguity documentation, stage pipeline validation
- [MCP Go SDK v1.4.1](https://github.com/modelcontextprotocol/go-sdk/releases) — official SDK, March 2026
- [Cobra v1.10.x](https://pkg.go.dev/github.com/spf13/cobra) — CLI framework, December 2025
- [rust-analyzer architecture](https://rust-analyzer.github.io/book/contributing/architecture.html) — LSP incremental analysis patterns
- [Go compiler README](https://go.dev/src/cmd/compile/README) — phase separation pattern
- [K-ST: Formal Executable Semantics for ST](https://cposkitt.github.io/files/publications/k-st_structured_text_tse23.pdf) — found 5 compiler bugs; standard library edge cases

### Secondary (MEDIUM confidence)
- [Agents4PLC paper](https://arxiv.org/html/2410.14209v1) — LLM failure modes with ST, agent-friendly tool design requirements
- [AutoPLC paper](https://arxiv.org/html/2412.02410v2) — multi-vendor ST generation demand validation
- [GLSP on GitHub](https://github.com/tliron/glsp) — Go LSP SDK, last updated March 2024
- [LSP Implementation Practices Study](https://peldszus.com/wp-content/uploads/2022/08/2022-models-lspstudy.pdf) — 13/28 servers re-parse on every change
- [RuSTy architecture](https://plc-lang.github.io/rusty/arch/architecture.html) — alternative compiler pipeline reference

### Tertiary (LOW confidence / needs validation)
- Vendor dialect compatibility claims (TwinCAT vs CODESYS 1% incompatibility) — anecdotal; validate with real production code in Phase 5
- Allen Bradley ST support scope — needs dedicated research phase before v2 planning

---
*Research completed: 2026-03-26*
*Ready for roadmap: yes*
