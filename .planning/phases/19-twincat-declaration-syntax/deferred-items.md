
## From 19-02 (AST contracts)

- **CallArg, ElsIf, CaseBranch, CaseLabelValue/Range, SubrangeSpec, EnumValue, StructMember have no NodeKind.** Parsers leave NodeKind at 0, so `stc parse --format json` reports `"kind": "SourceFile"` for them. This was already visible on `CallExpr.args`; 19-02 adds `CallStmt.args`, which inherits the same wrong kind on each argument. Fix needs new NodeKinds appended to the enum plus parser wiring; out of scope for 19-02 (pre-existing, not caused by this plan).

## From 19-03 (attribute parsing and printing)

- **pkg/emit writes comment trivia raw, without newline or indent.** `emitLeadingTrivia`/`emitTrailingTrivia` in emit.go concatenate `t.Text` directly, so `// c1` followed by `x : BOOL;` emits `// c1    x : BOOL;` and the declaration is commented out. Pre-existing; pkg/format handles this correctly. Attribute and pragma comments printed by the new `emitAttrLine` are correct; the owner's own comments still go through the broken path.
- **PROPERTY GET/SET accessors print as `METHOD P ... END_METHOD` in both fmt and emit.** Reparsing that output drops the accessor bodies, so fmt is not idempotent for properties with accessors. Pre-existing (emitPropertyDecl calls emitMethodDecl for Getter/Setter).
- **Comments before struct members and enum values attach to the TypeDecl.** StructMember and EnumValue are not in `nodeBaseOf`, so their comments print above `TYPE`. Pre-existing. Comments before an attribute on such a member now attach to the Attribute and print in place.
- **Pragmas with no owner are dropped:** a pragma at end of file, in an empty STRUCT, or in an empty enum. No diagnostic, no crash.
