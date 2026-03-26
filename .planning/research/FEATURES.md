# Feature Research

**Domain:** IEC 61131-3 Structured Text Compiler Toolchain
**Researched:** 2026-03-26
**Confidence:** HIGH (core compiler/LSP features well-understood; LLM integration features MEDIUM -- rapidly evolving space)

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete or unusable.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **IEC 61131-3 Ed.3 Parser** | Minimum standard compliance; MATIEC only does Ed.2; RuSTy targets Ed.2. Must parse PROGRAM, FUNCTION, FUNCTION_BLOCK, TYPE, VAR sections, all ST statements | HIGH | Hand-written recursive descent for error recovery. Must handle CODESYS extensions (POINTER TO, REFERENCE TO, OOP, 64-bit). Ed.4 (2025) removed IL; ST is now the primary textual language |
| **CODESYS Extension Support** | Production ST code uses POINTER TO, REFERENCE TO, INTERFACE, METHOD, PROPERTY, EXTENDS, IMPLEMENTS. Ignoring these means unable to parse real-world code | HIGH | Beckhoff and Schneider both derive from CODESYS. OOP (interfaces, methods, properties, inheritance) is used heavily in modern ST codebases |
| **Type Checking** | Every compiler does this. Catching type errors before download to PLC is the minimum value proposition | HIGH | Must handle IEC type system: elementary types, derived types, structured types, enumerations, arrays, subranges, POINTER TO, REFERENCE TO, function block instances |
| **Semantic Analysis** | Variable resolution, scope checking, constant folding, unreachable code detection. CODESYS and TwinCAT both provide this | HIGH | Includes: undeclared variable detection, type compatibility checks, assignment validation, function call arity/type matching |
| **Clear Error Messages** | Engineers using this are not compiler engineers. Errors must point to exact location with actionable fix suggestions | MEDIUM | Source-mapped errors with line:column, context snippet, suggestion. JSON output for machine consumption |
| **IEC Standard Library** | TON, TOF, TP, CTU, CTD, CTUD, R_TRIG, F_TRIG, SR, RS, standard math/string/conversion functions. Every PLC environment ships these | MEDIUM | Must implement with correct IEC semantics. Timer blocks need deterministic time injection for testability |
| **CLI Interface** | Modern developer tools are CLI-first. Must compose in scripts and CI pipelines | LOW | Subcommands: parse, check, lint, format, test, emit, sim. Every command supports --format json |
| **Syntax Highlighting (VS Code)** | TextMate grammar for ST. Multiple VS Code extensions already exist (ControlForge, Serhioromano). Bare minimum for editor integration | LOW | TextMate grammar in JSON/YAML. Can bootstrap from existing open-source grammars, then extend for CODESYS extensions |
| **Multi-file Project Support** | Real PLC projects have hundreds of POUs across many files. Single-file-only tools are toys | MEDIUM | Project manifest (stc.toml or similar) defining source roots, vendor target, library paths. Dependency ordering and cross-file symbol resolution |
| **PLCopen XML Import** | IEC 61131-10 exchange format. Engineers export from TwinCAT/CODESYS and expect to import into tools | MEDIUM | Parse PLCopen XML schema, extract ST source from POUs, reconstruct project structure. Primarily an import path for adoption |
| **Vendor-Aware Diagnostics** | Flag code that won't compile on target vendor. "POINTER TO not supported on Allen Bradley" type warnings | MEDIUM | Vendor capability profiles that restrict analysis to vendor-supported subset. Critical for the "write once, deploy many" value prop |

### Differentiators (Competitive Advantage)

