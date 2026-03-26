---
phase: 01-project-bootstrap-parser
verified: 2026-03-26T17:30:00Z
status: passed
score: 5/5 must-haves verified
re_verification: false
---

# Phase 1: Project Bootstrap & Parser Verification Report

**Phase Goal:** Users can parse any IEC 61131-3 Ed.3 ST source file (including CODESYS OOP extensions) and get a structured AST or actionable error messages via a single CLI binary
**Verified:** 2026-03-26T17:30:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | User can run `stc parse <file>` on ST containing PROGRAM, FB, FUNCTION, TYPE, INTERFACE, METHOD, PROPERTY and get a JSON AST back | VERIFIED | `stc parse motor_control.st --format json` returns valid JSON with kind-discriminated nodes; OOP fixture produces InterfaceDecl + FunctionBlockDecl with EXTENDS/IMPLEMENTS/methods |
| 2 | User can run `stc parse` on broken/incomplete ST and get a partial AST with error nodes plus actionable diagnostics showing file:line:col | VERIFIED | `stc parse broken_input.st --format json` returns has_errors=true, partial AST, 3 diagnostics each with file:line:col; TestParse_ErrorRecovery passes |
| 3 | User can run `stc --version` and every subcommand supports `--format json` | VERIFIED | `stc --version` outputs "stc dev (commit: unknown, built: unknown)"; all stub subcommands return JSON `{"error":"not yet implemented"}` with --format json; TestCLI_Version and TestCLI_StubCommandsJSON pass |
| 4 | User can create an `stc.toml` project manifest defining source roots and vendor target | VERIFIED | pkg/project/config.go provides LoadConfig/FindConfig; testdata/stc.toml has source_roots and vendor_target="beckhoff"; TestLoadConfig passes |
| 5 | Parser correctly handles CODESYS extensions (OOP, POINTER TO, REFERENCE TO, 64-bit types), all control structures, all VAR sections, arrays, structs, enums, pragmas | VERIFIED | TestParse_FunctionBlockOOP (EXTENDS/IMPLEMENTS), TestParse_PointersRefs (PointerType/ReferenceType), TestParse_ControlFlow (IF/CASE/FOR/WHILE/REPEAT), TestParse_VarSections (6 section types), TestParse_TypeDeclarations (enum/struct/array/subrange), TestParse_Pragmas — all pass; KwLint/KwLreal/KwLword/KwUlint in lexer |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `go.mod` | Go module definition `github.com/centroid-is/stc` | VERIFIED | Module + go 1.22 + cobra/toml/testify dependencies present |
| `Makefile` | Build automation with build/test/lint/install targets | VERIFIED | All 6 targets confirmed: build, test, lint, install, test-full, clean |
| `.github/workflows/ci.yml` | CI pipeline with golangci-lint | VERIFIED | ubuntu/macos/windows matrix + golangci-lint-action@v4 |
| `pkg/diag/diagnostic.go` | Diagnostic type with Severity, Pos | VERIFIED | `type Diagnostic struct` with source.Pos; String() returns "file:line:col: severity: message" |
| `pkg/source/source.go` | Pos, Span, SourceFile types | VERIFIED | All three types present with PosFromOffset and LineContent |
| `pkg/project/config.go` | Config struct, LoadConfig | VERIFIED | LoadConfig and FindConfig present; BurntSushi/toml import |
| `pkg/ast/node.go` | Node interface, NodeBase, NodeKind enum | VERIFIED | 40+ NodeKind constants, Node interface, NodeBase with trivia, marker interfaces |
| `pkg/ast/decl.go` | All POU declaration nodes | VERIFIED | SourceFile, ProgramDecl, FunctionBlockDecl (Extends/Implements), FunctionDecl, InterfaceDecl, MethodDecl, PropertyDecl, TypeDecl, ActionDecl |
| `pkg/ast/expr.go` | Expression nodes including DerefExpr | VERIFIED | BinaryExpr, UnaryExpr, Literal, CallExpr, MemberAccessExpr, IndexExpr, DerefExpr, ParenExpr |
| `pkg/ast/stmt.go` | All control structure statement nodes | VERIFIED | IfStmt, CaseStmt, ForStmt, WhileStmt, RepeatStmt, AssignStmt, CallStmt, ReturnStmt, ExitStmt |
| `pkg/ast/types.go` | Type specifiers including PointerType, ReferenceType | VERIFIED | NamedType, ArrayType, PointerType, ReferenceType, StringType, SubrangeType, EnumType, StructType |
| `pkg/lexer/token.go` | TokenKind enum with 64-bit types | VERIFIED | KwLint, KwLreal, KwLword, KwUlint present; Pragma, TimeLiteral, TypedLiteral tokens |
| `pkg/lexer/keywords.go` | Case-insensitive keyword lookup | VERIFIED | LookupKeyword present; FUNCTION_BLOCK, EXTENDS, IMPLEMENTS in table |
| `pkg/lexer/lexer.go` | Scanner with trivia and nested comments | VERIFIED | LookupKeyword called at line 355; nested comment depth tracking confirmed |
| `pkg/parser/parser.go` | Parser struct, Parse function, ParseResult | VERIFIED | ParseResult{File, Diags}, Parse entry point, peek/advance/expect helpers |
| `pkg/parser/expr.go` | Pratt expression parser | VERIFIED | parseExpr, infixPrecedence, parseUnaryExpr, parsePrimaryExpr present |
| `pkg/parser/error.go` | Error recovery, synchronize | VERIFIED | synchronize(), recoverStatement(), recoverDeclaration() present |
| `cmd/stc/main.go` | Cobra CLI root with all subcommands | VERIFIED | cobra.Command, --format persistent flag, all 7 subcommands registered |
| `cmd/stc/parse.go` | parse subcommand wired to parser.Parse | VERIFIED | parser.Parse called at line 46; json.MarshalIndent used for JSON output |
| `cmd/stc/stubs.go` | Stub subcommands (intentional) | VERIFIED | check/test/emit/lint/fmt/pp all print "not yet implemented"; JSON mode returns {"error":"not yet implemented"} |
| `pkg/version/version.go` | Version info for ldflags | VERIFIED | Version/Commit/Date variables; String() function; wired in main.go line 18 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/diag/diagnostic.go` | `pkg/source/source.go` | Diagnostic.Pos uses source.Pos type | WIRED | Lines 51-52: `Pos source.Pos`, `EndPos source.Pos` |
| `pkg/ast/node.go` | `pkg/ast/trivia.go` | NodeBase embeds LeadingTrivia/TrailingTrivia []Trivia | WIRED | Lines 145-146 confirmed |
| `pkg/ast/json.go` | `pkg/ast/decl.go` | MarshalJSON for declaration types | WIRED | MarshalNode dispatches via nodeToMap; kind discriminator on all nodes verified by live output |
| `pkg/lexer/lexer.go` | `pkg/lexer/keywords.go` | Keyword lookup during identifier scan | WIRED | Line 355: `LookupKeyword(text)` called in scanner |
| `pkg/parser/parser.go` | `pkg/lexer/lexer.go` | Parser consumes token stream | WIRED | Line 24: `lexer.Tokenize(filename, src)` |
| `pkg/parser/parser.go` | `pkg/ast/decl.go` | Parser produces AST nodes | WIRED | Line 168+: returns `*ast.SourceFile` |
| `pkg/parser/error.go` | `pkg/ast/node.go` | Error recovery creates ErrorNode | WIRED | Lines 70, 87: `ast.ErrorNode{}` |
| `cmd/stc/parse.go` | `pkg/parser/parser.go` | Parse subcommand calls parser.Parse | WIRED | Line 46: `parser.Parse(filename, ...)` |
| `cmd/stc/parse.go` | `pkg/ast/json.go` | JSON output marshals AST | WIRED | `json.MarshalIndent` used; AST implements json.Marshaler |
| `cmd/stc/main.go` | `pkg/version/version.go` | Version command reads version.String() | WIRED | Line 18: `Version: version.String()` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `cmd/stc/parse.go` | ParseResult.File (AST) | `parser.Parse(filename, content)` from lexer + recursive descent | Yes — tested live: JSON output contains real AST nodes from actual ST source | FLOWING |
| `cmd/stc/parse.go` | ParseResult.Diags | `diag.Collector` populated during parse | Yes — broken_input.st produces 3 real diagnostics with accurate file:line:col | FLOWING |
| `pkg/parser/parser.go` | ast.SourceFile.Declarations | `lexer.Tokenize` → recursive descent | Yes — all 11 parser tests pass parsing real ST fixture files | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `stc --version` outputs version string | `/tmp/stc --version` | "stc version stc dev (commit: unknown, built: unknown)" | PASS |
| `stc --help` lists all 7 subcommands | `/tmp/stc --help` | parse, check, test, emit, lint, fmt, pp all listed | PASS |
| `stc parse <file> --format json` produces valid JSON AST | parse motor_control.st | Valid JSON with kind="BinaryExpr", "Ident" nodes, spans with file:line:col | PASS |
| `stc parse <broken>` produces partial AST + diagnostics | parse broken_input.st | has_errors=true, 3 diagnostics, partial AST with IfStmt present | PASS |
| Stub subcommands respond correctly | `stc check` / `stc check --format json` | Text: "not yet implemented"; JSON: {"error":"not yet implemented"} | PASS |
| OOP parsing (EXTENDS/IMPLEMENTS) | parse function_block_oop.st | InterfaceDecl + FunctionBlockDecl with extends=FB_Base, implements=[IMotor], 2 methods | PASS |
| Pointer/reference types | parse pointers_refs.st | px: PointerType, rx: ReferenceType, arr: ArrayType — no diagnostics | PASS |
| Full test suite | `go test ./... -count=1` | All 7 packages pass (cmd/stc, ast, diag, lexer, parser, project, source) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| PARS-01 | 01-02, 01-04 | Parse IEC ST source (PROGRAM, FB, FUNCTION, TYPE) | SATISFIED | TestParse_ProgramBasic, TestParse_FunctionBlockOOP, TestParse_TypeDeclarations pass |
| PARS-02 | 01-02, 01-04 | CODESYS OOP (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS) | SATISFIED | FunctionBlockDecl has Extends/Implements; TestParse_FunctionBlockOOP passes; live parse confirms |
| PARS-03 | 01-02, 01-04 | POINTER TO, REFERENCE TO | SATISFIED | PointerType/ReferenceType nodes; TestParse_PointersRefs passes; live parse confirms |
| PARS-04 | 01-02, 01-03 | 64-bit types (LINT, LREAL, LWORD, ULINT) | SATISFIED | KwLint/KwLreal/KwLword/KwUlint in token.go; TestTokenize_64BitTypes passes |
| PARS-05 | 01-04 | Partial ASTs from broken code with error nodes | SATISFIED | TestParse_ErrorRecovery passes; live broken_input.st produces partial AST + 3 diagnostics |
| PARS-06 | 01-05 | `stc parse <file> --format json` outputs AST as JSON | SATISFIED | TestCLI_ParseJSONFormat passes; live parse verified |
| PARS-07 | 01-03, 01-04 | All control structures with full operator precedence | SATISFIED | TestParse_ControlFlow (IF/CASE/FOR/WHILE/REPEAT), TestParse_Expressions (precedence) pass |
| PARS-08 | 01-02, 01-04 | All VAR sections (VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_TEMP, VAR_GLOBAL) | SATISFIED | TestParse_VarSections with 6 VarBlocks passes; VarSection constants in ast/var.go |
| PARS-09 | 01-02, 01-04 | Arrays, structs, enums, subranges, typed literals | SATISFIED | TestParse_TypeDeclarations (enum/struct/array/subrange) passes; Literal kinds in expr.go |
| PARS-10 | 01-03, 01-04 | {attribute '...'} Beckhoff pragmas | SATISFIED | TestParse_Pragmas passes; Pragma TokenKind in lexer; PragmaNode in ast/var.go |
| CLI-01 | 01-05 | Single binary with subcommands: parse, check, test, emit, lint, fmt, pp | SATISFIED | TestCLI_Help verifies all 7 subcommands present in help output |
| CLI-02 | 01-05 | Every subcommand supports `--format json` | SATISFIED | Persistent --format flag; TestCLI_StubCommandsJSON passes; all stubs return JSON |
| CLI-03 | 01-05 | `stc --version` outputs version info | SATISFIED | TestCLI_Version passes; live output confirmed |
| CLI-04 | 01-01 | stc.toml project manifest with source roots, vendor target, library paths | SATISFIED | pkg/project/config.go with LoadConfig; testdata/stc.toml with source_roots/vendor_target/library_paths |
| CLI-05 | 01-01 | All diagnostics include file:line:col with actionable error messages | SATISFIED | Diagnostic.String() = "file:line:col: severity: message"; live broken_input shows accurate positions |

**All 15 requirements satisfied.**

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `cmd/stc/stubs.go` | "not yet implemented" in check/test/emit/lint/fmt/pp | Info | Intentional — future phases activate these; stubs correctly documented |

No unintentional stubs or placeholders found in any core package (lexer, parser, ast, diag, source, project, CLI parse command).

### Human Verification Required

None required. All success criteria are programmatically verifiable and have been verified.

### Summary

Phase 1 fully achieves its goal. All five observable success criteria from ROADMAP.md are satisfied:

1. The `stc parse` command produces real JSON ASTs from any valid ST source including full CODESYS OOP constructs (EXTENDS, IMPLEMENTS, METHOD, PROPERTY).
2. Broken ST input produces partial ASTs with error nodes and actionable file:line:col diagnostics — tested live.
3. `stc --version` works; every subcommand (including stubs) supports `--format json`.
4. `stc.toml` project manifest parsing is fully implemented and tested.
5. All CODESYS extensions, 64-bit types, control structures, VAR sections, arrays/structs/enums, and pragmas parse correctly — confirmed by 11 parser unit tests and 10 CLI integration tests.

All 15 requirements (PARS-01 through PARS-10, CLI-01 through CLI-05) are satisfied with passing unit tests and live behavioral verification. The full test suite (`go test ./...`) is green across all 7 packages.

The only "not yet implemented" code is in `cmd/stc/stubs.go`, which is explicitly intentional per the plan design — those subcommands will be activated in future phases.

---
_Verified: 2026-03-26T17:30:00Z_
_Verifier: Claude (gsd-verifier)_
