---
phase: 12-io-address-parser-table
verified: 2026-03-30T00:00:00Z
status: passed
score: 11/11 must-haves verified
re_verification: false
---

# Phase 12: I/O Address Parser & Table Verification Report

**Phase Goal:** AT-addressed variables mapped to mock I/O table with scan-cycle sync
**Verified:** 2026-03-30
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from Plan 01 + Plan 02 must_haves)

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | ParseAddress correctly parses %IX0.0, %IB0, %IW0, %ID0, %QX0.0, %QB4, %QW8, %MD48 into structured IOAddress | VERIFIED | TestParseAddress_ValidBitAddresses and TestParseAddress_ValidNonBitAddresses pass; all 12 cases covered |
| 2  | ParseAddress handles wildcards %I*, %Q*, %M* returning IsWildcard=true | VERIFIED | TestParseAddress_Wildcards passes all 3 wildcard forms |
| 3  | ParseAddress rejects malformed addresses like %ZZ0, %IX0.8, %IW-1 | VERIFIED | TestParseAddress_Invalid passes 7 invalid cases including invalid area, bit offset > 7, negative offset, missing % prefix |
| 4  | IOTable provides typed read/write for bit, byte, word, dword across I/Q/M areas | VERIFIED | iomap.go exports GetBit/SetBit/GetByte/SetByte/GetWord/SetWord/GetDWord/SetDWord; all round-trip tests pass |
| 5  | IOTable auto-grows when accessing offsets beyond initial capacity | VERIFIED | ensureCapacity doubles slice or grows to needed; auto-grow test in iomap_test.go passes |
| 6  | Lexer tokenizes %IX0.0, %QW4, %MD12, %I* as DirectAddr tokens after AT keyword context | VERIFIED | scanDirectAddr in lexer.go produces DirectAddr tokens; lexer_test.go AT context test passes |
| 7  | ScanCycleEngine reads AT %I* variables from IOTable at start of each Tick | VERIFIED | Tick step 0 copies AreaInput bindings; TestIOTableInputBit and TestIOTableInputWord pass |
| 8  | ScanCycleEngine writes AT %Q* variables to IOTable at end of each Tick | VERIFIED | Tick step 5 copies AreaOutput bindings; TestIOTableOutputBit and TestIOTableOutputWord pass |
| 9  | AT %M* variables read from and write to IOTable (memory area syncs both directions) | VERIFIED | AreaMemory handled in both step 0 (read) and step 5 (write); TestIOTableMemoryBidirectional passes |
| 10 | Checker emits a warning when two AT addresses overlap the same byte range | VERIFIED | checkATAddresses runs O(n^2) byte-range intersection; TestATOverlapWordAndBit, TestATOverlapDWordAndWord pass |
| 11 | Checker validates AT address format and rejects malformed addresses | VERIFIED | ParseAddress called on every AtAddress; TestATAddressInvalidFormat passes with SEMA030 error code |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/iomap/address.go` | IOAddress struct, ParseAddress, Area/Size constants | VERIFIED | Exports ParseAddress, IOAddress, Area, Size, AreaInput, AreaOutput, AreaMemory, SizeBit, SizeByte, SizeWord, SizeDWord, ByteSpan, String |
| `pkg/iomap/iomap.go` | IOTable with flat byte arrays and typed accessors | VERIFIED | Exports IOTable, NewIOTable, GetBit/SetBit/GetByte/SetByte/GetWord/SetWord/GetDWord/SetDWord, Reset |
| `pkg/lexer/token.go` | DirectAddr token kind | VERIFIED | DirectAddr at line 181, name "DirectAddr" at line 334 |
| `pkg/lexer/lexer.go` | Scanner for % direct address tokens | VERIFIED | scanDirectAddr method at line 458; case '%' branch at line 449 |
| `pkg/interp/scan.go` | IOTable field on ScanCycleEngine, I/O sync in Tick, IOBinding type | VERIFIED | IOBinding type, ioTable/ioBindings fields, Tick steps 0+5, IOTable() accessor, readIOValue/writeIOValue helpers |
| `pkg/checker/check.go` | AT address validation and overlap detection | VERIFIED | checkATAddresses method with ParseAddress call, POU type check, O(n^2) overlap detection |
| `pkg/checker/diag_codes.go` | SEMA030/SEMA031/SEMA032 codes | VERIFIED | CodeInvalidATAddress="SEMA030", CodeATNotAllowedHere="SEMA031", CodeATOverlap="SEMA032" |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/iomap/iomap.go` | `pkg/iomap/address.go` | Read/Write accept IOAddress | VERIFIED | GetBit/SetBit/GetWord/SetWord take Area (from IOAddress), called by readIOValue/writeIOValue passing addr.Area, addr.ByteOffset |
| `pkg/lexer/lexer.go` | `pkg/lexer/token.go` | produces DirectAddr tokens | VERIFIED | scanDirectAddr returns makeToken(DirectAddr, start) at lines 489 and 504 |
| `pkg/interp/scan.go` | `pkg/iomap/iomap.go` | ScanCycleEngine holds *iomap.IOTable | VERIFIED | ioTable field typed *iomap.IOTable; NewIOTable() called in NewScanCycleEngine; GetBit/SetBit/GetWord/SetWord/GetDWord/SetDWord called in readIOValue/writeIOValue |
| `pkg/interp/scan.go` | `pkg/iomap/address.go` | ParseAddress called during initializeEnv | VERIFIED | iomap.ParseAddress(vd.AtAddress.Name) at line 244 of scan.go |
| `pkg/checker/check.go` | `pkg/iomap/address.go` | ParseAddress called during type checking | VERIFIED | iomap.ParseAddress(vd.AtAddress.Name) at line 80 of check.go |
| `pkg/parser/var.go` | `pkg/lexer/token.go` | Parser accepts DirectAddr after AT | VERIFIED | Line 125: `p.at(lexer.Ident) || p.at(lexer.DirectAddr)` |

