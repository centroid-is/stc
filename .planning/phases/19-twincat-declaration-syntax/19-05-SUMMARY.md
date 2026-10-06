---
phase: 19-twincat-declaration-syntax
plan: 05
subsystem: parser, checker, emit, format, incremental
tags: [parser, checker, emit, format, twincat, gvl, qualified_only, constants]

requires:
  - phase: 19-02
    provides: ast.GVLDecl, SanitizeGVLName, HasAttribute, VarBlock.Attributes
  - phase: 19-03
    provides: collectPragmas, emitAttrs in both printers, GVLDecl in nodeBaseOf
  - phase: 19-04
    provides: checkATAddresses treats "GVL" like "PROGRAM"
provides:
  - parseGVLBlock aggregates every top-level VAR_GLOBAL of a file into one GVLDecl
  - emitGVLDecl in pkg/emit and pkg/format
  - symbols.KindGVL, symbols.GVLInfo, Symbol.IsConstant
  - Resolver.pendingGVLs deferred pass and resolveGVL (GVL symbol typed as StructType)
  - SEMA033 CodeGVLQualifiedOnly, SEMA034 CodeAssignToConstant
  - DepGraph.ScanFile declares the GVL name and references its member types
affects: [19-09, 19-10, 21, 22, 23, 24]

tech-stack:
  added: []
  patterns:
    - "A GVL is a global KindGVL symbol whose Type is a types.StructType of its variables, so GVL.x uses ordinary member access"
    - "GVL access rules live on the GVL symbol (Symbol.GVL *GVLInfo), so Table.PurgeFile removes them with the symbol"
    - "GVLs resolve in a second pass after all files' TYPE declarations are registered"

key-files:
  created:
    - pkg/parser/gvl_test.go
    - pkg/emit/gvl_test.go
    - pkg/format/gvl_test.go
    - pkg/checker/gvl_test.go
  modified:
    - pkg/parser/parser.go
    - pkg/parser/decl.go
    - pkg/parser/error.go
    - pkg/emit/emit.go
    - pkg/format/format.go
    - pkg/symbols/symbol.go
    - pkg/checker/diag_codes.go
    - pkg/checker/resolve.go
    - pkg/checker/check.go
    - pkg/checker/usage.go
    - pkg/incremental/depgraph.go
    - pkg/incremental/depgraph_test.go

key-decisions:
  - "The qualified_only and constant side tables are stored on the GVL symbol (GVLInfo), not in separate maps, so incremental purges cannot leave stale entries"
  - "SEMA033 lists every qualified_only GVL that declares the name: the first one sorted, then '(also declared in X)'"
  - "SEMA034 covers GVL constants only; local VAR CONSTANT assignment is still unchecked"
  - "A user GVL overriding a library GVL also deletes that library GVL's bare variables"
  - "fmt and emit print all aggregated VAR_GLOBAL blocks at the first block's position (19-RESEARCH.md Open Question 4, accepted reordering)"

patterns-established:
  - "Undeclared-name reporting goes through Checker.reportUndeclared so qualified_only hints apply to idents and calls alike"

requirements-completed: []  # DIAL-01 was already complete (19-03). DIAL-02 also needs --gvl-name, which is 19-10.

duration: 9min
completed: 2026-10-05
---

# Phase 19 Plan 05: GVL declarations, qualified_only and constants Summary

**A file's top-level `VAR_GLOBAL` blocks now parse into one GVLDecl named after the file. The GVL prints back with its attributes. The checker resolves `GVL.x`, rejects bare access to qualified_only GVLs with SEMA033, and rejects writes to `VAR_GLOBAL CONSTANT` members with SEMA034.**

## Performance

- **Duration:** about 9 min
- **Started:** 2026-10-05T23:16:00Z
- **Completed:** 2026-10-05T23:25:00Z
- **Tasks:** 2 of 2
- **Files:** 4 created, 12 modified

## Accomplishments

- `parseGVLBlock` handles `VAR_GLOBAL` at the top level, with or without leading pragmas. The first block creates the GVLDecl and its pragmas go on the GVL. Later blocks keep their own pragmas and extend the GVL span.
- The GVL name is `SanitizeGVLName` of the file basename without its extension. `dir/My-GVL.st` gives `My_GVL` and an empty filename gives `GVL`.
- `KwVarGlobal` is in `declarationStarts`, so panic-mode recovery stops at a GVL.
- Both printers print GVL attributes, then each block with its own attributes. gvl1.st, gvl2.st and ECT.st format idempotently. The `// ==== Device 1` comment stays directly above `{attribute 'TcLinkTo'}`.
- The resolver queues GVLs in `pendingGVLs` and resolves them after library, user and mock files have registered their TYPEs. `A_ECT.X : ST_EL1008` therefore gets the real struct type even though its DUT file comes later.
- Variables of a plain GVL are also VAR_GLOBAL symbols in the global scope, so bare `x` resolves. Variables of a qualified_only GVL are not inserted, so EPW01.WA01 and FPW01.WA01 coexist.
- SEMA033 fires on bare idents and on bare calls. SEMA034 fires on `c := ...` and `G.c := ...`. A local variable that shadows the constant is not flagged.
- `checkUnusedVars` skips KindGVL symbols and VAR_GLOBAL symbols explicitly.
- GVL blocks go through `checkATAddresses(..., "GVL")`. Explicit addresses are allowed and invalid ones report SEMA030.
- `DepGraph.ScanFile` declares the GVL name and records the GVL's member types as references.

