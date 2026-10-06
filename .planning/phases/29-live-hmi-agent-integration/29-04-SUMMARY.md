---
phase: 29-live-hmi-agent-integration
plan: 04
subsystem: docs
tags: [docs, twincat, ethercat, opcua, hmi, mcp, coverage]
requires:
  - phase: 29-02
    provides: "stc opcua snapshot, TF6100 diff test, HMI keymappings stand-in"
  - phase: 29-03
    provides: "stc-mcp --project/--io/--opcua and the four sim tools"
provides:
  - "docs/TWINCAT_IMPORT.md built from real command output"
  - "OPCUA.md sections: live writes and subscriptions, Connect the Flutter HMI, Capture a TF6100 snapshot, MCP tools"
  - "README quick start for a TwinCAT project with simulated EtherCAT and OPC UA"
  - "tests/docs_cli_test.go: docs cannot name an stc subcommand or flag that does not exist"
  - "Signed 29-VALIDATION.md"
affects: [27 docs follow-up, 29-01]
tech-stack:
  added: []
  patterns:
    - "Fenced blocks tagged `bash pending-phase-27` or `bash proposed` document unlanded features and are skipped by the docs test"
key-files:
  created:
    - docs/TWINCAT_IMPORT.md
    - tests/docs_cli_test.go
  modified:
    - docs/ETHERCAT_SIMULATION.md
    - docs/OPCUA.md
    - docs/CLI_REFERENCE.md
    - docs/TESTING_GUIDE.md
    - docs/ST_LANGUAGE_SUPPORT.md
    - docs/VENDOR_LIBRARIES.md
    - stdlib/vendor/beckhoff/ethercat_io.md
    - README.md
    - .planning/phases/29-live-hmi-agent-integration/29-VALIDATION.md
key-decisions:
  - "Phase 27 features (scenarios, SIM_* built-ins, stc test --io, serve --scenario) are documented from 27-CONTEXT and marked 'Lands with Phase 27'; their command blocks carry a pending-phase-27 tag"
  - "README repeats --io per export because stc sim and stc serve do not expand globs (only stc-mcp does)"
  - "OPCUA.md does not recommend a 127.0.0.1 bind as a safety measure, because the listener always binds all interfaces; it recommends an isolated network or --security basic256sha256"
requirements-completed: [DEVX-03, OPCUA-10]
duration: 60min
completed: 2026-10-06
---

# Phase 29 Plan 04: Docs, HMI and TF6100 procedures, docs-CLI test and coverage gate Summary

**A TwinCAT import guide and a README quick start built from real command output, OPC UA procedures for the Flutter HMI and the TF6100 capture, corrected stale claims, and a test that fails when a doc names a CLI flag that does not exist.**

## Performance

- **Duration:** about 60 min
- **Completed:** 2026-10-06
- **Tasks:** 4 of 4
- **Files:** 2 created, 9 modified

## Accomplishments

