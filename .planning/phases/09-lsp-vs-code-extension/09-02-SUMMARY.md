---
phase: 09-lsp-vs-code-extension
plan: 02
subsystem: lsp
tags: [lsp, navigation, go-to-definition, hover, completion, references, rename]

requires:
  - phase: 09-lsp-vs-code-extension
    plan: 01
    provides: "LSP server with document sync and DocumentStore"
  - phase: 03-semantic-analysis
    provides: "Symbol table with scope chain for symbol lookup"
provides:
  - "Go-to-definition resolving to symbol declaration position"
  - "Hover showing symbol kind and type as markdown"
  - "Completion for IEC keywords, types, and declared symbols"
  - "Find-references for all occurrences of a symbol"
  - "Rename with WorkspaceEdit replacing all references"
affects: [09-03]

tech-stack:
  added: []
  patterns: [position-based AST lookup, scope-chain symbol resolution, LSP handler closures]

key-files:
  created:
    - pkg/lsp/navigate.go
    - pkg/lsp/navigate_test.go
    - pkg/lsp/definition.go
    - pkg/lsp/hover.go
    - pkg/lsp/completion.go
    - pkg/lsp/references.go
    - pkg/lsp/rename.go
  modified:
    - pkg/lsp/server.go

key-decisions:
  - "Position-based lookup via AST walk with span containment check"
  - "Symbol resolution tries global scope first, then POU child scopes"
  - "Case-insensitive reference finding using strings.EqualFold"
  - "Completion combines static keyword/type lists with dynamic symbol table"

requirements-completed: [LSP-02, LSP-03, LSP-04, LSP-05, LSP-06]

duration: 188s
completed: 2026-03-28
---

# Phase 09 Plan 02: LSP Navigation and Refactoring Summary

**Position-based symbol lookup with go-to-definition, hover, completion, find-references, and rename handlers for full IDE navigation**

## Performance

- **Duration:** 188s
- **Started:** 2026-03-28T20:48:03Z
- **Completed:** 2026-03-28T20:51:11Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Created shared navigation utility layer (navigate.go) with position-based AST lookup, symbol resolution, reference finding, and symbol collection
- Implemented textDocument/definition handler resolving cursor position to symbol declaration
- Implemented textDocument/hover handler showing symbol kind and type as markdown
- Implemented textDocument/completion handler with IEC 61131-3 keywords (47), primitive types (21), and declared symbols from analysis
- Implemented textDocument/references handler finding all case-insensitive occurrences
- Implemented textDocument/rename handler building WorkspaceEdit for all references
- All 5 capabilities registered in server initialization
- 10 navigation tests + all existing tests passing (30+ total)

## Task Commits

Each task was committed atomically:

1. **Task 1: Position-based symbol lookup and reference finding** - `e46aa1a` (feat)
2. **Task 2: LSP handlers for definition, hover, completion, references, rename** - `477fc90` (feat)

## Files Created/Modified
- `pkg/lsp/navigate.go` - Shared navigation utilities: findIdentAtPosition, findSymbolAtPosition, findAllReferences, collectAllSymbols, symbolTypeString
- `pkg/lsp/navigate_test.go` - 10 tests covering all navigation utilities with ST test fixture
- `pkg/lsp/definition.go` - textDocument/definition handler returning Location at symbol declaration
- `pkg/lsp/hover.go` - textDocument/hover handler with markdown content showing kind and type
- `pkg/lsp/completion.go` - textDocument/completion handler combining keywords, types, and symbols
- `pkg/lsp/references.go` - textDocument/references handler collecting all name occurrences
- `pkg/lsp/rename.go` - textDocument/rename handler building WorkspaceEdit with text edits
- `pkg/lsp/server.go` - Added 5 handler registrations and server capabilities

## Decisions Made
- Position lookup walks AST recursively checking span containment at each node (line/col bounds)
- Symbol resolution tries GlobalScope.Lookup first, then iterates POU child scopes
- References found via case-insensitive name matching per IEC 61131-3 convention
- Completion items use LSP CompletionItemKind mappings: Variable->Variable(6), Function->Function(3), FunctionBlock->Class(7), etc.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None - all functionality is fully wired.

## Next Phase Readiness
- Full navigation suite available for VS Code extension integration (Plan 03)
- Navigate.go utilities reusable for future features (document symbols, workspace symbols)

---
*Phase: 09-lsp-vs-code-extension*
*Completed: 2026-03-28*
