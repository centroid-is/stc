---
phase: 12-io-address-parser-table
plan: 01
subsystem: io
tags: [iec-61131-3, iomap, lexer, direct-address, plc-io]

requires:
  - phase: none
    provides: standalone foundation for I/O support
provides:
  - ParseAddress for IEC 61131-3 direct address parsing (%IX0.0, %QW4, %MD48, wildcards)
  - IOTable with flat byte arrays for I/Q/M areas with typed Get/Set accessors
  - DirectAddr lexer token for % address literals
  - Parser AT keyword accepts DirectAddr tokens
affects: [12-02, scan-cycle-engine, interp-io]

tech-stack:
  added: [encoding/binary for little-endian word/dword storage]
  patterns: [flat byte-array I/O model per IEC 61131-3, single-token direct address scanning]

key-files:
  created:
    - pkg/iomap/address.go
    - pkg/iomap/address_test.go
    - pkg/iomap/iomap.go
    - pkg/iomap/iomap_test.go
  modified:
    - pkg/lexer/token.go
    - pkg/lexer/lexer.go
    - pkg/lexer/lexer_test.go
    - pkg/parser/var.go

key-decisions:
  - "IOTable is pure byte-level storage with no interp.Value dependency -- integration deferred to Plan 02"
  - "DirectAddr token added after KwEndTestCase to avoid shifting existing iota values"
  - "Address parsing is case-insensitive with canonical String() output always uppercase"

patterns-established:
  - "IOTable area() returns *[]byte pointer for in-place slice growth"
  - "ensureCapacity doubles slice size or grows to needed, whichever is larger"

requirements-completed: [IO-01, IO-02]

duration: 3min
completed: 2026-03-30
---

# Phase 12 Plan 01: I/O Address Parser & Table Summary

**IEC 61131-3 direct address parser (%IX0.0, %QW4, %MD48, wildcards) with flat byte-array IOTable and DirectAddr lexer token**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-30T10:48:43Z
- **Completed:** 2026-03-30T10:52:05Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- ParseAddress handles all IEC address forms: bit/byte/word/dword, optional X prefix, wildcards, case-insensitive
- IOTable provides typed Get/Set for bit, byte, word (uint16), dword (uint32) across I/Q/M areas with little-endian storage
- IOTable auto-grows when accessing offsets beyond initial capacity
- Lexer tokenizes %IX0.0, %QW4, %MD12, %I* as single DirectAddr tokens
- Parser accepts DirectAddr tokens after AT keyword in variable declarations

## Task Commits

Each task was committed atomically:

1. **Task 1: Create pkg/iomap package (RED)** - `fb17f50` (test)
2. **Task 1: Create pkg/iomap package (GREEN)** - `8bb3b26` (feat)
3. **Task 2: Extend lexer with DirectAddr token** - `7957430` (feat)

## Files Created/Modified
- `pkg/iomap/address.go` - IOAddress struct, ParseAddress function, Area/Size constants, ByteSpan, String
- `pkg/iomap/address_test.go` - Comprehensive tests for valid, invalid, wildcard, and string formatting
- `pkg/iomap/iomap.go` - IOTable with flat byte arrays, typed Get/Set for bit/byte/word/dword, Reset
- `pkg/iomap/iomap_test.go` - Round-trip tests for all sizes, auto-grow, reset, bit isolation
- `pkg/lexer/token.go` - Added DirectAddr token kind
- `pkg/lexer/lexer.go` - Added scanDirectAddr method for % prefix tokens
- `pkg/lexer/lexer_test.go` - Tests for DirectAddr tokenization and AT context
- `pkg/parser/var.go` - Accept DirectAddr after AT keyword

## Decisions Made
- IOTable kept as pure byte-level storage (no interp.Value dependency) to avoid import cycles; Value integration deferred to Plan 02
- DirectAddr token added at end of const block (after KwEndTestCase) to avoid shifting existing iota values
- Address parsing uses strings.ToUpper for case-insensitive matching with canonical String() output always uppercase

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- IOTable and ParseAddress ready for scan cycle engine integration in Plan 02
- DirectAddr tokens ready for parser AT address binding
- All existing tests pass with zero regressions

## Self-Check: PASSED

All 9 files verified present. All 3 task commits verified in git log.

---
*Phase: 12-io-address-parser-table*
*Completed: 2026-03-30*
