---
phase: 08-formatter-linter
verified: 2026-03-28T21:15:00Z
status: passed
score: 11/11 must-haves verified
re_verification: true
  previous_status: gaps_found
  previous_score: 10/11
  gaps_closed:
    - "Formatter preserves all comments (line and block) in correct positions — parser trivia attachment implemented in pkg/parser/trivia.go; parse->format round-trip verified end-to-end"
  gaps_remaining: []
  regressions: []
---

# Phase 08: Formatter-Linter Verification Report

**Phase Goal:** Users can auto-format ST code to a consistent style and check it against coding standards, with no commercial tool dependency
**Verified:** 2026-03-28T21:15:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (Plan 03 added parser trivia attachment)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can run `stc fmt <file>` and get consistently formatted ST code | VERIFIED | `cmd/stc/fmt_cmd.go` calls `format.Format()`; `TestFmtBasic` passes |
| 2 | Formatter preserves all comments (line and block) in correct positions | VERIFIED | `pkg/parser/trivia.go` `attachTrivia()` called from `Parse()`; 5/5 `TestTrivia*` and 6/6 `TestFormatRoundTrip*` pass; binary spot-check confirms both `// sensor count` and `(* set to zero *)` survive real format |
| 3 | Formatter is idempotent: `fmt(fmt(code)) == fmt(code)` | VERIFIED | `TestFormatIdempotent` and `TestFmtIdempotent` pass; `TestFormatRoundTripIdempotentWithComments` passes (4 comment-containing inputs) |
| 4 | Formatter style is configurable via flags (`--indent`, `--uppercase-keywords`) | VERIFIED | Both flags wired in `fmt_cmd.go`; `TestFmtCustomIndent` and `TestFmtLowercaseKeywords` pass |
| 5 | `stc fmt --format json` returns structured JSON output | VERIFIED | `fmtOutput` struct marshaled to JSON; `TestFmtJSONFormat` passes |
| 6 | User can run `stc lint <files...>` and get coding standard violations | VERIFIED | `cmd/stc/lint_cmd.go` calls `lint.LintFile()`; `TestLintMagicNumber` and `TestLintNamingViolation` pass |
| 7 | Linter checks PLCopen coding guidelines (magic numbers, deep nesting, long POUs) | VERIFIED | `plcopen.go` implements all 4 checks; 10 unit tests pass |
| 8 | Linter checks naming conventions (configurable: PascalCase FBs, lower_snake variables) | VERIFIED | `naming.go` with `checkNaming()`; naming good/bad tests all pass |
| 9 | Naming conventions configurable via `stc.toml` `[lint] naming_convention` field | VERIFIED | `lint_cmd.go` reads `cfg.Lint.NamingConvention`, passes to `LintOptions` |
| 10 | `stc lint --format json` returns structured JSON diagnostics | VERIFIED | `lint_cmd.go` marshals `allDiags` to JSON; `TestLintJSONFormat` passes |
| 11 | Linter diagnostics include file:line:col, severity, code, and message | VERIFIED | All diagnostics use `diag.Diagnostic{Pos: spanPos(n), Code: "LINT0xx", ...}`; `TestDiagnosticPositions` and `TestDiagnosticCodesPrefix` pass |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/format/format.go` | Format function producing formatted ST from AST | VERIFIED | Full type-switch AST walk; exports `Format(file, opts)` |
| `pkg/format/options.go` | FormatOptions with configurable indent and casing | VERIFIED | Exports `FormatOptions` and `DefaultFormatOptions()` |
| `pkg/format/format_test.go` | Unit tests for formatting including idempotency | VERIFIED | 14 tests pass |
| `pkg/format/format_roundtrip_test.go` | End-to-end parse->format comment preservation tests | VERIFIED | 6 tests pass; covers leading, trailing, file-header, multiple, body comments, and idempotency with comments |
| `cmd/stc/fmt_cmd.go` | Real stc fmt command replacing stub | VERIFIED | Full implementation with `--indent`, `--uppercase-keywords`, `--format json` |
| `cmd/stc/fmt_cmd_test.go` | Integration tests for stc fmt command | VERIFIED | 10 tests pass |
| `pkg/parser/trivia.go` | Trivia attachment logic | VERIFIED | `attachTrivia()` implemented; `collectNodes`, `findInnermostNode`, `tokenToTrivia` all present |
| `pkg/parser/trivia_test.go` | Unit tests for trivia attachment | VERIFIED | 5 tests pass: leading, trailing, file-header, multiple comments, no-regression |
| `pkg/lint/lint.go` | Lint function orchestrating all lint rules | VERIFIED | Exports `Lint`, `LintFile`, `LintResult` |
| `pkg/lint/rules.go` | Rule interface and rule registry | VERIFIED | Exports `Rule` interface, `LintOptions`, `DefaultLintOptions()`, 7 `LINT0xx` constants |
| `pkg/lint/naming.go` | Naming convention lint rules | VERIFIED | Implements `checkNaming()` |
| `pkg/lint/plcopen.go` | PLCopen guideline lint rules | VERIFIED | All 4 PLCopen checks implemented |
| `pkg/lint/lint_test.go` | Unit tests for all lint rules | VERIFIED | 21 tests pass |
| `cmd/stc/lint_cmd.go` | Real stc lint command replacing stub | VERIFIED | Full implementation with JSON output, config loading, exit-code convention |
| `cmd/stc/lint_cmd_test.go` | Integration tests for stc lint command | VERIFIED | 8 tests pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/parser/trivia.go` | `pkg/parser/parser.go` | `attachTrivia` called in `Parse()` after `parseSourceFile()` | VERIFIED | Line 45 of `parser.go`: `attachTrivia(file, allTokens)` |
| `pkg/parser/trivia.go` | `pkg/ast/node.go` | Sets `LeadingTrivia`/`TrailingTrivia` on `NodeBase` | VERIFIED | Lines 66, 81 in `trivia.go` write to `node.TrailingTrivia` and `node.LeadingTrivia` |
| `cmd/stc/fmt_cmd.go` | `pkg/format/format.go` | Calls `format.Format(file, opts)` | VERIFIED | `format.Format(result.File, opts)` in `fmt_cmd.go` |
| `cmd/stc/fmt_cmd.go` | `pkg/parser` | Parses input file then formats | VERIFIED | `parser.Parse(filename, string(content))` in `fmt_cmd.go` |
| `pkg/lint/lint.go` | `pkg/ast/node.go` | Walks AST checking rules against each node | VERIFIED | `ast.SourceFile`, `ast.Declaration` used throughout |
| `pkg/lint/lint.go` | `pkg/diag/diagnostic.go` | Produces Diagnostic structs with lint codes | VERIFIED | `diag.Diagnostic` returned from all check functions |
| `cmd/stc/lint_cmd.go` | `pkg/lint/lint.go` | Calls `lint.LintFile(file, opts)` | VERIFIED | `lint.LintFile(parseResult.File, opts)` in `lint_cmd.go` |
| `pkg/lint/naming.go` | `pkg/project/config.go` | Reads `naming_convention` from `LintConfig` | VERIFIED | `cfg.Lint.NamingConvention` flows to `opts.NamingConvention` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `cmd/stc/fmt_cmd.go` | `code` (formatted ST) | `format.Format(result.File, opts)` where `result.File` comes from `parser.Parse()` which now calls `attachTrivia()` | Yes — AST from real parsed source with comment trivia attached | FLOWING |
| `cmd/stc/lint_cmd.go` | `allDiags` | `lint.LintFile(parseResult.File, opts)` + `parseResult.Diags` | Yes — diagnostics from real AST analysis | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Parser trivia unit tests | `go test ./pkg/parser/ -run TestTrivia -count=1` | 5/5 PASS | PASS |
| Format round-trip tests | `go test ./pkg/format/ -run TestFormatRoundTrip -count=1` | 6/6 PASS | PASS |
| All parser tests (regression) | `go test ./pkg/parser/ -count=1` | PASS | PASS |
| All format tests (regression) | `go test ./pkg/format/ -count=1` | PASS (14 original + 6 new = 20) | PASS |
| All lint tests (regression) | `go test ./pkg/lint/ -count=1` | PASS | PASS |
| CLI fmt + lint integration tests | `go test ./cmd/stc/ -run "TestFmt\|TestLint" -count=1` | PASS | PASS |
| go vet | `go vet ./pkg/parser/ ./pkg/format/ ./pkg/lint/ ./cmd/stc/` | No warnings | PASS |
| Real binary comment round-trip | `stc fmt /tmp/test_comments.st` (file with `// sensor count` and `(* set to zero *)`) | Both comments present in output | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| FMT-01 | 08-01-PLAN | `stc fmt <file>` auto-formats ST code with correct indentation, keyword casing, spacing | SATISFIED | `fmt_cmd.go` + `format.go`; `TestFmtBasic` verifies 4-space indent and uppercase keywords |
| FMT-02 | 08-01-PLAN | Formatter configurable via `--indent` and `--uppercase-keywords` flags | SATISFIED | Both flags present; `TestFmtCustomIndent` (2-space) and `TestFmtLowercaseKeywords` pass |
| FMT-03 | 08-01/03-PLAN | Formatter preserves all comments (line and block) in output | SATISFIED | `pkg/parser/trivia.go` attaches comment trivia post-parse; `pkg/format/format_roundtrip_test.go` proves full pipeline; real binary confirmed with physical file |
| LINT-01 | 08-02-PLAN | `stc lint <files...>` reports coding standard violations with file:line:col | SATISFIED | `lint_cmd.go` prints `d.String()` to stderr; `TestLintMagicNumber` verifies LINT001 code in output |
| LINT-02 | 08-02-PLAN | Linter checks PLCopen guidelines (magic numbers, deep nesting, long POUs, missing return type) | SATISFIED | All 4 checks in `plcopen.go`; 8 focused unit tests pass |
| LINT-03 | 08-02-PLAN | Linter checks naming conventions (POU PascalCase, vars lowercase, constants UPPER_SNAKE), configurable via stc.toml | SATISFIED | `naming.go` + config wiring in `lint_cmd.go`; `TestNamingConventionNone` verifies disable works |
| LINT-04 | 08-02-PLAN | `stc lint --format json` outputs machine-readable JSON diagnostics | SATISFIED | `allDiags` marshaled to JSON in `lint_cmd.go`; `TestLintJSONFormat` verifies valid JSON |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/stc/stubs.go` | 1-7 | File contains only a comment — no stub functions remain but file not deleted | Info | Cosmetic only; does not affect functionality |

No blockers. The previous warning (comment tests bypassing the parse pipeline) is resolved — `format_roundtrip_test.go` now tests the real end-to-end path and all 6 tests pass.

### Human Verification Required

None. The previous human verification item (real comment round-trip) has been resolved by behavioral spot-check: the compiled binary was run against a physical file with both line and block comments, and both were preserved in the output.

### Re-verification Summary

**Gap closed:** Truth 2 ("Formatter preserves all comments (line and block) in correct positions") was the sole gap from initial verification.

**What was done (Plan 03):**
- `pkg/parser/trivia.go`: Post-parse `attachTrivia()` function that walks all tokens, maps comment tokens to their nearest AST nodes, and populates `LeadingTrivia`/`TrailingTrivia` on `NodeBase`. Same-line comments become trailing trivia; preceding comments become leading trivia.
- `pkg/parser/parser.go`: `attachTrivia(file, allTokens)` called at line 45 after `parseSourceFile()` completes.
- `pkg/format/format.go`: Improved `emitLeadingTrivia`/`emitTrailingTrivia` to emit comments on their own indented lines (leading) or inline with space prefix (trailing), fixing concatenation artifacts.
- `pkg/parser/trivia_test.go`: 5 unit tests directly inspecting AST node trivia after `Parse()`.
- `pkg/format/format_roundtrip_test.go`: 6 end-to-end tests proving comments survive the full `parse -> format` pipeline, including idempotency with comments.

**No regressions:** All previously-passing tests continue to pass across all four packages.

**All 11 truths verified. All 7 requirements (FMT-01 through FMT-03, LINT-01 through LINT-04) satisfied. Phase 08 goal achieved.**

---

_Verified: 2026-03-28T21:15:00Z_
_Verifier: Claude (gsd-verifier)_
