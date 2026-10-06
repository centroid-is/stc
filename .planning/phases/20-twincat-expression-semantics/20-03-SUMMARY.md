---
phase: 20-twincat-expression-semantics
plan: 03
subsystem: parser
tags: [lexer, parser, twincat, initialisers, enums, namespaces, fmt]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 01
    provides: "StructInit, FieldInit, ArrayInit, ArrayInitElem, NamedType.Namespace, TypeDecl.InitValue, printers for enum base types and TYPE defaults"
provides:
  - "Lexer: typed based literals such as BYTE#16#10 are one TypedLiteral; typed literals never swallow ':'"
  - "EnumType.BaseType set for integer and bit-string types after ')' in TYPE and inline VAR enums"
  - "TypeDecl.InitValue from 'TYPE X : spec := value;'"
  - "NamedType{Namespace, Name} for Lib.Type in every type position"
  - "Stray ';' skipped in VAR blocks, STRUCT members and between top-level declarations"
  - "parseInitializer: StructInit, ArrayInit, N(value) and N() repetition, nesting cap of 64"
  - "Zero parse diagnostics on flattened st301.st and svncorecomponents.st"
affects: [20-04, 20-05, 20-06, 20-07, 20-08, 20-09]

tech-stack:
  added: []
  patterns:
    - "Initialiser lists recover by skipping to the matching close, stopping before ';'"
    - "A missing initialiser value reports without consuming the delimiter"

key-files:
  created:
    - pkg/parser/initializer.go
    - pkg/parser/initializer_test.go
    - pkg/parser/enum_base_test.go
    - pkg/parser/namespace_type_test.go
    - pkg/lexer/typed_based_literal_test.go
    - pkg/format/phase20_decl_roundtrip_test.go
  modified:
    - pkg/lexer/lexer.go
    - pkg/parser/types.go
    - pkg/parser/var.go
    - pkg/parser/decl.go

key-decisions:
  - "Enum base types accept only the integer and bit-string keywords (SINT..ULINT, BYTE..LWORD); ') REAL;' stays a parse error"
  - "A struct initialiser needs '(' Ident ':='; any other '(' stays a ParenExpr"
  - "A repetition count must be an integer literal; the parser never expands it"
  - "Initialisers nested deeper than 64 levels report 'initialiser nested too deeply' once and skip the subtree"
  - "Typed literals stop at ':'; only time, date and time-of-day literals keep colons (fixes deferred INT#5:)"
  - "One level of namespace qualification is parsed; NamedType has a single Namespace ident"

requirements-completed: []  # DIAL-07/DIAL-10 need checker, interpreter and probe-gate plans

duration: 7min
completed: 2026-10-06
---

# Phase 20 Plan 03: Declaration-level parse gaps Summary

**The parser now handles struct and array initialisers, enum base types, TYPE defaults, namespace-qualified types, typed based literals and stray semicolons, and both oracle files parse with zero diagnostics.**

## Performance

- **Duration:** about 7 min
- **Started:** 2026-10-06T01:55Z
- **Completed:** 2026-10-06T02:02Z
- **Tasks:** 2 of 2
- **Files created:** 6, modified: 4

## Accomplishments

- `BYTE#16#10`, `WORD#2#1010`, `DWORD#8#17` and `LWORD#16#FFFF_FFFF` lex as one token. The parser gives TypePrefix `BYTE` and value `16#10`, which the interpreter already evaluates.
- `INT#5:` now lexes as a typed literal followed by a colon. This closes the 20-02 deferred item.
- `( ... ) UINT;` and the other integer base types set `EnumType.BaseType`, in TYPE blocks and inline VAR enums.
- `TYPE E : (a, b) := b;` and `TYPE E : (a, b) UINT := b;` store the default in `TypeDecl.InitValue`.
- `Tc2_EtherCAT.ST_EcSlaveState` parses to a `NamedType` with a namespace, including after POINTER TO, REFERENCE TO and ARRAY OF.
- `rDropPoint : REAL;;` and stray top-level `;` produce no diagnostic.
- `parseInitializer` handles every initialiser shape in the probes. VAR, VAR_GLOBAL, STRUCT member and TYPE default initialisers all use it.
- Broken initialisers report errors, and the following declarations still parse.

## Task Commits

1. **Task 1: Typed based literals, enum base type, TYPE default, namespace types, stray semicolons**
   - `b11cf79` test(20-03): failing tests (RED)
   - `9b73b57` feat(20-03): implementation (GREEN)
2. **Task 2: Struct and array initialisers, fmt round-trip, wave gate**
   - `3499b2b` test(20-03): failing tests (RED)
   - `d91b5f4` feat(20-03): implementation (GREEN)

## Verification

