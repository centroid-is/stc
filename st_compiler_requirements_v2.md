# IEC 61131-3 Structured Text Toolchain — Requirements & Strategy v2

## Complete Requirements

### R1 — Multi-vendor ST compatibility
Write ST once, deploy to Schneider (M340/M580/M241), Beckhoff (TwinCAT 3), and later Allen Bradley (ControlLogix/CompactLogix via Studio 5000). Vendor selection via ifdef-style conditional compilation. Portable common library with vendor-specific adapters for I/O, pragmas, and system calls.

### R2 — Runnable host artifacts
Compile ST into native executables that run on Linux and Windows without a PLC. These artifacts must preserve PLC scan-cycle semantics — read inputs, execute logic, write outputs, repeat.

### R3 — Unit testing on host
Write tests in ST (or a thin DSL alongside ST). Run them in CI (GitHub Actions, GitLab CI). Tests must support mocking I/O, advancing time deterministically, and asserting on outputs. Produce JUnit XML for standard CI integration.

### R4 — Simulation
Run closed-loop simulations on host: inject sensor waveforms, simulate plant behavior (motor, valve, cylinder), verify control logic over time. Deterministic replay for regression testing.

### R5 — LSP and syntax highlighting in VS Code
Syntax highlighting for `.st` files. Language server providing diagnostics, go-to-definition, hover, completion, rename. Must understand vendor ifdef directives (gray out inactive blocks, vendor-aware diagnostics).

### R6 — LLM agent friendliness
The toolchain must be usable by LLM agents (Claude Code, Codex, etc.) to generate and validate ST code with minimal token overhead. This means: clear CLI interface (`stc build`, `stc test`, `stc emit`), machine-readable output (JSON diagnostics, JUnit XML), deterministic behavior, and small context footprint for tool descriptions.

### R7 — No Java runtime dependency
Parser and compiler must not require Java at runtime. Build-time Java for ANTLR grammar generation is acceptable.

### R8 — Machine-verifiable phase gates
Every development phase must have completion criteria that an LLM agent can verify by running commands and checking outputs.

### R9 — Vendor ST re-emission
Emit vendor-flavored ST from the internal representation, so the toolchain can round-trip: parse → analyze → emit back to Beckhoff/Schneider/AB-compatible source.

### R10 — Incremental adoption
Must be usable alongside existing vendor IDEs. Engineers paste generated ST into TwinCAT, Unity Pro, Studio 5000. The toolchain augments their workflow; it does not replace their IDE.

---

## Requirements you likely missed

### R11 — PLCopen XML import/export
All three vendors (Beckhoff, Schneider, AB) can import/export via PLCopen XML. This is the most realistic interop format. STruC++ already has an `xml2st` tool for this. Your toolchain should parse and emit PLCopen XML, not just raw `.st` files.

### R12 — Standard library coverage
IEC 61131-3 defines standard FBs: TON, TOF, TP, CTU, CTD, CTUD, R_TRIG, F_TRIG, SR, RS, plus standard functions (ADD, MUL, ABS, SQRT, MIN, MAX, SEL, MUX, etc.). Your toolchain must implement all of these in the host runtime, with behavior matching the IEC spec. Without this, tests are meaningless.

### R13 — CODESYS extension compatibility
Both Beckhoff and Schneider are CODESYS-derived. Real-world ST code uses CODESYS extensions: POINTER TO, REFERENCE TO, 64-bit types (LINT/LREAL/LWORD), typed literals, OOP (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS). Your parser must handle these or you cannot parse production code.

### R14 — Source-level debugging info
When tests fail, engineers need to see which line of ST caused the failure, not which line of generated C++. Source maps from ST → generated code are essential for usability.

### R15 — Deterministic cross-platform behavior
REAL/LREAL arithmetic must produce identical results on host and PLC. TIME overflow, integer promotion, and FB initialization order differ between vendors. These differences must be documented and, where possible, configurable per vendor target.

---

## What exists and what to reuse

### Immediate value — install today

| Tool | What it gives you | Effort |
|------|-------------------|--------|
| `Serhioromano/vscode-st` extension | Syntax highlighting, snippets, keyword formatting for `.st` in VS Code | Install from marketplace, zero effort |
| `ControlForge Structured Text` extension | LSP with diagnostics, go-to-def, hover, FB member completion | Install from marketplace, zero effort |

### Strong reuse candidates

