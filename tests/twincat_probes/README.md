# TwinCAT declaration-syntax probes

These fixtures drive Phase 19 (TwinCAT declaration syntax). `gvl1.st`, `gvl2.st`, `structat.st`, `structpragma.st`, `prog.st`, `action.st`, `link.st` and `fbat.st` are one-construct probes copied verbatim from the sildarvinnsla flattening work; each isolates a single construct and carries no customer logic. `action_inside.st`, `ECT.st` and `enum_attr.st` are hand-written synthetic shapes that reproduce the structure of real TwinCAT exports (ACTIONs inside a POU after METHODs, an EtherCAT GVL with `TcLinkTo` and OPC UA attributes, attributes on enum values) without any of their content. The large flattened customer sources (`st301.st`, `svncorecomponents.st` and similar) are used only as a local oracle and are never committed. None of these files ends in `_test.st`, so `stc test tests/` does not run them. Phase 20 (TwinCAT expression semantics) made every probe parse with zero diagnostics, including the bit-access line `b := w.3;` in `prog.st` and the named function-argument calls to `F_X` in `link.st`. There are no per-line allowances in `tests/twincat_probes_test.go`.

## Phase 20 probes

These four synthetic probes were copied verbatim from the same local probe directory. Each is under 15 lines and uses only placeholder names.

| File | Construct |
|------|-----------|
| `case.st` | CASE over an enum with qualified labels (`E_X.a:`) and a multi-label branch (`E_X.b, E_X.c:`) |
| `enum.st` | `qualified_only` and `strict` enum with explicit values and a base type, plus a plain enum used bare |
| `fcall.st` | FUNCTION called positionally and with named arguments in an expression |
| `ptr.st` | `POINTER TO` with `ADR` and `^`, `REFERENCE TO` with `REF=`, and `SIZEOF` |
