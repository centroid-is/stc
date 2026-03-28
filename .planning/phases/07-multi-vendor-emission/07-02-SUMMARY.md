---
phase: 07-multi-vendor-emission
plan: 02
subsystem: cli-emit
tags: [cli, emit, vendor-targeting, beckhoff, schneider, portable, cobra]

requires:
  - phase: 07-multi-vendor-emission
    plan: 01
    provides: "AST-to-ST emitter with vendor profiles (pkg/emit)"
  - phase: 01-parser-foundation
    provides: "Parser and AST types"
provides:
  - "CLI `stc emit` command with --target and --format flags"
  - "User-facing vendor emission from command line"
affects: [08-cli-integration, 09-lsp]

tech-stack:
  added: []
  patterns: ["cobra RunE handler with file iteration and JSON/text output modes"]

key-files:
  created:
    - cmd/stc/emit_cmd.go
    - cmd/stc/emit_cmd_test.go
    - cmd/stc/testdata/emit_simple.st
    - cmd/stc/testdata/emit_oop.st
  modified:
    - cmd/stc/stubs.go
    - cmd/stc/main_test.go

key-decisions:
  - "Default target is portable (safest cross-vendor subset)"
  - "File separator markers (// --- file: name ---) for multi-file text output"
  - "Single-object JSON for one file, array for multiple (matching parse.go pattern)"

patterns-established:
  - "Emit command follows check.go pattern: RunE handler, format flag, exit 1 on errors"

requirements-completed: [EMIT-01, EMIT-02, EMIT-05]

duration: 2min
completed: 2026-03-28
---

# Phase 07 Plan 02: CLI Emit Command Summary

**Wire emit package into CLI as `stc emit <file> --target <vendor>` with text/JSON output**

## Performance

- **Duration:** 2 min (126s)
- **Started:** 2026-03-28T19:46:24Z
- **Completed:** 2026-03-28T19:48:30Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files created:** 4, modified: 2

## Accomplishments
- Full `stc emit` CLI command replacing the stub implementation
- Three vendor targets via `--target` flag: beckhoff, schneider, portable (default: portable)
- Text output prints emitted ST to stdout with file separator markers for multiple files
- JSON output with code, target, diagnostics, and has_errors fields
- Error handling for missing args, nonexistent files, and parse errors (exit 1)
- 10 integration tests covering all targets, both formats, error cases, and multi-file emission
- All 43 cmd/stc tests pass including existing parse/check/test/sim/pp tests (zero regressions)

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests** - `a4a6c51` (test)
2. **Task 1 (GREEN): Full emit command implementation** - `d4fd60f` (feat)

## Files Created/Modified
- `cmd/stc/emit_cmd.go` - CLI emit command with newEmitCmd and runEmit (131 lines)
- `cmd/stc/emit_cmd_test.go` - 10 integration tests (203 lines)
- `cmd/stc/testdata/emit_simple.st` - Simple PROGRAM fixture
- `cmd/stc/testdata/emit_oop.st` - FUNCTION_BLOCK with METHOD fixture
- `cmd/stc/stubs.go` - Removed newEmitCmd stub (keep lint/fmt stubs)
- `cmd/stc/main_test.go` - Updated stub tests to exclude emit

## Decisions Made
- Default target is "portable" (safest cross-vendor subset for users who omit the flag)
- File separator markers use `// --- file: <name> ---` format for multi-file text output
- JSON output follows parse.go pattern: single object for 1 file, array for multiple files
- Emit command follows check.go pattern for consistency (RunE handler, format flag, exit codes)

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs
None - all emission paths are fully implemented.

## Next Phase Readiness
- `stc emit` is fully operational for all three vendor targets
- JSON output enables tool integration and MCP server consumption
- Command integrates cleanly with existing CLI infrastructure

---
*Phase: 07-multi-vendor-emission*
*Completed: 2026-03-28*

## Self-Check: PASSED
- All 4 created files exist
- Both commit hashes verified (a4a6c51, d4fd60f)
- All 43 cmd/stc tests passing, go vet clean, build succeeds
