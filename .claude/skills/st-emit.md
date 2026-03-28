# ST Emit Skill

Emit vendor-specific Structured Text from portable ST source.

## When to Use

- Preparing ST code for deployment to a specific PLC vendor
- Converting portable ST to vendor-specific dialect
- Verifying cross-vendor compatibility of ST source
- Generating vendor-flavored ST for import into vendor IDEs

## Workflow

### 1. Validate Source First

Always validate before emitting:

```bash
stc check <file> --format json
```

Fix any errors before proceeding to emission.

### 2. Emit for Target Vendor

```bash
stc emit <file> --target <vendor> --format json
```

### Vendor Targets

| Target | Flag | Features Supported | Restrictions |
|--------|------|--------------------|--------------|
| **Beckhoff** | `--target beckhoff` | Full CODESYS OOP, pointers (`POINTER TO`), references (`REFERENCE TO`), 64-bit types (`LINT`, `LREAL`, `LWORD`) | TwinCAT-specific pragmas |
| **Schneider** | `--target schneider` | Standard IEC types, timers, counters | No OOP (classes, interfaces, methods), no pointers/references |
| **Portable** | `--target portable` | Safest cross-vendor subset | No OOP, no pointers, no references, no 64-bit types |

### 3. Review Emitted Code

After emission, review for vendor-specific differences:

- **Pragmas/attributes**: Vendor-specific `{attribute}` or `(*pragma*)` syntax
- **Type mapping**: 64-bit types may be downcast or rejected depending on target
- **OOP constructs**: Methods, interfaces, and inheritance removed for non-Beckhoff targets
- **Variable qualifiers**: `POINTER TO` and `REFERENCE TO` stripped for Schneider/portable

### 4. Round-Trip Verification

To confirm emission stability, parse the emitted code back:

```bash
# Emit to vendor target
stc emit source.st --target beckhoff --format json > emitted.json

# Parse emitted code to verify it's valid
stc parse emitted_output.st --format json
```

## Expected JSON Output

### Successful Emission
```json
{
  "file": "motor_control.st",
  "target": "beckhoff",
  "output_file": "motor_control_beckhoff.st",
  "diagnostics": [],
  "filtered": []
}
```

### Emission with Filtered Constructs
```json
{
  "file": "motor_control.st",
  "target": "schneider",
  "output_file": "motor_control_schneider.st",
  "diagnostics": [
    {"severity": "warning", "message": "POINTER TO removed: not supported on schneider target"}
  ],
  "filtered": ["POINTER TO", "METHOD declarations"]
}
```

## Common Patterns

### Write Once, Emit for Each Target

The recommended workflow is to write portable ST and emit for each vendor separately:

```bash
# Write portable code, then emit for each target
stc emit myprogram.st --target beckhoff --format json
stc emit myprogram.st --target schneider --format json
stc emit myprogram.st --target portable --format json
```

### Vendor-Specific Validation Before Emission

```bash
# Check compatibility with target vendor before emitting
stc check myprogram.st --vendor beckhoff --format json
stc emit myprogram.st --target beckhoff --format json
```

## Error Handling

- **Unsupported construct for target**: The emitter warns and filters. Review `filtered` array in output.
- **Parse errors in source**: Emission requires a valid AST. Run `stc parse` first.
- **Type errors in source**: Emission may produce invalid output. Run `stc check` first.
- **Round-trip failure**: If emitted code fails to parse, report as a bug -- emitted ST should always be valid.
- **Default target**: If no `--target` specified, defaults to `portable` (safest cross-vendor subset).
