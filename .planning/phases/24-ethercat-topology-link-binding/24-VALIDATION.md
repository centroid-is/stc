---
phase: 24
slug: ethercat-topology-link-binding
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.25) + exec CLI tests + env-gated STC_SILD_DIR equivalence test |
| **Config file** | .testcoverage.yml, scripts/coverage-gate.sh, CI workflows |
| **Quick run command** | targeted `go test ./pkg/<pkg> -run <Pattern> -count=1` |
| **Full suite command** | `go test ./... -count=1 && bash scripts/coverage-gate.sh` |
| **Estimated runtime** | ~24 seconds |

---

## Sampling Rate

- **After every task commit:** Run the targeted quick command
- **After every plan wave:** Run the full suite (gate in final plan)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 24 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 24-01-01 | 01 | 1 | ECAT-01 | T-24-01 | Oversized BitLen/BitOffs rejected with an error | unit | `go test ./pkg/ecat -run 'TestLoad' -count=1` | ✅ | ✅ green |
| 24-01-02 | 01 | 1 | ECAT-01 | — | N/A | unit | `go test ./pkg/ecat -run 'TestClassifyRole\|TestAssignParents\|TestLinkPath\|TestIECType' -count=1` | ✅ | ✅ green |
| 24-01-03 | 01 | 1 | ECAT-03, ECAT-07 | T-24-02 | Out-of-range ReadBits/WriteBits read 0 and never panic | unit | `go test ./pkg/ecat -count=1 -cover` | ✅ | ✅ green |
| 24-02-01 | 02 | 2 | ECAT-02 | — | Malformed values return an error | unit | `go test ./pkg/ecat -run TestParseTcLinkTo -count=1` | ✅ | ✅ green |
| 24-02-02 | 02 | 2 | ECAT-02 | T-24-03 | Member depth capped at 16, EXTENDS cycles terminate | unit | `go test ./pkg/ecat -run 'TestCollectLinks\|TestBitWidth' -count=1` | ✅ | ✅ green |
| 24-02-03 | 02 | 2 | ECAT-02 | — | N/A | unit | `go test ./pkg/ecat -run TestResolve -count=1` | ✅ | ✅ green |
| 24-02-04 | 02 | 2 | ECAT-02 | T-24-04 | Read-only; prints only user-supplied paths | exec CLI | `go test ./cmd/stc -run TestEcat -count=1` | ✅ | ✅ green |
| 24-03-01 | 03 | 3 | ECAT-03, ECAT-07 | T-24-06 | Step linear in slots | unit | `go test ./pkg/ecat -run 'TestNetwork\|TestRegistry\|TestFault' -count=1 -cover` | ✅ | ✅ green |
| 24-03-02 | 03 | 3 | ECAT-03 | T-24-05 | Decode by declared type and slot width; bad shapes dropped with an error | unit | `go test ./pkg/interp -count=1 -run 'TestIOBind'` | ✅ | ✅ green |
| 24-03-03 | 03 | 3 | ECAT-03, ECAT-07 | T-24-05 | Nil binder leaves Tick unchanged | integration | `go test ./pkg/interp -count=1 -run 'TestIOBind\|TestScan'` | ✅ | ✅ green |
| 24-04-01 | 04 | 4 | ECAT-01, ECAT-02 | T-24-07 | Env-gated; logs counts only, no customer source | env-gated integration | `STC_SILD_DIR=... STC_PROBES_DIR=... go test ./tests -run TestEcatST301Equivalence -count=1 -v` | ✅ | ✅ green |
| 24-04-02 | 04 | 4 | ECAT-02 | — | N/A | docs grep | `for c in ECAT001..ECAT007: grep -q $c docs/CLI_REFERENCE.md` | ✅ | ✅ green |
| 24-04-03 | 04 | 4 | ECAT-01, ECAT-02, ECAT-03, ECAT-07 | — | N/A | coverage gate | `go vet ./... && bash scripts/coverage-gate.sh` | ✅ | ✅ thresholds green; the script exits 1 only on the pre-existing TestEmptyFBCall |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. The Demo Device 1/2 exports and the demo_types/demo_ect/demo_bad ST fixtures in tests/ecat_fixtures were created in 24-01 and 24-02.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| ST301 generator equivalence and zero unresolved links | ECAT-01, ECAT-02 | Local-only customer data, never committed; CI runs the test as a skip | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests -run TestEcatST301Equivalence -count=1 -v` |

Last recorded run, 2026-10-06, PASS:

| Master | Slaves | In bytes | Out bytes | Bindings |
|--------|--------|----------|-----------|----------|
| Device 1 (EtherCAT) | 23 | 1846 | 1543 | 233 |
| Device 2 (EtherCAT) | 49 | 3747 | 3085 | 563 |
| Device 3 (EtherCAT) | 34 | 1963 | 1543 | 353 |
| Device 4 (EtherCAT) | 16 | 1809 | 1548 | 191 |

114 TcLinkTo pragmas carry 1340 TIID^ targets. All 1340 exist among the 1970 topology paths and all 1340 linked leaves bind, with zero errors and zero warnings. 32 ATV320 drives sit at master level.


---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 24s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06

Coverage gate (merged profile): pkg/interp 98.25%, pkg/ecat 99.84%, total 96.34%; all gated packages pass. The only unit failure is the pre-existing pkg/checker TestEmptyFBCall, which is fixed on main by Phase 21 and disappears on merge (see deferred-items.md).
