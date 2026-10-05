
## From 19-02 (AST contracts)

- **CallArg, ElsIf, CaseBranch, CaseLabelValue/Range, SubrangeSpec, EnumValue, StructMember have no NodeKind.** Parsers leave NodeKind at 0, so `stc parse --format json` reports `"kind": "SourceFile"` for them. This was already visible on `CallExpr.args`; 19-02 adds `CallStmt.args`, which inherits the same wrong kind on each argument. Fix needs new NodeKinds appended to the enum plus parser wiring; out of scope for 19-02 (pre-existing, not caused by this plan).
