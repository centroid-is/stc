---
phase: 01-project-bootstrap-parser
plan: 03
subsystem: lexer
tags: [iec-61131-3, lexer, scanner, tokenizer, structured-text, cst]

# Dependency graph
requires:
  - phase: 01-project-bootstrap-parser/01-01
    provides: Go module scaffold and project structure
  - phase: 01-project-bootstrap-parser/01-02
    provides: AST node types with Pos/Span for position compatibility
provides:
  - TokenKind enum covering all IEC 61131-3 keywords, operators, literals, and trivia
  - Case-insensitive keyword lookup table (~80 keywords)
  - Hand-written scanner producing complete token streams with trivia preservation
  - Full position tracking (file:line:col:offset) on every token
  - Typed literal scanning (INT#42, REAL#3.14, T#5s, D#2024-01-15)
  - Nested block comment support
  - Pragma token scanning
affects: [parser, formatter, lsp]

# Tech tracking
tech-stack:
  added: []
  patterns: [hand-written-lexer, trivia-preservation, case-insensitive-keywords]

key-files:
  created:
    - pkg/lexer/token.go
    - pkg/lexer/keywords.go
    - pkg/lexer/position.go
    - pkg/lexer/lexer.go
    - pkg/lexer/lexer_test.go
    - pkg/lexer/testdata/motor_control.st
    - pkg/lexer/testdata/typed_literals.st
    - pkg/lexer/testdata/oop_extensions.st
    - pkg/lexer/testdata/pragmas.st
  modified: []

key-decisions:
  - "Lexer-local Pos/Span types mirroring ast.Pos/Span to avoid circular imports; parser will translate"
  - "Time/date literal prefixes (T, TIME, D, DATE, DT, TOD) scanned as compound tokens including # and value"
  - "Typed literal prefixes (INT, REAL, BOOL, etc.) followed by # scanned as single TypedLiteral token"

patterns-established:
  - "Trivia preservation: whitespace and comments emitted as tokens, not discarded"
  - "Case-insensitive keywords: uppercase lookup, original casing preserved in token text"
  - "Nested block comments: depth counter incremented on (*, decremented on *)"

requirements-completed: [PARS-04, PARS-07, PARS-09, PARS-10]

# Metrics
duration: 3min
completed: 2026-03-26
---

# Phase 01 Plan 03: Lexer Summary

**Hand-written IEC 61131-3 lexer with ~80 case-insensitive keywords, nested block comments, typed/time/date literals, multi-base integers, pragmas, and trivia preservation for CST fidelity**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-26T16:42:15Z
- **Completed:** 2026-03-26T16:45:15Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Complete TokenKind enum with all IEC 61131-3 + CODESYS keywords, operators, literals, and trivia types
- Case-insensitive keyword lookup table covering ~80 keywords including 64-bit types (LINT, LREAL, LWORD, ULINT) and OOP extensions
- Hand-written scanner handling nested block comments, typed literals (INT#42), time literals (T#5s), date literals (D#2024-01-15), multi-base integers (16#FF), pragmas, string escapes, and all operators
- 16 passing tests covering motor control FB, typed literals, OOP syntax, pragmas, nested comments, case insensitivity, position tracking, operators, 64-bit types, empty input, illegal characters, string escapes, wide strings, range operator, assign/arrow, and trivia preservation

## Task Commits

Each task was committed atomically:

1. **Task 1: Create token types, keyword table, and position tracking** - `9f2b034` (feat)
2. **Task 2: Implement lexer scanner with trivia, literals, nested comments, and comprehensive tests** - `e32f3e3` (feat)

## Files Created/Modified
- `pkg/lexer/token.go` - TokenKind enum, Token struct, Pos struct, IsKeyword/IsOperator/IsTrivia helpers
- `pkg/lexer/keywords.go` - Case-insensitive keyword lookup table with ~80 IEC 61131-3 keywords
- `pkg/lexer/position.go` - Pos.String() and Span type with SpanFrom constructor
- `pkg/lexer/lexer.go` - Hand-written scanner with trivia preservation, nested comments, typed literals
- `pkg/lexer/lexer_test.go` - 16 test functions covering all lexer constructs
- `pkg/lexer/testdata/motor_control.st` - FB with VAR_INPUT/OUTPUT, IF/AND/NOT control flow
- `pkg/lexer/testdata/typed_literals.st` - INT#, REAL#, T#, D#, 16#, 2# literals
- `pkg/lexer/testdata/oop_extensions.st` - INTERFACE, METHOD, PROPERTY, EXTENDS, IMPLEMENTS
- `pkg/lexer/testdata/pragmas.st` - {attribute '...'} pragma tokens

## Decisions Made
- Lexer defines its own Pos/Span types rather than importing from ast package, avoiding circular imports. The parser will translate between lexer.Pos and ast.Pos.
- Time/date literal prefixes (T, TIME, D, DATE, DT, TOD) and typed literal prefixes (INT, REAL, BOOL, etc.) are scanned as compound tokens when followed by #, not as separate keyword + hash + value tokens.
- The literal value scanner after # is permissive, consuming alphanumerics, dots, colons, and signs to handle all time/date formats.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Token stream ready for recursive descent parser (Plan 04)
- All keyword tokens defined for parser dispatch
- Trivia tokens available for CST node attachment
- Position tracking on every token for diagnostic messages

## Self-Check: PASSED

All 9 created files verified present. Both task commits (9f2b034, e32f3e3) verified in git log.

---
*Phase: 01-project-bootstrap-parser*
*Completed: 2026-03-26*
