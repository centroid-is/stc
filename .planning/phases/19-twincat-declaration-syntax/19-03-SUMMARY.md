---
phase: 19-twincat-declaration-syntax
plan: 03
subsystem: parser
tags: [parser, emit, format, twincat, attributes, pragmas, trivia]

requires:
  - phase: 19-02
    provides: ast.Attribute, ast.PragmaNode kinds, Attributes/Pragmas fields, JSON marshalling
provides:
  - parseAttributeText hand scanner and Parser.collectPragmas (pkg/parser/pragma.go)
  - attributes and pragmas attached to POUs, TypeDecl, METHOD, PROPERTY, VarBlock, VarDecl, StructMember, EnumValue
  - Attribute, PragmaNode and GVLDecl in trivia attachment (nodeBaseOf)
  - emitAttrs / emitAttrLine / emitInlineAttrs / emitStructMember in pkg/emit and pkg/format
affects: [19-04, 19-05, 19-07, 19-09, 24, 28]

tech-stack:
  added: []
  patterns:
    - "collectPragmas() before every declaration item; parseVarBlocks backtracks when the pragmas are not followed by a VAR keyword"
    - "parseStatements stops at Pragma only when the caller lists lexer.Pragma in its stop set (FB body loop)"
    - "Printers emit owner attributes first (each with its own comments), then the owner's leading trivia, then the owner"

key-files:
  created:
    - pkg/parser/pragma.go
    - pkg/parser/pragma_test.go
    - pkg/emit/attribute_test.go
    - pkg/format/attribute_test.go
    - cmd/stc/parse_attr_test.go
  modified:
    - pkg/parser/parser.go
    - pkg/parser/decl.go
    - pkg/parser/var.go
    - pkg/parser/types.go
    - pkg/parser/stmt.go
    - pkg/parser/error.go
    - pkg/parser/trivia.go
    - pkg/emit/emit.go
    - pkg/format/format.go

key-decisions:
  - "Trailing pragmas before END_VAR attach to the enclosing VarBlock; before END_STRUCT or ')' they attach to the last member/value. fmt moves them above the owner, which is idempotent"
  - "Pragmas with no owner (end of file, empty STRUCT/enum) are dropped silently"
  - "Statement-level pragmas inside bodies are still skipped; inside an FB body they are collected and dropped unless a METHOD or PROPERTY follows"
  - "Attributes are printed for every emit target, including schneider and portable"
  - "Inline enums print value attributes inline: ({attribute 'ev'} e1, e2)"

patterns-established:
  - "New owners (GVLDecl in 19-05, ActionDecl in 19-07) call emitAttrs(x.Attributes, x.Pragmas) before their leading trivia in both printers"

requirements-completed: [DIAL-01]

duration: 15min
completed: 2026-10-05
---

# Phase 19 Plan 03: Attribute and pragma parsing, attachment and round-trip Summary

**`{attribute ...}` and other pragmas are now parsed by a bounds-checked hand scanner, attached to the declaration that follows them at every declaration site, shown in `stc parse --format json`, and printed back by `stc fmt` and `stc emit` with comment order kept and idempotent output.**

## Performance

- **Duration:** about 15 min
- **Started:** 2026-10-05T22:50:00Z
- **Completed:** 2026-10-05T23:05:22Z
- **Tasks:** 3 of 3
- **Files:** 5 created, 9 modified

## Accomplishments

- `parseAttributeText` accepts `'` or `"` quotes with doubled-quote escapes, a case-insensitive keyword, extra whitespace, and an optional `:=` value. Any other shape stays verbatim as a `PragmaNode`. It has no regexp and is covered by 27 table rows plus a String() round-trip test.
- `skipPragmas` is gone. Every owner from the plan gets its pragmas: PROGRAM, FUNCTION_BLOCK, FUNCTION, INTERFACE, TYPE, METHOD and PROPERTY inside an FB, each VAR section, VarDecl, StructMember and EnumValue.
- An attribute before an enum value used to be a parse error. It now parses, and `enum_attr.st` has zero diagnostics.
- `declarationStarts` includes `Pragma`, so panic-mode recovery no longer eats the attributes of the next POU.
- A comment above an attribute becomes the Attribute's leading trivia, so fmt prints `// c1`, the attribute, `// c2`, then the variable, in that order.
- Both printers render attributes with `Attribute.String()`, so emit and format always agree. Struct-member printing is shared between type bodies and inline structs.

## Task Commits

1. **Task 1: Attribute text scanner and collectPragmas**
   - `4b47833` test(19-03): failing tests (RED)
   - `276e364` feat(19-03): implementation (GREEN)
