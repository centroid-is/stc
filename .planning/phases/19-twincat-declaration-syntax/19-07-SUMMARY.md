---
phase: 19-twincat-declaration-syntax
plan: 07
subsystem: parser, emit, format, checker, symbols
tags: [parser, checker, action, twincat, codesys, fmt, emit]

requires:
  - phase: 19-02
    provides: ast.ActionDecl, ProgramDecl.Actions, FunctionBlockDecl.Actions
  - phase: 19-03
    provides: collectPragmas, attribute printing (emitAttrs)
  - phase: 19-05
    provides: GVL symbols and KindGVL (KindAction appended after it)
provides:
  - parseAction and ACTIONs inside PROGRAM/FUNCTION_BLOCK and after END_PROGRAM/END_FUNCTION_BLOCK
  - orphan ACTION diagnostic with a top-level ActionDecl
  - fmt/emit print POU actions after the POU; expression statements print as A1();
  - symbols.KindAction; actions and FB methods resolve in the owning POU scope
  - action bodies checked for undeclared names, usage and unreachable code
affects: [19-08, 19-10, 20, 21]

tech-stack:
  added: []
  patterns:
    - "Actions share the POU scope: they are VOID parameterless FunctionType symbols in the POU's scope"
    - "FB methods are FunctionType symbols in the FB scope with VAR_INPUT/VAR_IN_OUT as parameters"
    - "After-POU ACTIONs are parsed by parseDeclaration and attached by parseSourceFile to the preceding POU"

key-files:
  created:
    - pkg/parser/action_test.go
    - pkg/emit/action_test.go
    - pkg/format/action_test.go
    - pkg/checker/action_test.go
  modified:
    - pkg/parser/decl.go
    - pkg/parser/parser.go
    - pkg/parser/error.go
    - pkg/emit/emit.go
    - pkg/format/format.go
    - pkg/symbols/symbol.go
    - pkg/checker/resolve.go
    - pkg/checker/check.go
    - pkg/checker/usage.go
    - .planning/phases/19-twincat-declaration-syntax/deferred-items.md

key-decisions:
  - "An ACTION body stops at END_ACTION, the next ACTION, the POU end, or any top-level declaration keyword, so a missing END_ACTION cannot swallow the next POU"
  - "Pragmas right after the ACTION header go to ActionDecl.Pragmas; printers put attributes above ACTION and other pragmas as the first body lines"
  - "FB methods are registered in the FB scope so actions can call them; without this an FB calling its own method was already SEMA010"
  - "A FunctionType callee in a call statement (M(a := x);) is accepted; input names are checked against its parameters and output bindings are not validated"
  - "DIAL-08 stays pending: runtime action execution is plan 19-08"

requirements-completed: []  # DIAL-08 completes with 19-08 (runtime)

duration: 12min
completed: 2026-10-05
---

# Phase 19 Plan 07: ACTION parse, print and check Summary

**ACTION blocks now parse inside a POU and after it, print after their POU in `stc fmt` and `stc emit`, and check clean in the owning POU. This covers `A1();`, `inst.coe();` and calls between FB actions and methods.**

## Performance

- **Duration:** about 12 min
- **Started:** 2026-10-05T23:35:30Z
- **Completed:** 2026-10-05T23:47:30Z
- **Tasks:** 3 of 3
- **Files:** 4 created, 10 modified

## Accomplishments

- `parseAction` accepts `ACTION name`, `ACTION name:` and `ACTION name;`, collects the pragmas after the header, and ends at END_ACTION with an optional `;`.
- `parseProgram` is now a loop like `parseFunctionBlock`. ACTIONs and attributes before them attach to the PROGRAM. The FB loop accepts ACTIONs after METHODs.
- After-POU ACTIONs attach to the immediately preceding PROGRAM or FUNCTION_BLOCK. Any other position reports "ACTION without a preceding PROGRAM or FUNCTION_BLOCK" and keeps a top-level ActionDecl.
- `KwAction` is in `declarationStarts`, so recovery stops there.
- Both printers print each action after END_PROGRAM or END_FUNCTION_BLOCK, separated by a blank line. Attributes print above ACTION, and `{warning disable C0139}` prints as the first body line.
- The ACTION header has no colon any more.
- `symbols.KindAction` ("Action") exists. Actions are VOID parameterless functions in the POU scope, and a clash with a variable or method is SEMA011.
- FB methods are now in the FB scope as functions, with their inputs as parameters.
- Action bodies are checked in the POU scope. A variable used only in an action is not reported unused. Unreachable code after RETURN in an action is reported.

## Task Commits

1. **Task 1: Parse ACTIONs inside and after POUs**
   - `2abdd25` test(19-07): failing parser tests (RED)
   - `eeb37b0` feat(19-07): implementation (GREEN)
2. **Task 2: Print POU actions after the POU**
   - `3983753` feat(19-07): printers and tests in one commit, because the tests were written after the code
3. **Task 3: Checker resolves actions in the owning POU**
   - `f6fdbfc` test(19-07): failing checker tests (RED, build fails on `symbols.KindAction`)
   - `2037bd9` feat(19-07): implementation (GREEN)

## Verification

