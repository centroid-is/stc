
## From 19-02 (AST contracts)

- **CallArg, ElsIf, CaseBranch, CaseLabelValue/Range, SubrangeSpec, EnumValue, StructMember have no NodeKind.** Parsers leave NodeKind at 0, so `stc parse --format json` reports `"kind": "SourceFile"` for them. This was already visible on `CallExpr.args`; 19-02 adds `CallStmt.args`, which inherits the same wrong kind on each argument. Fix needs new NodeKinds appended to the enum plus parser wiring; out of scope for 19-02 (pre-existing, not caused by this plan).

## From 19-03 (attribute parsing and printing)

- **pkg/emit writes comment trivia raw, without newline or indent.** `emitLeadingTrivia`/`emitTrailingTrivia` in emit.go concatenate `t.Text` directly, so `// c1` followed by `x : BOOL;` emits `// c1    x : BOOL;` and the declaration is commented out. Pre-existing; pkg/format handles this correctly. Attribute and pragma comments printed by the new `emitAttrLine` are correct; the owner's own comments still go through the broken path.
- **PROPERTY GET/SET accessors print as `METHOD P ... END_METHOD` in both fmt and emit.** Reparsing that output drops the accessor bodies, so fmt is not idempotent for properties with accessors. Pre-existing (emitPropertyDecl calls emitMethodDecl for Getter/Setter).
- **Comments before struct members and enum values attach to the TypeDecl.** StructMember and EnumValue are not in `nodeBaseOf`, so their comments print above `TYPE`. Pre-existing. Comments before an attribute on such a member now attach to the Attribute and print in place.
- **Pragmas with no owner are dropped:** a pragma at end of file, in an empty STRUCT, or in an empty enum. No diagnostic, no crash.

## From 19-04 (struct AT, empty call args)

- **`stc check` does not know standard FB signatures (TON, R_TRIG, ...).** `t(IN := b, PT := T#1S); b := t.Q;` reports `TON has no input parameter "IN"`, `"PT"` and `type TON has no member "Q"`. Reproduced on `main`, so it predates Phase 19. It means `stc check` on prog.st or action_inside.st still shows errors on TON calls, but they are unrelated to empty arguments. The interpreter runs these calls correctly.
- **pkg/parser/expr.go, pkg/emit/emit_final_coverage_test.go, pkg/checker/check_coverage_test.go and pkg/checker/vendor.go are not gofmt-clean.** Pre-existing; not touched by 19-04.
- **Lexer rejects unknown address areas such as `%Z9` as Illegal tokens.** The parse error comes before the checker, so SEMA030 only fires for addresses that lex but fail `iomap.ParseAddress` (e.g. `%IX0.9`).
- **Trailing comma before `)` in call arguments is still a parse error.** st301.st has 5 calls like `SPB03.speedBatcher(a := x, q => y,\n);` and svncorecomponents.st has 1 similar error. This is not an empty formal argument, so 19-04 did not change it. Accepting it is a one-line change in parseCallArgs (stop when `,` is followed by `)`), but it needs a decision on whether fmt keeps or drops the comma.

## From 19-06 (GVL runtime)

- **Zero-argument FB instance calls fail at runtime.** `b();` on an FB instance parses as an expression statement with a CallExpr, and `evalCall` reports `undefined function: B`. `G.f();` on a GVL FB instance hits `evalMethodCall` and reports `method 'f' not found`. Reproduced without any GVL, so it predates 19-06. Calls with at least one argument go through `execCallStmt` and work. 19-08 touches the same evalCall branch for actions and could fall back to running an FB instance there.
- **Library and mock files' GVLs are not registered in the test runner.** Only GVLs declared in the `*_test.st` file itself are registered per TEST_CASE.
- **`stc sim` does not register TYPE or FUNCTION_BLOCK declarations.** A GVL member of a user struct type in a sim program zero-fills as DINT, so `G.s.a` is a RuntimeError there. It works in the test runner.

## From 19-07 (ACTION parse and check)

