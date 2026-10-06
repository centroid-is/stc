# Phase 20 Deferred Items

Out-of-scope discoveries logged during execution. Each line names the plan that found it.

- **[20-02] Lexer swallows `:` after a typed literal.** `INT#5:` lexes as one `TypedLiteral("INT#5:")`, so a CASE label written without a space before the colon fails to parse. `INT#5 :` works. The cause is probably `scanLiteralValue` accepting `:` for `TOD#12:00`. The fix belongs in the lexer: stop at `:` unless the prefix is a time-of-day or date-and-time type.
- **[20-02] Statement-head call with positional-first mixed args.** `fb(1, b := 2);` at statement head is not a named-arg call by the lookahead, so it now parses as an expression statement over a `CallExpr` with `NamedArgs`. It does not become a `CallStmt`. Before 20-02 it was a parse error. The interpreter and checker should treat an expression statement whose value is a `CallExpr` on an FB instance like a `CallStmt`, or the parser should fold it.