- `go test ./... -count=1` passes.
- `go run ./cmd/stc test tests/` reports 210 passed and 0 failed.
- `stc parse` reports 0 diagnostics on tests/twincat_probes/action.st and action_inside.st.
- Formatting action_inside.st twice gives identical output, and the result has 4 lines starting with `ACTION `. `stc emit` is also idempotent on it.
- `bash scripts/coverage-gate.sh` passes. Every new function is fully covered except one statement, the no-progress guard in `parseProgram`, which the loop cannot reach.

| package | covered | percent | min |
|---------|---------|---------|-----|
| pkg/parser | 1006/1039 | 96.82% | 95% |
| pkg/lexer | 228/234 | 97.44% | 95% |
| pkg/checker | 837/873 | 95.88% | 94% |
| pkg/interp | 1592/1649 | 96.54% | 95% |
| pkg/types | 124/124 | 100.00% | 95% |
| pkg/emit | 571/586 | 97.44% | 95% |
| total | 7856/8356 | 94.02% | 85% |

### Oracle diagnostic counts

| file | command | before (83f9097) | after |
|------|---------|------------------|-------|
| st301.st | stc parse | 950 | 917 |
| svncorecomponents.st | stc parse | 624 | 615 |
| st301.st | stc check errors | 2450 | 2384 |
| svncorecomponents.st | stc check errors | 1766 | 1745 |
| action.st | stc check errors | 7 | 2 |
| action_inside.st | stc check errors | 21 | 4 |

- `stc parse` adds no diagnostics on either oracle file; all changes are removals. The 6 `{warning disable C0139}` pragma-line diagnostics from 19-03 are gone, and no diagnostic mentions ACTION any more.
- The errors left in the probes are TON/R_TRIG parameters (RUNT-08, Phase 20) and the INT/DINT literal issue.

## Requirements

- **DIAL-08** stays pending. This plan delivers its parse and check half. Runtime action execution is plan 19-08.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Expression statements printed as `A1() := ;`**
- **Found during:** Task 2
- **Issue:** Both printers always wrote ` := ` for AssignStmt, so a zero-argument call came out as `A1() := ;`, which does not re-parse. fmt could not be idempotent.
- **Fix:** A nil Value is printed as `target;`. A bad right-hand side parses as an ErrorNode, so only expression statements are affected.
- **Files modified:** pkg/emit/emit.go, pkg/format/format.go
- **Commit:** 3983753

**2. [Rule 3 - Blocking] An FB could not call its own METHOD unqualified**
- **Found during:** Task 3
- **Issue:** Methods were never put in any scope, so `M();` inside an FB was SEMA010 even before this plan. The plan requires an action to call a method of the same FB.
- **Fix:** `resolveMethods` inserts each method as a FunctionType with its VAR_INPUT and VAR_IN_OUT parameters.
- **Files modified:** pkg/checker/resolve.go
- **Commit:** 2037bd9

**3. [Rule 1 - Bug] `inst.coe();` left `inst` reported as unused**
- **Issue:** Member callees returned before any lookup, so the instance was never marked used.
- **Fix:** `markRootUsed` marks the root identifier of the member chain. Unknown roots add no diagnostic.
- **Commit:** 2037bd9

**4. [Rule 1 - Bug] `remove_batch(uindex := i);` reported "is not a function block"**
- **Issue:** Once methods resolve, a method called as a statement with formal arguments reached the FB-only path in `checkCallStmt`.
- **Fix:** A FunctionType callee is accepted. Input names are checked against its parameters (SEMA024 otherwise), and every argument value is checked.
- **Commit:** 2037bd9

### Other notes

- pkg/parser/stmt.go needed no change. The action body uses `parseStatements` with a stop set, and pragmas mid-body are skipped as before.
- The checker tests use DINT counters because `n := n + 1` with INT is a pre-existing DINT/INT error (logged in deferred-items.md).
- POU spans still end at END_PROGRAM or END_FUNCTION_BLOCK. After-POU actions keep their own spans.

## Issues Encountered

- Named-argument calls in expression position still fail to parse (Phase 20). On those lines the checker now names the method type instead of reporting an undeclared identifier. That is 2 lines in st301 and 4 in svncorecomponents, with the same error count.
- METHOD bodies are still not checked, and inherited methods are not in the derived FB's scope. Both are logged in deferred-items.md.

## Notes for Later Plans

- **19-08:** an action symbol has `Kind == symbols.KindAction` and a `*types.FunctionType` with VOID return. `A1();` is an `AssignStmt{Target: CallExpr}` with a nil Value. Owner lookup goes through `ProgramDecl.Actions` and `FunctionBlockDecl.Actions`. Top-level ActionDecls are orphans and should not run.
- **Phase 21:** TcPOU `<Action>` import can build ActionDecls and append them to the POU's Actions. The printers then emit the text form.

## Known Stubs

None.

## Threat Flags

None. T-19-14 is covered by the adversarial subtests: unterminated ACTION, `ACTION ACTION ACTION` at top level and inside a PROGRAM, ACTION at EOF, and an unterminated ACTION before a FUNCTION. T-19-15 is covered by the orphan diagnostic tests.

## Self-Check: PASSED