- **Integer literals type as DINT, so `n := n + 1;` with `n : INT` reports `cannot assign DINT to INT`.** Reproduced on the 19-06 binary. It hits action_inside.st line 13. The 19-07 checker tests use DINT counters instead.
- **METHOD bodies are never checked.** `CheckBodies` checks PROGRAM, FUNCTION_BLOCK, FUNCTION and now ACTION bodies, but not methods, and methods have no scope of their own. A method calling an action is therefore accepted without any checking.
- **Inherited methods (EXTENDS) are not in the derived FB's scope.** 19-07 inserts only the FB's own methods, so calling a base-class method unqualified still reports SEMA010.
- **Named arguments in expression position still fail to parse** (Phase 20). `u := find_order(prio := x);` parses as `u := find_order` plus a parse error. Since 19-07 registers methods, the checker now says `cannot assign find_order to UINT` there instead of `undeclared identifier "find_order"`. That is 2 lines in st301 and 4 in svncorecomponents. The error count is unchanged.
- **pkg/symbols/scope.go is not gofmt-clean.** Pre-existing; not touched by 19-07.

## From 19-08 (ACTION runtime)

- **Resolved: zero-argument FB instance calls (from 19-06).** `b();`, `G.f();`, `outer.inner();` and `s.fb();` now run the instance in evalCall/evalMethodCall.
- **Unqualified METHOD calls inside an FB fail at runtime.** RESOLVED in 20-07 (commit ab8e88a): evalCall resolves a METHOD of `Env.CurrentFB()`, inherited ones included, after actions. `Inc();` in an FB body or in one of its actions reports `undefined function: INC`. Only `inst.Inc()` works. The 19-07 checker accepts the unqualified form, so an action that calls its FB's method checks clean and then fails when run. Pre-existing for FB bodies; fixing it means resolving methods through the env like actions.
- **Action lookup walks past the FB boundary.** RESOLVED in 20-07 (commit ab8e88a): `LookupAction` stops at the FB instance env. `Env.LookupAction` follows the parent chain, and an FB instance env's parent is the env that declared the instance. An FB body calling an action name that only the enclosing PROGRAM defines would run the PROGRAM's action. The checker reports SEMA010 for that call, so only unchecked code reaches it.
- **ErrExit/ErrContinue leaking out of an action body are not caught.** Same as METHOD bodies today: `EXIT;` at the top level of an action propagates to the caller's loop.

## From the Phase 19 code review (19-REVIEW.md)

- **Resolved by ME-01: pragmas in an empty STRUCT or empty enum (from 19-03).** They are now kept as the struct's or enum's end pragmas and printed before `END_STRUCT` or `)`. A pragma at end of file still has no owner.
- **ME-04 (partial): ACTIONs, attributes and `AT %I*` struct members are emitted unchanged for every target.** Schneider is CODESYS-derived and supports ACTIONs and attributes, so its output is correct. No Allen Bradley target exists in `pkg/emit` or the checker's vendor profiles yet. Gating these constructs belongs with that target. `POINTER TO` and similar types in struct members are also not stripped by `stc emit` for schneider and portable, though `stc check --vendor` now warns on them.
- **LO-02: a METHOD sharing a name with an FB variable is now a redeclaration.** Needs confirmation against TwinCAT before choosing between keeping the error and a separate callable namespace.
- **LO-03: `--gvl-name` on `fmt` and `emit` has no effect.** The GVL name is never printed. Decide whether to drop the flag there or print the name as a leading comment.
- **LO-04: LSP go-to-definition for a `qualified_only` GVL member lands on the first `VAR_GLOBAL`, and the hover fallback over-matches unresolved identifiers.** Needs per-member positions in `GVLInfo` and a fallback limited to `MemberAccessExpr` on a `KindGVL` object.
- **LO-05: GVL JSON uses `blocks` while POUs use `var_blocks`.** Changing the key is a JSON contract change for MCP and `--format json` consumers.
- **LO-06: statement-level pragmas in POU bodies are still deleted by fmt.** Needs a `PragmaStmt` node or body trivia.
- **LO-08: SEMA034 misses output bindings (`q => c`), indexed writes (`arr[1] := ...`) and member writes (`s.a := ...`) to constants.**
- **LO-10: `scripts/coverage-gate.sh` duplicates the `.testcoverage.yml` thresholds and never removes its work directory.**
- **LO-11: the probe fixture test checks idempotence only, and skips probes with any diagnostic.** ME-01 now has anchor-order tests in `pkg/format` and `pkg/emit`; the probe-level comparison of pragma sequences is still open.
