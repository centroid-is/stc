# Pitfalls Research

**Domain:** IEC 61131-3 Structured Text Compiler Toolchain
**Researched:** 2026-03-26
**Confidence:** HIGH (compiler domain well-documented; IEC 61131-3 pitfalls confirmed across MATIEC, OpenPLC, STruC++ projects)

## Critical Pitfalls

### Pitfall 1: The IEC 61131-3 Grammar Is Not LALR(1)-Parseable As Specified

**What goes wrong:**
The IEC 61131-3 specification defines a grammar riddled with reduce/reduce and shift/reduce conflicts. Identifiers are ambiguous -- the parser cannot tell if `FOO` is a variable name, an enumerated value, a function name, a function block type, or a user-defined type without consulting a symbol table. Teams that try to implement the grammar directly from the specification hit a wall of conflicts. MATIEC's documentation explicitly states: "the syntax cannot be parsed by a LALR(1) parser as presented in the specification."

**Why it happens:**
The IEC standard was written for human understanding, not parser-generator consumption. The same identifier syntax appears in variable declarations, function calls, type references, and enumerated values with no syntactic disambiguation.

**How to avoid:**
Use a hand-written recursive descent parser (already decided in PROJECT.md). Build symbol tables during parsing, not as a separate pass. The parser must know whether an identifier refers to a type, a function, or a variable to parse correctly. This means a multi-pass or interleaved approach: scan declarations first (or resolve forward references lazily), then parse bodies. STruC++ and MATIEC both solve this with symbol-table-augmented parsing.

**Warning signs:**
- Parser accepting syntactically ambiguous constructs without errors
- Test cases where `TYPE foo` and `VAR foo` in the same scope produce wrong AST nodes
- Inability to parse production code that uses type names as identifiers in different scopes

**Phase to address:**
Phase 1 (Parser). Get this right from the start. Retrofitting symbol-table-aware parsing into a naive parser is effectively a rewrite.

---

### Pitfall 2: MATIEC's Edition 2 Trap -- Supporting Only Old Standard Features

**What goes wrong:**
MATIEC supports only IEC 61131-3 Edition 2 (2003). It lacks Edition 3 OOP features (INTERFACE, METHOD, EXTENDS, IMPLEMENTS, access modifiers, ABSTRACT, FINAL, OVERRIDE), REFERENCE TO, namespaces, and Edition 4 features (UTF-8 strings). Any compiler that starts with only Edition 2 features cannot parse real-world production code from Beckhoff/CODESYS environments, which heavily use OOP and REFERENCE TO. This is exactly what killed MATIEC's relevance for modern use cases.

**Why it happens:**
OOP in IEC 61131-3 is genuinely complex. It adds methods (with their own VAR sections), properties (GET/SET), inheritance hierarchies, interface tables, and virtual dispatch. Teams underestimate the scope and defer it, then discover that 60-80% of production TwinCAT/CODESYS code uses these features.

**How to avoid:**
Design the AST, type system, and symbol tables for OOP from day one, even if OOP type-checking is deferred. The parser must handle FUNCTION_BLOCK with EXTENDS, IMPLEMENTS, METHOD, PROPERTY from the first milestone. STruC++ demonstrates this is achievable -- it supports the full OOP surface including access modifiers. Parse early, type-check incrementally.

**Warning signs:**
- Parser tests only cover simple PROGRAM/FUNCTION/FUNCTION_BLOCK without methods
- AST node types have no place for methods, properties, or inheritance
- Test corpus excludes OOP-heavy production files
- "We'll add OOP later" appears in planning docs

**Phase to address:**
Phase 1 (Parser) for syntax. Phase 2 (Type System) for semantic checking. Do not defer OOP parsing to a later phase.

---

### Pitfall 3: The ANY Type Hierarchy and Overloaded Function Resolution

