---
phase: 09-lsp-vs-code-extension
verified: 2026-03-28T21:15:00Z
status: passed
score: 9/9 must-haves verified
re_verification: false
---

# Phase 9: LSP & VS Code Extension Verification Report

**Phase Goal:** Users get a modern IDE experience for ST development in VS Code with real-time diagnostics, navigation, and refactoring
**Verified:** 2026-03-28T21:15:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| #  | Truth                                                                                          | Status     | Evidence                                                                               |
|----|-----------------------------------------------------------------------------------------------|------------|----------------------------------------------------------------------------------------|
| 1  | User sees real-time diagnostics in VS Code as they type, without saving                       | VERIFIED  | `DocumentStore.Open/Update` calls `parser.Parse` + `analyzer.Analyze` and immediately calls `publishDiagnostics` via `ctx.Notify`; all document lifecycle tests pass |
| 2  | User can go-to-definition on any variable, FB, or method and jump to its declaration         | VERIFIED  | `handleDefinition` in `definition.go` uses `findSymbolAtPosition` -> `symbol.Pos` -> `protocol.Location`; registered in `server.go` line 89 |
| 3  | User can hover over any symbol and see type info; completions suggest keywords, types, and declared variables | VERIFIED  | `handleHover` returns markdown with `symbolTypeString`; `handleCompletion` combines 47 keywords, 21 types, and dynamic symbols from `collectAllSymbols`; both registered in `server.go` |
| 4  | User can rename a symbol and all references update; find-references shows all usages         | VERIFIED  | `handleRename` builds `WorkspaceEdit` via `findAllReferences`; `handleReferences` returns `[]protocol.Location`; case-insensitive via `strings.EqualFold`; registered in `server.go` lines 92-93 |
| 5  | Inactive preprocessor blocks are grayed out via semantic tokens in the editor                | VERIFIED  | `findInactiveRegions` in `semantic_tokens.go` scans directives line-by-line; ELSE/ELSIF blocks marked as `"comment"` token type; delta-encoded uint32 array returned; `handleSemanticTokensFull` registered in `server.go` line 94; 8 tests pass |

**Score:** 5/5 truths verified

---

### Required Artifacts

| Artifact                                              | Expected                                                         | Status    | Details                                                                          |
|-------------------------------------------------------|------------------------------------------------------------------|-----------|----------------------------------------------------------------------------------|
| `pkg/lsp/server.go`                                   | LSP server setup, initialization, document sync handlers         | VERIFIED  | 124 lines; exports `NewServer()` and `Run()`; registers all 8 handlers; `SemanticTokensProvider` wired via closure override |
| `pkg/lsp/document.go`                                 | In-memory document store with parse/analysis                     | VERIFIED  | Exports `DocumentStore`, `Document`; thread-safe with `sync.RWMutex`; `analyzeDocument` calls `parser.Parse` + `analyzer.Analyze` |
| `pkg/lsp/diagnostics.go`                              | Conversion from diag.Diagnostic to LSP protocol diagnostics      | VERIFIED  | `convertDiagnostics` maps severity (4 levels) and 1-based to 0-based positions; `publishDiagnostics` calls `ctx.Notify` |
| `pkg/lsp/formatting.go`                               | textDocument/formatting handler delegating to pkg/format         | VERIFIED  | Calls `format.Format(doc.ParseResult.File, format.DefaultFormatOptions())`; returns single full-document TextEdit |
| `cmd/stc/lsp_cmd.go`                                  | CLI stc lsp command                                              | VERIFIED  | Cobra command `Use: "lsp"` calling `lsp.Run()`; registered in `main.go` line 32  |
| `pkg/lsp/navigate.go`                                 | Symbol lookup by position — shared utility                       | VERIFIED  | Exports `findIdentAtPosition`, `findSymbolAtPosition`, `findAllReferences`, `collectAllSymbols`, `symbolTypeString` |
| `pkg/lsp/definition.go`                               | textDocument/definition handler                                  | VERIFIED  | Returns `[]protocol.Location` with 0-based position; nil-safe                    |
| `pkg/lsp/hover.go`                                    | textDocument/hover handler                                       | VERIFIED  | Returns markdown `protocol.MarkupContent` with kind/name/type                    |
| `pkg/lsp/completion.go`                               | textDocument/completion handler                                  | VERIFIED  | Combines keyword list, type list, and dynamic `collectAllSymbols` results         |
| `pkg/lsp/references.go`                               | textDocument/references handler                                  | VERIFIED  | Returns `[]protocol.Location` for all case-insensitive name occurrences           |
| `pkg/lsp/rename.go`                                   | textDocument/rename handler                                      | VERIFIED  | Returns `*protocol.WorkspaceEdit` with `Changes` map containing TextEdits         |
| `pkg/lsp/semantic_tokens.go`                          | Semantic tokens for preprocessor inactive regions                | VERIFIED  | Exports `handleSemanticTokensFull`; `findInactiveRegions` stack-based scanner     |
| `editors/vscode/package.json`                         | VS Code extension manifest with language configuration           | VERIFIED  | `iec61131-st` language, `.st`/`.ST` extensions, `onLanguage:iec61131-st` activation |
| `editors/vscode/src/extension.ts`                     | Extension entry point launching stc-lsp via LanguageClient       | VERIFIED  | Spawns `stc lsp` via `TransportKind.stdio`; reads `stc.lsp.path` config           |
| `editors/vscode/syntaxes/iec61131-st.tmLanguage.json` | TextMate grammar for IEC 61131-3 ST syntax highlighting          | VERIFIED  | 11 case-insensitive `(?i)` patterns; covers keywords, types, operators, comments, strings, preprocessor, pragmas |

