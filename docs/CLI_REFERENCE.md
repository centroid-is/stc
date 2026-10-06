# CLI Reference

Complete reference for the `stc` command-line interface.

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | `-f` | `text` | Output format: `text`, `json` |
| `--version` | | | Print version information |
| `--help` | `-h` | | Print help for any command |

## Commands

---

### `stc parse`

Parse ST source files and output the abstract syntax tree.

```
stc parse <file...> [flags]
```

**Arguments**: One or more `.st` source files.

**Output (text)**:
```
Parsed 3 declaration(s), 0 diagnostic(s) in myfile.st
```

**Output (JSON)**:
```json
{
  "file": "myfile.st",
  "ast": { "kind": "SourceFile", "declarations": [...] },
  "diagnostics": [],
  "has_errors": false
}
```

**Exit codes**: 0 on success, 1 if any file has parse errors.

---

### `stc check`

Run semantic analysis on ST source files.

```
stc check <file...> [flags]
```

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--vendor` | | | Vendor target: `beckhoff`, `schneider`, `portable` |
| `--define` | `-D` | | Define preprocessor symbols (repeatable) |

Reports type errors, undeclared variables, unused variables, unreachable code, and vendor compatibility warnings. Automatically loads `stc.toml` configuration if present (walks up from current directory). Uses incremental compilation -- only re-parses changed files.

**Output (text)**:
```
myfile.st:15:5: error: type mismatch: cannot assign REAL to INT (SEMA003)
myfile.st:22:3: warning: unused variable 'temp' (SEMA008)
1 error(s), 1 warning(s)
(1/2 files re-parsed)
```

**Output (JSON)**:
```json
[
  {
    "file": "myfile.st",
    "line": 15,
    "col": 5,
    "severity": "error",
    "code": "SEMA003",
    "message": "type mismatch: cannot assign REAL to INT"
  }
]
```

**Exit codes**: 0 if no errors (warnings allowed), 1 if errors exist.

**TwinCAT projects**: `stc check <x.tsproj|x.plcproj>` imports the project on the fly (see `stc vendor import`) and checks it with its sibling libraries and shipped stubs. Positions point into the original `.TcPOU`/`.TcGVL`/`.TcDUT` files. TwinCAT-specific checker code: SEMA039 warns about an attribute name in double quotes, which TwinCAT ignores.

---

### `stc test`

Discover and run ST unit tests.

```
stc test [dir] [flags]
```

**Arguments**: Directory to search for `*_test.st` files (default: current directory). Searches recursively.

**Output formats**:

| Format | Flag | Description |
|--------|------|-------------|
| text | (default) | Human-readable pass/fail output |
| json | `--format json` | Machine-readable test results |
| junit | `--format junit` | JUnit XML for CI integration |

**Behavior**:
- Automatically defines `STC_TEST` preprocessor symbol
- Loads `stc.toml` for `library_paths` and `mock_paths` if present
- Library stub FBs without mocks auto-generate zero-value instances (with fidelity warnings)
- Mock FBs override stub FBs when both exist

**Output (text)**:
```
=== RUN  motor_control_test.st
--- PASS: Motor does not start without interlocks (0.000s)
--- PASS: Motor starts with all interlocks OK (0.000s)
--- FAIL: Speed ramp test (0.001s)
    motor_control_test.st:45:5: Expected speed > 100.0 but got 0.0

ok
3 tests, 2 passed, 1 failed
```

**Output (JUnit XML)**:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="3" failures="1">
  <testsuite name="motor_control_test.st" tests="3" failures="1">
    <testcase name="Motor does not start without interlocks" time="0.000"/>
    <testcase name="Speed ramp test" time="0.001">
      <failure message="Expected speed > 100.0 but got 0.0"/>
    </testcase>
  </testsuite>
</testsuites>
```

**Exit codes**: 0 if all tests pass, 1 if any test fails.

**TwinCAT projects**: `stc test tests/ --project x.tsproj` loads the project's POUs, GVLs and DUTs (and sibling library sources) as real code under the tests. Only embedded stubs and `library_paths` stubs are auto-stubbed.

---

### `stc sim`

Run a deterministic closed-loop simulation of a ST program.

```
stc sim <file> [flags]
```