Features that set STC apart. No existing open-source ST tool does these well (or at all).

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Host-Based Unit Testing** | Run ST tests on your development machine without PLC hardware. No existing open-source tool does this. TcUnit and CfUnit require a PLC runtime. STC compiles to executable host artifacts that preserve scan-cycle semantics | HIGH | Transpile to C++ (or interpret), wrap in test harness with deterministic time, mock I/O injection. Assert library in ST syntax. JUnit XML output for CI. This is the killer feature -- the single biggest pain point in PLC development |
| **Vendor ST Re-emission** | Parse standard ST, emit vendor-flavored ST (TwinCAT, CODESYS/Schneider). Engineers paste output into vendor IDE. No round-trip headaches | HIGH | Vendor-specific emitters handle: naming conventions, pragma differences, type mapping, library call translation. AutoPLC research (2024) validates demand. Start with Beckhoff + Schneider (both CODESYS-derived, similar targets) |
| **LSP with Full Intelligence** | Go-to-definition, hover types, find references, rename, completions, diagnostics. CODESYS and TwinCAT have this in their IDEs but no open tool provides it for VS Code/Neovim/etc. | HIGH | Requires: incremental parsing, partial AST from broken code, symbol table, type info. The LSP is what makes STC feel like a "real" development environment rather than a batch compiler |
| **Closed-Loop Simulation** | Inject sensor values, model plant behavior, watch PLC logic respond. Simulate a conveyor, a valve, a temperature loop -- without hardware | VERY HIGH | Plant model interface (simple transfer functions, state machines). Sensor/actuator binding. Cycle-accurate execution. Visualization TBD (CLI tables first, web UI later) |
| **LLM Agent Integration (MCP Server)** | Expose parse, check, test, emit as MCP tools. LLM agents can write ST, validate it, run tests, fix errors -- in a loop. No other ST tool has this | MEDIUM | MCP server wrapping CLI commands. Tools: parse (AST), check (diagnostics), test (results), emit (vendor ST), lint (suggestions). JSON in/out. Minimal token footprint in responses |
| **Claude Code Skills for ST** | Pre-built skills for ST workflows: /st:generate (from natural language), /st:validate (parse+check+lint), /st:test (write and run tests), /st:emit (vendor output), /st:review (code review). Makes Claude Code a first-class ST development environment | MEDIUM | Markdown skill files in .claude/skills/. Each skill chains CLI commands. SKILL.md frontmatter controls auto-invocation. Supporting files provide ST patterns, IEC conventions, vendor-specific templates |
| **Formatter (stc fmt)** | Opinionated ST formatter like gofmt/prettier. STweep exists commercially but no open-source equivalent. Consistent code style across team, easier code review | MEDIUM | Configurable indent, alignment, casing conventions. Must handle comments correctly. Format-on-save via LSP. The existence of STweep (commercial, IDE-plugin only) validates demand |
| **Linter (stc lint)** | Static analysis rules beyond type checking. PLCopen coding guidelines, naming conventions, complexity warnings, unused variables, magic numbers | MEDIUM | CODESYS has 100+ static analysis rules as paid add-on. Open-source equivalent with configurable rule sets. Rules should be addable as plugins |
| **Incremental Compilation** | Only re-analyze changed files and their dependents. Fast feedback loop for large projects | HIGH | File-level dependency tracking, cached symbol tables, invalidation on change. Critical for LSP performance on large codebases |
| **Source-Level Debug Mapping** | Map generated code (C++ or vendor ST) back to original ST source lines. When debugging in TwinCAT, know which original line you're on | MEDIUM | Emit source maps or debug comments. For C++ output: #line directives. For vendor ST: comment annotations with original line numbers |
| **Preprocessor / Conditional Compilation** | Handle vendor-specific code paths in a single source file: {IF defined(TWINCAT)}...{END_IF}. CODESYS/TwinCAT support this; essential for multi-vendor codebases | MEDIUM | Must support CODESYS-style pragmas. Parse and resolve before main compilation. Different from C preprocessor -- PLC pragmas have their own syntax |
| **PLCopen XML Export** | Round-trip: import from vendor, modify, export back. Enables STC as part of a CI pipeline that checks in/checks out PLC code | MEDIUM | Generate valid PLCopen XML from AST. Preserve graphical layout metadata if imported. Less critical than import but enables full workflow |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems or violate project constraints.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Full PLC Runtime** | "Just run the PLC program on my PC" | Real-time guarantees impossible on general-purpose OS. I/O mapping is hardware-specific. Liability/safety concerns. Would compete with OpenPLC (which does this already) | Host-based test runner with simulated scan cycles. Preserves timing semantics without pretending to be a PLC |
| **GUI / Visual IDE** | "Engineers want visual tools" | Massive scope. Would compete with CODESYS (free IDE) and TwinCAT (free with VS integration). GUI development has nothing to do with compiler quality | CLI + LSP for VS Code. Engineers keep using their vendor IDE for visual work; STC augments their workflow |
| **Ladder Diagram / FBD Support** | "IEC 61131-3 has 5 languages" | Graphical languages need a graphical editor. Parsing LD/FBD from PLCopen XML is feasible but editing is not. Massive scope expansion | Import LD/FBD via PLCopen XML for analysis. Focus on ST as the textual language. SFC support only if it's ST-body SFC |
| **Allen Bradley Support in v1** | "AB is the biggest PLC vendor in North America" | Most restrictive ST dialect: no OOP, no POINTER TO, different tag model, AOI vs FB. Completely different ecosystem. Would distort the compiler architecture | Design the portable subset around AB limitations. Defer actual AB emission to v2. Vendor profiles can warn about AB-incompatible code now |
| **LLVM Backend in v1** | "Native compilation for performance" | Adds massive complexity (LLVM bindings in Go are painful). Transpile-to-C++ gets tests running months earlier. Performance is not the bottleneck for a development tool | Transpile to C++17 first. C++ compilers handle optimization. LLVM backend only if/when there's a concrete performance or deployment need |
| **Cloud IDE / SaaS Platform** | "Web-based development is the future" | Splits focus, requires infrastructure, authentication, session management. Industrial customers often have air-gapped networks | Local CLI + LSP. MCP server for AI integration. If someone wants a cloud IDE, they can wrap the CLI |
| **Automated PLC Deployment** | "Deploy directly to hardware from CI" | Safety-critical domain. Automated deployment to a PLC controlling physical equipment is a liability nightmare. Vendor-specific protocols (ADS, OPC UA download) are complex and proprietary | Emit vendor-flavored ST that engineers paste/import into vendor IDE. The human-in-the-loop for deployment is a feature, not a bug |
| **Real-time Co-simulation with Hardware** | "Connect to a real PLC for hybrid testing" | Requires vendor-specific communication protocols, real-time scheduling, hardware access. Fundamentally different product | Closed-loop simulation with software plant models. Hardware-in-the-loop testing is a separate tool category |

