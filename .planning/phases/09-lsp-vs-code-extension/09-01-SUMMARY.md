---
phase: 09-lsp-vs-code-extension
plan: 01
subsystem: lsp
tags: [glsp, lsp, vscode, diagnostics, formatting, stdio]

requires:
  - phase: 01-parser-ast
    provides: "parser.Parse producing AST with diagnostics"
  - phase: 03-semantic-analysis
    provides: "analyzer.Analyze for type checking and usage analysis"
  - phase: 08-lint-format
    provides: "format.Format for ST code formatting"
provides:
  - "LSP server with document sync, real-time diagnostics, and formatting"
  - "DocumentStore for in-memory document management"
  - "stc lsp CLI command for editor integration"
affects: [09-02, 09-03]

tech-stack:
  added: [github.com/tliron/glsp]
  patterns: [GLSP handler-based LSP server, document store with parse-on-change]

key-files:
  created:
    - pkg/lsp/server.go
    - pkg/lsp/document.go
    - pkg/lsp/diagnostics.go
    - pkg/lsp/formatting.go
    - pkg/lsp/server_test.go
    - cmd/stc/lsp_cmd.go
    - cmd/stc/lsp_cmd_test.go
  modified:
    - cmd/stc/main.go
    - go.mod
    - go.sum

key-decisions:
  - "GLSP v0.2.2 chosen for LSP protocol handling (Go-native, stdio support)"
  - "Full document sync mode (TextDocumentSyncKindFull) for simplicity"
  - "Parse + analyze on every document change for real-time diagnostics"

patterns-established:
  - "DocumentStore pattern: thread-safe document management with parse/analyze on change"
  - "Diagnostic conversion: 1-based stc positions to 0-based LSP positions"

requirements-completed: [LSP-01, LSP-08]

duration: 5min
completed: 2026-03-28
---

# Phase 09 Plan 01: LSP Server Core Summary

**GLSP-based LSP server with document sync, real-time parse/analysis diagnostics, and full-document formatting via stc lsp command**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-28T20:39:54Z
- **Completed:** 2026-03-28T20:45:08Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- LSP server initializes and shuts down via GLSP on stdio
- Document open/change/close triggers re-parse and re-analysis, publishing diagnostics to the client
- textDocument/formatting returns formatted ST using pkg/format with full-document TextEdit
- stc lsp CLI command starts the server binary on stdio
- 12 unit tests covering diagnostics conversion, document store lifecycle, and formatting

## Task Commits

Each task was committed atomically:

1. **Task 1: LSP server core with document sync and diagnostics** - `ac472fb` (feat)
2. **Task 2: CLI stc lsp command and integration test** - `39fd9ce` (feat)

## Files Created/Modified
- `pkg/lsp/server.go` - GLSP server setup with initialize, shutdown, text sync handlers
- `pkg/lsp/document.go` - Thread-safe DocumentStore with parse/analyze on open/update
- `pkg/lsp/diagnostics.go` - stc diagnostic to LSP protocol diagnostic conversion (1-based to 0-based)
- `pkg/lsp/formatting.go` - textDocument/formatting handler delegating to pkg/format
- `pkg/lsp/server_test.go` - Unit tests for diagnostics, store, formatting
- `cmd/stc/lsp_cmd.go` - Cobra command for stc lsp
- `cmd/stc/lsp_cmd_test.go` - CLI integration tests
- `cmd/stc/main.go` - Added lsp command registration
- `go.mod` - Added github.com/tliron/glsp dependency
- `go.sum` - Updated checksums

## Decisions Made
- Used GLSP v0.2.2 (Go-native LSP library with stdio support, adequate for our needs)
- Full document sync mode (TextDocumentSyncKindFull) -- simplest approach, entire document content on each change
- Parse and analyze on every document change for immediate feedback (no debouncing yet)
- Combined parse + analysis diagnostics published together to avoid flicker

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all functionality is fully wired.

## Next Phase Readiness
- LSP server foundation ready for go-to-definition, hover, and completion (Phase 09 Plans 02-03)
- DocumentStore pattern established for additional language features
- stc lsp command available for VS Code extension client

---
*Phase: 09-lsp-vs-code-extension*
*Completed: 2026-03-28*