**What goes wrong:**
IEC 61131-3 standard functions like ADD, MUL, SEL, MUX operate on ANY_NUM, ANY_INT, ANY_REAL, etc. These are not concrete types -- they are type classes. Resolving which concrete function to call when given `ADD(myInt, myReal)` requires a multi-pass type inference algorithm. MATIEC implements this as a two-pass system: `fill_candidate_datatypes` (enumerate all possible types per expression node) then `narrow_candidate_datatypes` (resolve to a single type per node). Getting this wrong means either rejecting valid code or silently allowing type errors.

**Why it happens:**
The ANY type hierarchy creates a lattice of implicit conversions. An INT literal `42` could be SINT, INT, DINT, LINT, USINT, UINT, UDINT, ULINT, REAL, or LREAL. Combined with overloaded operators, the number of candidate combinations explodes. Implicit promotion rules (smaller to larger type) interact with explicit conversion functions (*_TO_*) in non-obvious ways. CODESYS/TwinCAT are more permissive than the standard about implicit conversions, so code that compiles on a vendor IDE may technically violate the spec.

**How to avoid:**
Implement MATIEC's proven two-pass approach: candidate enumeration then narrowing. Build the type lattice explicitly as a data structure with clear promotion rules. Test extensively with mixed-type expressions. Support a strict mode (IEC spec) and a permissive mode (CODESYS-compatible) for implicit conversions. Document every conversion rule with a test case.

**Warning signs:**
- Type checker rejects code that compiles in TwinCAT/CODESYS
- Literal types are hardcoded rather than inferred from context
- No test cases for mixed-type arithmetic (INT + REAL, DINT + LINT)
- Single-pass type resolution

**Phase to address:**
Phase 2 (Type System). This is the single hardest part of the semantic analysis. Budget at least 2x the time you think it needs.

---

### Pitfall 4: Parser Error Recovery That Produces Garbage ASTs

**What goes wrong:**
LSP requires parsing broken, mid-edit code and producing a usable partial AST. Naive panic-mode recovery (skip to next semicolon) produces ASTs with huge gaps that make completions, hover, and diagnostics useless. The AST becomes technically "partial" but practically worthless -- missing the exact context where the user is typing.

**Why it happens:**
Compiler-grade parsers are designed for valid input. Error recovery is bolted on as an afterthought. The fundamental tension: a compiler wants to reject bad code fast; an LSP wants to understand bad code deeply. These goals directly conflict and require different parser architectures.

**How to avoid:**
Design error recovery as a first-class concern from the start:
1. Use error nodes in the AST (not null/missing nodes, but explicit `ErrorNode` with the token range and partial children)
2. Implement statement-level recovery: when a statement fails, consume tokens to the next `;` or `END_*` keyword, wrap them in an ErrorNode, and continue
3. Implement expression-level recovery: when an expression fails mid-parse, return what you have so far
4. Track "recovery point" tokens: `;`, `END_IF`, `END_FOR`, `END_WHILE`, `END_FUNCTION`, `END_FUNCTION_BLOCK`, `END_PROGRAM`, `END_VAR`
5. Test with real editing scenarios: cursor in middle of expression, incomplete IF, missing END_*, unclosed parentheses

**Warning signs:**
- Parser returns empty AST for any syntax error
- No `ErrorNode` or equivalent in AST types
- Error recovery tests only cover missing semicolons
- LSP completions stop working when any error exists in file

**Phase to address:**
Phase 1 (Parser). Error recovery must be baked into the recursive descent parser from the beginning. Adding it later means rewriting every parsing function.

---

### Pitfall 5: Vendor Dialect Differences That Surface Late

