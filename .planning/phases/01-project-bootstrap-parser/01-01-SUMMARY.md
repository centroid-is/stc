---
phase: 01-project-bootstrap-parser
plan: 01
subsystem: infra
tags: [go, ci, makefile, source-positions, diagnostics, toml-config]

requires:
  - phase: none
    provides: greenfield project
provides:
  - Go module github.com/centroid-is/stc with cobra, toml, testify, go-cmp
  - CI pipeline on macOS, Windows, Linux with golangci-lint
  - Makefile with build/test/lint/install targets
  - source.Pos, source.Span, source.SourceFile types for position tracking
  - diag.Diagnostic, diag.Collector types for compiler diagnostics
  - project.Config, project.LoadConfig for stc.toml parsing
affects: [lexer, parser, ast, cli, semantic-analysis, lsp]

tech-stack:
  added: [go-1.22, cobra, BurntSushi-toml, testify, go-cmp, golangci-lint]
  patterns: [pkg-layout, table-driven-tests, json-tags-on-types, toml-tags-on-config]

key-files:
  created:
    - go.mod
    - Makefile
    - .github/workflows/ci.yml
    - .golangci.yml
    - .gitignore
    - pkg/source/source.go
    - pkg/source/source_test.go
    - pkg/diag/diagnostic.go
    - pkg/diag/collector.go
    - pkg/diag/diagnostic_test.go
    - pkg/project/config.go
    - pkg/project/config_test.go
    - pkg/project/testdata/stc.toml
  modified: []

key-decisions:
  - "Go 1.22 minimum in go.mod for broad compatibility with 1.26 installed"
  - "Pos type uses 1-based line and column numbers with 0-based byte offset"
  - "Diagnostic.String() format: file:line:col: severity: message (per CLI-05)"
  - "Severity as iota enum with JSON marshaling to string values"
  - "Config uses BurntSushi/toml with struct tags for clean parsing"

patterns-established:
  - "pkg/ layout: all compiler logic in pkg/, CLI in cmd/stc/"
  - "JSON tags on all exported types for --format json support"
  - "Table-driven tests with testify assert/require"
  - "Source positions as value types (Pos struct) passed by value"

requirements-completed: [CLI-04, CLI-05]

duration: 2min
completed: 2026-03-26
---

# Phase 01 Plan 01: Project Bootstrap Summary

**Go module with CI pipeline, Makefile, and foundation packages (source positions, diagnostics, TOML config) providing types imported by all downstream compiler packages**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-26T16:35:46Z
- **Completed:** 2026-03-26T16:38:16Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments
- Go module initialized at github.com/centroid-is/stc with all core dependencies
- CI pipeline running on macOS, Windows, Linux with separate lint job
- Makefile with build, test, lint, install, test-full, clean targets
- pkg/source: Pos, Span, SourceFile with binary-search offset-to-position conversion
- pkg/diag: Diagnostic with file:line:col formatting, Collector for accumulating errors/warnings
- pkg/project: Config struct with LoadConfig and FindConfig for stc.toml

## Task Commits

Each task was committed atomically:

1. **Task 1: Initialize Go module, Makefile, CI, and gitignore** - `57fc1a6` (feat)
2. **Task 2: Create source, diag, and project foundation packages** - `0f08b7a` (feat)

## Files Created/Modified
- `go.mod` - Go module definition with cobra, toml, testify, go-cmp dependencies
- `go.sum` - Dependency checksums
- `Makefile` - Build automation with 6 targets
- `.github/workflows/ci.yml` - CI pipeline with 3-OS matrix and lint job
- `.golangci.yml` - Linter config with govet, errcheck, staticcheck, unused, gosimple, ineffassign
- `.gitignore` - Binary, coverage, and vendor exclusions
- `pkg/source/source.go` - Pos, Span, SourceFile types with line offset computation
- `pkg/source/source_test.go` - 5 tests covering position string format, offset conversion, line content
- `pkg/diag/diagnostic.go` - Severity enum, Diagnostic type with String() and MarshalJSON
- `pkg/diag/collector.go` - Collector for accumulating and filtering diagnostics
- `pkg/diag/diagnostic_test.go` - 6 tests covering string format, collector operations, JSON marshaling
- `pkg/project/config.go` - Config struct with LoadConfig and FindConfig
- `pkg/project/config_test.go` - 2 tests covering TOML loading and missing file error
- `pkg/project/testdata/stc.toml` - Test fixture with project, build, lint sections

## Decisions Made
- Set Go 1.22 as minimum version in go.mod (per CONTEXT.md requirement) while using installed Go 1.26
- Used 1-based line/col numbering in Pos (standard for editors and compilers)
- Diagnostic.String() follows "file:line:col: severity: message" format per CLI-05 requirement
- Severity enum uses iota with custom JSON marshaling to lowercase strings
- SourceFile lazily computes line offsets on first PosFromOffset call

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `go build ./...` fails due to incomplete `pkg/ast/` code from a parallel agent (01-02 plan); this is out of scope for this plan and does not affect our packages which all build and test cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Foundation types ready for lexer (source.Pos, source.SourceFile)
- Foundation types ready for parser (diag.Diagnostic, diag.Collector)
- Foundation types ready for CLI (project.Config, project.LoadConfig)
- All downstream packages can import these types immediately

## Self-Check: PASSED

All 14 files verified present. Both task commits (57fc1a6, 0f08b7a) verified in git log.

---
*Phase: 01-project-bootstrap-parser*
*Completed: 2026-03-26*