| Project | What to take from it | License | Language |
|---------|---------------------|---------|----------|
| **STruC++** | ST→C++17 compiler with Ed.3 + CODESYS extensions, built-in test runner, REPL, OSCAT library support. Validates the entire architecture you want to build. Study its parser, its C++ code generation patterns, its test DSL design. Consider forking or wrapping. | GPL-3.0 (runtime exception) | TypeScript |
| **STruC++ xml2st** | PLCopen XML → ST converter. Solves the vendor import problem. | GPL-3.0 | Python |
| **ControlForge ST extension** | Working LSP for ST with diagnostics, symbol indexing. Study its architecture for your own LSP, or fork it and connect to your compiler backend. | Check repo | TypeScript |
| **Serhioromano/vscode-st** | Mature TextMate grammar for ST syntax highlighting. Fork this grammar as your base — it handles SFC, namespaces, typed constants, vendor pragmas. | MIT | TypeScript |
| **tree-sitter-structured-text** | Tree-sitter grammar for ST. Useful for editor-side incremental parsing if you build a Neovim/Helix integration later. | Check repo | JS/C |
| **Zeugwerk/Plaincat** | TwinCAT XML ↔ plain text ST converter. Directly relevant to your Beckhoff workflow — extract ST from `.plcproj` and edit in VS Code. | Check repo | C# |
| **MATIEC** | Proven ST→C compiler used by OpenPLC for decades. Reference for IEC compliance edge cases. Parser is well-tested but only supports Ed.2, no OOP. | LGPL | C |
| **K-ST** | Formal executable semantics for ST. Use as conformance reference for your runtime model — it found real bugs in OpenPLC. | Academic | K framework |
| **TcUnit** | xUnit framework for TwinCAT. Study its assertion API design (`ASSERT_EQ`, etc.) for your test DSL. | MIT | ST |

### Key learning: STruC++ architecture

STruC++ is the closest thing to what you want to build. It's written in TypeScript, compiles to C++17, and already has 1400+ tests. Its architecture is:

```
ST source → Parser (TypeScript) → AST → Type checker → C++17 emitter → g++/clang++ → executable
                                      → Test runner (built-in TEST blocks, ASSERT_EQ, ADVANCE_TIME)
                                      → REPL (step through scan cycles, inspect variables)
```

What STruC++ does NOT have that you need:
- Vendor ifdef preprocessor
- Multi-vendor ST re-emission (it only emits C++, not vendor-flavored ST)
- Allen Bradley support
- Your specific HAL architecture (portable logic / vendor adapter / test harness layers)
- Integration with your centroid-hmi MCP tools

---

## The "quick tools" strategy — fastest path to value

You asked: "You can quickly create tools that can be used to speed up development of structured text."

Here is the priority order, from fastest-to-ship to most ambitious:

### Tool 1: `stc-lint` — ST linter (days, not weeks)
A standalone linter that reads `.st` files and reports common problems:
- Undeclared variables
- Type mismatches (BOOL assigned to INT)
- Missing END_IF / END_FOR
- Unused variables
- Naming convention violations (configurable)
- Vendor-specific construct warnings ("This uses Beckhoff PROPERTY — not portable")

This is a parser + basic semantic analysis. No code generation needed. Immediately useful in CI. An LLM agent can run `stc-lint *.st --format json` and get structured diagnostics.

**Implementation**: Hand-written recursive descent parser in C++ (or use ANTLR4 C++ target). Output JSON diagnostics. Wire it into a VS Code extension as a custom LSP diagnostic provider.

### Tool 2: `stc-fmt` — ST formatter (days)
Auto-format ST code: consistent indentation, keyword capitalization, spacing around `:=`. The vscode-st extension has basic formatting; a standalone tool enables CI enforcement.

**Implementation**: Reuse the parser from Tool 1. Emit formatted ST from the AST.

### Tool 3: `stc-pp` — Preprocessor (1 week)
The `{$if VENDOR_BECKHOFF}` preprocessor. Reads ST with directives, emits vendor-specific ST. Maintains source maps. This is the foundation of your multi-vendor strategy.

**Implementation**: Simple line-by-line processor. Doesn't even need a full parser — just directive recognition and block tracking.

### Tool 4: `stc-check` — Type checker and semantic analysis (2-3 weeks)
Full type checking, POU resolution, FB instance tracking. Feeds the LSP for go-to-definition, hover, references. This is the expensive piece but also the highest-value one.

