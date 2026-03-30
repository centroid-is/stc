<!-- GSD:project-start source:PROJECT.md -->
## Project

**STC — Structured Text Compiler Toolchain**

A Go-based IEC 61131-3 Structured Text compiler toolchain that makes ST development feel like modern software development — with an LSP, unit testing on host, CI integration, multi-vendor compilation, and first-class LLM agent support. Built for humans and AI agents to quickly produce, validate, and deploy structured text code across PLC vendors.

**Core Value:** Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor — no hardware required for development and testing.

### Constraints

- **No Java runtime**: Parser and compiler must not require Java at runtime
- **Go ecosystem**: All compiler core in Go; VS Code extension in TypeScript (mandatory for VS Code)
- **Vendor compatibility**: Must handle CODESYS extensions (OOP, pointers, 64-bit types) to parse production code
- **Error recovery**: Parser must produce partial ASTs from broken code (essential for LSP)
- **Determinism**: All test execution must be deterministic — no wall-clock dependencies
- **Machine-readable output**: Every CLI command supports `--format json`
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Recommended Stack
### Core Technologies (No New Dependencies)
| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go `encoding/xml` | stdlib | Parse TwinCAT `.TcPOU` XML for stub extraction | No external dep needed; TcPOU is simple XML with CDATA |
| Go `encoding/json` | stdlib | I/O table serialization for mock framework | Already used throughout stc for `--format json` |
| Existing stc parser | v1.0 | Parse `.st` stub files for vendor library loading | Stubs use plain ST syntax -- no new parser needed |
| Existing stc interpreter | v1.0 | Execute mock FBs written in ST | `FBInstance` + `StandardFB` interface already handles user-defined FBs |
### Supporting Libraries (None Required)
### New Internal Packages/Files
| Package/File | Purpose | Notes |
|--------------|---------|-------|
| `pkg/iomap/iomap.go` | I/O address table: `%I*`, `%Q*`, `%M*` areas with typed read/write | Flat byte arrays with typed access, not a full PLC memory model |
| `pkg/iomap/address.go` | Parse AT addresses: `%IX0.0`, `%QW4`, `%MD12`, `%I*` | Regex-based parser for `%[IQM][XBWD]?(\*\|\d+(\.\d+)*)` |
| `pkg/vendor/loader.go` | Load `.st` stub files from `[build.library_paths]` config | Calls parser, registers declarations in symbol table before user code |
| `pkg/vendor/extract.go` | Extract FB signatures from TwinCAT `.TcPOU` XML files | `stc vendor extract` command |
| `pkg/interp/mock.go` | Auto-stub generation for declaration-only FBs | Zero-value outputs, no-op Execute |
## I/O Memory Model: %I, %Q, %M Areas
### IEC 61131-3 Address Format (HIGH confidence -- from standard)
| Address | Meaning | Data Width |
|---------|---------|------------|
| `%IX0.0` | Input bit 0 of byte 0 | 1 bit |
| `%IX0.7` | Input bit 7 of byte 0 | 1 bit |
| `%IB0` | Input byte 0 | 8 bits |
| `%IW0` | Input word at byte 0 | 16 bits |
| `%IW2` | Input word at byte 2 | 16 bits |
| `%ID0` | Input double word at byte 0 | 32 bits |
| `%QX0.0` | Output bit 0 of byte 0 | 1 bit |
| `%QB4` | Output byte 4 | 8 bits |
| `%QW8` | Output word at byte 8 | 16 bits |
| `%MD48` | Memory double word at byte 48 | 32 bits |
| `%I*` | Auto-assigned input (TwinCAT/CODESYS wildcard) | Per variable type |
| `%Q*` | Auto-assigned output | Per variable type |
### AT Keyword Semantics (HIGH confidence -- from IEC 61131-3 + Beckhoff InfoSys)
- `VAR` blocks in `PROGRAM` declarations
- `VAR_GLOBAL` blocks (GVL in TwinCAT)
- `VAR_CONFIG` blocks (configuration-level linking)
- NOT valid in `FUNCTION_BLOCK` or `FUNCTION` VAR blocks (FBs are hardware-independent)
### Memory Model for stc Mock I/O
## EtherCAT Terminal I/O Mapping (MEDIUM confidence -- from Beckhoff product docs)
### How TwinCAT Maps Terminals to Addresses
| Terminal | Type | Channels | Process Data | Typical AT Address |
|----------|------|----------|-------------|-------------------|
| EL1008 | 8-ch digital input | 8 x BOOL | 1 byte (8 bits packed) | `%IB0` or `%IX0.0` thru `%IX0.7` |
| EL2008 | 8-ch digital output | 8 x BOOL | 1 byte (8 bits packed) | `%QB0` or `%QX0.0` thru `%QX0.7` |
| EL3064 | 4-ch analog input 0-10V, 12-bit | 4 x INT | 8 bytes (4 x 16-bit) | `%IW0`, `%IW2`, `%IW4`, `%IW6` |
| EL4034 | 4-ch analog output, 16-bit | 4 x INT | 8 bytes (4 x 16-bit) | `%QW0`, `%QW2`, `%QW4`, `%QW6` |
### Implications for stc
## Beckhoff .library File Format (MEDIUM confidence -- from Beckhoff InfoSys + CODESYS docs)
### Format Assessment
| Format | Internal Structure | Extractable? | Notes |
|--------|-------------------|-------------|-------|
| `.library` (source) | Proprietary binary container with embedded ST source | Partially -- CODESYS scripting API can export | Contains full source but format is undocumented |
| `.compiled-library` | Proprietary binary, source stripped | NO -- signatures present but binary format undocumented | Used for distribution, no public parser exists |
| `.TcPOU` (TwinCAT) | Simple XML with CDATA-wrapped ST | YES -- trivial to parse | Each POU in its own file, declaration in `<Declaration>` element |
| `.plcproj` (TwinCAT) | MSBuild XML | YES -- lists all `.TcPOU` references | Standard XML, references POU files |
| PLCopen XML | Standardized IEC 61131-10 XML | YES -- public XSD schema | Can represent POUs with or without bodies |
### Recommended Extraction Path
## Allen Bradley Dialect Differences (MEDIUM confidence -- from Rockwell docs + community sources)
### Feature Comparison: AB Logix 5000 vs IEC 61131-3 / CODESYS
| Feature | Beckhoff (CODESYS) | Schneider (CODESYS-derived) | Allen Bradley (Logix 5000) |
|---------|-------------------|---------------------------|---------------------------|
| `FUNCTION_BLOCK` | Full support | Full support | **NO** -- uses Add-On Instructions (AOI) |
| `FUNCTION` | Full support | Full support | Full support |
| `PROGRAM` | Full support | Full support | **Tasks + Routines** model instead |
| OOP (METHOD, INTERFACE, PROPERTY) | Full support | No | **No** |
| POINTER TO | Yes | No | **No** |
| REFERENCE TO | Yes | No | **No** |
| ENUM types | Full support | Full support | **No native ENUM** -- use DINT constants |
| LREAL (64-bit float) | Yes | Yes | **Yes** (newer firmware v31+) |
| LINT/ULINT (64-bit int) | Yes | Yes | **LINT yes, ULINT no** |
| WSTRING | Yes | Yes | **No** |
| STRING literal assignment | Yes | Yes | **No** -- cannot assign `'hello'` to STRING tag directly |
| AT %I/%Q/%M addressing | Yes | Yes | **No** -- tag-based, no direct addresses |
| VAR_GLOBAL (GVL) | Yes | Yes | **Controller-scoped tags** instead |
| REPEAT..UNTIL | Yes | Yes | **Not documented** |
| CASE with ranges | Yes | Yes | **Integer values only, no ranges** |
| Multi-dimensional arrays | Yes | Yes | **3 dimensions max** |
| Timer types | TON, TOF, TP (IEC) | TON, TOF, TP (IEC) | **TONR, TOF** -- different names/behavior |
### AB-Specific Concepts Needing Stubs
| AB Concept | IEC Equivalent | Stub Strategy |
|------------|---------------|---------------|
| Add-On Instruction (AOI) | FUNCTION_BLOCK | Declare as FUNCTION_BLOCK in stub, note in metadata |
| User-Defined Type (UDT) | TYPE..STRUCT..END_TYPE | Same syntax, no change needed |
| Controller-scoped tag | VAR_GLOBAL | Use VAR_GLOBAL in stubs |
| MSG instruction | ADSREAD/ADSWRITE equivalent | Stub as FUNCTION_BLOCK with Done/Error/EN/DN outputs |
| PID instruction | PID FB | Stub with AB-specific parameters (SP, PV, CV, etc.) |
| TONR (retentive timer) | No IEC equivalent | Custom stub FB |
| GSV/SSV (Get/Set System Value) | No IEC equivalent | Stub as FUNCTION with class/attribute params |
### Implications for stc Stubs
## Go XML Parsing for TwinCAT Files (HIGH confidence -- Go stdlib)
## Alternatives Considered
| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| Plain `.st` stubs | YAML/TOML/JSON stub format | Engineers know ST syntax; parser already handles it; zero new formats |
| Go `encoding/xml` | `github.com/beevik/etree` | etree is nicer API but adds dependency for simple XML; stdlib is sufficient |
| Flat byte array I/O table | Structured I/O model per terminal | Over-engineering; real PLCs use flat byte images too |
| Zero-value auto-stubs | Require explicit mocks for all FBs | Too much friction; most tests only care about a few FBs |
| Extract from `.TcPOU` XML | Parse `.library` binary files | `.library` format is proprietary and undocumented; `.TcPOU` is trivial XML |
## What NOT to Use
| Avoid | Why | Use Instead |
|-------|-----|-------------|
| EtherCAT terminal models in stc | stc is not a System Manager; terminal config is IDE concern | Model I/O image (byte arrays) only |
| `.library` binary parsing | Proprietary format, undocumented, changes between CODESYS versions | `.TcPOU` XML extraction or hand-written `.st` stubs |
| Complex mock framework in Go | Forces engineers to learn Go for testing | ST-based mocks using existing parser/interpreter |
| Address validation against hardware config | stc has no hardware config file | Trust AT addresses; validate format only |
| Allen Bradley tag-based addressing in stc | Fundamentally different paradigm from IEC %I/%Q/%M | AB stubs use standard IEC FUNCTION_BLOCK; no AT addresses |
## Sources
- [Beckhoff InfoSys: AT-Declaration](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/11948825611.html) -- AT syntax grammar, wildcard addressing (HIGH confidence)
- [Beckhoff InfoSys: Addresses](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2529360523.html) -- %I/%Q/%M format, size prefixes (HIGH confidence)
- [Beckhoff InfoSys: Library creation](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/4189255051.html) -- .library vs .compiled-library formats (MEDIUM confidence)
- [Beckhoff EL1008 product page](https://www.beckhoff.com/en-us/products/i-o/ethercat-terminals/el-ed1xxx-digital-input/el1008.html) -- 8-ch DI process data (HIGH confidence)
- [Beckhoff EL3064 product page](https://www.beckhoff.com/en-us/products/i-o/ethercat-terminals/el-ed3xxx-analog-input/el3064.html) -- 4-ch AI 0-10V 12-bit (HIGH confidence)
- [Rockwell Logix 5000 IEC 61131-3 Compliance (1756-PM018)](https://literature.rockwellautomation.com/idc/groups/literature/documents/pm/1756-pm018_-en-p.pdf) -- AB dialect restrictions (MEDIUM confidence, PDF partially accessible)
- [IEC 61131-3 Wikipedia](https://en.wikipedia.org/wiki/IEC_61131-3) -- Standard addressing overview (MEDIUM confidence)
- [Fernhill Software IEC 61131-3 Variable Declarations](https://www.fernhillsoftware.com/help/iec-61131/common-elements/variable-declaration.html) -- AT keyword syntax (MEDIUM confidence)
- [CODESYS Library Development](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_library_development_information.html) -- Library file types (MEDIUM confidence)
- [PLCopen XML Exchange](https://www.plcopen.org/standards/xml-echange/) -- IEC 61131-10 format (MEDIUM confidence)
- [GitHub: Beckhoff-PLC-TwinCAT TcPOU example](https://github.com/a-m-shotorbani/Beckhoff-PLC-TwinCAT/blob/main/FB_myFileRead.TcPOU) -- TcPOU XML structure (HIGH confidence)
- [Solisplc: Add-On Instructions](https://www.solisplc.com/tutorials/add-on-instructions-programming-aoi-rslogix-studio-5000-plc-programming-tutorial-example-logic) -- AOI vs FUNCTION_BLOCK (MEDIUM confidence)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