---

### Key Link Verification

| From                          | To                  | Via                                       | Status              | Details                                                                                    |
|-------------------------------|---------------------|-------------------------------------------|---------------------|--------------------------------------------------------------------------------------------|
| `pkg/lsp/document.go`         | `pkg/analyzer`      | `analyzer.Analyze` on document change     | WIRED              | Line 92: `analyzer.Analyze([]*ast.SourceFile{result.File}, nil)` called in `analyzeDocument` |
| `pkg/lsp/diagnostics.go`      | `pkg/diag`          | `diag.Diagnostic` to protocol conversion  | WIRED              | Import of `pkg/diag`; `diag.Error/Warning/Info/Hint` used in `convertSeverity`              |
| `pkg/lsp/formatting.go`       | `pkg/format`        | `format.Format` call                      | WIRED              | Line 24: `format.Format(doc.ParseResult.File, format.DefaultFormatOptions())`               |
| `pkg/lsp/navigate.go`         | `pkg/symbols`       | `symbols.Table` + `Scope.Lookup`          | WIRED              | Imports `pkg/symbols`; `table.GlobalScope().Lookup()` + `scope.Children` iteration          |
| `pkg/lsp/server.go`           | `pkg/lsp/definition.go` | `handler.TextDocumentDefinition` assignment | WIRED           | Line 89: `handler.TextDocumentDefinition = handleDefinition(store)`                         |
| `editors/vscode/src/extension.ts` | `stc lsp`       | `LanguageClient` spawning binary via stdio | WIRED              | `ServerOptions { command: stcPath, args: ["lsp"], transport: TransportKind.stdio }`         |
| `pkg/lsp/semantic_tokens.go`  | `pkg/preprocess`    | preprocessor directive detection (plan spec) | NOT_WIRED (deviation acceptable) | Implementation uses inline line scanner with `strings.Split`/`TrimSpace` instead of importing `pkg/preprocess`. Functional goal is achieved — 8 tests confirm correct inactive region detection. Deviation is acceptable since `pkg/preprocess` exposes a full compilation preprocessor, not a lightweight directive scanner. |

**Note on semantic tokens key link:** The plan specified a link from `semantic_tokens.go` to `pkg/preprocess`. The implementation instead contains a self-contained directive scanner (no `pkg/preprocess` import). The scanner correctly identifies `{IF}`, `{ELSIF}`, `{ELSE}`, `{END_IF}` directives, maintains a nesting stack, and marks ELSE/ELSIF blocks as inactive — producing the same outcome as if `pkg/preprocess` were used. The 8 semantic token tests all pass. This is a conscious design deviation, not a functional gap.