**What goes wrong:**
TwinCAT and CODESYS are "99% compatible" but the 1% is devastating. Specific differences include:
- **Pragmas:** TwinCAT uses `{attribute 'qualified_only'}`, `{attribute 'strict'}`, conditional pragmas. CODESYS uses different pragma syntax for the same features.
- **File structure:** TwinCAT wraps ST in XML (`.TcPOU`, `.TcGVL`, `.TcDUT` files). CODESYS uses `.st` or its own project format.
- **POINTER TO behavior:** Online changes can move variables in memory, breaking pointers. REFERENCE TO has the same problem. Vendor-specific workarounds differ.
- **64-bit types:** LWORD, LINT, ULINT support varies. Some older CODESYS runtimes lack them.
- **String handling:** Max string lengths, WSTRING support, and encoding differ.
- **Property syntax:** Subtle differences in GET/SET property declarations.

**Why it happens:**
CODESYS is a platform licensed by 500+ vendors, each of whom extends it. TwinCAT forked from CODESYS v2 and diverged. Teams build against one vendor's dialect and discover incompatibilities only when they try the second vendor.

**How to avoid:**
Define a "vendor profile" system from the start. Each profile specifies:
- Supported pragmas and their syntax
- Type availability (64-bit types, WSTRING, etc.)
- Implicit conversion permissiveness
- File format expectations
Parse all pragmas as opaque pragma nodes initially; validate against profile during semantic analysis. Test against both TwinCAT and CODESYS production code from day one -- do not defer multi-vendor testing.

**Warning signs:**
- All test files are in one vendor's format
- Pragma handling is hardcoded rather than configurable
- No vendor profile/dialect configuration exists
- Team has only tested with TwinCAT OR CODESYS, not both

**Phase to address:**
Phase 1 (Parser) for pragma parsing. Phase 2 (Type System) for dialect-specific type rules. Phase 3 (Emit) for vendor-specific output. Test with both vendors continuously from Phase 1.

---

### Pitfall 6: Scan Cycle Semantics That Don't Match Real PLC Behavior

**What goes wrong:**
PLC programs execute in a cyclic scan model: read inputs, execute program top-to-bottom, write outputs, repeat. Timers (TON, TOF, TP) depend on the scan cycle elapsed time. If host simulation uses wall-clock time or ignores scan semantics, tests pass on host but fail on real PLCs. Specific failures:
- TON timers fire at wrong times because host runs faster/slower than PLC cycle time
- Variable initialization differs (PLC initializes once at download, not every scan)
- Output coercion (writing to outputs at end of scan, not immediately) is invisible on host
- RETAIN/PERSISTENT variables have no host equivalent without explicit simulation

**Why it happens:**
Developers think of ST as "just another programming language" and execute it like C code. But PLC execution is fundamentally different: it's a periodic real-time loop where time advances discretely per scan. STruC++ handles this with `ADVANCE_TIME` in tests, which is the correct approach.

**How to avoid:**
Build a scan cycle simulator that:
1. Advances time in discrete steps (configurable cycle time, default 10ms)
2. Calls all program/task instances once per cycle in correct priority order
3. Updates timer function blocks with elapsed time per cycle, not wall-clock time
4. Provides `ADVANCE_TIME` or `ADVANCE_CYCLES` primitives for tests
5. Captures output state only at end of scan, not mid-execution
6. Simulates RETAIN variables with explicit persistence layer

**Warning signs:**
- Timer tests use `time.Sleep()` or wall-clock time
- Tests call program functions directly without a scan loop wrapper
- No cycle time configuration exists
- Timer tests are flaky or platform-dependent

**Phase to address:**
Phase 3 (Host Execution/Testing). Must be designed correctly before any timer-dependent tests work. Critical for user trust.

---

### Pitfall 7: PLCopen XML as a Reliable Interchange Format

**What goes wrong:**
PLCopen XML defines a subset of what any vendor actually uses. Importing a TwinCAT project via PLCopen XML loses: task configurations (they get duplicated or mangled on re-import), library references (excluded entirely), vendor-specific pragmas, and proprietary function blocks. Schneider, Beckhoff, and CODESYS all support "extended" PLCopen XML with vendor-specific additions that other tools cannot read. Round-tripping (export then import) loses data.

