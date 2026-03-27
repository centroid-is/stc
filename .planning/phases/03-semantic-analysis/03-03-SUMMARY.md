---
phase: 03-semantic-analysis
plan: 03
subsystem: compiler
tags: [type-checker, two-pass, expression-checking, candidate-resolution, iec-61131-3]

# Dependency graph
requires:
  - phase: 03-semantic-analysis
    provides: "Type system (types.Type, CommonType, CanWiden, BuiltinFunctions) and symbol table (Table, Scope, Symbol)"
provides:
  - "Two-pass type checker: Pass 1 declaration collection, Pass 2 body type checking"
  - "Diagnostic codes for all semantic error categories (SEMA001-SEMA025, VEND001-VEND006)"
  - "Expression type resolution for all AST expression types"
  - "Candidate resolution for overloaded generic functions (ANY_* params)"
  - "FB instance member access resolving inputs/outputs"
  - "Array indexing and struct member access type checking"
affects: [03-04-vendor, 03-05-analyzer, 04-standard-library, 09-lsp]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Two-pass analysis: collect declarations first, type-check bodies second"
    - "Literal type compatibility: integer literals assignable to any integer type"
    - "POU scope navigation via Table.LookupPOU for body checking"
    - "Candidate resolution with max 16 entries to prevent explosion"

key-files:
  created:
    - pkg/checker/diag_codes.go
    - pkg/checker/resolve.go
    - pkg/checker/check.go
    - pkg/checker/candidates.go
    - pkg/checker/resolve_test.go
    - pkg/checker/check_test.go
    - pkg/checker/testdata/valid_program.st
    - pkg/checker/testdata/forward_ref.st
    - pkg/checker/testdata/type_mismatch.st
    - pkg/checker/testdata/undeclared.st
    - pkg/checker/testdata/fb_instance.st
    - pkg/checker/testdata/array_struct.st
  modified: []

key-decisions:
  - "Integer literals (default DINT) compatible with any integer target type; real literals (default LREAL) compatible with any real target"
  - "FB call checking done via manual AST construction in tests since parser does not yet handle FB call syntax with named args"
  - "Forward references handled by resolving POU scope directly via Table.LookupPOU rather than Table.EnterScope"

patterns-established:
  - "Resolver inserts variables directly into POU scope (bypassing scope stack) for clean Pass 1 collection"
  - "Checker navigates to POU scope via LookupPOU for each body check"
  - "astPosToSource helper converts ast.Pos to source.Pos for diagnostics"
  - "isLiteralCompatible for integer/real literal flexibility per IEC conventions"

requirements-completed: [SEMA-01, SEMA-03]

# Metrics
duration: 5min
completed: 2026-03-27
---

# Phase 03 Plan 03: Type Checker Summary

**Two-pass type checker with expression/statement checking, forward reference support, candidate resolution for generic functions, and 20 diagnostic codes**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-27T14:29:15Z
- **Completed:** 2026-03-27T14:34:15Z
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- Two-pass architecture: Pass 1 collects all POU/type/variable declarations into symbol table; Pass 2 type-checks all expression and statement bodies
- Expression type checking for all AST expression types: binary ops, unary ops, function calls, member access, array indexing, deref, parenthesized
- Candidate resolution for built-in functions with ANY_* generic parameters (MATIEC-inspired algorithm with bounded candidate sets)
- 20 tests covering positive and negative cases for each diagnostic code used

## Task Commits

Each task was committed atomically:

1. **Task 1: Diagnostic codes and Pass 1 declaration resolver** - `20cbd64` (feat)
2. **Task 2: Pass 2 type checker for expressions, statements, and calls** - `7c91c71` (feat)

## Files Created/Modified
- `pkg/checker/diag_codes.go` - All SEMA and VEND diagnostic code constants
- `pkg/checker/resolve.go` - Pass 1: declaration collection into symbol table (Resolver, NewResolver, CollectDeclarations)
- `pkg/checker/check.go` - Pass 2: expression/statement type checking (Checker, NewChecker, CheckBodies)
- `pkg/checker/candidates.go` - ANY type candidate enumeration and narrowing (ResolveCandidates)
- `pkg/checker/resolve_test.go` - 4 tests for declaration resolution
- `pkg/checker/check_test.go` - 16 tests for type checking
- `pkg/checker/testdata/valid_program.st` - Simple PROGRAM with INT, REAL, BOOL variables
- `pkg/checker/testdata/forward_ref.st` - Two FBs with forward reference
- `pkg/checker/testdata/type_mismatch.st` - INT := STRING type mismatch
- `pkg/checker/testdata/undeclared.st` - Usage of undeclared variable
- `pkg/checker/testdata/fb_instance.st` - FB declaration + instance member access
- `pkg/checker/testdata/array_struct.st` - Array indexing and struct member access

## Decisions Made
- Integer literals default to DINT per IEC convention, but are assignable to any integer type (SINT through LINT) since the literal value typically fits. Same for real literals (LREAL default, assignable to REAL).
- FB call statement checking uses manual AST construction in tests because the parser does not yet fully handle `fb_instance(param := value)` syntax (parser treats `(` after ident as CallExpr in expression position, conflicting with named arg `:=` syntax). The checker code for CallStmt is complete and tested.
- Resolver inserts variables directly into POU scopes returned by RegisterPOU rather than using Table.EnterScope/ExitScope, avoiding creation of duplicate child scopes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Literal type assignment compatibility**
- **Found during:** Task 2 (type checking)
- **Issue:** `x := 42;` where x is INT failed because literal 42 defaults to DINT and DINT cannot widen to INT
- **Fix:** Added isLiteralCompatible check allowing integer literals to be assigned to any integer type and real literals to any real type
- **Files modified:** pkg/checker/check.go
- **Verification:** TestValidProgram now passes with literal assignments

**2. [Rule 1 - Bug] FB call argument literal compatibility**
- **Found during:** Task 2 (FB call checking)
- **Issue:** Passing integer literal `100` as INT parameter failed because literal defaults to DINT
- **Fix:** Added same literal compatibility check in FB call argument validation
- **Files modified:** pkg/checker/check.go
- **Verification:** TestFBInstanceCall passes

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes necessary for correct IEC literal semantics. No scope creep.

## Issues Encountered
- Parser does not handle FB call syntax `fb(param := value, ...)` as CallStmt because the expression parser greedily consumes `(` as a CallExpr. This is a pre-existing parser limitation, not a checker issue. Tests use manual AST construction to verify CallStmt checking.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all checker functionality is fully implemented per plan scope.

## Next Phase Readiness
- Checker is ready for the analyzer facade (Plan 05) to wire Pass 1 + Pass 2 together
- Vendor-aware diagnostics (Plan 04) can use the same diag_codes.go constants
- BuiltinFunctions candidate resolution ready for standard library expansion (Phase 4)

## Self-Check: PASSED

All 12 files verified present. Both commit hashes verified in git log.

---
*Phase: 03-semantic-analysis*
*Completed: 2026-03-27*