## Task Commits

1. **Task 1: Parse top-level VAR_GLOBAL into one GVLDecl and print it**
   - `8f9cb5c` test(19-05): failing tests (RED)
   - `8f21678` feat(19-05): implementation (GREEN)
2. **Task 2: Checker GVL resolution, SEMA033, SEMA034 and dependency graph**
   - `4e57644` test(19-05): failing tests (RED)
   - `cc41083` feat(19-05): implementation (GREEN)

## Verification

- `go test ./... -count=1` passes.
- `stc parse tests/twincat_probes/gvl2.st` exits 0 with no diagnostics.
- Formatting ECT.st twice gives identical bytes.
- `stc check` on gvl1.st and ECT.st reports 0 errors and 0 warnings.
- `bash scripts/coverage-gate.sh` passes. Every new function is at 100% in the unit profile.

| package | covered | percent | min |
|---------|---------|---------|-----|
| pkg/parser | 960/994 | 96.58% | 95% |
| pkg/lexer | 228/234 | 97.44% | 95% |
| pkg/checker | 777/814 | 95.45% | 94% |
| pkg/interp | 1530/1591 | 96.17% | 95% |
| pkg/types | 124/124 | 100.00% | 95% |
| pkg/emit | 564/579 | 97.41% | 95% |
| total | 7647/8173 | 93.56% | 85% |

Oracle diagnostics, comparing a build of `b6e7e0d` (before this plan) with this plan's code:

| file | parse before | after | check errors before | after | check warnings before | after |
|------|-----|-----|-----|-----|-----|-----|
| st301.st | 1610 | 950 | 3110 | 2450 | 13 | 13 |
| svncorecomponents.st | 628 | 624 | 1770 | 1766 | 25 | 25 |

Neither oracle file has any remaining `unexpected KwVarGlobal` parse errors. Neither reports SEMA033 or SEMA034.

## Requirements

- **DIAL-02** stays pending. Its text includes `--gvl-name`, which plan 19-10 delivers. Everything else in DIAL-02 is done end to end: parsing, printing, naming from the file, and qualified_only enforcement.
- **DIAL-01** was already complete from 19-03. This plan adds GVL-level attributes to it without changing its status.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Correctness] GVL access rules stored on the GVL symbol, not in a separate resolver table**
- **Found during:** Task 2
- **Issue:** The Resolver and Checker share only `symbols.Table`. A separate side-table map would also go stale after `Table.PurgeFile`, because that function only deletes global symbols.
- **Fix:** Added `Symbol.GVL *GVLInfo`, which holds QualifiedOnly, Vars and Constants, plus `Symbol.IsConstant` for bare constant variables. `PurgeFile` removes them together with the GVL symbol.
- **Files modified:** pkg/symbols/symbol.go, pkg/checker/resolve.go, pkg/checker/check.go
- **Commit:** cc41083

**2. [Rule 1 - Bug] Overriding a library GVL left its bare variables behind**
- **Found during:** Task 2
- **Issue:** When a user GVL replaces a library GVL of the same name, the library's bare variables stayed in the global scope. Redeclaring them in the user GVL then gave a false SEMA011.
- **Fix:** `removeGVL` calls `RemovePOU`, so a library POU's scope is also dropped. For a non-qualified library GVL it then deletes the library VAR_GLOBAL symbols.
- **Commit:** cc41083

**3. [Rule 2] The GVL also records references to its member types in the dependency graph**
- **Found during:** Task 2
- **Fix:** `ScanFile` calls `extractVarReferences(d.Blocks)`, so editing a DUT file marks the GVL file dirty. The plan only asked for the declaration.
- **Commit:** cc41083

### Other notes

- **Block modifier order:** `VAR_GLOBAL PERSISTENT RETAIN` prints as `VAR_GLOBAL RETAIN PERSISTENT`. That is the printers' existing fixed order. Output is idempotent after the first format.
- **Block reordering:** as accepted in 19-RESEARCH.md Open Question 4, fmt and emit print all aggregated VAR_GLOBAL blocks at the first block's position. A file with a GVL block, a PROGRAM, and a second GVL block prints both GVL blocks before the PROGRAM.

## Issues Encountered

None blocking.

## Deferred Issues

None new. The emit trivia bug in deferred-items.md also affects a comment directly before a GVL that has no attribute, in the same way it affects every other declaration. pkg/format handles that case correctly.

## Known Stubs

None.

## Notes for Later Plans

- **19-10 (`--gvl-name`):** call `ast.SetGVLName` after parsing, not in the incremental parse path. The checker reads the name from `GVLDecl.Name` during the deferred pass, so a rename before `CollectDeclarations` is picked up.
- **Interpreter (Phase 22/23):** `pkg/interp` does not reference GVLDecl. Its declaration switches ignore it, so nothing crashes, but `GVL.x` does not run yet. CONTEXT.md asks for a minimal global env layer. No plan so far has owned it.
- **SEMA034** only checks assignment statements. Passing a GVL constant as a VAR_IN_OUT or VAR_OUTPUT target (`=>`) is not checked.
- **qualified_only lookup** scans the global scope only on a miss, which is the error path. That is cheap at ST301 scale.

## Self-Check: PASSED

- Created files exist: pkg/parser/gvl_test.go, pkg/emit/gvl_test.go, pkg/format/gvl_test.go, pkg/checker/gvl_test.go.
- Commits 8f9cb5c, 8f21678, 4e57644 and cc41083 are in git log.