2. **Task 2: Attach pragmas at every declaration site, trivia and recovery**
   - `c6897e9` test(19-03): failing tests (RED)
   - `1627324` feat(19-03): implementation (GREEN)
3. **Task 3: Print attributes and pragmas in emit and format, plus CLI JSON test**
   - `7c39937` test(19-03): failing printer tests (RED). The CLI JSON test already passed here because the parser work was done.
   - `2436b6a` feat(19-03): implementation (GREEN)

## Verification

- `go test ./... -count=1` and `go vet` pass.
- `stc fmt tests/twincat_probes/structpragma.st` prints `{attribute 'OPC.UA.DA.Access' := '1'}` above `I1`. Formatting that output again gives identical bytes.
- `bash scripts/coverage-gate.sh` passes:

| package | covered | percent | min |
|---------|---------|---------|-----|
| pkg/parser | 935/970 | 96.39% | 95% |
| pkg/lexer | 228/234 | 97.44% | 95% |
| pkg/checker | 688/726 | 94.77% | 94% |
| pkg/interp | 1527/1589 | 96.10% | 95% |
| pkg/types | 124/124 | 100.00% | 95% |
| pkg/emit | 552/567 | 97.35% | 95% |
| total | 7501/8032 | 93.39% | 85% |

- Oracle spot check with `stc parse` against the 19-01 baseline:

| file | baseline | now | diagnostics on attribute/warning lines |
|------|----------|-----|-----|
| st301.st | 2711 | 2728 | 6 |
| svncorecomponents.st | 1430 | 1407 | 0 |

  All 6 remaining st301 diagnostics are `{warning disable C0139}` at the top of an ACTION body. They go away once ACTION parsing lands in 19-07. I diffed against a binary built at c084e27, before this plan. The st301 increase is exactly 16 extra `unexpected KwVarGlobal in declaration context` errors. Recovery now stops at the `{attribute 'qualified_only'}` before each VAR_GLOBAL, so each GVL reports its own error instead of being swallowed by the previous recovery. These go away when 19-05 parses VAR_GLOBAL.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] parseStatements swallowed pragmas before a METHOD that follows body statements**
- **Found during:** Task 2
- **Issue:** In `x := 1; {attribute 'm'} METHOD M`, `parseStatements` skipped the pragma before the FB loop could see it.
- **Fix:** `parseStatements` now stops at a Pragma when the caller lists `lexer.Pragma` in its stop set. Only the FB body loop does that. Other callers still skip statement pragmas as before.
- **Files modified:** pkg/parser/stmt.go. This file is not in the plan's files_modified.
- **Commit:** 1627324

**2. [Rule 1 - Bug] parseVarBlocks would have consumed pragmas belonging to a METHOD or the body**
- **Found during:** Task 2
- **Fix:** It now backtracks to the saved position when the collected pragmas are not followed by a VAR keyword.
- **Commit:** 1627324

### Other notes

- **Trailing pragmas in enums:** the plan only required END_VAR and END_STRUCT. Pragmas before an enum's closing `)` also attach to the last value.
- **gofmt whitespace changes:** running gofmt on touched files realigned the `primitiveTypeKeywords` map in types.go and part of trivia.go. The changes are whitespace only.
- **Format test properties:** the format test uses a PROPERTY without GET/SET because accessors do not round-trip today. This is a pre-existing bug, listed in deferred-items.md.

## Issues Encountered

None blocking.

## Deferred Issues

These are logged in deferred-items.md and are all pre-existing:
- pkg/emit writes comment trivia raw, so `// c` followed by a declaration comments the declaration out.
- PROPERTY GET/SET accessors print as METHOD blocks in both printers.
- Comments before struct members and enum values attach to the TypeDecl.

## Known Stubs

None.

## Next Phase Readiness

- **19-04 (struct AT):** `parseStructMember` already gets attributes from the loop. Add `AtAddress` parsing there and print it in `emitStructMember`, which is shared by both struct printing paths in each printer.
- **19-05 (GVL):** dispatch `KwVarGlobal` in `parseDeclaration` and add a `*ast.GVLDecl` case to `attachDeclPragmas` in decl.go. `nodeBaseOf` already handles GVLDecl. Call `emitAttrs` for the GVL in both printers. Pragmas before later VAR_GLOBAL blocks go to `VarBlock.Attributes` through `parseVarBlocks`.
- **19-07 (ACTION):** add ActionDecl to `attachDeclPragmas` if attributes may precede ACTION, and add `KwAction` to `declarationStarts`. `collectPragmas` is ready for `ActionDecl.Pragmas` at the start of the body.

## Self-Check: PASSED

All five created files exist, and all six task commits are in git log.