## Feature Dependencies

```
[Parser (lexer + grammar + AST)]
    |
    +--requires--> [CODESYS Extension Support]
    |                  (OOP, POINTER TO, REFERENCE TO in grammar)
    |
    +--enables--> [Type Checker]
    |                 |
    |                 +--enables--> [Semantic Analysis]
    |                 |                 |
    |                 |                 +--enables--> [Linter]
    |                 |                 |
    |                 |                 +--enables--> [Vendor-Aware Diagnostics]
    |                 |                 |
    |                 |                 +--enables--> [Vendor ST Re-emission]
    |                 |                                   |
    |                 |                                   +--enables--> [PLCopen XML Export]
    |                 |
    |                 +--enables--> [C++ Transpiler]
    |                                   |
    |                                   +--enables--> [Host-Based Unit Testing]
    |                                   |                 |
    |                                   |                 +--enables--> [Closed-Loop Simulation]
    |                                   |
    |                                   +--enables--> [Source-Level Debug Mapping]
    |
    +--enables--> [Formatter]
    |
    +--enables--> [LSP Server]
    |                 (requires partial AST / error recovery)
    |
    +--enables--> [PLCopen XML Import]

[IEC Standard Library]
    +--required-by--> [Type Checker] (must know standard types/FBs)
    +--required-by--> [Host-Based Unit Testing] (timers, counters need implementation)
    +--required-by--> [Vendor ST Re-emission] (map standard FBs to vendor equivalents)

[CLI Interface]
    +--wraps--> [All of the above as subcommands]
    +--enables--> [MCP Server] (wraps CLI tools as MCP tools)
                      |
                      +--enables--> [Claude Code Skills]

[Multi-file Project Support]
    +--required-by--> [Incremental Compilation]
    +--required-by--> [LSP Server] (for cross-file navigation)

[Preprocessor]
    +--required-by--> [Vendor ST Re-emission] (conditional vendor blocks)
    +--should-precede--> [Parser] (preprocessor runs before parse)
```

### Dependency Notes

- **Parser requires CODESYS Extensions:** Cannot parse production code without OOP/pointer support. Must be in grammar from day one, not bolted on later.
- **Type Checker requires IEC Standard Library definitions:** Must know signatures of TON, CTU, standard math functions to type-check code that uses them. Implementation can be stubs initially, but type signatures must exist.
- **Host-Based Unit Testing requires C++ Transpiler:** The test runner executes transpiled C++ on host. This is the critical path: parser -> type checker -> transpiler -> test runner.
- **MCP Server wraps CLI:** Thin wrapper exposing CLI tools as MCP tools. Low effort once CLI exists.
- **Claude Code Skills wrap MCP/CLI:** Markdown files chaining tool calls. Lowest effort once MCP server or CLI exists.
- **LSP Server requires error-recovering parser:** A parser that aborts on first error is useless for LSP. Error recovery must be designed into the parser from the start, not retrofitted.
- **Closed-Loop Simulation requires Host-Based Testing:** Simulation extends the test runner with plant model injection. Build testing first, simulation second.

