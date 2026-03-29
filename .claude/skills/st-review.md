# ST Review Skill

Review ST code against IEC 61131-3 best practices and PLCopen guidelines.

## When to Use

- Before merging ST code changes
- When auditing existing ST code quality
- After generating ST code (pair with st-generate skill)
- Periodic codebase health checks

## Workflow

### 1. Automated Lint Check

Run the linter for automated rule checking:

```bash
stc lint <file> --format json
```

The linter checks for:

| Rule | Description | Severity |
|------|-------------|----------|
| Magic numbers | Numeric literals used directly instead of named constants | warning |
| Nesting depth | Control flow nesting exceeds 3 levels | warning |
| POU length | POU body exceeds 200 lines | warning |
| Missing return type | FUNCTION without explicit return type | error |
| Naming conventions | POU names not PascalCase (configurable) | warning |

### 2. Review Lint Results

Interpret diagnostics by severity:

- **error**: Must fix before deployment
- **warning**: Should fix for code quality
- **info**: Consider for consistency

### 3. Manual Review Checklist

Beyond what the linter catches, manually review:

**Interface Design:**
- [ ] Are `VAR_INPUT` / `VAR_OUTPUT` clearly defined for function block interfaces?
- [ ] Are input/output variable names descriptive and self-documenting?
- [ ] Are `VAR_IN_OUT` used appropriately (only when caller's variable must be modified)?

**Constants and Magic Numbers:**
- [ ] Are timer presets using named constants, not magic numbers?
- [ ] Are threshold values defined as constants with meaningful names?
- [ ] Are array bounds using named constants?

**Error Handling:**
- [ ] Is error handling present for edge cases (divide by zero, out of range, etc.)?
- [ ] Are fault states properly handled in state machines?
- [ ] Do timers have reasonable timeout values?

**Code Clarity:**
- [ ] Are comments present for non-obvious logic?
- [ ] Is complex boolean logic broken into named intermediate variables?
- [ ] Are state machine states documented with their transitions?

**Modularity:**
- [ ] Could any logic be extracted into reusable function blocks?
- [ ] Are function blocks focused on a single responsibility?
- [ ] Is the code DRY (no duplicated logic)?

### 4. Vendor-Specific Review

If deploying to a specific vendor, run vendor-targeted type checking:

```bash
stc check <file> --vendor beckhoff --format json
stc check <file> --vendor schneider --format json
```

This catches vendor-specific compatibility issues:
- Schneider: No OOP constructs, no pointers/references
- Beckhoff: Verify TwinCAT-specific pragmas are correct
- Portable: Ensure no vendor-specific extensions used

### 5. Summarize Findings

Organize findings by severity with actionable fix suggestions:

```
## Review Summary

### Errors (must fix)
1. [file:line] Missing return type on FUNCTION CalcSpeed
   Fix: Add `: REAL` return type to function declaration

### Warnings (should fix)
1. [file:line] Magic number 3600 in timer preset
   Fix: Define `SECONDS_PER_HOUR : INT := 3600` as a constant
2. [file:line] Nesting depth 4 exceeds recommended maximum of 3
   Fix: Extract inner logic into a helper function

### Suggestions (consider)
1. MotorController FB is 180 lines -- consider splitting into sub-FBs
2. State machine in MainProgram lacks transition documentation
```

## Expected JSON Output

### Clean Lint
```json
{
  "file": "example.st",
  "diagnostics": [],
  "summary": {"errors": 0, "warnings": 0, "info": 0}
}
```

### Lint with Findings
```json
{
  "file": "example.st",
  "diagnostics": [
    {"file": "example.st", "line": 15, "col": 20, "severity": "warning", "code": "LINT001", "message": "magic number 42; use a named constant"},
    {"file": "example.st", "line": 30, "col": 1, "severity": "warning", "code": "LINT003", "message": "POU body exceeds 200 lines (found 245)"}
  ]
}
```

## Error Handling

- **File not found**: Verify path and `.st` extension
- **Parse errors prevent linting**: Fix syntax errors first with `stc parse`, then re-run lint
- **Vendor check not needed**: Skip step 4 if writing portable-only code
- **Large codebase**: Run lint on changed files first, then expand to full codebase review

---
**MCP equivalents:** lint→`stc_lint`, check→`stc_check`
