---
phase: 08-formatter-linter
plan: 02
subsystem: lint
tags: [plcopen, naming-conventions, linter, ast-walker, cli]

# Dependency graph
requires:
  - phase: 01-parser
    provides: AST node types, parser producing SourceFile
  - phase: 03-semantic
    provides: Diagnostic types, source.Pos, diag.Collector pattern
provides:
  - "pkg/lint package with rule-based linting engine"
  - "PLCopen coding guideline checks (magic numbers, deep nesting, long POUs, missing return type)"
  - "Configurable naming convention checks (POU PascalCase, var lowercase, constant UPPER_SNAKE)"
  - "stc lint CLI command with text and JSON output"
affects: [09-lsp, 10-ci]

# Tech tracking
tech-stack:
  added: []
  patterns: [rule-based-linting, ast-walker-visitor, configurable-conventions]

key-files:
  created:
    - pkg/lint/lint.go
    - pkg/lint/rules.go
    - pkg/lint/naming.go
    - pkg/lint/plcopen.go
    - pkg/lint/lint_test.go
    - cmd/stc/lint_cmd.go
    - cmd/stc/lint_cmd_test.go
  modified:
    - cmd/stc/stubs.go
    - cmd/stc/main_test.go

key-decisions:
  - "LINT0xx diagnostic code prefix for all lint rules (grep-friendly, distinct from SEMA/VEND)"
  - "Lint warnings exit 0, parse errors exit 1 (consistent with stc check convention)"
  - "Naming convention configurable via stc.toml [lint] naming_convention field"
  - "PascalCase regex allows underscore-separated segments (FB_Motor style) per ST convention"

patterns-established:
  - "AST walker pattern: recursive walkExprsInStmt/walkExpr for expression traversal in statements"
  - "Lint rule pattern: standalone check functions returning []diag.Diagnostic"

requirements-completed: [LINT-01, LINT-02, LINT-03, LINT-04]

# Metrics
duration: 4min
completed: 2026-03-28
---

# Phase 08 Plan 02: Linter Summary

**Rule-based ST linter with PLCopen coding guidelines, configurable naming conventions, and JSON output for CI integration**

## Performance

- **Duration:** 4 min (272s)
- **Started:** 2026-03-28T20:19:38Z
- **Completed:** 2026-03-28T20:24:10Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- PLCopen coding guideline checks: magic numbers, deep nesting (>3 levels), long POUs (>200 stmts), missing FUNCTION return type
- Naming convention checks: POU PascalCase, variable lowercase, constant UPPER_SNAKE_CASE with configurable "none" to disable
- stc lint CLI command with text and JSON output, reading naming convention from stc.toml
- 29 total tests (21 unit + 8 integration) covering all rules with positive and negative cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Create pkg/lint package (TDD)** - `45127ec` (test: failing tests) + `ff25e90` (feat: implementation)
2. **Task 2: Wire up stc lint CLI command** - `2f8f960` (feat)

**Plan metadata:** [pending] (docs: complete plan)

## Files Created/Modified
- `pkg/lint/rules.go` - Rule interface, LintOptions, diagnostic code constants (LINT001-LINT007)
- `pkg/lint/lint.go` - Lint/LintFile orchestrator, sorts diagnostics by position
- `pkg/lint/plcopen.go` - PLCopen rule implementations with AST walker
- `pkg/lint/naming.go` - Naming convention checks with PascalCase/UPPER_SNAKE regex
- `pkg/lint/lint_test.go` - 21 unit tests covering all rules
- `cmd/stc/lint_cmd.go` - Real stc lint command replacing stub
- `cmd/stc/lint_cmd_test.go` - 8 integration tests for CLI
- `cmd/stc/stubs.go` - Removed newLintCmd stub
- `cmd/stc/main_test.go` - Updated stub tests (lint removed from stub list)

## Decisions Made
- LINT0xx diagnostic code prefix for all lint rules (distinct from SEMA/VEND, grep-friendly)
- Lint warnings exit 0, parse errors exit 1 (consistent with stc check behavior)
- PascalCase regex `^[A-Z][a-zA-Z0-9]*(_[A-Z][a-zA-Z0-9]*)*$` allows FB_Motor style per ST convention
- Variable naming check uses simple "first char lowercase" rule (covers both lower_snake and lowerCamel)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Lint package ready for LSP integration (Phase 09 can call lint.LintFile on document changes)
- JSON output ready for CI pipeline integration
- stubs.go still contains fmt stub (to be handled by Plan 08-01)

---
*Phase: 08-formatter-linter*
*Completed: 2026-03-28*