### Tool 5: `stc-test` — Host test runner (2-4 weeks)
Requires: parser + type checker + either a reference interpreter or a C++ code generator.
Two viable approaches:
- **Interpreter**: Walk the AST and execute. Slow but correct. Good enough for tests.
- **Transpile to C++**: Emit C++17 (like STruC++), compile with g++, run. Fast execution.

The transpile approach is not slow in any practical sense. Parsing + emitting C++ text takes milliseconds. The g++ compilation takes 1-3 seconds for typical PLC-sized projects. Your test suite runs in under 10 seconds total. That's faster than deploying to a real PLC.

If you want direct LLVM later, the path is: AST → your typed IR → LLVM IR via C++ LLVM API. But start with transpiling to C++ — it gets you running tests months earlier.

### Tool 6: Custom LSP (ongoing)
Wire your parser + type checker (Tools 1 & 4) into an LSP server. The extension side is a thin TypeScript wrapper. The LSP server can be a C++ binary that speaks JSON-RPC over stdio.

---

## LLM agent interface design

This is your most unique requirement and deserves careful design. The goal: an LLM agent like Claude Code should be able to:

1. **Generate ST code** — produce valid, vendor-portable ST with correct syntax
2. **Validate it** — run `stc-lint` and `stc-check` to verify without token-heavy output
3. **Test it** — run `stc-test` and read pass/fail results
4. **Understand the project** — read a compact project manifest, not thousands of ST files

Design principles for LLM-friendliness:
- **JSON-in, JSON-out**: All CLI tools accept `--format json` for structured output
- **Short error messages**: Diagnostics include file:line:col and a one-line message
- **Manifest file**: A `stc.toml` or `stc.yaml` at project root listing all POUs, their vendor targets, and test suites — the agent reads this instead of scanning directories
- **Template library**: Ship standard FB templates (motor control, valve control, PID wrapper) that the agent can instantiate and customize
- **MCP server**: Expose `stc-lint`, `stc-check`, `stc-test` as MCP tools so Claude can call them directly without shell commands — this is directly compatible with your centroid-hmi MCP architecture

Example MCP tool definitions (minimal token footprint):
```json
{
  "name": "stc_lint",
  "description": "Lint ST files. Returns JSON array of diagnostics.",
  "parameters": { "files": ["string"], "vendor": "beckhoff|schneider|ab|portable" }
}
{
  "name": "stc_test",
  "description": "Run ST unit tests. Returns JUnit XML summary.",
  "parameters": { "test_dir": "string", "vendor": "string" }
}
{
  "name": "stc_emit",
  "description": "Emit vendor-specific ST from portable source.",
  "parameters": { "file": "string", "target": "beckhoff|schneider|ab" }
}
```

---

## Language choice recommendation

Given your preference for C++ and requirement for LLVM compatibility:

**C++ for the compiler core** (parser, type checker, IR, code generation). This aligns with LLVM/MLIR if you go that route later, and avoids FFI boundaries.

**TypeScript for the VS Code extension** (this is mandatory — VS Code extensions are JS/TS).

**C++ for the LSP server** (runs as a subprocess, communicates via JSON-RPC stdio with the TS extension).

Parser approach: ANTLR4 with C++ target (grammar in `.g4`, run Java generator once at build time, get C++ parser/lexer source), OR hand-written recursive descent. Hand-written is more work initially but gives you full control over error recovery, which matters enormously for LSP (you need partial ASTs from broken code).

---

## Vendor compatibility matrix (key differences)

| Feature | Beckhoff (TwinCAT 3) | Schneider (Unity/Machine Expert) | Allen Bradley (Studio 5000) |
|---------|---------------------|--------------------------------|---------------------------|
| Base language | CODESYS-derived ST | CODESYS-derived ST | IEC 61131-3 subset |
| OOP | Full (METHOD, INTERFACE, PROPERTY, EXTENDS) | Partial (depends on controller) | None (no OOP in ST) |
| Pragmas | `{attribute '...'}` | `{...}` pragmas | Not applicable |
| String handling | STRING(255) default | STRING(16) default on some | STRING[82] default |
| Timer resolution | 100μs typical | 1ms typical | 1ms typical |
| I/O mapping | AT %I*, AT %Q* or TwinCAT linking | AT %I*, AT %Q* | Tags (no AT syntax) |
| Project format | `.plcproj` XML | `.xef` / PLCopen XML | `.L5X` / `.L5K` |
| Import format | PLCopen XML, `.plcproj` | PLCopen XML, `.xef` | PLCopen XML (limited), `.L5X` |