- `go vet` on pkg/parser and pkg/lexer is clean. `go test ./... -count=1` passes.
- `stc parse tests/twincat_probes/enum_attr.st` reports 0 diagnostics.
- `STC_PROBES_DIR=... go test ./tests -run TestTwinCATProbeOracle -v` logs 0 parse diagnostics for both files.
- The Phase 20 probe allowances in tests/twincat_probes_test.go are untouched, and the test passes.
- The formatted oracle files re-parse with 0 diagnostics.
- `bash scripts/coverage-gate.sh` exits 0:

| package | coverage | min |
|---------|----------|-----|
| pkg/parser | 98.17% | 95% |
| pkg/lexer | 97.52% | 95% |
| pkg/checker | 95.92% | 94% |
| pkg/interp | 96.72% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.73% | 95% |
| total | 94.86% | 85% |

All new and changed functions are at 100% branch coverage in the parser and lexer unit tests. The one uncovered block in `parseNamedTypeOrSubrange` is the named subrange branch, which predates this plan.

Oracle parse error counts from `stc parse`:

| file | before 20-02 | before 20-03 | after 20-03 |
|------|--------------|--------------|-------------|
| st301.st | 917 | 703 | 0 |
| svncorecomponents.st | 615 | 39 | 0 |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Typed literals swallowed a following colon**
- **Found during:** Task 1, while editing the typed-literal branch
- **Issue:** `INT#5:` lexed as `TypedLiteral("INT#5:")`, so a CASE label without a space failed. The lead asked for this fix if cheap.
- **Fix:** `scanLiteralValue` takes an `allowColon` flag. Only time, date and time-of-day prefixes set it.
- **Files modified:** pkg/lexer/lexer.go
- **Commit:** 9b73b57

**2. [Rule 2 - Missing critical] A missing initialiser value no longer eats the delimiter**
- **Found during:** Task 2
- **Issue:** `parseExpr` skips a bad token, so `(a := )` would have consumed the `)` and broken recovery.
- **Fix:** At `)`, `]`, `,`, `;` or end of file, `parseInitializer` reports "expected expression" and leaves the token in place. This also applies to `x : INT := ;`, which no longer consumes the semicolon.
- **Files modified:** pkg/parser/initializer.go
- **Commit:** d91b5f4

### Other notes

- **Repetition count:** The plan allowed "an IntLiteral (or a constant Ident)" as the count. Only an integer literal is accepted, because `[F(1)]` is a function call, and IEC 61131-3 allows only an unsigned integer there. This is logged in deferred-items.md.
- **Empty repetition:** `3()` is accepted with a nil value, as IEC allows. fmt prints it back as `3()`.
- **Enum base type:** Only keywords are accepted after `)`. The plan's "or an Ident naming one" does not apply, because the lexer always turns those names into keywords.
- **Extra test:** `TestTypedBasedLiteralParse` in enum_base_test.go covers the parser side of typed based literals and the `INT#5:` CASE label.

## Issues Encountered

None blocking. Three out-of-scope findings are in deferred-items.md:

- fmt is not idempotent on a few orphan trailing comments in the oracle files. The output still re-parses clean. The cause is comment trivia attachment.
- `TYPE A : INT; B : (x, y); END_TYPE` parses only the first type. Neither oracle file uses this form.
- A repetition count given as a constant name is not supported.

## Notes for 20-04..20-09

- **Checker (20-05/20-09):** `VarDecl.InitValue`, `StructMember.InitValue` and `TypeDecl.InitValue` can now be `*ast.StructInit` or `*ast.ArrayInit`. Any code that type-checks an initialiser as an expression must handle them, or skip them until 20-09. An `ArrayInitElem.Value` may be nil for `N()`, and `Count` is a `LitInt` literal that can be huge. Count it without expanding.
- **Enum base types (20-05/20-06):** `EnumType.BaseType` is a `NamedType` with an uppercase keyword name such as `UINT`. Inline VAR enums can carry one too.
- **TYPE defaults:** `TypeDecl.InitValue` is set for enum and alias types, for example `TYPE T : INT := 5;`.
- **Namespaces (RUNT-08):** `NamedType.Namespace` is set for `Lib.Type`. Resolution should try the qualified name, then the bare name.
- **Error nodes:** Broken or too-deep initialisers leave `*ast.ErrorNode` values inside these trees.
- **20-08:** Both oracle files are at 0 parse diagnostics now. The probe allowances can be removed, and the oracle test can assert `total == 0`.

## Known Stubs

None. Runtime application of initialisers is Phase 22 by design (CONTEXT deferred list).

## Self-Check: PASSED

- All six created files exist on disk.
- Commits b11cf79, 9b73b57, 3499b2b and d91b5f4 are in git log.