**Arguments**: Exactly one `.st` file containing a `PROGRAM` declaration.

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--cycles` | | `100` | Number of scan cycles to run |
| `--dt` | | `10ms` | Cycle time as Go duration (e.g., `10ms`, `100us`) |
| `--wave` | | | Waveform bindings (repeatable) |
| `--define` | `-D` | | Define preprocessor symbols (repeatable) |

**Waveform format**: `INPUT_NAME:KIND:AMPLITUDE:FREQUENCY`

Waveform kinds: `step`, `ramp`, `sine`, `square`.

Automatically defines `STC_SIM` preprocessor symbol.

**Example**:
```bash
stc sim conveyor.st --cycles 200 --dt 10ms --wave "SENSOR:sine:100.0:0.5"
```

**Output (text)**:
```
Simulation: 200 cycles, duration 2s

Cycle    Time         OUTPUT1
-----    ----         ----------------
0        0s           0
1        10ms         0
2        20ms         1
...
```

**Output (JSON)**: Full simulation result with per-cycle input/output snapshots.

**Exit codes**: 0 on success, 1 on error.

**TwinCAT projects**: `stc sim <x.tsproj|x.plcproj>` runs the program called by the first task (falling back to the first PROGRAM). `--dt` defaults to that task's cycle time. User FBs and methods from other project files are not registered yet (Phase 23).

---

### `stc emit`

Emit vendor-specific Structured Text from parsed source files.

```
stc emit <file...> [flags]
```

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--target` | | `portable` | Vendor target: `beckhoff`, `schneider`, `portable` |
| `--define` | `-D` | | Define preprocessor symbols (repeatable) |

**Targets**:
- `beckhoff`: Full CODESYS OOP, pointers, references, 64-bit types
- `schneider`: CODESYS-derived, no OOP/pointers/references
- `portable`: Most restrictive -- no OOP, no pointers, no 64-bit types

**Example**:
```bash
stc emit src/main.st --target beckhoff
```

**Output (text)**: Vendor-flavored ST source to stdout.

**Output (JSON)**:
```json
{
  "file": "src/main.st",
  "code": "PROGRAM Main\nVAR\n    ...",
  "target": "beckhoff",
  "diagnostics": [],
  "has_errors": false
}
```

**Exit codes**: 0 on success, 1 if parse errors.

---

### `stc fmt`

Format ST source files with consistent style.

```
stc fmt <file...> [flags]
```

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--indent` | | `"    "` (4 spaces) | Indentation string |
| `--uppercase-keywords` | | `true` | Use uppercase keywords |
| `--define` | `-D` | | Define preprocessor symbols (repeatable) |

Parses each file and re-emits with normalized indentation, keyword casing, and spacing. Comments attached to AST nodes are preserved.

**Example**:
```bash
stc fmt src/main.st
# Formatted output to stdout

stc fmt src/main.st --indent "  " --uppercase-keywords=false
# 2-space indent, lowercase keywords
```

**Exit codes**: 0 on success, 1 if parse errors.

---

### `stc lint`

Lint ST source files against coding standards.

```
stc lint <file...> [flags]
```

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--define` | `-D` | | Define preprocessor symbols (repeatable) |

**Rules checked**:
- Magic numbers (unnamed numeric literals)
- Nesting depth > 3 levels
- POU length > 200 lines
- Missing return type on functions
- Naming convention violations (configurable via `stc.toml`)

Loads `stc.toml` for `naming_convention` configuration if present.

**Output (text)**:
```
myfile.st:10:15: warning: magic number 42 (LINT001)
myfile.st:30:1: warning: POU 'ProcessData' exceeds 200 lines (LINT003)
2 warning(s), 0 error(s)
```

**Exit codes**: 0 on success (lint warnings do not cause exit 1), 1 if parse errors.

---

### `stc pp`

Preprocess ST source files by evaluating conditional compilation directives.

