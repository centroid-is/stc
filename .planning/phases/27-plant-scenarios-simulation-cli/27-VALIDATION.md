---
phase: 27
slug: plant-scenarios-simulation-cli
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-06
---

# Phase 27 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib testing) |
| **Config file** | none; scripts/coverage-gate.sh for the gate |
| **Quick run command** | `go test ./pkg/scenario -count=1` |
| **Full suite command** | `go test ./pkg/scenario ./pkg/ecat/... ./pkg/interp ./pkg/testing ./cmd/stc -count=1 -cover` |
| **Estimated runtime** | ~90 seconds |

---

## Sampling Rate

- **After every task commit:** Run the task's `<automated>` command
- **After every plan wave:** Run the full suite command
- **Before `/gsd:verify-work`:** `go test ./... -count=1` and `bash scripts/coverage-gate.sh` green (TestEmptyFBCall tolerated only if the rebase did not remove it)
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 27-01-01 | 01 | 1 | ECAT-09 | T-27-01 | 1 MiB / 10 000 step caps | unit | `go test ./pkg/scenario -run 'TestParse\|TestLoad' -count=1` | ❌ W0 | ⬜ pending |
| 27-01-02 | 01 | 1 | ECAT-09 | T-27-02 | ramps removed at end | unit | `go test ./pkg/scenario -run TestExec -race -count=2` | ❌ W0 | ⬜ pending |
| 27-01-03 | 01 | 1 | ECAT-09 | — | N/A | unit | `go test ./pkg/scenario -count=1 -cover` | ❌ W0 | ⬜ pending |
| 27-02-01 | 02 | 2 | ECAT-09 | T-27-04, T-27-05 | forces bounded to BitLen, outputs rejected | unit | `go test ./pkg/ecat -run 'TestForce\|TestSlaveByName\|TestSlavePreset' -count=1` | ❌ W0 | ⬜ pending |
| 27-02-02 | 02 | 2 | ECAT-09 | — | N/A | unit | `go test ./pkg/interp -run TestIOBinderLookup -count=1 && go test ./pkg/scenario -run TestPlantSpec -count=1` | ❌ W0 | ⬜ pending |
| 27-02-03 | 02 | 2 | ECAT-09 | T-27-04 | N/A | unit | `go test ./pkg/scenario -run TestPlant -race -count=1` | ❌ W0 | ⬜ pending |
| 27-02-04 | 02 | 2 | ECAT-09 | T-27-06 | synthetic fixtures only | integration | `go test ./pkg/scenario -run TestPlantE2E -count=2` | ❌ W0 | ⬜ pending |
| 27-03-01 | 03 | 3 | ECAT-10 | T-27-08 | N/A | CLI | `go test ./cmd/stc -run 'TestSimScenario\|TestSim' -count=1` | ❌ W0 | ⬜ pending |
| 27-03-02 | 03 | 3 | ECAT-10 | T-27-09 | N/A | CLI | `go test ./cmd/stc -run TestSimScenario -count=2` | ❌ W0 | ⬜ pending |
| 27-03-03 | 03 | 3 | DEVX-01 | T-27-07 | RUN_CYCLES bounded | unit | `go test ./pkg/scenario -run TestBuiltins -race -count=1` | ❌ W0 | ⬜ pending |
| 27-04-01 | 04 | 4 | DEVX-01 | T-27-11 | fresh plant per TEST_CASE | unit/CLI | `go test ./pkg/testing -count=1 && go test ./cmd/stc -run TestTest -count=1` | ❌ W0 | ⬜ pending |
| 27-04-02 | 04 | 4 | DEVX-01, ECAT-10 | T-27-10 | env-gated, in-place reads | CLI/env | `go test ./cmd/stc -run TestTestProject -count=1 && go test ./tests -run TestScenarioST301 -count=1` | ❌ W0 | ⬜ pending |
| 27-04-03 | 04 | 4 | ECAT-09 | — | N/A | docs grep | see 27-04 Task 3 verify | ✅ | ⬜ pending |
| 27-04-04 | 04 | 4 | all | — | N/A | gate | `bash scripts/coverage-gate.sh` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing go test infrastructure covers the phase. Each task creates its own test file before implementation (tdd="true").

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real ST301 scenario run | ECAT-10 | customer project is local-only | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run TestScenarioST301 -v` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