## MVP Definition

### Launch With (v1.0)

Minimum viable product -- what's needed to validate the core value proposition ("write ST once, validate it instantly on your machine, deploy to any supported vendor").

- [ ] **IEC 61131-3 Parser with CODESYS extensions** -- cannot parse real code without this
- [ ] **Type checker and semantic analysis** -- catches errors without a PLC
- [ ] **IEC Standard Library (type signatures + basic implementation)** -- required for type checking and test execution
- [ ] **C++ transpiler for host execution** -- enables the killer feature (host testing)
- [ ] **Host-based unit test runner** -- the single biggest differentiator; JUnit XML for CI
- [ ] **CLI with core subcommands** (parse, check, test) -- composable tool interface
- [ ] **Clear error messages with JSON output** -- usable by humans and machines
- [ ] **Multi-file project support** -- real projects have many files
- [ ] **VS Code syntax highlighting** -- minimum editor experience

### Add After Validation (v1.x)

Features to add once core is working and validated with real-world ST codebases.

- [ ] **Vendor ST re-emission (Beckhoff, then Schneider)** -- trigger: when users confirm parser/checker handles their code
- [ ] **LSP server (diagnostics, go-to-def, hover, completions)** -- trigger: when parser and type checker are stable
- [ ] **Formatter (stc fmt)** -- trigger: when users start collaborating and code review matters
- [ ] **Linter (stc lint)** -- trigger: when basic analysis works and users want more checks
- [ ] **MCP server** -- trigger: when CLI interface is stable
- [ ] **Claude Code skills** -- trigger: when MCP server or CLI is usable
- [ ] **PLCopen XML import** -- trigger: when users want to bring existing vendor projects into STC
- [ ] **Vendor-aware diagnostics** -- trigger: when re-emission targets are defined
- [ ] **Preprocessor support** -- trigger: when multi-vendor emission requires conditional compilation

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Closed-loop simulation with plant modeling** -- needs test runner to be proven first; very high complexity
- [ ] **Allen Bradley emission** -- different dialect entirely; needs its own research phase
- [ ] **PLCopen XML export** -- lower priority than import; enables round-trip workflow
- [ ] **Incremental compilation** -- performance optimization; not needed until projects are large
- [ ] **Source-level debug mapping** -- nice-to-have; engineers debug in vendor IDE anyway
- [ ] **LLVM backend** -- only if C++ transpilation proves insufficient
- [ ] **SFC support** -- only ST-body SFC; graphical SFC is out of scope

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Parser (with CODESYS ext) | HIGH | HIGH | P1 |
| Type Checker | HIGH | HIGH | P1 |
| IEC Standard Library | HIGH | MEDIUM | P1 |
| C++ Transpiler | HIGH | HIGH | P1 |
| Host-Based Unit Testing | HIGH | HIGH | P1 |
| CLI Interface | HIGH | LOW | P1 |
| Error Messages (JSON) | HIGH | LOW | P1 |
| Multi-file Projects | HIGH | MEDIUM | P1 |
| VS Code Syntax Highlighting | MEDIUM | LOW | P1 |
| Vendor ST Re-emission | HIGH | HIGH | P2 |
| LSP Server | HIGH | HIGH | P2 |
| Formatter | MEDIUM | MEDIUM | P2 |
| Linter | MEDIUM | MEDIUM | P2 |
| MCP Server | MEDIUM | LOW | P2 |
| Claude Code Skills | MEDIUM | LOW | P2 |
| PLCopen XML Import | MEDIUM | MEDIUM | P2 |
| Vendor-Aware Diagnostics | MEDIUM | MEDIUM | P2 |
| Preprocessor | MEDIUM | MEDIUM | P2 |
| Closed-Loop Simulation | HIGH | VERY HIGH | P3 |
| PLCopen XML Export | LOW | MEDIUM | P3 |
| Incremental Compilation | MEDIUM | HIGH | P3 |
| Source Debug Mapping | LOW | MEDIUM | P3 |
| Allen Bradley Emission | MEDIUM | HIGH | P3 |

**Priority key:**
- P1: Must have for launch -- validates core value proposition
- P2: Should have, add when possible -- extends value, builds ecosystem
- P3: Nice to have, future consideration -- high effort or niche value

## Competitor Feature Analysis