**Why it happens:**
PLCopen XML was designed for exchange of program logic, not complete project interchange. Vendor IDEs store far more than just ST code -- they store hardware configurations, library bindings, task mappings, and deployment settings. Teams expect PLCopen XML to be "save/load" when it's actually "share logic snippets."

**How to avoid:**
1. Support PLCopen XML import for program logic (POUs, data types, global variables) only
2. Never promise round-trip fidelity -- document what is preserved and what is lost
3. Implement vendor-specific importers for TwinCAT `.TcPOU`/`.tsproj` and CODESYS `.project` formats separately
4. Use PLCopen XML as one import path, not the primary project format
5. Define your own canonical project format (JSON or TOML-based) for the toolchain

**Warning signs:**
- PLCopen XML is the only import/export format
- Test suite expects round-trip preservation of all metadata
- No documentation of what gets lost during import
- Vendor-specific file parsers are deferred indefinitely

**Phase to address:**
Phase 4 or later (Interop). PLCopen XML import is nice-to-have, not critical path. Direct `.TcPOU` parsing is more valuable for TwinCAT users.

---

### Pitfall 8: Making Tools Genuinely LLM-Agent-Friendly

**What goes wrong:**
Research shows that state-of-the-art LLMs (GPT-4, Claude, LLaMA) fail to produce valid IEC 61131-3 programs without tooling support. The Agents4PLC paper documents that even with multi-agent architectures, LLMs struggle with ST's unique syntax (`:=` assignment, `END_IF` blocks, VAR declarations, function block instantiation). Simply having a CLI is not enough -- agents need:
- Structured error messages that pinpoint exactly what's wrong (not "syntax error on line 42")
- Fast validation loops (compile in <1s for agent iteration)
- JSON output with machine-parseable diagnostics
- Example-driven prompting (the tool should be able to emit canonical examples)

**Why it happens:**
ST is a niche language with very little training data. LLMs confuse ST syntax with Pascal, Ada, or BASIC. Without tight compiler-in-the-loop feedback, agents generate plausible-looking but invalid code. Teams build the CLI for human users and bolt on `--format json` as an afterthought.

**How to avoid:**
Design the CLI for agents from day one:
1. Every error includes: file, line, column, error code, message, and a fix suggestion where possible
2. `--format json` is not optional -- it's tested in CI for every command
3. Provide a `stc check --stdin` mode for agent piping without temp files
4. Include a `stc examples` command that emits canonical ST patterns
5. Keep CLI surface small and consistent -- agents work best with few, predictable commands
6. Build MCP server early so Claude/agents can call tools directly
7. Validation must complete in under 1 second for interactive agent loops

**Warning signs:**
- Error messages are human-readable but not machine-parseable
- JSON output is untested or inconsistent across commands
- No stdin input mode
- Compile times exceed 2 seconds on typical files
- Agent testing is deferred to "later"

**Phase to address:**
Every phase. JSON output and fast validation from Phase 1. MCP server by Phase 3. Agent-specific testing throughout.

---

### Pitfall 9: Underestimating the Standard Library Surface Area

**What goes wrong:**
IEC 61131-3 defines 80+ standard functions and function blocks. STruC++ implements them as compiled ST with a header-only C++ runtime. But the devil is in the details:
- String functions (FIND, INSERT, DELETE, REPLACE, MID, LEFT, RIGHT) have edge cases around empty strings, out-of-range indices, and max string length
- Timer FBs (TON, TOF, TP) have specific edge cases around elapsed time rollover, re-triggering while active, and Q output behavior at exact boundary conditions
- Type conversion functions (*_TO_*) have 100+ combinations with rounding, truncation, and overflow rules that differ subtly between vendors
- Math functions (EXPT, LOG, LN, SQRT) must handle edge cases (negative inputs, zero, overflow) per the standard

