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

### `stc vendor extract`

Extract function block stubs from TwinCAT project files.

```
stc vendor extract <path.plcproj> [flags]
```

**Arguments**: Path to a TwinCAT `.plcproj` file.

**Flags**:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | (stdout) | Output directory for extracted `.st` files |

Parses the `.plcproj` XML, finds all referenced `.TcPOU` files, and extracts `FUNCTION_BLOCK` declarations without implementation bodies.

**Example**:
```bash
# Extract to stdout
stc vendor extract MyProject.plcproj

# Extract to directory
stc vendor extract MyProject.plcproj --output vendor/custom/
# Extracted: FB_Motor -> vendor/custom/FB_Motor.st
# Extracted: FB_Valve -> vendor/custom/FB_Valve.st
# 2 POU(s) extracted to vendor/custom/
```

**Exit codes**: 0 on success, 1 on error.

---

## Exit Code Summary

| Code | Meaning |
|------|---------|
| 0 | Success (warnings may still be present) |
| 1 | Errors found (parse errors, type errors, test failures) |

All commands write diagnostics to stderr and primary output (formatted code, JSON, JUnit XML) to stdout.
