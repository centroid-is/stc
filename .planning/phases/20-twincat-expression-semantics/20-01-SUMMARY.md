---
phase: 20-twincat-expression-semantics
plan: 01
subsystem: ast
tags: [ast, json, format, emit, lint, twincat, enums]

requires:
  - phase: 19-twincat-declaration-syntax
    provides: "Attribute/GVLDecl nodes, appended-NodeKind convention, nil-valued CallArg printing"
provides:
  - "ast.BitAccessExpr{Target, Index}, ast.ThisExpr, ast.SuperExpr (all Expr)"
  - "ast.StructInit{Fields []*FieldInit}, ast.FieldInit{Name, Value}, ast.ArrayInit{Elements []*ArrayInitElem}, ast.ArrayInitElem{Count, Value} (StructInit/ArrayInit are Expr)"
  - "ast.RefAssignStmt{Target, Value} (Statement)"
  - "CallExpr.NamedArgs []*CallArg (json named_args), NamedType.Namespace *Ident (json namespace), TypeDecl.InitValue Expr (json init_value)"
  - "KindBitAccessExpr, KindThisExpr, KindSuperExpr, KindRefAssignStmt, KindCallArg, KindStructInit, KindFieldInit, KindArrayInit, KindArrayInitElem appended after KindPragma"
  - "ast.EnumOrdinals(*EnumType) []EnumOrdinal{Name, Value int64, Known bool}: shared previous+1 numbering"
  - "fmt/emit printers and lint walker cases for every new node"
affects: [20-02, 20-03, 20-04, 20-05, 20-06, 20-07, 20-09]

tech-stack:
  added: []
  patterns:
    - "JSON kind strings for every Phase 20 node and CallArg are forced in nodeToMap, so a node built without NodeKind never serialises as SourceFile"
    - "fmt and emit share one emitCallArg helper for CallStmt.Args and CallExpr.NamedArgs"
    - "TYPE defaults print via emitTypeInit, enum base types via emitEnumBase, in both printers"

key-files:
  created:
    - pkg/ast/enum_ordinals.go
    - pkg/ast/enum_ordinals_test.go
    - pkg/ast/phase20_nodes_test.go
    - pkg/format/phase20_nodes_test.go
    - pkg/emit/phase20_nodes_test.go
    - pkg/lint/phase20_nodes_test.go
  modified:
    - pkg/ast/node.go
    - pkg/ast/expr.go
    - pkg/ast/stmt.go
    - pkg/ast/types.go
    - pkg/ast/decl.go
    - pkg/ast/json.go
    - pkg/format/format.go
    - pkg/emit/emit.go
    - pkg/lint/plcopen.go

key-decisions:
  - "EnumOrdinals propagates Known=false to implicit successors of a non-computable value; Value still holds the positional guess (last known + 2 for the next entry)"
  - "CallArg JSON kind is forced to CallArg, which also changes CallStmt.args output from the old SourceFile kind"
  - "Lint walker skips a literal BitAccessExpr index, so w.3 is not flagged as a magic number; a constant-name index is still visited"
  - "emit prints Phase 20 nodes identically for beckhoff, schneider and portable; no vendor stripping in this phase"

patterns-established:
  - "Phase 20 parser plans set NodeKind on new nodes, but JSON does not depend on it"

requirements-completed: []  # contracts only; DIAL-04/06/07/09/10 are delivered end to end by plans 20-02..20-09

duration: 10min
completed: 2026-10-06
---

# Phase 20 Plan 01: AST contracts for TwinCAT expression semantics Summary

**Nine new AST node kinds, named call arguments, namespace types and TYPE defaults, with JSON, fmt, emit and lint support, plus one shared enum numbering helper that implements the previous+1 rule with based and typed literals.**

## Performance

- **Duration:** about 10 min
- **Started:** 2026-10-06T01:35Z
- **Completed:** 2026-10-06T01:45Z
- **Tasks:** 2 of 2
- **Files created:** 6, modified: 9

## Accomplishments

- `pkg/ast/expr.go` defines `BitAccessExpr`, `ThisExpr`, `SuperExpr`, `StructInit`, `FieldInit`, `ArrayInit` and `ArrayInitElem`. `CallExpr` gained `NamedArgs`, which `Children()` includes after `Args`.
- `pkg/ast/stmt.go` defines `RefAssignStmt`. `NamedType` gained `Namespace`, listed first in `Children()`. `TypeDecl` gained `InitValue`, listed after `Type`.
- The nine new NodeKinds come after `KindPragma`, so no existing numeric value moved.
- `nodeToMap` emits `named_args`, `namespace` and `init_value` only when set. Positional-only `CallExpr` JSON is unchanged.
- `ast.EnumOrdinals` numbers values by the previous+1 rule. It handles `2#`, `8#` and `16#` bases, underscores, a leading sign, parentheses, and typed literals such as `UINT#5` and `BYTE#16#10`. Overflow and malformed input give `Known=false` and never panic.
- fmt and emit print every construct in TwinCAT form. Before this plan, both printers dropped the enum base type.
- The lint walker visits the children of every new node.

Sample fmt output from the hand-built test AST:

