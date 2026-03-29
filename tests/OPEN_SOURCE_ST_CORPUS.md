# Open-Source IEC 61131-3 Structured Text Corpus Sources

Compiled: 2026-03-29

This document catalogs open-source repositories containing IEC 61131-3 Structured Text (ST)
code suitable for use as a test corpus for the `stc` compiler.

---

## License Compatibility Key

- **OK** = MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause, public domain (compatible with our use)
- **CAUTION** = LGPL (usable as test data, not linkable)
- **COPYLEFT** = GPL-2.0, GPL-3.0 (usable as test input only, cannot incorporate into our codebase)
- **UNKNOWN** = No license specified (risky; do not redistribute)

---

## Tier 1: ST Compiler/Parser Test Suites (Best for compiler testing)

### 1. PLC-lang/rusty (RuSTy)
- **URL:** https://github.com/PLC-lang/rusty
- **License:** LGPL-3.0 / GPL-3.0 (COPYLEFT)
- **Content:** ST-to-LLVM compiler written in Rust. Contains extensive test suites under
  `tests/lit/` (LIT-style tests), `tests/correctness/`, and `tests/integration/`.
  Also has `examples/` directory with `.st` files (e.g., `hello_world.st`).
- **ST constructs:** Comprehensive: PROGRAM, FUNCTION_BLOCK, FUNCTION, VAR sections,
  all data types, arrays, structs, enums, pointers, FOR/WHILE/REPEAT loops,
  IF/CASE, function calls, type conversions.