```
stc pp <file...> [flags]
```

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--define` | `-D` | | Define preprocessor symbols (repeatable) |

**Directives supported**: `{IF defined(X)}`, `{ELSIF defined(Y)}`, `{ELSE}`, `{END_IF}`, `{DEFINE X}`, `{ERROR "message"}`.

**Example**:
```bash
stc pp myfile.st -D VENDOR_BECKHOFF -D DEBUG
# Preprocessed output to stdout
```

**Output (JSON)** includes source map entries mapping preprocessed lines to original positions:
```json
{
  "file": "myfile.st",
  "output": "...",
  "source_map": [
    { "preproc_line": 1, "orig_file": "myfile.st", "orig_line": 1 },
    { "preproc_line": 2, "orig_file": "myfile.st", "orig_line": 5 }
  ],
  "diagnostics": [],
  "has_errors": false
}
```

**Exit codes**: 0 on success, 1 if preprocessor errors.

---

### `stc lsp`

Start the Language Server Protocol server on stdio.

```
stc lsp
```

No flags. Designed to be launched by editors (e.g., VS Code). Communicates via JSON-RPC 2.0 over stdin/stdout.

**LSP capabilities**:
- Real-time diagnostics (parse errors + type errors)
- Go-to-definition
- Hover (type information)
- Completion (keywords, types, variables, FB members)
- Find references
- Rename
- Semantic tokens (preprocessor block highlighting)
- Document formatting

---

### `stc vendor import`

Import a TwinCAT solution or PLC project into one stc project model.

```
stc vendor import <x.tsproj|x.plcproj> [--out dir] [--format json] [-D sym]
```

Reads the tsproj (and `_Config/PLC/*.xti`), the plcproj and every TcPOU, TcGVL and TcDUT it lists, including methods, actions and properties. It reads the PLC name, AMS port and tasks, and resolves library references (project, sibling plcproj, `[build.library_paths]`, embedded stubs; see docs/VENDOR_LIBRARIES.md). Text output summarises the PLC, tasks, source counts and library resolution. `--out dir` writes compact `.st` files at the plcproj paths, libraries under `libs/`, and an `stc.toml`; every target is validated before the first write.

**JSON example** (abridged):
```json
{
  "plc_name": "ST301",
  "ams_port": 851,
  "tasks": [{"name": "PlcTask", "cycle_time_ns": 1000000, "priority": 20, "programs": ["MAIN"]}],
  "sources": [{"rel_path": "POUs/MAIN.TcPOU", "kind": "pou", "name": "MAIN"}],
  "libraries": [
    {"name": "SVNCoreComponents", "resolved_from": "sibling"},
    {"name": "Tc2_EtherCAT", "resolved_from": "stub"},
    {"name": "Tc2_Standard", "resolved_from": "builtin"}
  ],
  "diagnostics": []
}
```

`resolved_from` is one of `project`, `sibling`, `library_path`, `stub`, `builtin` or `unresolved`.

**Import diagnostics**:

| Code | Severity | Meaning |
|------|----------|---------|
| VEND020 | warning | Library reference resolved nowhere |
| VEND021 | warning | plcproj Compile item with an unsupported extension |
| VEND022 | info | TwinSAFE project, extra PLC project or non-ST implementation skipped |
| VEND023 | warning | A CDATA segment could not be placed at its XML line |
| VEND024 | warning | No task information; a 10 ms default task is used |
| VEND025 | warning | tsproj and TcTTO cycle times disagree |
| VEND026 | warning | More than one sibling library candidate |
| VEND027 | error | A project or object file is not valid XML |

**Exit codes**: 0 on success, 1 on error.

---

### `stc vendor extract`

Extract declaration-only stubs from a TwinCAT project.

```
stc vendor extract <path.plcproj> [flags]
```

**Arguments**: Path to a TwinCAT `.plcproj` file.

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | (stdout) | Output directory for extracted `.st` files |
| `--format` | `-f` | `text` | `json` prints `{"stubs": [...], "diagnostics": [...]}` |

Renders every POU, GVL, DUT and interface the plcproj lists, in plcproj order. Methods and properties keep their signatures and bodies are dropped. Every stub parses. Items that cannot be converted are reported as diagnostics instead of being skipped.

**Example**:
```bash
stc vendor extract MyProject.plcproj --output vendor/custom/
stc vendor extract MyProject.plcproj --format json
```

**Exit codes**: 0 on success, 1 on error.

---

### `stc ecat validate`

Resolve `{attribute 'TcLinkTo' := '...'}` links in ST sources against TwinCAT EtherCAT exports.

```
stc ecat validate --io <Device N.xml> [--io <Device M.xml>...] [-D SYM...] <file.st>...
```

**Arguments**: One or more ST files. A file holding a GVL names the GVL after the file name, so `ECT.st` declares `ECT`. Type and function block declarations (DUTs, FBs with `AT %I*`/`%Q*` members) can be passed alongside the GVLs.

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--io` | | (required) | TwinCAT EtherCATConfig export (`Device N.xml`), one per master. Repeatable. |
| `--define` | `-D` | | Define a preprocessor symbol. Repeatable. |
| `--format` | | `text` | `text` or `json` |

The exports are loaded with the same rules as `generate_gvl.py`: terminals nest under the EK coupler or CX head they hang off, and link paths read `TIID^<master>^<coupler>^<box>^<pdo or module>^<entry>`. Process image offsets come from the export's `<ProcessImage>` when present and are computed from the PDO layout otherwise. TwinCAT's pseudo-inputs are also linkable: per slave `WcState^WcState`, `InfoData^State` and `InfoData^AdsAddr`, and per master `Inputs^DevState`, `Inputs^SlaveCount`, `Inputs^Frm0State`, `Inputs^Frm0WcState`, `InfoData^AmsNetId` and `InfoData^ChangeCount`.

Every linked leaf is matched to a process image slot. A struct link (`.I1 := TIID^...; .O1 := TIID^...`) produces one binding per member, and member paths may descend into nested structs and function block instances.

**Example** (demo fixtures in `tests/ecat_fixtures`, with `demo_ect.st` copied to `ECT.st`):

```bash
stc ecat validate --io "Demo Device 1.xml" --io "Demo Device 2.xml" demo_types.st ECT.st
# VARIABLE        DIR  MASTER               BYTE.BIT  BITS  LINK
# ECT.A1_01.I1    in   Device 1 (EtherCAT)  0.0       1     TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input
# ECT.V1_C1       out  Device 1 (EtherCAT)  15.0      8     TIID^Device 1 (EtherCAT)^DEMO.V1 (CTEU-EtherCAT Modular)^Module 1 (VAEM-L1-S-8-PT [16DO])^Outputs^C1 Output
# ...
# 35 bindings, 0 errors, 0 warnings
```

**Text output**: a table with columns `VARIABLE`, `DIR` (`in` or `out`), `MASTER`, `BYTE.BIT` (offset in that master's input or output image), `BITS` and `LINK`, followed by one `file:line:col: severity: CODE message` line per diagnostic and a `N bindings, N errors, N warnings` summary.

**JSON output** (`--format json`):

```json
{
  "bindings": [
    {
      "var": "ECT.A1_01.I1",
      "link": "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input",
      "master": "Device 1 (EtherCAT)",
      "dir": "in",
      "byte": 0,
      "bit": 0,
      "bitLen": 1,
      "typeName": "BOOL"
    }
  ],
  "diagnostics": [],
  "images": {
    "Device 1 (EtherCAT)": { "inBytes": 145, "outBytes": 17 },
    "Device 2 (EtherCAT)": { "inBytes": 227, "outBytes": 14 }
  }
}
```

A load or read failure prints `{"error": "..."}` in JSON mode and exits 1.

**Diagnostic codes**:

| Code | Severity | Meaning |
|------|----------|---------|
| `ECAT001` | error | Link target not found in the topology. The message lists up to three nearest child segments. |
| `ECAT002` | error | A `.member` in a multi-member TcLinkTo is not declared by the variable's type, or the member path is deeper than 16 levels. |
| `ECAT003` | error | Size mismatch: the variable's bit width differs from the entry's `BitLen`. |
| `ECAT004` | error | Direction mismatch: `AT %Q*` linked to an input entry, or `AT %I*` linked to an output entry. |
| `ECAT005` | warning | Two variables bind the same slot. Both bindings are kept. |
| `ECAT006` | error | Malformed TcLinkTo value, such as a missing `:=` or mixed single and member forms. |
| `ECAT007` | warning | A linked leaf has no `AT %I*`/`%Q*` declaration, or uses a non-I/O area such as `%M`. |

**Exit codes**: 0 when no error is reported (warnings allowed), 1 on any error or when an export or source file cannot be loaded.

---

### `stc serve`

Run a project's scan and serve its OPC.UA.DA symbols over OPC UA with the
TwinCAT TF6100 address space. See [OPCUA.md](OPCUA.md).

```bash
stc serve <project.tsproj|project.plcproj|file.st|dir ...> [--opcua :4840] [--security none|basic256sha256]
          [--cert FILE --key FILE] [--pki-dir DIR] [--cycle 10ms] [--realtime] [--run-for 0] [-D SYM]
```

Exits 0 on SIGINT/SIGTERM or after `--run-for`; exits 1 when the project
cannot be loaded or instantiated, a flag is invalid, or the listener cannot
start (for example, the port is in use).

## Exit Code Summary

| Code | Meaning |
|------|---------|
| 0 | Success (warnings may still be present) |
| 1 | Errors found (parse errors, type errors, test failures) |

All commands write diagnostics to stderr and primary output (formatted code, JSON, JUnit XML) to stdout.
