---
phase: 08-formatter-linter
plan: 03
subsystem: parser, formatter
tags: [trivia, comments, round-trip, AST, CST]

requires:
  - phase: 08-01
    provides: Formatter with trivia emission infrastructure
  - phase: 08-02
    provides: Lint rules and formatter structure
provides:
  - Parser trivia attachment (comments attached to AST nodes)
  - End-to-end comment preservation in parse->format pipeline
  - Idempotent formatting with comments
affects: [09-lsp, format, emit]

tech-stack:
  added: []
  patterns: [post-parse trivia attachment pass, buffer capture for trailing trivia insertion]

key-files:
  created:
    - pkg/parser/trivia.go
    - pkg/parser/trivia_test.go
    - pkg/format/format_roundtrip_test.go
  modified:
    - pkg/parser/parser.go
    - pkg/format/format.go

key-decisions:
  - "Post-parse trivia attachment via offset-to-node mapping rather than inline during parsing"
  - "Trailing trivia detection by same-line check against previous non-trivia token"
  - "Formatter emits leading trivia as indented comment lines, trailing trivia inline with space prefix"

patterns-established:
  - "Trivia attachment: collectNodes + findInnermostNode + token walk with pending buffer"
  - "Statement trailing trivia: buffer capture to insert before final newline"

requirements-completed: [FMT-03]

duration: 5min
completed: 2026-03-28
---

# Phase 08 Plan 03: Parser Trivia Attachment Summary

**Post-parse trivia attachment pass that maps lexer comment tokens to AST nodes, closing the parse->format comment preservation gap**

## Performance

- **Duration:** 5 min (313s)
- **Started:** 2026-03-28T20:20:28Z
- **Completed:** 2026-03-28T20:25:41Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Parser now attaches LineComment and BlockComment tokens as LeadingTrivia/TrailingTrivia on AST nodes
- End-to-end comment preservation verified: parse -> format preserves all line and block comments
- Formatter trivia emission improved to handle proper indentation and same-line positioning
- Format idempotency maintained with comments present

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement trivia attachment in parser and add unit tests**
   - `740350e` (test) - RED: failing trivia tests
   - `48dd03b` (feat) - GREEN: trivia attachment implementation + parser hook
2. **Task 2: End-to-end parse->format round-trip comment preservation tests**
   - `30c4157` (feat) - Round-trip tests + formatter trivia emission fixes

## Files Created/Modified
- `pkg/parser/trivia.go` - Post-parse trivia attachment: collectNodes, findInnermostNode, attachTrivia
- `pkg/parser/trivia_test.go` - Unit tests for leading/trailing/header/multiple comment attachment
- `pkg/parser/parser.go` - Hook attachTrivia() call after parseSourceFile()
- `pkg/format/format.go` - Improved emitLeadingTrivia/emitTrailingTrivia for proper formatting
- `pkg/format/format_roundtrip_test.go` - End-to-end tests proving comments survive formatting

## Decisions Made
- Used post-parse attachment (walking allTokens after AST is built) rather than inline attachment during parsing, to avoid complicating the recursive descent parser
- Trailing trivia identified by same-line check: if a comment token is on the same line as the previous non-trivia token's end position, it becomes TrailingTrivia
- Formatter uses buffer capture approach for statement trailing trivia to insert comments before the final newline without refactoring all statement emitters

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Formatter trivia emission produced concatenated output**
- **Found during:** Task 2
- **Issue:** emitTrivia wrote raw text without newlines/indentation, causing comments to merge with adjacent code
- **Fix:** Rewrote emitLeadingTrivia to emit each comment on its own indented line, and emitTrailingTrivia to emit with space prefix on the same line
- **Files modified:** pkg/format/format.go
- **Verification:** All round-trip tests pass including idempotency
- **Committed in:** 30c4157

**2. [Rule 1 - Bug] VarDecl trailing trivia emitted after newline**
- **Found during:** Task 2
- **Issue:** emitVarDecl called emitTrailingTrivia after newline(), putting trailing comments on wrong line
- **Fix:** Moved trailing trivia emission before newline in emitVarDecl
- **Files modified:** pkg/format/format.go
- **Committed in:** 30c4157

---

**Total deviations:** 2 auto-fixed (2 bugs in formatter trivia emission)
**Impact on plan:** Both fixes necessary for correct comment round-tripping. No scope creep.

## Issues Encountered
- Initial trivia attachment algorithm (findNodeForOffset with containment check) failed for trailing comments that fell between node boundaries. Redesigned to use token-centric approach: group comments between non-trivia tokens, then map tokens to nodes via offset lookup.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- FMT-03 gap is closed: stc fmt now preserves all comments through the parse->format pipeline
- All parser and formatter tests pass with no regressions
- Binary builds cleanly

## Known Stubs
None - all functionality is wired end-to-end.

---
*Phase: 08-formatter-linter*
*Completed: 2026-03-28*
