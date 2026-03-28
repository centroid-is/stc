# ST Generate Skill

Generate IEC 61131-3 Structured Text code from natural language descriptions.

## When to Use

- User asks to create a new PLC program, function block, or function
- User describes control logic in natural language
- User needs ST scaffolding for a specific automation task

## Workflow

### 1. Understand Intent

Determine what POU type the user needs:

| POU Type | Use When |
|----------|----------|
| `PROGRAM` | Main logic, one instance per task, direct I/O access |
| `FUNCTION_BLOCK` | Reusable stateful component, multiple instances allowed |
| `FUNCTION` | Pure computation, no state, returns a single value |

### 2. Generate ST Code

Follow IEC 61131-3 Ed.3 conventions:

- **Keywords**: UPPERCASE (`PROGRAM`, `FUNCTION_BLOCK`, `VAR`, `END_VAR`, `IF`, `THEN`, `END_IF`, etc.)
- **Indentation**: 4 spaces
- **Variables**: Declare ALL variables in `VAR` blocks before use
- **POU names**: PascalCase (e.g., `MotorController`, `CalculateSpeed`)
- **Variable names**: camelCase (e.g., `motorSpeed`, `isRunning`)

### 3. Standard Library Reference

**Standard Function Blocks** (stateful, need instances):
- `TON` - On-delay timer (IN, PT -> Q, ET)
- `TOF` - Off-delay timer (IN, PT -> Q, ET)
- `TP` - Pulse timer (IN, PT -> Q, ET)
- `CTU` - Count up (CU, RESET, PV -> Q, CV)
- `CTD` - Count down (CD, LOAD, PV -> Q, CV)
- `R_TRIG` - Rising edge detect (CLK -> Q)
- `F_TRIG` - Falling edge detect (CLK -> Q)
- `SR` - Set-dominant bistable (S1, R -> Q1)
- `RS` - Reset-dominant bistable (S, R1 -> Q1)

**Standard Functions** (pure, no state):
- Math: `ABS`, `SQRT`, `MIN`, `MAX`
- String: `LEN`, `CONCAT`, `LEFT`, `RIGHT`, `MID`, `FIND`
- Conversion: `INT_TO_REAL`, `REAL_TO_INT`, `BOOL_TO_INT`, etc.

### 4. Validate Generated Code

After writing the `.st` file, immediately validate:

```bash
# Step 1: Parse - check for syntax errors
stc parse <file> --format json

# Step 2: Type check - check for type errors, undeclared variables
stc check <file> --format json
```

### 5. Fix Any Errors

If parse errors occur:
- Check for missing `END_*` keywords
- Verify variable declarations precede usage
- Ensure semicolons terminate statements

If type errors occur:
- Check variable types match operations
- Verify function call argument types
- Ensure assignments have compatible types

## Expected JSON Output

### Parse Success
```json
{
  "file": "example.st",
  "pous": [{"name": "MotorControl", "kind": "FUNCTION_BLOCK"}],
  "diagnostics": []
}
```

### Parse Error
```json
{
  "file": "example.st",
  "diagnostics": [
    {"file": "example.st", "line": 10, "col": 5, "severity": "error", "message": "expected END_IF"}
  ]
}
```

## Error Handling

- **File not found**: Verify the path and `.st` extension
- **Parse errors**: Fix syntax issues, re-run `stc parse`
- **Type errors**: Fix type mismatches, re-run `stc check`
- **Multiple POUs**: Each POU in its own file or all in one file -- both are valid
