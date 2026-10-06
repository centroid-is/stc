---
phase: 29
slug: live-hmi-agent-integration
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
approved: 2026-10-06
note: "Plans 29-02, 29-03 and 29-04 are validated. Plan 29-01 waits for Phase 27 and its rows stay pending."
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (testify), awcullen/opcua client, go-sdk MCP in-memory transport |
| **Config file** | .testcoverage.yml (coverage gate) |
| **Quick run command** | `go test -race ./pkg/opcua/... ./pkg/scenario/ ./cmd/stc-mcp/` |
| **Full suite command** | `go test -race -count=1 ./...` |
| **Estimated runtime** | ~180 seconds |

---

## Sampling Rate

- **After every task commit:** Run the task's `<automated>` command
- **After every plan wave:** Run `go test -race -count=1 ./...`
- **Before `/gsd:verify-work`:** Full suite and coverage gate must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 29-01-01 | 01 | 1 | OPCUA-07 | T-29-01 | Writes only between scans; p_stat_* not writable | integration | `go test -race -count=3 -run 'TestLive' ./pkg/opcua/bind/` | ❌ W0 | ⬜ pending (29-01 not executed; needs Phase 27) |
| 29-01-02 | 01 | 1 | OPCUA-08 | T-29-02 | Push only changed nodes | integration | `go test -race -count=2 -run 'TestSubscri' ./pkg/opcua/` | ❌ W0 | ⬜ pending (29-01 not executed; needs Phase 27) |
| 29-01-03 | 01 | 1 | OPCUA-07, OPCUA-08 | T-29-03 | Scenario validated before start | exec | `go test -count=2 -run 'TestServeLive\|TestServeScenario' ./cmd/stc/` | ❌ W0 | ⬜ pending (29-01 not executed; needs Phase 27) |
| 29-02-01 | 02 | 2 | OPCUA-09 | — | N/A | unit | `go test -race -cover ./pkg/opcua/opcuatest/` | ✅ | ✅ green |
| 29-02-02 | 02 | 2 | OPCUA-09 | T-29-05, T-29-06 | Snapshot is read-only | exec | `go test -count=1 -run 'TestOpcuaSnapshot' ./cmd/stc/ && go test -count=1 -run 'TestTF6100' ./tests/` | ✅ | ✅ green |
| 29-02-03 | 02 | 2 | OPCUA-10 | T-29-06 | Logs counts only | exec | `go test -count=1 -run 'TestHMI' ./tests/` | ✅ | ✅ green |
| 29-03-01 | 03 | 3 | DEVX-02 | T-29-08 | Bounded steps | unit | `go test -race -run 'TestSimSession' ./cmd/stc-mcp/` | ✅ | ✅ green |
| 29-03-02 | 03 | 3 | DEVX-02 | T-29-09, T-29-10 | Coerced writes, read-only browse | unit | `go test -race -run 'TestSim\|TestOpcuaBrowse\|TestToolDefinitions' ./cmd/stc-mcp/` | ✅ | ✅ green |
| 29-03-03 | 03 | 3 | DEVX-02 | — | N/A | integration | `go test -race -cover ./cmd/stc-mcp/` | ✅ | ✅ green |
| 29-04-01 | 04 | 4 | DEVX-03 | — | N/A | doc check | `test -f docs/TWINCAT_IMPORT.md` | ✅ | ✅ green |
| 29-04-02 | 04 | 4 | DEVX-03, OPCUA-10, OPCUA-09 | T-29-12, T-29-13 | No plant identifiers in docs | doc check | `grep -c "Connect the Flutter HMI" docs/OPCUA.md` | ✅ | ✅ green |
| 29-04-03 | 04 | 4 | DEVX-03 | — | N/A | exec | `go test -count=1 -run 'TestDocsCLI' ./tests/` | ✅ | ✅ green |
| 29-04-04 | 04 | 4 | all | — | N/A | gate | `go test -race -count=1 ./...` plus the coverage gate | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `tests/opcua_live/live.st`: live handshake and sensor fixture (29-01 Task 1, pending Phase 27)
- [x] `tests/opcua_golden/hmi_keymappings.json`: HMI stand-in (29-02 Task 3)

Each plan creates its own test files test-first. No framework install is needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions | Status |
|----------|-------------|------------|-------------------|--------|
| The Flutter HMI (tfc-hmi) shows live drive, sensor and conveyor structs with `rdy(2)` | OPCUA-10 | Needs the Flutter app and the proprietary ST301 project | docs/OPCUA.md "Connect the Flutter HMI" | pending-user |
| Capture the real TF6100 browse snapshot | OPCUA-09 | Needs the plant network and a live TF6100 | docs/OPCUA.md "Capture a TF6100 snapshot" | pending-user |

---

## Coverage (2026-10-06, `bash scripts/coverage-gate.sh`, merged unit and exec profile)

| Package | Coverage | Minimum | Result |
|---------|----------|---------|--------|
| total | 96.99% | 85% | PASS |
| pkg/interp | 98.77% | 95% | PASS |
| pkg/symtree | 98.51% | 95% | PASS |
| pkg/parser, lexer, checker, types, emit | 97.3% to 100% | 94 to 95% | PASS |
| pkg/opcua | 95.42% | 85% | PASS |
| pkg/opcua/bind | 100.00% | 85% | PASS |
| pkg/opcua/opcuatest | 90.72% | 85% | PASS |
| cmd/stc-mcp | 91.47% | 85% | PASS |
| pkg/projectload | 95.88% | 85% | PASS |
| pkg/scenario | n/a | 85% | Not on this branch (Phase 27) |

- `go test -race -count=1 ./...` is green.
- The live and OPC UA tests passed three times in a row with `-race -count=3` in tests, cmd/stc, cmd/stc-mcp and pkg/opcua/...

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (29-01 fixture pending with 29-01)
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06 for 29-02, 29-03 and 29-04. Plan 29-01 rows stay pending until Phase 27 merges. The two manual rows are handed to the user.