### Data-Flow Trace (Level 4)

Not applicable — all artifacts are logic/storage components, not data-rendering components. The IOTable is the data store itself; data flow is verified through behavioral tests (input inject -> Tick -> output read).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| ParseAddress parses all IEC forms | `go test ./pkg/iomap/... -run TestParseAddress` | PASS (all sub-tests pass) | PASS |
| IOTable round-trips bit/byte/word/dword | `go test ./pkg/iomap/...` | PASS | PASS |
| Lexer produces DirectAddr tokens | `go test ./pkg/lexer/... -run TestDirectAddr` | PASS | PASS |
| Scan-cycle I/O sync end-to-end | `go test ./pkg/interp/... -run TestIOTable` | PASS (11 IO tests) | PASS |
| Checker AT validation and overlap | `go test ./pkg/checker/... -run TestAT\|TestOverlap` | PASS (9 tests) | PASS |
| Zero regressions across all packages | `go test ./...` | PASS (all 19 packages) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| IO-01 | 12-01 | Parser handles AT address syntax in VAR blocks | SATISFIED | DirectAddr token, parser var.go accepts it after AT keyword; marked complete in REQUIREMENTS.md |
| IO-02 | 12-01, 12-02 | Interpreter maintains a mock I/O table mapping addresses to values | SATISFIED | IOTable in pkg/iomap with flat byte arrays; ScanCycleEngine holds *iomap.IOTable; marked complete |
| IO-03 | 12-02 | I/O values sync at scan cycle boundaries | SATISFIED | Tick step 0 reads inputs, step 5 writes outputs; TestIOTableInputBit/OutputBit confirm boundary sync |
| IO-05 | 12-02 | Address overlap detection warns when byte and bit addresses conflict | SATISFIED | checkATAddresses byte-span intersection; TestATOverlapWordAndBit, TestATOverlapDWordAndWord confirm |

Note: IO-04 ("Tests can inject I/O values via the mock I/O table") is assigned to Phase 14 and is out of scope for Phase 12.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

Scanned all phase-modified files for TODO/FIXME, placeholder comments, empty returns, and hardcoded empty data. None found. All functions have real implementations with actual logic.

### Human Verification Required

None. All behaviors are verifiable programmatically via the Go test suite. No visual rendering, real-time behavior, or external service integration is involved.

### Gaps Summary

No gaps. All 11 observable truths are verified, all 7 artifacts are substantive and wired, all 6 key links are confirmed, all 4 requirements (IO-01, IO-02, IO-03, IO-05) are satisfied, and the full test suite (19 packages) passes with zero regressions.

The phase goal — "AT-addressed variables mapped to mock I/O table with scan-cycle sync" — is fully achieved:

1. IEC 61131-3 AT address syntax (%IX0.0, %QW4, %MD48, wildcards) is parsed by the lexer as single DirectAddr tokens and stored by the parser on VarDecl.AtAddress.
2. ParseAddress converts AT address strings into structured IOAddress values with area, size, byte offset, bit offset, and wildcard flag.
3. IOTable provides flat byte-array storage for %I/%Q/%M areas with typed Get/Set for bit, byte, word, and dword widths, with auto-growing capacity.
4. ScanCycleEngine holds an IOTable, builds IOBinding records during initialization, and synchronizes AT-bound variables at scan-cycle boundaries (inputs before execution, outputs after).
5. The checker validates AT address format using ParseAddress, warns on AT use in FUNCTION_BLOCK/FUNCTION, and detects byte-range overlapping addresses within the same area.

---

_Verified: 2026-03-30_
_Verifier: Claude (gsd-verifier)_
