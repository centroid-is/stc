# ST Validate Skill

Validate ST code through the full parse + check + lint pipeline.

## When to Use

- After writing or modifying any `.st` file
- Before committing ST code changes
- When debugging ST compilation errors
- As part of CI/CD validation

## Workflow

Run all 3 stages in sequence. Only stop on parse errors (stages 2-3 require a valid AST).

### Stage 1: Parse

Check for syntax errors:

```bash
stc parse <file> --format json
```

**Exit codes:** 0 = no errors, 1 = parse errors found

If parse errors exist, fix them before proceeding. Common parse errors:
- Missing `END_*` keywords (e.g., `END_IF`, `END_VAR`, `END_PROGRAM`)
- Missing semicolons at end of statements
- Undeclared variable blocks
- Malformed expressions

### Stage 2: Check

Check for semantic errors (types, declarations, usage):

```bash
stc check <file> --format json
```

**Exit codes:** 0 = no errors (warnings OK), 1 = type/semantic errors found

Catches:
- Undeclared variables
- Type mismatches in assignments and expressions
- Unused variable warnings
- Invalid function/FB call arguments

For vendor-specific checking:
```bash
stc check <file> --vendor beckhoff --format json
stc check <file> --vendor schneider --format json
```

### Stage 3: Lint

Check for coding standard violations:

```bash
stc lint <file> --format json
```

**Exit codes:** 0 = clean or warnings only (lint warnings do not cause exit 1)

Catches:
- Magic numbers (use named constants)
- Excessive nesting depth
- Overly long POUs
- Naming convention violations

## Interpreting Results

Results have three severity levels:

| Severity | Action | Example |
|----------|--------|---------|
| `error` | Must fix | Type mismatch, undeclared variable |
| `warning` | Should fix | Unused variable, magic number |
| `info` | Consider | Naming suggestion, style preference |

## Expected JSON Output

### Clean Validation
```json
{
  "file": "example.st",
  "diagnostics": [],
  "summary": {"errors": 0, "warnings": 0, "info": 0}
}
```

### Validation with Issues
```json
{
  "file": "example.st",
  "diagnostics": [
    {"file": "example.st", "line": 5, "col": 12, "severity": "error", "code": "SEMA001", "message": "undeclared variable 'x'"},
    {"file": "example.st", "line": 8, "col": 1, "severity": "warning", "code": "LINT001", "message": "magic number 42; use a named constant"}
  ]
}
```

## Full Pipeline Example

```bash
# Run all 3 stages
stc parse myprogram.st --format json && \
stc check myprogram.st --format json && \
stc lint myprogram.st --format json
```

Always run all 3 stages even if earlier stages produce warnings. Only stop the pipeline if `stc parse` returns errors (exit code 1), since check and lint require a valid AST.

## Error Handling

- **File not found**: Verify path and `.st` extension
- **Parse errors block pipeline**: Fix syntax first, then re-run full pipeline
- **Vendor-specific errors**: Use `--vendor` flag on `stc check` to catch vendor compatibility issues early
- **Multiple files**: Run pipeline on each file individually, or use directory mode if supported

---
**MCP equivalents:** parse→`stc_parse`, check→`stc_check`, lint→`stc_lint`