| Feature | MATIEC / OpenPLC | STruC++ | RuSTy (PLC-lang) | CODESYS IDE | TwinCAT XAE | STC (Our Approach) |
|---------|-----------------|---------|-------------------|-------------|-------------|-------------------|
| IEC Standard | Ed.2 only | Ed.3 | Ed.2 target | Ed.3 + extensions | Ed.3 + extensions | Ed.3 + CODESYS extensions |
| Output | C (macro-heavy) | C++17 (clean) | LLVM IR / native | Vendor bytecode | Vendor bytecode | C++17 + vendor ST |
| Unit Testing | None built-in | Built-in runner | None built-in | CfUnit (on PLC) | TcUnit (on PLC) | Host-based, no PLC needed |
| LSP | None | None | None | Proprietary IDE | VS integration | Full LSP for VS Code |
| Formatter | None | None | None | IDE built-in | IDE built-in | CLI + LSP format |
| Linter | None | None | None | Static Analysis (paid) | Limited | CLI linter, PLCopen rules |
| Multi-vendor | No | No | No | CODESYS targets | Beckhoff only | Beckhoff + Schneider (v1) |
| LLM Integration | None | None | None | None | None | MCP server + Claude skills |
| License | LGPL | GPL-3.0 | LGPL-3.0 | Proprietary (free IDE) | Proprietary (free) | MIT |
| Language | C | TypeScript | Rust | N/A | N/A | Go |
| Error Recovery | Limited | Unknown | Unknown | Full (IDE) | Full (IDE) | Designed-in from start |
| PLCopen XML | No | No | No | Import/Export | Import/Export | Import (v1), Export (v2) |
| Simulation | No | No | No | Built-in (paid) | Built-in | Closed-loop (v2) |

### Key Competitive Gaps We Fill

1. **No open-source tool does host-based unit testing of ST.** TcUnit and CfUnit require a PLC runtime. This is our primary differentiator.
2. **No open-source tool has an LSP.** CODESYS and TwinCAT have IDE intelligence, but nothing for VS Code/Neovim users.
3. **No open-source tool does multi-vendor emission.** AutoPLC research validates demand but is LLM-based generation, not compiler-based transpilation.
4. **No ST tool has LLM/agent integration.** MCP server and Claude Code skills are entirely novel in this domain.
5. **MIT license.** MATIEC is LGPL, STruC++ is GPL-3.0, RuSTy is LGPL-3.0. MIT removes all barriers to adoption and embedding.

## Sources

- [IEC 61131-3:2025 (Ed.4)](https://webstore.iec.ch/en/publication/68533) -- standard update removing IL
- [MATIEC compiler](https://openplcproject.gitlab.io/matiec/) -- open-source Ed.2 compiler
- [STruC++ on GitHub](https://github.com/Autonomy-Logic/STruCpp) -- ST to C++17 compiler, GPL-3.0
- [RuSTy (PLC-lang)](https://github.com/PLC-lang/rusty) -- Rust/LLVM ST compiler
- [AutoPLC paper](https://arxiv.org/html/2412.02410v2) -- LLM-based vendor-aware ST generation
- [LLM4PLC paper](https://arxiv.org/html/2401.05443v1) -- LLM for verifiable PLC programming
- [Agents4PLC paper](https://arxiv.org/html/2410.14209v1) -- multi-agent PLC code generation
- [CODESYS Static Analysis](https://store.codesys.com/en/codesys-static-analysis.html) -- 100+ analysis rules
- [TcUnit](https://tcunit.org/) -- Beckhoff unit testing (requires PLC runtime)
- [CfUnit](https://forge.codesys.com/lib/counit/code/160/tree/landingpage/index.html) -- CODESYS unit testing (requires runtime)
- [UniTest](https://github.com/tkucic/UniTest) -- vendor-agnostic test library (still runs on PLC)
- [STweep](https://www.stweep.com/) -- commercial ST formatter
- [PLCopen XML standard](https://www.plcopen.org/standards/xml-echange/) -- IEC 61131-10 exchange format
- [CODESYS POINTER TO docs](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_datatype_pointer.html)
- [CODESYS REFERENCE TO docs](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_datatype_reference.html)
- [Claude Code Skills documentation](https://code.claude.com/docs/en/skills)
- [MCP Specification (2025-11-25)](https://modelcontextprotocol.io/specification/2025-11-25)
- [Structured Text PLCs VS Code Extension](https://marketplace.visualstudio.com/items?itemName=SmookCreative.StructuredText)

---
*Feature research for: IEC 61131-3 Structured Text Compiler Toolchain*
*Researched: 2026-03-26*