**Why it happens:**
Teams implement the "happy path" for each function and move on. Production code exercises every edge case. Timer FBs alone have 15+ documented behavioral edge cases in the specification.

**How to avoid:**
1. Implement standard library functions in ST where possible (self-hosting validates the compiler)
2. Write property-based tests for type conversions (test all 100+ *_TO_* combinations)
3. Use K-ST formal semantics research as a reference for edge case behavior
4. Compare behavior against TwinCAT and CODESYS simulators for every function
5. Start with the 20 most-used functions (ADD, SUB, MUL, DIV, MOD, TON, TOF, TP, CTU, CTD, R_TRIG, F_TRIG, AND, OR, XOR, NOT, SEL, MUX, MOVE, basic string ops)

**Warning signs:**
- Standard library functions only have 2-3 test cases each
- Timer tests don't cover re-trigger, elapsed rollover, or boundary conditions
- Type conversion tests only cover happy-path conversions
- No cross-vendor behavioral comparison

**Phase to address:**
Phase 3 (Host Execution/Testing) for initial implementation. Ongoing refinement through all later phases.

---

### Pitfall 10: LSP Performance Death by Full Re-Parse

**What goes wrong:**
On every keystroke, the LSP server receives a textDocument/didChange notification. If the server re-parses the entire file (and all its dependencies) on every change, latency spikes to 200ms+ for large files (1000+ lines of ST, common in production). Users perceive this as "laggy" and abandon the tool. Worse, if semantic analysis (type checking, reference resolution) also runs on every keystroke, it can take 500ms+.

**Why it happens:**
The compiler is designed for batch processing: parse entire file, analyze, report errors. LSP demands incremental processing: update the part that changed, re-analyze only affected scopes. These are fundamentally different architectures. Research shows 13 of 28 surveyed LSP servers re-parse on every change; only the performant ones use incremental approaches.

**How to avoid:**
1. Use incremental document sync (LSP supports sending only changed ranges, not full file content)
2. Parse at statement/declaration granularity -- when a change occurs within a function body, only re-parse that function
3. Cache symbol tables and type information per-scope; invalidate only affected scopes on change
4. Debounce semantic analysis (run type checking 200ms after last keystroke, not on every keystroke)
5. Separate fast operations (syntax highlighting, bracket matching) from slow operations (type checking, reference resolution)
6. Profile early -- if parse takes >50ms on a 1000-line file, investigate before it gets worse

**Warning signs:**
- LSP re-parses entire file on every didChange notification
- No caching of parse results or symbol tables
- Completion requests take >100ms
- Performance has never been measured on files >200 lines
- No debouncing of expensive operations