- `docs/TWINCAT_IMPORT.md` covers inputs, the library resolution order, TwinCAT syntax, preserved attributes and VEND020 to VEND027. Its worked example uses real output from `stc vendor import`, `stc check`, `stc sim` and `stc test --project` on `pkg/twincat/testdata/sln`.
- `docs/ETHERCAT_SIMULATION.md` gains a device model table, a fixture run with `--io`, the scenario TOML reference with every action key, the expect semantics, SCN001 to SCN010, live mode and the SIM_* built-ins. The Phase 27 parts are marked "Lands with Phase 27".
- `docs/OPCUA.md` adds four sections: live writes and the `p_cmd_*` handshake, the Flutter HMI procedure with checklist and troubleshooting, the TF6100 capture procedure, and the MCP tools with one example per tool.
- `docs/CLI_REFERENCE.md` documents `stc opcua snapshot`, `stc-mcp` flags and tools, and the 10 ms cycle advice for serve.
- `README.md` has the quick start the lead asked for, plus new CLI table rows.
- `tests/docs_cli_test.go` scans shell blocks in docs/*.md, README.md and ethercat_io.md. It resolves subcommands from `--help` and checks every flag. A mutation run proved it catches a misspelled flag.

## Stale-claim review (D-19)

| Doc | Claim | Action | Reason |
|-----|-------|--------|--------|
| ethercat_io.md | "stc does not model specific terminal hardware" | Changed | Topology from Device*.xml, TcLinkTo binding and device models exist |
| ethercat_io.md | "assign specific addresses when the hardware layout is finalized" | Changed | `%I*` with TcLinkTo is the supported path |
| ETHERCAT_SIMULATION.md | "scenario scripting ... is planned for Phase 27" | Changed | Replaced by the device model table and a marked scenario section |
| TESTING_GUIDE.md | I/O mocking only by explicit address | Changed | Added project mode, wildcard binding and the Phase 27 built-ins |
| ST_LANGUAGE_SUPPORT.md | THIS and SUPER "Partial" | Changed to Yes | `TestThisSuper` and `TestSuperInInheritedCode` cover runtime dispatch |
| ST_LANGUAGE_SUPPORT.md | POINTER and REFERENCE "Parsed and type-checked" | Changed | The interpreter executes ADR, `^` and REF= |
| ST_LANGUAGE_SUPPORT.md | Attribute pragma "used by emitter" | Changed | Attributes drive TcLinkTo, OPC UA exposure and qualified_only |
| ST_LANGUAGE_SUPPORT.md | WSTRING limitation | Kept, made precise | A probe showed LEN of a non-ASCII WSTRING returns UTF-8 bytes (13, not 11) |
| ST_LANGUAGE_SUPPORT.md | No native code generation | Kept, reworded | Still true; now mentions sim and serve as development runtimes |
| ST_LANGUAGE_SUPPORT.md | Interpreter performance | Kept, made concrete | About 8 ms per ST301 scan, so use `--cycle 10ms` |
| ST_LANGUAGE_SUPPORT.md | NAMESPACE "Partial" | Kept | Only qualified type lookup exists in the checker |
| ST_LANGUAGE_SUPPORT.md | SFC not supported | Kept | ST only |
| ST_LANGUAGE_SUPPORT.md | AB emission deferred | Kept | `stc emit --target` offers beckhoff, schneider and portable only |
| ST_LANGUAGE_SUPPORT.md | Cross-reference completeness gaps | Kept | Not contradicted by the code |
| VENDOR_LIBRARIES.md | "Today, stc cannot parse ... these FBs" | Status note added | The research document is dated; stubs, mocks, extract and import now exist, while `vendor init` does not |

## Task Commits

1. **Task 1: TwinCAT import guide, EtherCAT simulation and CLI reference** - `bb64f89` (docs)
2. **Task 2: OPC UA procedures, README quick start and stale-claim fixes** - `642cc93` (docs)
3. **Task 3 RED: failing docs checker tests** - `c88d043` (test)
4. **Task 3 GREEN: docs-to-CLI consistency test** - `cbb94d0` (feat)
5. **Task 4: validation sign-off** - `ae3f992` (docs)

## Verification

- `go test -race -count=1 ./...` is green.
- `bash scripts/coverage-gate.sh` passes with a 96.99% total. pkg/opcua is at 95.42%, bind at 100%, opcuatest at 90.72%, cmd/stc-mcp at 91.47% and pkg/projectload at 95.88%.
- The live and OPC UA tests passed three runs with `-race -count=3`.
- The docs contain no plant IPs or passwords (T-29-12).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Phase 27 and plan 29-01 are not merged**
- **Issue:** The plan documents `--scenario`, SIM_* built-ins, `stc test --io` and serve live mode as existing.
- **Fix:** They are documented from 27-CONTEXT under "Lands with Phase 27". Their command blocks use the `pending-phase-27` tag so the docs test skips them. The subscription paragraph says that 29-01 adds the proof test.

**2. [Rule 1 - Bug] The plan's suggested 127.0.0.1 bind would not limit exposure**
- **Issue:** `stc serve` binds all interfaces even with a loopback host.
- **Fix:** The security text recommends an isolated network or `--security basic256sha256`.

**3. [Rule 1 - Bug] Quoted --io glob would fail**
- **Issue:** The lead's example used `--io ".../Device *.xml"`, but stc sim and serve take the pattern literally.
- **Fix:** The README repeats `--io` once per export and says only the shell expands globs.

**4. [Rule 2 - Missing] Docs test caught `stc vendor init` in VENDOR_LIBRARIES.md**
- **Fix:** The block is tagged `bash proposed`, and a status note says the command is not implemented.

**5. [Scope] README and VENDOR_LIBRARIES.md were edited**
- They are not in the plan's file list. The lead asked for the README quick start, and D-19 covers other contradicted claims.

## Known Stubs

None in code. The "Lands with Phase 27" sections describe unmerged features and must be re-checked when Phase 27 merges. At that point, remove the `pending-phase-27` tags so the docs test covers those commands.

## User Setup Required

- Run the "Connect the Flutter HMI" checklist in docs/OPCUA.md against the real ST301 project (OPCUA-10 manual half).
- Capture the real TF6100 snapshot on the plant network with `stc opcua snapshot` (OPCUA-09 manual half).

## Self-Check: PASSED

- docs/TWINCAT_IMPORT.md (160 lines) and tests/docs_cli_test.go exist.
- Commits bb64f89, 642cc93, c88d043, cbb94d0 and ae3f992 are in git log.