Allen Bradley is the hardest target because it has no OOP, uses a different tag/variable model, and its ST dialect is the most restrictive. Plan your portable subset around AB's limitations.

---

## Revised phase plan (realistic for 1 person)

### Phase 0 — Setup & existing tools (Week 1)
- Install vscode-st and ControlForge extensions
- Install STruC++ and run it against your sanitized corpus
- Set up monorepo: `stc/` (compiler), `stc-lsp/` (LSP), `stc-vscode/` (extension), `stdlib/`, `tests/`, `vendor/`
- Write `docs/portable-subset.md` and `docs/vendor-matrix.md`
- Gate: `stc build` skeleton compiles, vendor matrix has ≥20 features

### Phase 1 — Preprocessor (Week 2)
- `stc-pp` crate/binary
- `{$if}`, `{$elif}`, `{$else}`, `{$endif}`, `{$define}`, `{$error}`
- Source map emission (`.stmap.json`)
- Gate: `stc-pp motor.st --define VENDOR_BECKHOFF` and `--define VENDOR_SCHNEIDER` produce different, valid outputs

### Phase 2 — Parser (Weeks 3-5)
- Hand-written recursive descent or ANTLR4 C++ target
- Parse portable subset: PROGRAM, FUNCTION_BLOCK, FUNCTION, IF/CASE/FOR/WHILE, basic types, arrays, structs, enums
- Parse CODESYS extensions: METHOD, INTERFACE, PROPERTY, EXTENDS
- Error recovery for LSP (produce partial AST from broken code)
- Gate: Parse ≥95% of sanitized corpus. Zero panics. JSON diagnostic output for failures.

### Phase 3 — Type checker (Weeks 6-8)
- Symbol tables, type resolution, POU signatures
- FB instance tracking, overload resolution
- Vendor-aware diagnostics (warn on non-portable constructs)
- Gate: `stc-check *.st --format json` produces diagnostics. Type errors caught on test corpus.

### Phase 4 — LSP & VS Code extension (Weeks 7-9, overlaps Phase 3)
- C++ LSP server speaking JSON-RPC over stdio
- Fork vscode-st TextMate grammar, add TS extension wrapper
- Diagnostics, go-to-def, hover, completion, rename
- Preprocessor awareness (gray out inactive vendor blocks)
- Gate: Extension installs in VS Code. Diagnostics appear on save. Go-to-def works.

### Phase 5 — Runtime model & interpreter (Weeks 9-12)
- Define scan-cycle semantics: read inputs → execute → write outputs
- Implement reference interpreter in C++ (walk the typed AST)
- Standard library: TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, SR, RS
- Deterministic time advancement
- Gate: Interpreter passes conformance suite for timers, counters, edge detection. Traces are deterministic.

### Phase 6 — Test runner (Weeks 11-13, overlaps Phase 5)
- TEST_CASE / ASSERT_EQ / ASSERT_TRUE / ASSERT_NEAR DSL
- `stc test` CLI with JUnit XML output
- I/O mocking, time mocking
- Gate: `stc test tests/ --format junit` runs 20+ tests. Non-zero exit on failure.

### Phase 7 — Vendor ST emitters (Weeks 13-15)
- AST → Beckhoff-flavored ST text
- AST → Schneider-flavored ST text
- Pragma/attribute adaptation
- PLCopen XML export
- Gate: Round-trip parse → emit → parse stable for portable subset.

### Phase 8 — MCP server for LLM agents (Week 14)
- Expose `stc-lint`, `stc-check`, `stc-test`, `stc-emit` as MCP tools
- JSON schema for all inputs/outputs
- Gate: Claude Code can call MCP tools, generate ST, and validate it.

### Later: LLVM backend, Allen Bradley, simulation, property testing

---

## Bottom line

You don't need to build everything from scratch. The ecosystem has matured significantly:
- STruC++ proves the ST→C++→host architecture works, has 1400+ tests, and handles CODESYS extensions
- Existing VS Code extensions give you syntax highlighting and basic LSP today
- PLCopen XML is the realistic vendor interchange format
- The fastest path to value is a series of small CLI tools (lint, format, preprocess, check, test) that compose together and that LLM agents can call

Start with the preprocessor (your unique value-add), then the parser, then the type checker. Wire each into the LSP as you go. Defer LLVM until you have a working interpreter and test runner — transpiling to C++ is fast enough and gets you running tests months earlier.