**Phase to address:**
Phase 1 (Parser) for incremental-friendly AST design. Phase 4 (LSP) for implementation. Design for incrementality from the start even if LSP comes later.

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip OOP in parser | Faster initial parser delivery | Cannot parse production code; requires parser rewrite to add | Never -- parse OOP syntax from day one |
| Hardcode type promotion rules | Simpler type checker | Cannot support vendor-specific conversion permissiveness | Never -- build type lattice as data, not code |
| Use wall-clock time in tests | Timer tests "work" on host | Flaky tests, non-deterministic failures, wrong timer behavior | Never -- use simulated time from first timer test |
| Single-vendor test corpus | Faster test development | Vendor-specific assumptions baked into parser/analyzer | Only in first 2 weeks of development |
| String-based error messages | Faster error reporting | Cannot machine-parse errors; agent integration breaks | Only for first prototype; add structured errors within 1 month |
| Parse entire file for LSP | Simpler LSP implementation | Unusable latency on production files | Only for initial LSP demo; must fix before public release |
| PLCopen XML as primary format | Standards-compliant interchange | Lossy round-trips, missing vendor metadata | Never as primary -- use as one import option |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| TwinCAT `.TcPOU` files | Treating them as plain ST | They are XML-wrapped ST; parse the XML envelope first, extract ST body |
| CODESYS project files | Expecting PLCopen XML compatibility | CODESYS uses its own binary/XML project format; PLCopen XML export loses data |
| VS Code extension | Bundling the Go binary inside the extension | Ship Go binary separately; extension spawns it as a child process via LSP |
| CI/CD pipelines | Requiring vendor IDE for validation | Toolchain must validate independently; JUnit XML output for CI integration |
| MCP server | Exposing every CLI command as a tool | Curate a small tool surface; agents need validate, format, and emit -- not 30 commands |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full file re-parse on each edit | LSP latency >200ms, CPU spikes during typing | Incremental parsing, scope-level invalidation | Files >500 lines |
| Unbounded type candidate explosion | Type checker hangs on deeply nested expressions | Limit candidate set size, prune early in narrowing pass | Expressions with 5+ mixed-type operands |
| Naive symbol table lookup | Slow completions, go-to-definition lag | Hash-based symbol tables with scope chains | Projects with 100+ POUs and global variables |
| Synchronous semantic analysis in LSP | Editor freezes during type checking | Async analysis with cancellation on new edits | Any file with type errors |
| Loading all project files at LSP start | 5-10s startup delay, high memory | Lazy loading -- parse files on first reference | Projects with 50+ files |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Cryptic error messages ("unexpected token") | Engineers waste time guessing what's wrong | Include expected tokens, context, and suggestion: "Expected `:=` after variable name, found `=`. Did you mean `:=`?" |
| No location info in errors | Cannot find the problem in large files | Always include file:line:column, even for semantic errors |
| Reporting only first error | User fixes one error, gets another, repeats 20 times | Report all errors per scope/function; stop at 50 to avoid noise |
| Different behavior than vendor IDE | Engineers distrust the tool | Document every intentional difference; provide `--strict` vs `--vendor` modes |
| Silent acceptance of vendor extensions | Code works in STC but fails in target PLC | Warn on non-standard features; show which vendors support each extension |

## "Looks Done But Isn't" Checklist

- [ ] **Parser:** Handles nested comments (`(* (* inner *) outer *)`) -- many parsers fail on these
- [ ] **Parser:** Handles multi-line string literals with embedded quotes correctly
- [ ] **Parser:** Handles CASE with ranges (`1..10:`) and comma-separated values (`1, 3, 5:`)
- [ ] **Type System:** Handles ANY_NUM resolution when literals and variables are mixed in one expression
- [ ] **Type System:** Handles ARRAY indexing with expressions (not just constants)
- [ ] **Type System:** Handles STRUCT member access chained with method calls (`myFB.myStruct.member`)
- [ ] **Timers:** TON.Q stays TRUE after ET reaches PT (does not pulse)
- [ ] **Timers:** TON resets correctly when IN goes FALSE while timer is running
- [ ] **Timers:** TOF behavior when IN toggles faster than PT
- [ ] **Standard Library:** MID/LEFT/RIGHT with length exceeding string length (vendor behavior varies)
- [ ] **PLCopen XML:** Handles vendor-extended XML namespaces without crashing
- [ ] **Emit:** Generated vendor ST compiles without modification in target IDE
- [ ] **LSP:** Works correctly with unsaved/modified files (dirty buffer content, not disk content)
- [ ] **LSP:** Handles files with BOM (TwinCAT exports with UTF-8 BOM)

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| No OOP in parser | HIGH | Redesign AST node types, add METHOD/PROPERTY/INTERFACE nodes, update all visitors and transformations |
| Wrong type inference approach | HIGH | Replace type checker with two-pass candidate/narrow system; affects all semantic analysis |
| Wall-clock timer tests | MEDIUM | Add simulated time layer, update all timer tests to use ADVANCE_TIME, audit for remaining non-determinism |
| Single-vendor assumptions | MEDIUM | Add vendor profile system, audit all hardcoded assumptions, add second vendor's test corpus |
| Non-incremental LSP | MEDIUM | Add scope-level caching, change parser to support partial re-parse, add debouncing |
| Lossy PLCopen XML round-trip | LOW | Document limitations clearly; implement vendor-specific importers as alternative |
| Poor error messages | LOW | Add structured error type with code, location, message, suggestion; update error emitters incrementally |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Grammar not LALR(1) | Phase 1 (Parser) | Parser handles identifier ambiguity via symbol tables; passes MATIEC-style test suite |
| Edition 2 only trap | Phase 1 (Parser) | Parser successfully parses OOP-heavy production files from TwinCAT/CODESYS |
| ANY type resolution | Phase 2 (Type System) | Two-pass type inference passes; mixed-type expression tests all green |
| Bad error recovery | Phase 1 (Parser) | Partial AST tests: broken code still produces usable AST for LSP completions |
| Vendor dialect drift | Phase 1-3 (All) | Both TwinCAT and CODESYS test corpora pass continuously |
| Wrong scan cycle semantics | Phase 3 (Execution) | Timer tests use simulated time; deterministic across platforms |
| PLCopen XML expectations | Phase 4+ (Interop) | Import/export tests document and verify exactly what is preserved vs lost |
| Agent-unfriendly tools | Phase 1+ (All) | JSON output tested in CI; agent integration tests from Phase 2 |
| Standard library gaps | Phase 3 (Execution) | Property-based tests for conversions; cross-vendor behavioral comparison for timers |
| LSP performance | Phase 1 + Phase 4 | Incremental AST design in Phase 1; <100ms completion latency measured on 1000-line files |

