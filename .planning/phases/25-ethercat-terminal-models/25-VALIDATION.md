---
phase: 25
slug: ethercat-terminal-models
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-06
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test ./pkg/ecat/... -count=1` |
| **Full suite command** | `go test ./... -count=1 && bash scripts/coverage-gate.sh` |
| **Estimated runtime** | ~5 seconds quick, ~120 seconds full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/ecat/... -count=1`
- **After every plan wave:** Run `go test ./pkg/ecat/... -count=1 -cover`
- **Before `/gsd:verify-work`:** Full suite must be green (known pkg/checker TestEmptyFBCall failure tolerated on this branch base)
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 25-01-01 | 01 | 1 | ECAT-04 | T-25-01 | out-of-range fields are no-ops | unit | `go test ./pkg/ecat -count=1` | ❌ W0 | ⬜ pending |
| 25-01-02 | 01 | 1 | ECAT-04 | T-25-02 | bad entry returns error | unit | `go test ./pkg/ecat/devices -run 'Base\|Passive\|Registry'` | ❌ W0 | ⬜ pending |
| 25-01-03 | 01 | 1 | ECAT-04 | T-25-03 | exact id beats pattern | unit | `go test ./pkg/ecat/... -count=1` | ❌ W0 | ⬜ pending |
| 25-02-01 | 02 | 2 | ECAT-04 | T-25-04 | NaN/Inf handled | unit | `go test ./pkg/ecat/devices -run Analog` | ❌ W0 | ⬜ pending |
| 25-02-02 | 02 | 2 | ECAT-04 | T-25-05 | channel bounds checked | unit | `go test ./pkg/ecat/devices -run EL9222` | ❌ W0 | ⬜ pending |
| 25-02-03 | 02 | 2 | ECAT-04 | T-25-06 | N/A | unit | `go test ./pkg/ecat/... -cover` | ❌ W0 | ⬜ pending |
| 25-03-01 | 03 | 3 | ECAT-06 | T-25-09 | request buffer capped | unit | `go test ./pkg/ecat/devices -run Peer` | ❌ W0 | ⬜ pending |
| 25-03-02 | 03 | 3 | ECAT-06 | T-25-07, T-25-08 | length clamped, FIFO capped | unit | `go test ./pkg/ecat/devices -run EL6001` | ❌ W0 | ⬜ pending |
| 25-03-03 | 03 | 3 | ECAT-06 | — | N/A | integration | `go test ./pkg/ecat/... -cover` | ❌ W0 | ⬜ pending |
| 25-04-01 | 04 | 4 | ECAT-04 | T-25-10 | env-gated, no customer data committed | integration | `STC_SILD_DIR=... go test ./tests -run TestEcatModelCoverage` | ❌ W0 | ⬜ pending |
| 25-04-02 | 04 | 4 | ECAT-04, ECAT-06 | — | N/A | docs | `grep -c "How to Add an EtherCAT Device Model" docs/ARCHITECTURE.md` | ✅ | ⬜ pending |
| 25-04-03 | 04 | 4 | ECAT-04, ECAT-06 | — | N/A | gate | `bash scripts/coverage-gate.sh` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `tests/ecat_fixtures/Demo Analog.xml` — created in 25-02 Task 1
- [ ] `tests/ecat_fixtures/Demo Serial.xml` — created in 25-03 Task 2

*Test files are created test-first inside each task (tdd="true").*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Every real ST101/ST201/ST301/Baader slave gets a model | ECAT-04 | local-only customer exports | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run TestEcatModelCoverage -v` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
