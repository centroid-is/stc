---
phase: 03-semantic-analysis
plan: 01
subsystem: types
tags: [iec-61131-3, type-system, widening-lattice, type-checking]

# Dependency graph
requires:
  - phase: 01-parser
    provides: AST node types (TypeSpec interface, NodeKind pattern)
provides:
  - Type interface and TypeKind enum for all IEC elementary types
  - Widening lattice with IEC-strict implicit conversion rules
  - Category membership functions (IsAnyInt, IsAnyReal, IsAnyBit, IsAnyNum)
  - CommonType for finding smallest common supertype
  - Built-in type constants (TypeBOOL through TypeWCHAR)
  - LookupElementaryType for case-insensitive type name resolution
  - BuiltinFunctions registry with 37 standard function signatures
affects: [03-02-symbol-table, 03-03-checker, 03-04-vendor, 03-05-analyzer, 04-standard-library]

# Tech tracking
tech-stack:
  added: []
  patterns: [data-driven-type-lattice, category-membership-via-range-checks, generic-constraint-functions]

key-files:
  created:
    - pkg/types/types.go
    - pkg/types/lattice.go
    - pkg/types/builtin.go
    - pkg/types/types_test.go
    - pkg/types/lattice_test.go
    - pkg/types/builtin_test.go
  modified: []

key-decisions:
  - "IEC-strict widening only: LINT->LREAL rejected (precision loss), no CODESYS permissive mode"
  - "Category membership separate from widening: BOOL in ANY_BIT but not implicitly convertible to BYTE"
  - "GenericConstraint as func(TypeKind) bool on Parameter for ANY_* validation in built-in functions"

patterns-established:
  - "TypeKind range-based constants: signed ints contiguous, unsigned contiguous, enabling range checks for category membership"
  - "Widening rules as map[TypeKind][]TypeKind data table, not if/else chains"
  - "Type constants as package-level vars for identity comparison"

requirements-completed: [SEMA-02]

# Metrics
duration: 4min
completed: 2026-03-27
---

# Phase 03 Plan 01: Type System Summary

**IEC 61131-3 type system with 23 elementary types, data-driven widening lattice, and 37 built-in function signatures**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-27T14:23:03Z
- **Completed:** 2026-03-27T14:27:01Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Complete IEC type system with Type interface, 23 elementary TypeKind constants, and 8 concrete type structs (Primitive, Array, Struct, Enum, FunctionBlock, Function, Pointer, Reference)
- Data-driven widening lattice encoding IEC-strict implicit conversion rules with CommonType finding smallest common supertype
- Built-in function registry with 37 standard function signatures including generic constraints for ANY_NUM/ANY_INT/ANY_REAL validation
- Zero dependencies on other stc packages -- fully self-contained foundation

## Task Commits

Each task was committed atomically:

1. **Task 1: Type system core (TDD RED)** - `8f98e63` (test)
2. **Task 1: Type system core (TDD GREEN)** - `f781231` (feat)
3. **Task 2: Built-in type constants and function signatures** - `e296bc1` (feat)

## Files Created/Modified
- `pkg/types/types.go` - Type interface, TypeKind enum (23 elementary + composite kinds), concrete type structs
- `pkg/types/lattice.go` - Widening rules map, CanWiden, CommonType, category membership functions
- `pkg/types/builtin.go` - Type constants (TypeBOOL..TypeWCHAR), LookupElementaryType, BuiltinFunctions registry
- `pkg/types/types_test.go` - 15 tests covering type kinds, interface compliance, equality, all concrete types
- `pkg/types/lattice_test.go` - 18 tests covering widening rules, CommonType, category membership
- `pkg/types/builtin_test.go` - 15 tests covering type constants, lookup, case-insensitivity, function signatures

## Decisions Made
- IEC-strict widening only: LINT->LREAL rejected due to precision loss (64-bit int cannot fit precisely in 64-bit float). This matches the locked user decision.
- Category membership is separate from widening: BOOL is in ANY_BIT per IEC hierarchy but cannot be implicitly widened to BYTE. This is tracked via IsAnyBit() for category checks and wideningRules for conversion.
- GenericConstraint stored as `func(TypeKind) bool` on Parameter struct, allowing the type checker to validate arguments against ANY_NUM, ANY_INT, ANY_REAL categories without encoding category names as strings.
- TypeKind constants are arranged contiguously by category (signed ints KindSINT..KindLINT, unsigned KindUSINT..KindULINT) enabling efficient range-based category membership checks.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all types, lattice rules, and function signatures are fully implemented.

## Next Phase Readiness
- Type system is ready for the symbol table (03-02) to use as the type representation for symbols
- Checker (03-03) can use CanWiden/CommonType for expression type checking
- BuiltinFunctions registry provides signatures for standard function call validation

## Self-Check: PASSED

All 6 files verified present. All 3 commit hashes verified in git log.

---
*Phase: 03-semantic-analysis*
*Completed: 2026-03-27*
