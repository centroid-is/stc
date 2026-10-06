---
phase: 28
slug: opc-ua-address-space
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
---

# Phase 28 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.25) with in-process awcullen client integration tests |
| **Config file** | .testcoverage.yml, scripts/coverage-gate.sh, CI workflows |
| **Quick run command** | targeted `go test ./pkg/opcua -run <Pattern> -count=1` |
| **Full suite command** | `go test ./... -count=1 && bash scripts/coverage-gate.sh` |
| **Estimated runtime** | ~28 seconds |

---

## Sampling Rate

- **After every task commit:** Run the targeted quick command
- **After every plan wave:** Run the full suite (gate in final plan)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 28 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 28-01-01 | 01 | 1 | OPCUA-01 | T-28-SC | awcullen pinned at v1.4.0, go.sum verified | unit | `go test -race ./pkg/opcua -run 'TestMapSource|TestKind' -count=1` | ✅ | ✅ green |
| 28-01-02 | 01 | 1 | OPCUA-01 | T-28-04 | key written 0600, PKI dir 0700 | unit | `go test -race ./pkg/opcua -run 'TestEnsureCert|TestNew|TestStartStop' -count=1` | ✅ | ✅ green |
| 28-01-03 | 01 | 1 | OPCUA-01, OPCUA-02 | T-28-01, T-28-02 | secure-only server rejects None | integration | `go test -race -count=2 ./pkg/opcua -run 'TestConnect|TestServerStatus|TestSecureOnly|TestBrowseObjects'` | ✅ | ✅ green |
| 28-02-01 | 02 | 2 | OPCUA-06 | T-28-05 | range and type checks before NodeSource.Write | unit | `go test ./pkg/opcua -run 'TestUADataType|TestUAGoType|TestArrayShape|TestToUA|TestFromUA' -count=1` | ✅ | ✅ green |
| 28-02-02 | 02 | 2 | OPCUA-05 | T-28-08 | bounded process-global type registry | unit | `go test -race ./pkg/opcua -run 'TestEnsureEnum|TestEnsureStruct|TestGenericDecode' -count=1` | ✅ | ✅ green |
| 28-02-03 | 02 | 2 | OPCUA-04, OPCUA-05, OPCUA-06 | T-28-03, T-28-06, T-28-07 | AccessLevel gates writes; read-only struct members never written | integration | `go test -race -count=2 ./pkg/opcua -run 'TestPublish'` | ✅ | ✅ green |
| 28-03-01 | 03 | 3 | OPCUA-03 | T-28-09, T-28-10, T-28-11 | default deny, '0' prunes, depth cap OPCUA008 | unit | `go test ./pkg/opcua -run 'TestAttr|TestUnescape|TestBuild' -count=1` | ✅ | ✅ green |
| 28-03-02 | 03 | 3 | OPCUA-02 | — | N/A | integration | `go test -race ./pkg/opcua -run 'TestDeviceSet|TestPublish' -count=1` | ✅ | ✅ green |
| 28-03-03 | 03 | 3 | OPCUA-02, OPCUA-03, OPCUA-04, OPCUA-05 | — | N/A | integration | `go test -race -count=3 ./pkg/opcua -run 'TestST301|TestGoldenST301Shape'` | ✅ | ✅ green |
| 28-04-01 | 04 | 4 | OPCUA-02, OPCUA-03 | — | N/A | unit | `go test -race ./pkg/opcua/bind -run TestSymtree -count=1` | ✅ | ✅ green |
| 28-04-02 | 04 | 4 | OPCUA-04, OPCUA-06 | T-28-12, T-28-13, T-28-15 | writes validated, queued (bounded) and applied between scans; CONSTANT rejected; Snapshot in one critical section | unit | `go test -race -count=3 ./pkg/opcua/bind -run TestRuntimeSource` | ✅ | ✅ green |
| 28-04-03 | 04 | 4 | OPCUA-01..06 | T-28-14 | CLI states the listener binds all interfaces; basic256sha256 secure-only | integration (in-process + exec) | `go test -race ./cmd/stc -run TestServe -count=1` | ✅ | ✅ green |
| 28-04-04 | 04 | 4 | all | — | N/A | gate | `go test ./... -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |

Recorded on 2026-10-06: pkg/opcua 95.4% and pkg/opcua/bind 100.0% statement coverage (target 85%, not in the gated list per CONTEXT). The coverage gate passed with every gated package at or above its minimum and 96.62% in total. `TestServeST301Parity` matches the parsed ST fixture with `tests/opcua_golden/st301_shape.json` byte for byte.

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements: go test, and the awcullen client that ships with the pinned v1.4.0 dependency.

---

## Manual-Only Verifications

All phase behaviors have automated verification. The following are informational only; Phase 29 owns HMI acceptance.

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| python-asyncua decodes the custom struct DataTypes | OPCUA-05 | Third-party client, not in the Go toolchain | `stc serve tests/opcua_golden/st301_shape`, then `load_data_type_definitions()` in asyncua and read `ns=4;s=GVL_BatchLines.Drives_Line1[1].HMI` |
| UaExpert browse of a production project | OPCUA-02, OPCUA-03 | GUI client | `stc serve "ST301 solution.tsproj"`, then browse Objects/DeviceSet/PLC1 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 28s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06