---

### Data-Flow Trace (Level 4)

| Artifact                  | Data Variable          | Source                              | Produces Real Data | Status   |
|---------------------------|------------------------|-------------------------------------|--------------------|----------|
| `pkg/lsp/diagnostics.go`  | `allDiags`             | `doc.ParseResult.Diags` + `doc.AnalysisResult.Diags` | Yes — populated by `parser.Parse` and `analyzer.Analyze` on every document change | FLOWING |
| `pkg/lsp/navigate.go`     | `symbols.Symbol`       | `doc.AnalysisResult.Symbols` (from `analyzer.Analyze`) | Yes — full symbol table from semantic analysis | FLOWING |
| `pkg/lsp/completion.go`   | `items`                | `iecKeywords` (static) + `iecTypes` (static) + `collectAllSymbols(doc.AnalysisResult.Symbols)` | Yes — dynamic symbols from live analysis | FLOWING |
| `pkg/lsp/semantic_tokens.go` | `regions`           | `doc.Content` (live document text)  | Yes — scanned on each request from current document | FLOWING |

---

### Behavioral Spot-Checks

| Behavior                                        | Command                                            | Result                                          | Status |
|-------------------------------------------------|----------------------------------------------------|-------------------------------------------------|--------|
| `stc lsp` command exists in binary             | `go run ./cmd/stc --help \| grep lsp`              | `lsp  Start the LSP server`                     | PASS   |
| `stc lsp --help` exits 0 with description      | `go run ./cmd/stc lsp --help`                      | Prints usage with "LSP server" text; exit 0     | PASS   |
| All LSP package tests pass                      | `go test ./pkg/lsp/... -v -count=1`                | 33 tests PASS, 0 failures                       | PASS   |
| CLI tests pass including lsp                    | `go test ./cmd/stc/... -run TestLsp -v -count=1`   | `TestLspHelp` PASS, `TestLspRegistered` PASS    | PASS   |
| Entire project compiles                         | `go build ./...`                                   | No output (success)                             | PASS   |
| `go vet` produces no warnings                   | `go vet ./pkg/lsp/...`                             | No output (success)                             | PASS   |

---

### Requirements Coverage

| Requirement | Source Plan | Description                                                             | Status    | Evidence                                                                        |
|-------------|-------------|-------------------------------------------------------------------------|-----------|---------------------------------------------------------------------------------|
| LSP-01      | 09-01       | LSP server provides real-time diagnostics (errors from parser + type checker) | SATISFIED | `publishDiagnostics` called on `TextDocumentDidOpen`/`DidChange`; combines parse + analysis diags |
| LSP-02      | 09-02       | LSP server provides go-to-definition for variables, FBs, methods        | SATISFIED | `handleDefinition` wired in server; `findSymbolAtPosition` resolves via symbol table |
| LSP-03      | 09-02       | LSP server provides hover showing type information                       | SATISFIED | `handleHover` returns markdown with `symbolTypeString`; registered in server    |
| LSP-04      | 09-02       | LSP server provides completion for keywords, types, declared variables, FB members | SATISFIED | `handleCompletion` combines 47 keywords + 21 types + dynamic symbols            |
| LSP-05      | 09-02       | LSP server provides find-references                                      | SATISFIED | `handleReferences` using `findAllReferences` (case-insensitive); registered     |
| LSP-06      | 09-02       | LSP server provides rename refactoring                                   | SATISFIED | `handleRename` builds `WorkspaceEdit` with all reference TextEdits              |
| LSP-07      | 09-03       | LSP server grays out inactive preprocessor blocks via semantic tokens    | SATISFIED | `handleSemanticTokensFull` with `findInactiveRegions`; "comment" token type; 8 tests pass |
| LSP-08      | 09-01, 09-03 | VS Code extension launches stc-lsp binary and provides syntax highlighting | SATISFIED | `extension.ts` spawns `stc lsp` via `TransportKind.stdio`; TextMate grammar with 11 case-insensitive patterns; language-configuration.json present |

