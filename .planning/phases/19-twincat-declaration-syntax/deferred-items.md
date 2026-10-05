
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
