---
phase: 21-twincat-project-import-library-stubs
plan: 01
subsystem: parser, checker, ast
tags: [twincat, property, fb-call, attribute, pragma, sema039]

requires:
  - phase: 20-twincat-expression-semantics
    provides: SEMA037 undeclared type, attribute parsing, named-arg call parsing
provides:
  - PROPERTY headers accept PUBLIC/PRIVATE/PROTECTED/INTERNAL/ABSTRACT/FINAL in FBs and interfaces
  - statement-level `fb();` and `fb(1, b := x);` checked as FB call statements
  - no SEMA022 cascade when calling a symbol whose type is already Invalid
  - ast.Attribute.DoubleQuoted; double-quoted attribute names are inert and warn SEMA039
  - fmt and emit preserve double-quoted attribute names
affects: [21-02, 21-03, 21-04, 21-05, 21-06, twincat import, sildarvinnsla]

tech-stack:
  added: []
  patterns:
    - "callExprAsStmt: synthesize a CallStmt from a value-less AssignStmt whose target is a CallExpr on an FB instance"
    - "checkDoubleQuotedAttrs: ast.Inspect over each user declaration for per-attribute warnings"

key-files:
  created:
    - pkg/parser/property_modifier_test.go
    - pkg/checker/empty_fb_call_test.go
    - pkg/checker/double_quoted_attr_test.go
    - pkg/format/double_quoted_attr_test.go
  modified:
    - pkg/parser/decl.go
    - pkg/parser/pragma.go
    - pkg/ast/attribute.go
    - pkg/ast/json.go
    - pkg/checker/check.go
    - pkg/checker/diag_codes.go

key-decisions:
  - "Property modifiers are skipped, not stored on the AST"
  - "Double-quoted attribute names are ignored for semantics (assumption A2) and reported once each as SEMA039 warnings, user files only"
  - "Attribute.String re-renders double-quoted attributes with double quotes, so fmt and emit never activate an ignored attribute"
  - "Positional arguments in a CallStmt are type-checked for usage only, not matched to parameters"

patterns-established:
  - "Value-less AssignStmt with CallExpr target on an FB instance routes through checkCallStmt"

requirements-completed: []
requirements-contributed: [IMPT-01, IMPT-04]

duration: 6min
completed: 2026-10-06
---

# Phase 21 Plan 01: TwinCAT import parser and checker fixes Summary

**PROPERTY access modifiers parse, `fb();` checks as an FB call without cascading SEMA022, and double-quoted `{attribute "..."}` names are inert with a SEMA039 warning while fmt keeps the quotes.**

## Performance

- **Duration:** about 6 min of commits (plan read and analysis before that)
- **Started:** 2026-10-06T06:31:00Z
- **Completed:** 2026-10-06T06:38:00Z
- **Tasks:** 3
- **Files modified:** 17 (6 source, 11 test)

## Accomplishments

- `PROPERTY PUBLIC P : INT` and every modifier combination parse in FUNCTION_BLOCK and INTERFACE bodies.
- `fb();`, `fb(1, b := n);` at statement level, in CASE arms and IF branches, check through `checkCallStmt`. Unknown named inputs still give SEMA024.
- Calling an instance of an undeclared FB type reports only SEMA037.
- `{attribute "qualified_only"}` no longer makes a GVL qualified_only. `strict` and `to_string` in double quotes are also inert.
- Each double-quoted attribute in a user file yields one SEMA039 warning. Library files never warn.

## Task Commits

1. **Task 1: PROPERTY access modifiers** - `9139138` (test), `baecc46` (feat)
2. **Task 2: statement-level fb() and Invalid-type calls** - `385dced` (test), `bba66e8` (feat)
3. **Task 3: double-quoted attributes** - `739b388` (test), `4007da7` (feat)

## Files Created/Modified

- `pkg/parser/decl.go` - `skipPropertyModifiers` used by `parseProperty` and `parsePropertySignature`
- `pkg/parser/pragma.go` - `parseAttributeText` returns `doubleQuoted`; node gets `DoubleQuoted`
- `pkg/ast/attribute.go` - `DoubleQuoted` field, double-quote rendering, `HasAttribute` skip
- `pkg/ast/json.go` - `double_quoted: true` in JSON output
- `pkg/checker/check.go` - `isFBInstanceCallee`, `callExprAsStmt`, positional-arg usage, Invalid-type guard in `checkCallExpr`, `checkDoubleQuotedAttrs`
- `pkg/checker/diag_codes.go` - `CodeAttrDoubleQuoted = "SEMA039"`

## Decisions Made

See key-decisions in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Out-of-bounds read in parseAttributeText for `{attribute }`**
- **Found during:** Task 3
- **Issue:** Reading the quote byte at `s[j]` would panic when the keyword is followed only by whitespace.
- **Fix:** Guard with `j < len(s)`. Added test cases for `{attribute }` and `{attribute \t}` (threat T-21-01).
- **Files modified:** pkg/parser/pragma.go, pkg/parser/pragma_test.go
- **Commit:** 4007da7

**2. [Rule 2 - Missing functionality] JSON marshaller did not emit DoubleQuoted**
- **Found during:** Task 3
- **Issue:** `ast/json.go` uses a custom nodeToMap, so the struct tag alone did not make the flag JSON-visible.
- **Fix:** Emit `double_quoted: true` when set; covered in `TestMarshalAttributes`.
- **Files modified:** pkg/ast/json.go, pkg/ast/json_test.go
- **Commit:** 739b388 (test), 4007da7 (feat)

**3. [Rule 1 - Expectation update] Existing tests asserted double-to-single quote canonicalization**
- **Found during:** Task 3
- **Issue:** Format and emit tests and the ECT GVL checker test expected `{attribute "qualified_only"}` to render single-quoted and to check with zero diagnostics.
- **Fix:** Updated expectations to the new behavior. The ECT checker test now asserts no errors and exactly one SEMA039.
- **Files modified:** pkg/format/attribute_test.go, pkg/format/gvl_test.go, pkg/emit/attribute_test.go, pkg/emit/gvl_test.go, pkg/checker/gvl_test.go
- **Commit:** 4007da7

**4. [Rule 2] Positional CallStmt argument values are now checked**
- **Found during:** Task 2
- **Issue:** `checkCallStmt` skipped unnamed args entirely, so `fb(m, b := n)` would leave `m` unused.
- **Fix:** Check the value expression of positional args; the Invalid-type path in `checkCallExpr` also checks its arguments.
- **Commit:** bba66e8

## Issues Encountered

- `tests/twincat_probes/ECT.st` uses `{attribute "qualified_only"}`. Every consumer of that probe now sees one SEMA039 warning. Later plans that assert an empty diagnostic list on this probe must allow it.
- `pkg/checker/gvl_test.go` and several other files were already not gofmt-clean before this plan; left untouched.

## Verification

- `go test ./... -count=1` green.
- `bash scripts/coverage-gate.sh`: parser 98.35%, checker 98.58%, emit 97.32%, all gates PASS.

## Next Phase Readiness

- Library-type resolution plans can rely on `fb();` checking cleanly and on SEMA039 replacing false SEMA033 for the sildarvinnsla GVLs.
- REQUIREMENTS.md not updated: IMPT-01 and IMPT-04 are only partly delivered by this plan.

## Self-Check: PASSED
