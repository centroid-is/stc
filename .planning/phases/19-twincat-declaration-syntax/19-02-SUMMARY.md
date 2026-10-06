---
phase: 19-twincat-declaration-syntax
plan: 02
subsystem: ast
tags: [ast, json, twincat, attributes, gvl, actions]

requires: []
provides:
  - ast.Attribute with canonical single-quote String() and HasAttribute
  - ast.GVLDecl plus SanitizeGVLName and SetGVLName
  - KindGVLDecl, KindAttribute, KindPragma (appended after KindVarDecl)
  - Attributes/Pragmas on VarBlock, VarDecl, StructMember, EnumValue, TypeDecl and all POU/method/property/interface/action decls
  - Actions on ProgramDecl and FunctionBlockDecl; AtAddress on StructMember
  - JSON for attributes, pragmas, GVLDecl, actions, struct at_address and CallStmt args
affects: [19-03, 19-04, 19-05, 19-06, 19-07, 19-08, 19-09, 19-10]

tech-stack:
  added: []
  patterns:
    - "Every node with Attributes/Pragmas calls appendAttrs first in Children() so leading comments attach to the Attribute node"
    - "New NodeKinds are only ever appended; TestNodeKind_StableValues pins KindVarDecl == 40"
    - "Pragma and Attribute JSON kind strings are forced in nodeToMap, independent of NodeBase.NodeKind"

key-files:
  created:
    - pkg/ast/attribute.go
    - pkg/ast/attribute_test.go
    - .planning/phases/19-twincat-declaration-syntax/deferred-items.md
  modified:
    - pkg/ast/node.go
    - pkg/ast/var.go
    - pkg/ast/decl.go
    - pkg/ast/types.go
    - pkg/ast/json.go
    - pkg/ast/json_test.go
    - pkg/ast/json_marshal_test.go
    - pkg/ast/node_test.go

key-decisions:
  - "Attribute.HasValue is json:\"-\"; JSON goes through nodeToMap, which emits value only when HasValue, so an empty-but-present value still emits \"value\": \"\""
  - "Attribute.String() quotes the name as well as the value with '' doubling, so neither can break out of the pragma"
  - "VarBlock also carries Attributes/Pragmas (CONTEXT discretion item) so a pragma before a later VAR_GLOBAL block round-trips"
  - "SetGVLName creates the Name ident when a GVLDecl has none, rather than skipping it"
  - "SanitizeGVLName maps each non-ASCII rune to a single '_' (identifier charset is ASCII only)"

patterns-established:
  - "marshalAttrs(m, attrs, pragmas) and marshalActions(m, actions) only set keys when non-empty, keeping JSON for non-TwinCAT files unchanged"

requirements-completed: []  # contracts only; DIAL-01/02/03/05/08 are delivered end to end by plans 19-03..19-10

duration: 6min
completed: 2026-10-05
---

# Phase 19 Plan 02: AST contracts for TwinCAT declaration syntax Summary

**Attribute and GVLDecl nodes, three appended NodeKinds, attribute/pragma/action/AT fields on every declaration node, and JSON output for all of them, with pre-existing parse JSON byte-identical apart from the new CallStmt args.**

## Performance

- **Duration:** about 6 min
- **Completed:** 2026-10-05
- **Tasks:** 2 of 2
- **Files created:** 3, modified: 8

## Accomplishments

- `pkg/ast/attribute.go` defines `Attribute`, `GVLDecl`, `HasAttribute`, `SanitizeGVLName`, `SetGVLName` and the shared `appendAttrs` Children helper.
- `KindGVLDecl`, `KindAttribute` and `KindPragma` come after `KindVarDecl`, so no existing numeric kind moved.
- Twelve node types gained `Attributes` and `Pragmas`. `ProgramDecl` and `FunctionBlockDecl` gained `Actions`, and `StructMember` gained `AtAddress`. All Children() lists start with attributes and pragmas and end with actions.
- `nodeToMap` emits `attributes`, `pragmas`, `actions`, `blocks`, struct `at_address` and `CallStmt.args`. Empty call arguments have no `value` key. `PragmaNode` now reports `"kind": "Pragma"` instead of the zero kind `SourceFile`.

## Task Commits

1. **Task 1: Attribute, GVLDecl, NodeKinds and new fields**
   - `dfbf92e` test(19-02): failing tests (RED)
   - `184aed7` feat(19-02): implementation (GREEN)
2. **Task 2: JSON marshalling**
   - `507aa4a` test(19-02): failing tests (RED)
   - `f6a07ff` feat(19-02): implementation (GREEN), plus deferred-items.md

## Verification

- `go build ./...` and `go test ./... -count=1` pass.
- `bash scripts/coverage-gate.sh` passes with gated packages unchanged: parser 95.88%, lexer 97.44%, checker 94.77%, interp 96.10%, types 100%, emit 95.55%, total 93.13%.
- I compared `stc parse --format json` from the pre-plan commit `4e82466` against the new binary on all 120 tracked `.st` files. With `CallStmt.args` stripped, the outputs are identical (0 diffs). Without stripping, only 3 files differ, and only by the new `args` key.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pragma JSON kind was the zero kind**
- **Found during:** Task 2
- **Issue:** `PragmaNode` had no NodeKind, so JSON reported `"kind": "SourceFile"`.
- **Fix:** nodeToMap forces `"Pragma"` and `"Attribute"` for those two nodes, whatever NodeKind is set. The stale expectation comment in `json_marshal_test.go` was updated to match.
- **Files modified:** pkg/ast/json.go, pkg/ast/json_marshal_test.go
- **Commit:** f6a07ff

### Other notes

- **Pre-existing misformatting:** running gofmt on the touched files also realigned some struct fields and the `KindTestCaseDecl` entry in node.go/decl.go/types.go/var.go. The changes are whitespace only.
- **Acceptance grep:** the plan's grep for `Actions \[\]\*ActionDecl` needs `Actions +\[\]` because gofmt aligns columns. Both fields exist, at decl.go lines 51 and 85.
- **Extra test file:** `json_marshal_test.go` was touched even though it is not listed in files_modified. The change is comment and expectation text only.

## Issues Encountered

None blocking.

## Deferred Issues

- CallArg, ElsIf, CaseBranch, CaseLabelValue/Range, SubrangeSpec, EnumValue and StructMember have no NodeKind, so their JSON shows `"kind": "SourceFile"`. This was already true on `CallExpr.args`, and it now also shows on the new `CallStmt.args`. Fixing it needs NodeKinds appended to the enum plus parser wiring. Details are in deferred-items.md.

## Known Stubs

None. The new fields are intentionally unpopulated until the parser wiring lands: attributes in 19-03, struct AT and actions in 19-04, GVL in 19-05, and empty args in 19-07.

## Next Phase Readiness

Plans 19-03 onward can fill these fields directly. Printers in `pkg/format` and `pkg/emit` should use `Attribute.String()` for canonical output.

## Self-Check: PASSED

All created files and all four task commits verified present.
