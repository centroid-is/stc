---
phase: 02-preprocessor
plan: 01
subsystem: preprocessor
tags: [iec-61131-3, conditional-compilation, source-map, preprocessor]

# Dependency graph
requires:
  - phase: 01-parser-foundation
    provides: "source.Pos/Span types, diag.Diagnostic type"
provides:
  - "Preprocess function: (source, defines) -> (output, sourcemap, diagnostics)"
  - "SourceMap type for remapping preprocessed positions to original positions"
  - "Directive parser supporting IF/ELSIF/ELSE/END_IF/DEFINE/ERROR"
  - "Condition evaluator with defined()/NOT/AND/OR"
affects: [03-type-checker, 07-vendor-emit, cli-preprocess-command]

# Tech tracking
tech-stack:
  added: []
  patterns: [recursive-descent-condition-evaluator, line-based-preprocessing, source-map-position-remapping]

key-files:
  created:
    - pkg/preprocess/preprocess.go
    - pkg/preprocess/directive.go
    - pkg/preprocess/sourcemap.go
    - pkg/preprocess/preprocess_test.go
    - pkg/preprocess/sourcemap_test.go
  modified: []

key-decisions:
  - "Line-based preprocessing with stack-based IF nesting — simple and correct for IEC 61131-3 directives"
  - "Source map stores per-line mappings (not per-character) — sufficient for ST where directives occupy full lines"
  - "Condition evaluator uses recursive descent parser with NOT > AND > OR precedence"
  - "Diagnostic codes: PP001 (ERROR directive), PP002 (unmatched END_IF/ELSIF/ELSE), PP003 (unclosed IF)"

patterns-established:
  - "Preprocessor returns Result struct with Output, SourceMap, Diags — consistent with parser's diagnostic pattern"
  - "Non-preprocessor pragmas pass through unchanged — allows {attribute '...'} to reach the parser"

requirements-completed: [PREP-01, PREP-02, PREP-03, PREP-04]

# Metrics
duration: 3min
completed: 2026-03-26
---

# Phase 02 Plan 01: Preprocessor Core Summary

**IEC 61131-3 conditional compilation preprocessor with IF/ELSIF/ELSE/END_IF, DEFINE, ERROR directives and line-level source map for position remapping**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-26T17:15:39Z
- **Completed:** 2026-03-26T17:18:39Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Preprocessor evaluates all 6 directive types (IF, ELSIF, ELSE, END_IF, DEFINE, ERROR) with nested block support
- Source map correctly maps preprocessed line:col back to original file:line:col
- Condition evaluator supports defined(NAME), NOT, AND, OR with correct precedence
- Non-preprocessor pragmas ({attribute '...'}) pass through unchanged to the parser
- 27 tests covering all directive types, nesting, edge cases, and source map accuracy

## Task Commits

Each task was committed atomically:

1. **Task 1: Source map type and directive parser** - `c9b68d0` (test+feat)
2. **Task 2: Preprocess function with conditional compilation** - `87dbee5` (feat)

**Plan metadata:** (pending final commit)

_Note: TDD tasks committed RED+GREEN together since package was created from scratch_

## Files Created/Modified
- `pkg/preprocess/sourcemap.go` - SourceMap type with AddMapping/OriginalPos for position remapping
- `pkg/preprocess/directive.go` - Directive parser (parseDirective) and condition evaluator (evalCondition)
- `pkg/preprocess/preprocess.go` - Preprocess function with Result/Options types, line-by-line directive evaluation
- `pkg/preprocess/sourcemap_test.go` - Tests for SourceMap, parseDirective, evalCondition
- `pkg/preprocess/preprocess_test.go` - Tests for Preprocess function including source map validation

## Decisions Made
- Line-based preprocessing with stack-based IF nesting -- simple and correct for IEC 61131-3 directives where each directive occupies a full line
- Source map stores per-line mappings (not per-character) -- sufficient for ST where directives occupy full lines
- Condition evaluator uses recursive descent with NOT > AND > OR precedence
- Diagnostic codes: PP001 (ERROR directive), PP002 (unmatched END_IF/ELSIF/ELSE), PP003 (unclosed IF)
- DEFINE only adds to file-local defines map (copy of input) -- does not mutate caller's defines

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- pkg/preprocess package fully operational with Preprocess, SourceMap, directive parser
- Ready for CLI integration (preprocess subcommand) in plan 02-02
- Source map ready for downstream parser/checker to remap diagnostics to original positions

## Self-Check: PASSED

- All 5 created files verified present on disk
- Commit c9b68d0 (Task 1) verified in git log
- Commit 87dbee5 (Task 2) verified in git log
- All 27 tests pass, go vet clean

---
*Phase: 02-preprocessor*
*Completed: 2026-03-26*
