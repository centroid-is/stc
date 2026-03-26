---
phase: 01-project-bootstrap-parser
plan: 05
subsystem: cli
tags: [cobra, cli, parse, json, version, stubs]

# Dependency graph
requires:
  - phase: 01-project-bootstrap-parser
    provides: "Parser (01-04), AST (01-02), Lexer (01-03), Project/Diag (01-01)"
provides:
  - "stc binary with parse subcommand (text and JSON output)"
  - "stc --version with ldflags injection"
  - "Stub subcommands: check, test, emit, lint, fmt, pp"
  - "--format/-f flag on all commands"
  - "CLI integration test suite (10 tests)"
affects: [phase-02-type-checker, phase-05-lsp, phase-07-emit, phase-08-formatter]

# Tech tracking
tech-stack:
  added: [cobra, pflag]
  patterns: [cobra-subcommand-stubs, binary-integration-tests, ldflags-version-injection]

key-files:
  created:
    - cmd/stc/main.go
    - cmd/stc/parse.go
    - cmd/stc/version.go
    - cmd/stc/stubs.go
    - cmd/stc/main_test.go
    - pkg/version/version.go
    - testdata/parse/motor_control.st
    - testdata/parse/broken_input.st
  modified:
    - Makefile
    - go.mod
    - go.sum

key-decisions:
  - "Cobra for CLI framework with persistent --format flag"
  - "Stub subcommands return exit 0 with 'not yet implemented' message"
  - "Single-file JSON outputs object, multi-file outputs array"
  - "Binary integration tests via TestMain build + exec.Command"

patterns-established:
  - "stubCommand helper for future subcommand activation"
  - "parseOutput struct for JSON CLI output"
  - "TestMain binary build pattern for integration tests"

requirements-completed: [CLI-01, CLI-02, CLI-03, PARS-06]

# Metrics
duration: 2min
completed: 2026-03-26
---

# Phase 01 Plan 05: CLI Binary Summary

**Cobra-based stc CLI with parse subcommand (text/JSON), version info, stub subcommands, and 10 integration tests**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-26T16:57:31Z
- **Completed:** 2026-03-26T16:59:52Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Working stc binary with `stc parse` producing AST in text and JSON formats
- Version command via `stc --version` with ldflags injection in Makefile
- Stub subcommands (check, test, emit, lint, fmt, pp) respecting --format flag
- 10 integration tests covering parse, version, help, stubs, error cases, format flags
- Full project test suite green (all packages passing)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create CLI with parse command, version, stubs, and format flag** - `ef9162a` (feat)
2. **Task 2: CLI integration tests and end-to-end validation** - `016f181` (test)

## Files Created/Modified
- `cmd/stc/main.go` - Cobra root command with persistent --format flag, registers all subcommands
- `cmd/stc/parse.go` - Parse subcommand: reads files, calls parser.Parse, outputs text or JSON
- `cmd/stc/version.go` - Documents version handling via Cobra --version flag
- `cmd/stc/stubs.go` - Stub subcommands (check, test, emit, lint, fmt, pp) with JSON support
- `cmd/stc/main_test.go` - 10 integration tests building and running the stc binary
- `pkg/version/version.go` - Version, Commit, Date variables for ldflags injection
- `testdata/parse/motor_control.st` - Valid FUNCTION_BLOCK test fixture
- `testdata/parse/broken_input.st` - Broken input for error recovery testing
- `Makefile` - Added ldflags for version injection in build and install targets
- `go.mod` / `go.sum` - Added cobra dependency

## Decisions Made
- Used Cobra for CLI framework with persistent --format/-f flag inherited by all subcommands
- Stub subcommands return exit code 0 (not-implemented is informational, not an error)
- Single-file parse outputs a JSON object; multi-file outputs an array
- Integration tests use TestMain to build binary once, then exec.Command for each test
- Diagnostics printed to stderr in text mode, embedded in JSON object in json mode

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
- `cmd/stc/stubs.go` - check, test, emit, lint, fmt, pp commands all print "not yet implemented" (intentional; will be activated in future phases)

## Next Phase Readiness
- Phase 01 complete: lexer, parser, AST, diagnostics, project config, and CLI all working
- Ready for Phase 02 (type checker) which will activate the `check` subcommand
- Ready for Phase 07 (emit) which will activate the `emit` subcommand

## Self-Check: PASSED

All 8 created files verified present. Both task commits (ef9162a, 016f181) verified in git log.

---
*Phase: 01-project-bootstrap-parser*
*Completed: 2026-03-26*
