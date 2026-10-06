# Phase 20 Deferred Items

Out-of-scope discoveries logged during execution. Each line names the plan that found it.

- **[20-02] Lexer swallows `:` after a typed literal.** RESOLVED in 20-03 (commit 9b73b57): typed literals no longer accept `:`, so `INT#5:` lexes as TypedLiteral then Colon. Time, date and time-of-day literals still keep their colons.
- **[20-02] Statement-head call with positional-first mixed args.** `fb(1, b := 2);` at statement head is not a named-arg call by the lookahead, so it now parses as an expression statement over a `CallExpr` with `NamedArgs`. It does not become a `CallStmt`. Before 20-02 it was a parse error. The interpreter and checker should treat an expression statement whose value is a `CallExpr` on an FB instance like a `CallStmt`, or the parser should fold it.
- **[20-03] fmt is not idempotent on orphan trailing comments.** Formatting st301.st or svncorecomponents.st twice moves a few end-of-block `//` comments, for example after an attribute before END_STRUCT. The output re-parses clean. The cause is comment trivia attachment, not Phase 20 nodes.
- **[20-03] TYPE blocks with several declarations.** `TYPE A : INT; B : (x, y); END_TYPE` parses only the first type and reports errors for the rest. `parseTypeDecls` handles one declaration per block. Neither oracle file uses this form.
- **[20-03] Repetition count is an integer literal only.** `[N(0)]` with a constant name `N` parses as a call expression element, not as a repetition. IEC 61131-3 allows only an unsigned integer there; CODESYS behaviour with a constant was not verified.