- **CODESYS extensions:** Yes, supports some OOP and pointer extensions.
- **Corpus quality:** EXCELLENT. Hundreds of test files covering edge cases. Best single source.
- **Note:** The companion **PLC-lang/oscat** repo (https://github.com/PLC-lang/oscat)
  contains the OSCAT Basic library adapted for RuSTy, with `oscat.st` and `stubs.st` files.

### 2. 61131/echidna
- **URL:** https://github.com/61131/echidna
- **License:** BSD-2-Clause (OK)
- **Content:** IEC 61131-3 compiler + VM runtime in C. Test suite under `tests/` with
  subdirectories: `block/`, `cast/`, `grammar/`, `operator/`, `standard/`, `unit/`, `value/`, etc.
  Also has `examples/` directory with runnable ST programs.
- **ST constructs:** Standard IEC 61131-3: programs, function blocks, standard functions,
  type casting, operator precedence, grammar edge cases.
- **CODESYS extensions:** No, sticks to the standard.
- **Corpus quality:** EXCELLENT. BSD license is ideal. Good grammar and edge case coverage.

### 3. jubnzv/iec-checker
- **URL:** https://github.com/jubnzv/iec-checker
- **License:** LGPL-3.0 (CAUTION)
- **Content:** Static analysis tool for IEC 61131-3 in OCaml. Test ST files under `test/st/`.
  Also accepts PLCOpen XML.
- **ST constructs:** Programs, function blocks, various coding patterns that trigger
  static analysis warnings (dead code, unused variables, naming conventions).
- **CODESYS extensions:** Compatible with matiec dialect.
- **Corpus quality:** GOOD. Focused on static analysis patterns rather than language coverage.

### 4. ironplc/ironplc
- **URL:** https://github.com/ironplc/ironplc
- **License:** MIT (OK)
- **Content:** Rust-based SoftPLC prototype. Full parser and semantic analyzer for IEC 61131-3.
  Has `examples/` directory and test files.
- **ST constructs:** Parser covers full IEC 61131-3 grammar. Code generation limited to
  PROGRAM, INT variables, assignment, integer literals, arithmetic (+, -, *, /).
- **CODESYS extensions:** No.
- **Corpus quality:** GOOD. MIT license is ideal. Parser test coverage is broad even if
  code generation is limited.

### 5. nucleron/matiec (and sm1820/matiec)
- **URL:** https://github.com/nucleron/matiec
- **Alt URL:** https://github.com/sm1820/matiec
- **Official:** https://openplcproject.gitlab.io/matiec/
- **License:** GPL-3.0 (COPYLEFT)
- **Content:** The reference IEC 61131-3 to C transpiler (used by OpenPLC). Has `tests/`
  directory. Supports IL, ST, and SFC textual formats.
- **ST constructs:** Full IEC 61131-3 coverage: type definitions, derived types, functions,
  function blocks, programs, variable scoping, FOR loops, expressions.
- **CODESYS extensions:** No, strict standard compliance.
- **Corpus quality:** GOOD. Reference compiler, but GPL limits redistribution.

### 6. Felipeasg/matiec_examples
- **URL:** https://github.com/Felipeasg/matiec_examples
- **License:** UNKNOWN
- **Content:** Example programs for matiec compiler. Directories include: `and_logic/`,
  `arithmetic_1/`, `pid/`, `debug_program/`, `concepts/`, etc.
- **ST constructs:** Basic programs, logic operations, arithmetic, PID control.
- **CODESYS extensions:** No.
- **Corpus quality:** MODERATE. Simple but practical examples.

---

## Tier 2: ST Libraries (Real-world function blocks and functions)

### 7. OSCAT Basic Library (multiple repos)

The OSCAT (Open Source Community for Automation Technology) library is the most widely-used
open-source ST library, with 500+ functions and function blocks.

#### a) simsum/oscat
- **URL:** https://github.com/simsum/oscat
- **License:** UNKNOWN (OSCAT was historically open-source, but license unclear)
- **Content:** CODESYS export of OSCAT Basic. Contains 560+ `.EXP` files, each a single
  function or function block. Also has `.lib` files in `Codesys Lib And Manual/`.
- **ST constructs:** Math (trig, hyperbolic, statistics), control (PID, timers, sequencers),
  string operations, time/date handling, data conversion, temperature, arrays.
- **CODESYS extensions:** Minimal, mostly standard IEC 61131-3.
- **Corpus quality:** EXCELLENT for real-world library code. License needs verification.

#### b) PLC-lang/oscat
- **URL:** https://github.com/PLC-lang/oscat
- **License:** UNKNOWN (derived from OSCAT)
- **Content:** OSCAT Basic adapted for RuSTy compiler. Contains `oscat.st` (main library)
  and `stubs.st`. Not all features compile yet.
- **ST constructs:** Same as OSCAT Basic but in single consolidated `.st` files.
- **Corpus quality:** GOOD. Already in `.st` format ready for parsing.

#### c) tkucic/brOscatLib
- **URL:** https://github.com/tkucic/brOscatLib
- **License:** MIT (OK) + separate OSCAT license
- **Content:** B&R Automation Studio port of OSCAT (Basic 3.34, Building 1.00, Network 1.35.2).
  Includes `oscatBasic/`, `oscatBasic_tests/`, `oscatBuild/`, `oscatNetw/`.
- **ST constructs:** Full OSCAT library: math, control, string, time, building automation,
  network functions.
- **CODESYS extensions:** Adapted for B&R, may have vendor-specific idioms.
- **Corpus quality:** EXCELLENT. MIT license. Complete OSCAT port with tests.

#### d) mihaiginta/TcOscatBasic
- **URL:** https://github.com/mihaiginta/TcOscatBasic
- **License:** Same as original OSCAT (UNKNOWN exact terms)
- **Content:** TwinCAT 3 fork of OSCAT Basic. Includes `TcOscatBasic/` (library),
  `TcOscatBasicTest/` (tests using TcUnit).
- **ST constructs:** Full OSCAT library adapted for TwinCAT/Beckhoff.
- **CODESYS extensions:** TwinCAT-specific, may include OOP features.
- **Corpus quality:** GOOD. TwinCAT XML format may need extraction.

### 8. WengerAG/structured-text-utilities
- **URL:** https://github.com/WengerAG/structured-text-utilities
- **License:** MIT (OK)
- **Content:** Utility library for IEC 61131-3 ST. Six `.st` files:
  `UTILITIES.st`, `UTILITIES_ARRAY.st`, `UTILITIES_BYTE.st`,
  `UTILITIES_MATH.st`, `UTILITIES_STRING.st`, `UTILITIES_TIME.st`.
- **ST constructs:** Array handling, byte manipulation, math functions, string operations,
  time/date utilities. Designed for basic IEC 61131-3 compliance (no extensions).
- **CODESYS extensions:** Explicitly avoids them. Pure standard ST.
- **Corpus quality:** EXCELLENT. MIT license, clean standard-compliant ST, well-organized.

---

## Tier 3: Parser Test Files (Useful for grammar testing)

### 9. klauer/blark
- **URL:** https://github.com/klauer/blark
- **License:** GPL-2.0 (COPYLEFT)
- **Content:** TwinCAT ST parser in Python (Lark/Earley). Test files in `blark/tests/source/`
  (plain `.st` files) and `blark/tests/POUs/` (TwinCAT XML format).
- **ST constructs:** Functions, function blocks, programs, data type declarations, global
  variable declarations, methods, attributes, array of objects.
- **CODESYS extensions:** Yes, TwinCAT/CODESYS OOP extensions (methods, properties).
- **Corpus quality:** GOOD for testing OOP extension parsing.

### 10. chathhorn/structured-text
- **URL:** https://github.com/chathhorn/structured-text
- **License:** UNKNOWN
- **Content:** Haskell-based tools for IEC 61131-3 ST. Has `test/`, `demo/`, `examples/`,
  `samples/`, `ex/` directories.
- **ST constructs:** Parser and analysis tools suggest broad language coverage.
- **CODESYS extensions:** Unknown.
- **Corpus quality:** MODERATE. Multiple example directories, but license unclear.

### 11. vlsi/iec61131-parser
- **URL:** https://github.com/vlsi/iec61131-parser
- **License:** Unknown (check repo)
- **Content:** Java-based IEC 61131-3 grammar parser.
- **ST constructs:** Parsing-focused, likely has grammar test files.
- **CODESYS extensions:** Unknown.
- **Corpus quality:** MODERATE. Useful for grammar edge cases.

### 12. amal029/st
- **URL:** https://github.com/amal029/st
- **License:** MIT (OK)
- **Content:** IEC 61131-3 Structured Text parser in OCaml. Has `error/`, `language/`,
  `parser/` directories.
- **ST constructs:** Parser tests covering language features and error recovery.
- **CODESYS extensions:** No.
- **Corpus quality:** GOOD. MIT license, focused on parsing edge cases.

### 13. knordman/esstee
- **URL:** https://github.com/knordman/esstee
- **License:** GPL-3.0 (COPYLEFT)
- **Content:** ST interpreter in C. Test programs in `src/tests/programs/` including
  `example.ST`.
- **ST constructs:** Basic ST interpretation: variables, expressions, control flow.
- **CODESYS extensions:** No.
- **Corpus quality:** MODERATE. Small test set.

### 14. highlightjs/highlightjs-structured-text
- **URL:** https://github.com/highlightjs/highlightjs-structured-text
- **License:** BSD-3-Clause (likely, standard for highlight.js plugins) (OK)
- **Content:** Syntax highlighter for ST. Contains `example.iecst` with representative ST code.
- **ST constructs:** Sample code demonstrating syntax highlighting coverage.
- **CODESYS extensions:** Covers CoDeSys keywords.
- **Corpus quality:** MINIMAL. Single example file, but useful as a quick smoke test.

---

## Tier 4: Application Code (Real-world PLC programs)

### 15. Fortiphyd/GRFICSv2 (and GRFICSv3)
- **URL:** https://github.com/Fortiphyd/GRFICSv2
- **License:** GPL-3.0 (COPYLEFT)
- **Content:** ICS security simulation framework. Contains `simplified_te.st` and other
  ST files for simulated chemical plant PLC control.
- **ST constructs:** PROGRAM blocks, real-world process control logic, Modbus I/O.
- **CODESYS extensions:** No, uses OpenPLC dialect.
- **Corpus quality:** GOOD. Real industrial-style control programs.

### 16. gracesrm/OpenPLC_Sample_Programs
- **URL:** https://github.com/gracesrm/OpenPLC_Sample_Programs
- **License:** GPL-3.0 (COPYLEFT)
- **Content:** Three ST programs: `Wait_busy_protocol.st`, `logic_game_31.st`, `water_level.st`.
- **ST constructs:** Basic PROGRAM blocks, digital I/O, timers, logic.
- **CODESYS extensions:** No.
- **Corpus quality:** MODERATE. Simple but real programs.

### 17. TcOpenGroup/TcOpen
- **URL:** https://github.com/TcOpenGroup/TcOpen
- **License:** MIT (OK)
- **Content:** Application framework for TwinCAT3 industrial automation.
  OOP-heavy ST code using IEC 61131-3 extensions.
- **ST constructs:** Full OOP: INTERFACE, METHOD, PROPERTY, EXTENDS, IMPLEMENTS,
  abstract classes, inheritance hierarchies.
- **CODESYS extensions:** Heavy use of OOP extensions. Excellent for testing extended grammar.
- **Corpus quality:** GOOD. MIT license. Real-world OOP ST code. Note: repo is archived.
  Code is in TwinCAT XML format (.TcPOU), may need extraction.

### 18. loupeteam/ToolBox
- **URL:** https://github.com/loupeteam/ToolBox
- **License:** MIT (OK)
- **Content:** B&R Automation Studio library for advanced timer scenarios.
  Expands TON function capabilities.
- **ST constructs:** FUNCTION_BLOCK, timer patterns, tolerance checking.
- **CODESYS extensions:** B&R-specific idioms.
- **Corpus quality:** MODERATE. Small but practical library.

### 19. BhanuKiranChaluvadi/ST_DesignPattern
- **URL:** https://github.com/BhanuKiranChaluvadi/ST_DesignPattern
- **License:** UNKNOWN
- **Content:** Design patterns implemented in Beckhoff Structured Text.
  Organized into `Behavioral/` and `Creational/` categories.
- **ST constructs:** OOP patterns: likely includes INTERFACE, METHOD, EXTENDS.
- **CODESYS extensions:** Yes, Beckhoff/TwinCAT OOP.
- **Corpus quality:** MODERATE. Interesting for OOP ST testing.

---

## Tier 5: Benchmarks and Datasets

### 20. Luoji-zju/Agents4PLC_release
- **URL:** https://github.com/Luoji-zju/Agents4PLC_release
- **License:** Apache-2.0 (OK)
- **Content:** PLC code generation benchmark. `benchmark_v1/` (easy/medium tasks),
  `benchmark_v2/` (96 tasks: medium, hard, high-fidelity). Includes reference ST code
  that compiles and satisfies formal specifications.
- **ST constructs:** Programs and function blocks for industrial control tasks.
- **CODESYS extensions:** Targets CODESYS and Siemens TIA Portal.
- **Corpus quality:** EXCELLENT. Apache license, verified-compilable ST code with specs.

### 21. cangkui/AutoPLC
- **URL:** https://github.com/cangkui/AutoPLC
- **License:** UNKNOWN (check repo)
- **Content:** LLM-based ST generation framework. `data/benchmarks/` includes 914 tasks
  including OSCAT-derived tasks. Targets CODESYS and Siemens TIA Portal.
- **ST constructs:** Generated ST programs for various automation tasks.
- **CODESYS extensions:** Vendor-aware (CODESYS and Siemens dialects).
- **Corpus quality:** MODERATE. Large dataset but license unclear and LLM-generated code.

### 22. AICPS/PLCBEAD_PLCEmbed
- **URL:** https://github.com/AICPS/PLCBEAD_PLCEmbed
- **License:** UNKNOWN
- **Content:** 700+ PLC programs with source code and binaries. Compiled with GEB, CoDeSys,
  OpenPLC-V3, and OpenPLC-V2 toolchains. 96% Smalltalk (likely `.st` files).
- **ST constructs:** Wide variety of PLC programs across multiple compiler dialects.
- **CODESYS extensions:** Mixed, covers multiple toolchains.
- **Corpus quality:** POTENTIALLY EXCELLENT due to sheer volume. License unclear.

---

## Tier 6: Other Notable Resources

### 23. VerifAPS/verifaps-lib
- **URL:** https://github.com/VerifAPS/verifaps-lib
- **License:** GPL-3.0 (COPYLEFT)
- **Content:** Kotlin-based verification tools for automated production systems.
  Contains ST parser, symbolic executor, and regression verifier.
  24.5% Smalltalk content suggests significant `.st` test files.
- **ST constructs:** Full ST parsing, SFC, function blocks.
- **CODESYS extensions:** Unknown.
- **Corpus quality:** MODERATE.

### 24. Thewbi/IEC-63313-ST-Simulator
- **URL:** https://github.com/Thewbi/IEC-63313-ST-Simulator
- **License:** Unknown (check repo)
- **Content:** ST simulator with custom grammar extensions. Uses OpenPLC editor for
  creating `.st` files.
- **ST constructs:** Standard ST plus custom extensions for actions and implementation blocks.
- **CODESYS extensions:** Custom extensions beyond standard.
- **Corpus quality:** MODERATE. Interesting for testing non-standard constructs.

### 25. Eclipse OSCAT (Proposed)
- **URL:** https://projects.eclipse.org/proposals/eclipse-oscat
- **License:** Would be Eclipse Public License (if accepted)
- **Content:** Proposed Eclipse Foundation project to maintain OSCAT libraries.
- **Status:** Proposal stage. May become the canonical open-source OSCAT distribution.

---

## Recommended Priority for Test Corpus

### Phase 1: Permissively Licensed (use freely)
1. **61131/echidna** - BSD-2-Clause, good grammar tests
2. **WengerAG/structured-text-utilities** - MIT, clean standard ST library
3. **ironplc/ironplc** - MIT, parser tests
4. **amal029/st** - MIT, parser edge cases
5. **tkucic/brOscatLib** - MIT, full OSCAT port with tests
6. **TcOpenGroup/TcOpen** - MIT, OOP ST (needs extraction from TwinCAT XML)
7. **Luoji-zju/Agents4PLC_release** - Apache-2.0, verified benchmark ST code
8. **loupeteam/ToolBox** - MIT, practical FB library

### Phase 2: Copyleft (use as test input only, do not redistribute)
9. **PLC-lang/rusty** tests - LGPL/GPL, most comprehensive ST test suite
10. **klauer/blark** test source - GPL-2.0, OOP extension tests
11. **Fortiphyd/GRFICSv2** - GPL-3.0, real industrial control programs
12. **nucleron/matiec** - GPL-3.0, reference compiler tests
13. **jubnzv/iec-checker** - LGPL-3.0, static analysis test patterns

### Phase 3: License Verification Needed
14. **simsum/oscat** - OSCAT Basic .EXP files (560+ functions)
15. **PLC-lang/oscat** - Consolidated OSCAT in .st format
16. **AICPS/PLCBEAD_PLCEmbed** - 700+ PLC programs
17. **cangkui/AutoPLC** - 914 benchmark tasks

---

## Notes

- GitHub classifies `.st` files as "Smalltalk" (not Structured Text), which makes searching
  by language unreliable. Search by content keywords (FUNCTION_BLOCK, PROGRAM, END_VAR) instead.
- Many TwinCAT projects store ST code in `.TcPOU` XML files rather than plain `.st` files.
  The tool [Zeugwerk/Plaincat](https://github.com/Zeugwerk/Plaincat) can extract `.st` from TwinCAT projects.
- The [klauer/blark](https://github.com/klauer/blark) parser can also extract ST from TwinCAT XML.
- CODESYS `.library` and `.EXP` files contain ST source but in proprietary container formats.
- The [awesome-structured-text](https://github.com/myutzy/awesome-structured-text) curated list
  and [benhar-dev/twincat-resources](https://github.com/benhar-dev/twincat-resources) are good
  meta-indexes for finding more ST code.