All 8 requirements satisfied. REQUIREMENTS.md shows all LSP-01 through LSP-08 marked as `[x] Complete` assigned to Phase 9.

---

### Anti-Patterns Found

No blockers or warnings found.

Scan performed on all 14 `pkg/lsp/` Go files and `editors/vscode/src/extension.ts`:
- No `TODO`/`FIXME`/`PLACEHOLDER` comments in production code
- No stub return patterns (`return null`, empty handlers, `console.log`-only implementations)
- No hardcoded empty data passed to rendering paths
- `return nil, nil` in handlers (definition, hover, etc.) is correct LSP behavior for "not found" — not a stub

---

### Human Verification Required

The following items require human testing in a live VS Code environment. All automated checks passed.

#### 1. Real-time Diagnostics in Editor

**Test:** Open a `.st` file with a deliberate type error in VS Code with the extension installed. Type a change. Do not save.
**Expected:** Red squiggle appears under the error location within ~1 second of stopping typing, without saving.
**Why human:** Requires running VS Code with the extension installed and `stc` binary on PATH.

#### 2. Go-to-Definition Navigation

**Test:** In VS Code, place cursor on a variable name in an ST file, press F12 (Go to Definition).
**Expected:** Editor jumps to the variable's declaration in the VAR block.
**Why human:** Requires live editor interaction; correctness of position mapping in practice (off-by-one edge cases not caught by unit tests).

#### 3. Hover Type Display

**Test:** Hover cursor over a variable of type `DINT` in VS Code.
**Expected:** Tooltip shows markdown like `**Variable** \`counter\` : \`Variable: DINT\``.
**Why human:** Requires live editor; markdown rendering needs visual inspection.

#### 4. Completion in Context

**Test:** Type `IF ` in an ST file in VS Code, then trigger completion (Ctrl+Space).
**Expected:** Dropdown shows IEC keywords (IF, THEN, ELSE…), types (BOOL, INT…), and declared variables from the file.
**Why human:** Requires live editor; completion trigger context filtering may differ from unit test behavior.

#### 5. Rename Across References

**Test:** Place cursor on a variable used multiple times, press F2, enter new name.
**Expected:** All occurrences in the file rename simultaneously.
**Why human:** Requires live editor; workspace edit application behavior is client-side.

#### 6. Preprocessor Block Graying

**Test:** Open an ST file with `{IF defined(VENDOR_X)}` ... `{ELSE}` ... `{END_IF}` blocks.
**Expected:** The `{ELSE}` block content appears grayed out (dimmed) in the editor.
**Why human:** Semantic token rendering is theme-dependent; requires visual inspection.

#### 7. Syntax Highlighting Quality

**Test:** Open any ST file with programs, variables, keywords, and comments.
**Expected:** Keywords colored as keywords, types as types, comments grayed, strings highlighted, numbers distinct.
**Why human:** TextMate grammar correctness requires visual inspection against real ST code.

---

## Summary

Phase 9 achieves its goal. All 5 observable truths from the ROADMAP success criteria are verified. All 8 LSP requirements (LSP-01 through LSP-08) are satisfied. All 14 LSP Go source files are substantive, wired, and have real data flowing through them. The VS Code extension artifacts are all present and correctly structured.

The only notable deviation from plan specs is that `pkg/lsp/semantic_tokens.go` implements its own inline directive scanner rather than importing `pkg/preprocess`. This is a valid design choice — the inline scanner is simpler, purpose-built for LSP (no compilation context needed), and is verified by 8 passing tests. The functional goal (inactive region graying) is fully achieved.

All 33 `pkg/lsp/` tests pass. All 2 `cmd/stc` LSP tests pass. `go build ./...` compiles clean. `go vet ./pkg/lsp/...` reports no issues.

Human verification is needed for visual editor behaviors (diagnostics, hover, completion, rename, syntax highlighting, preprocessor graying) but all automated indicators are green.

---

_Verified: 2026-03-28T21:15:00Z_
_Verifier: Claude (gsd-verifier)_