## Sources

- [MATIEC Compiler Documentation and Architecture](https://openplcproject.gitlab.io/matiec/)
- [MATIEC Source -- Type Narrowing (stage3)](https://github.com/nucleron/matiec)
- [STruC++ Compiler](https://github.com/Autonomy-Logic/STruCpp) -- validates OOP support, test framework design, and C++17 transpilation approach
- [K-ST: Formal Executable Semantics for Structured Text](https://cposkitt.github.io/files/publications/k-st_structured_text_tse23.pdf) -- found 5 bugs and 9 functional defects in OpenPLC compiler
- [Agents4PLC: LLM-based PLC Code Generation](https://arxiv.org/html/2410.14209v1) -- documents LLM failure modes with ST
- [Multi-Agent Framework for ST Generation](https://arxiv.org/html/2412.02410v1/) -- vendor fragmentation challenges
- [Training LLMs for IEC 61131-3 ST](https://arxiv.org/html/2410.22159v3) -- limited training data, compilation success rates
- [PLCopen XML Exchange Standard](https://www.plcopen.org/standards/xml-echange/)
- [CODESYS PLCopen XML Import Issues](https://forge.codesys.com/forge/talk/Engineering/thread/c3f728a1ad/)
- [Beckhoff TwinCAT PLCopen XML Export Documentation](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2526208651.html)
- [TwinCAT Attribute Pragmas](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2529567115.html)
- [IEC 61131-3 Type Conversion Functions (Fernhill)](https://www.fernhillsoftware.com/help/iec-61131/common-elements/conversion-functions/type-casts.html)
- [IEC 61131-3 OOP Extensions Research](https://www.researchgate.net/publication/224089615_Object-oriented_extensions_for_IEC_61131-3)
- [Incremental Packrat Parsing for Fast Language Servers](https://unallocated.com/blog/incremental-packrat-parsing-the-secret-to-fast-language-servers/)
- [LSP Implementation Practices Study](https://peldszus.com/wp-content/uploads/2022/08/2022-models-lspstudy.pdf)

---
*Pitfalls research for: IEC 61131-3 Structured Text Compiler Toolchain*
*Researched: 2026-03-26*
