---
phase: 01-project-bootstrap-parser
plan: 02
subsystem: parser
tags: [ast, cst, iec-61131-3, codesys, oop, json, visitor-pattern, trivia]

# Dependency graph
requires: []
provides:
  - "Complete pkg/ast/ package with 40+ node types for IEC 61131-3 Ed.3 + CODESYS OOP"
  - "Node interface with NodeBase embedding trivia for CST fidelity"
  - "JSON marshaling with kind discriminator for all polymorphic node types"
  - "Visitor pattern (Walk, Inspect) for AST traversal"
  - "All VAR section kinds (VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_TEMP, VAR_GLOBAL, VAR_ACCESS, VAR_EXTERNAL, VAR_CONFIG)"
  - "Type specifiers: NamedType, ArrayType, PointerType, ReferenceType, StringType, SubrangeType, EnumType, StructType"
affects: [parser, cli, semantic-analysis, formatter, lsp]

# Tech tracking
tech-stack:
  added: [encoding/json, stretchr/testify]
  patterns: [marker-interfaces, embedded-nodebase, visitor-pattern, trivia-attachment, kind-discriminator-json]

key-files:
  created:
    - pkg/ast/node.go
    - pkg/ast/trivia.go
    - pkg/ast/visitor.go
    - pkg/ast/decl.go
    - pkg/ast/stmt.go
    - pkg/ast/expr.go
    - pkg/ast/types.go
    - pkg/ast/var.go
    - pkg/ast/json.go
    - pkg/ast/json_test.go
  modified: []

key-decisions:
  - "Local Pos/Span types in ast package to avoid circular imports with future source package"
  - "Marker interfaces (Declaration, Statement, Expr, TypeSpec) for type-safe node categorization"
  - "JSON marshaling via nodeToMap dispatch rather than per-type MarshalJSON to keep kind discriminator consistent"
  - "Ident implements Expr interface (used as expression in assignments and member access)"

patterns-established:
  - "NodeBase embedding: all nodes embed NodeBase for kind, span, and trivia"
  - "Children() []Node: every node returns its child nodes for generic traversal"
  - "Marker interfaces with private methods (declNode(), stmtNode(), exprNode(), typeSpecNode())"
  - "JSON kind discriminator: every serialized node has a 'kind' field with its NodeKind string"

requirements-completed: [PARS-01, PARS-02, PARS-03, PARS-04, PARS-08, PARS-09]

# Metrics
duration: 5min
completed: 2026-03-26
---

# Phase 01 Plan 02: AST Node Types Summary

**Complete IEC 61131-3 + CODESYS OOP AST/CST node types with trivia attachment, visitor pattern, and JSON marshaling**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-26T16:35:53Z
- **Completed:** 2026-03-26T16:40:25Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments

- Defined 40+ AST node types covering all IEC 61131-3 Ed.3 POU types (PROGRAM, FUNCTION_BLOCK, FUNCTION, TYPE, INTERFACE) and CODESYS OOP (METHOD, PROPERTY, EXTENDS, IMPLEMENTS)
- Every node carries LeadingTrivia/TrailingTrivia via NodeBase for lossless CST round-tripping
- JSON marshaling produces discriminated output with "kind" field on every node; all 6 tests pass
- Visitor pattern (Walk, Inspect) enables generic AST traversal for analysis and transformation

## Task Commits

Each task was committed atomically:

1. **Task 1: Create AST node base, trivia, and visitor infrastructure** - `2eea991` (feat)
2. **Task 2: Create all declaration, statement, expression, type spec, and var nodes with JSON marshaling and tests** - `139a1f0` (feat)

## Files Created/Modified

- `pkg/ast/node.go` - Node interface, NodeBase, NodeKind enum (40+ kinds), Pos/Span, marker interfaces, ErrorNode, Ident
- `pkg/ast/trivia.go` - TriviaKind (Whitespace, LineComment, BlockComment) and Trivia struct
- `pkg/ast/visitor.go` - Visitor interface, Walk, Inspect traversal functions
- `pkg/ast/decl.go` - Declaration nodes: SourceFile, ProgramDecl, FunctionBlockDecl, FunctionDecl, InterfaceDecl, MethodDecl, PropertyDecl, MethodSignature, PropertySignature, TypeDecl, ActionDecl, AccessModifier
- `pkg/ast/stmt.go` - Statement nodes: AssignStmt, CallStmt, CallArg, IfStmt, ElsIf, CaseStmt, CaseBranch, CaseLabel, CaseLabelValue, CaseLabelRange, ForStmt, WhileStmt, RepeatStmt, ReturnStmt, ExitStmt, ContinueStmt, EmptyStmt
- `pkg/ast/expr.go` - Expression nodes: BinaryExpr, UnaryExpr, Literal, LiteralKind, CallExpr, MemberAccessExpr, IndexExpr, DerefExpr, ParenExpr, Token
- `pkg/ast/types.go` - Type spec nodes: NamedType, ArrayType, SubrangeSpec, PointerType, ReferenceType, StringType, SubrangeType, EnumType, EnumValue, StructType, StructMember
- `pkg/ast/var.go` - VarSection (9 kinds), VarBlock, VarDecl, PragmaNode
- `pkg/ast/json.go` - MarshalNode, nodeToMap dispatch, custom MarshalJSON for enums
- `pkg/ast/json_test.go` - 6 tests: SourceFile, BinaryExpr, OOP, ArrayType/PointerType/StructType/EnumType, Trivia, Walk/Inspect

## Decisions Made

- Local Pos/Span types defined in ast package to avoid circular dependency with source package (will be reconciled in Plan 03)
- Ident implements Expr interface since identifiers appear in expression contexts (assignments, member access, function calls)
- JSON marshaling uses centralized nodeToMap dispatch rather than per-type MarshalJSON methods for consistent kind discriminator
- Marker interfaces use private methods (declNode/stmtNode/exprNode/typeSpecNode) to prevent external implementation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added missing PropertySignature type**
- **Found during:** Task 2 (decl.go compilation)
- **Issue:** InterfaceDecl referenced PropertySignature type which was listed in plan but implementation was not included in the initial decl.go write
- **Fix:** Added PropertySignature struct with Name, Type fields, Children(), and declNode() marker
- **Files modified:** pkg/ast/decl.go
- **Verification:** go build ./pkg/ast/... succeeds
- **Committed in:** 139a1f0 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor omission caught at compile time. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all node types are fully defined with Children() and interface implementations.

## Next Phase Readiness

- Complete AST type system ready for parser (Plan 04) to produce and CLI (Plan 05) to consume
- JSON marshaling ready for `stc parse --format json` output
- Visitor pattern ready for semantic analysis (Phase 3) and future transformations

---
*Phase: 01-project-bootstrap-parser*
*Completed: 2026-03-26*