```
TYPE E :
(
    a := 0,
    b
) UINT := b;
END_TYPE

TYPE T_Count :
INT := 5;
END_TYPE

PROGRAM Main
VAR
    s : ST_X := (a := 1, b := 'x');
    arr : ARR_T := [1, 2, 3(0), (a := 1)];
    m : MAT_T := [[1, 2], [3, 4]];
    ec : Tc2_EtherCAT.ST_EcSlaveState;
    e : (a := 0, b := 1) UINT;
END_VAR
    q := w.3;
    a.b.0 := arr[0].3;
    n := F(1, b := 2, c => x);
    k := G(a :=, 7);
    r REF= x; // rebind
    THIS^.x := 1;
    SUPER^.M();
END_PROGRAM
```

## Task Commits

1. **Task 1: AST nodes, fields, kinds, JSON and enum numbering**
   - `33764c6` test(20-01): failing tests (RED)
   - `243ef0d` feat(20-01): implementation (GREEN)
2. **Task 2: fmt, emit and lint support**
   - `b5d0ea7` test(20-01): failing tests (RED)
   - `6e2722e` feat(20-01): implementation (GREEN)

## Verification

- `go build ./...`, `go vet ./...` and `go test ./... -count=1` pass.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/parser | 96.93% | 95% |
| pkg/lexer | 97.44% | 95% |
| pkg/checker | 95.92% | 94% |
| pkg/interp | 96.72% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.73% | 95% |
| total | 94.63% | 85% |

- pkg/ast is at 99.5%, and the new lint walker branches are fully covered. Every new fmt and emit branch is covered. The remaining uncovered blocks in those files predate this plan.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] JSON kinds forced for new nodes and CallArg**
- **Found during:** Task 1
- **Issue:** A node built without `NodeKind` serialises as `SourceFile`. This is the bug that 19-02 deferred for CallArg.
- **Fix:** `nodeToMap` forces the kind string for all nine new node types, as it already does for Pragma and Attribute. CallStmt and CallExpr arguments now report `"kind": "CallArg"` instead of `"SourceFile"`. No test pinned the old value.
- **Files modified:** pkg/ast/json.go
- **Commit:** 243ef0d

**2. [Rule 1 - Bug] Literal bit index would be a lint magic number**
- **Found during:** Task 2
- **Issue:** If the walker visited a literal `BitAccessExpr.Index`, every `w.3` above 1 would raise a magic-number warning.
- **Fix:** The walker visits a non-literal index, such as a constant name, and skips a literal one. A test covers both cases.
- **Files modified:** pkg/lint/plcopen.go
- **Commit:** 6e2722e

### Other notes

- **TYPE default layout:** The plan's example `TYPE E : (a, b) := b;` is a single line. The formatter already prints TYPE enums one value per line, so the output ends with `) := b;` or `) UINT := b;`. The tests assert those substrings.
- **Enum Known flag:** The plan only specified `Known=false` for the non-literal entry. Its implicit successors also report `Known=false`, because their values depend on it. Their `Value` still follows the documented "last known + 2" positional rule.
- **Extra test file:** `pkg/lint/phase20_nodes_test.go` is not listed in files_modified. It holds the walker tests.
- **Pre-existing gofmt drift:** gofmt flags `pkg/ast/test_nodes.go`, `pkg/ast/trivia.go`, `pkg/emit/emit_final_coverage_test.go`, `pkg/lint/lint_coverage_test.go` and `pkg/lint/rules.go`. These files were not touched and were left alone.

## Issues Encountered

None blocking.

## Notes for Later Plans

- **20-02/20-03 (parser):** Set `NodeKind` on new nodes for `Kind()`. JSON does not need it. Put arguments into `CallExpr.Args` until the first named argument, then put every later argument into `NamedArgs`, using `CallArg{Name: nil}` for a positional one. A trailing comma adds no entry.
- **Bit access:** Store the index as `BitAccessExpr.Index`. A constant-name index can stay a `MemberAccessExpr` per ruling A3, or become an `Ident` index. Both printers emit `Target.Index` either way.
- **THIS^ and SUPER^:** Build these as `DerefExpr{Operand: &ThisExpr{}}` and `DerefExpr{Operand: &SuperExpr{}}`.
- **Initialisers:** `StructInit` and `ArrayInit` go directly into `VarDecl.InitValue`, `StructMember.InitValue` and `TypeDecl.InitValue`. A repetition `3(0)` is `ArrayInitElem{Count: 3, Value: 0}`.
- **Namespaces:** `Tc2_EtherCAT.ST_EcSlaveState` is `NamedType{Namespace: Tc2_EtherCAT, Name: ST_EcSlaveState}`.
- **20-05/20-06 (interp/checker):** Use `ast.EnumOrdinals`. When `Known` is false, evaluate the constant yourself and continue numbering from it.
- **Checker and interpreter switches:** pkg/checker and pkg/interp do not handle the new nodes yet. Those plans own them.

## Known Stubs

None. The new fields are intentionally unpopulated until the parser plans 20-02 and 20-03 land.

## Self-Check: PASSED

- All six created files exist on disk.
- Commits 33764c6, 243ef0d, b5d0ea7 and 6e2722e are in git log.
