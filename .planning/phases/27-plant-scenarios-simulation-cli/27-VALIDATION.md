---
phase: 27
slug: plant-scenarios-simulation-cli
status: approved
approved: 2026-10-06
nyquist_compliant: true
wave_0_complete: true
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
| 27-01-01 | 01 | 1 | ECAT-09 | T-27-01 | 1 MiB / 10 000 step caps | unit | `go test ./pkg/scenario -run 'TestParse\|TestLoad' -count=1` | ✅ | ✅ green |
| 27-01-02 | 01 | 1 | ECAT-09 | T-27-02 | ramps removed at end | unit | `go test ./pkg/scenario -run TestExec -race -count=2` | ✅ | ✅ green |
| 27-01-03 | 01 | 1 | ECAT-09 | — | N/A | unit | `go test ./pkg/scenario -count=1 -cover` | ✅ | ✅ green |
| 27-02-01 | 02 | 2 | ECAT-09 | T-27-04, T-27-05 | forces bounded to BitLen, outputs rejected | unit | `go test ./pkg/ecat -run 'TestForce\|TestSlaveByName\|TestSlavePreset' -count=1` | ✅ | ✅ green |
| 27-02-02 | 02 | 2 | ECAT-09 | — | N/A | unit | `go test ./pkg/interp -run TestIOBinderLookup -count=1 && go test ./pkg/scenario -run TestPlantSpec -count=1` | ✅ | ✅ green |
| 27-02-03 | 02 | 2 | ECAT-09 | T-27-04 | N/A | unit | `go test ./pkg/scenario -run TestPlant -race -count=1` | ✅ | ✅ green |
| 27-02-04 | 02 | 2 | ECAT-09 | T-27-06 | synthetic fixtures only | integration | `go test ./pkg/scenario -run TestPlantE2E -count=2` | ✅ | ✅ green |
| 27-03-01 | 03 | 3 | ECAT-10 | T-27-08 | N/A | CLI | `go test ./cmd/stc -run 'TestSimScenario\|TestSim' -count=1` | ✅ | ✅ green |
| 27-03-02 | 03 | 3 | ECAT-10 | T-27-09 | N/A | CLI | `go test ./cmd/stc -run TestSimScenario -count=2` | ✅ | ✅ green |
| 27-03-03 | 03 | 3 | DEVX-01 | T-27-07 | RUN_CYCLES bounded | unit | `go test ./pkg/scenario -run TestBuiltins -race -count=1` | ✅ | ✅ green |
| 27-04-01 | 04 | 4 | DEVX-01 | T-27-11 | fresh plant per TEST_CASE | unit/CLI | `go test ./pkg/testing -count=1 && go test ./cmd/stc -run TestTest -count=1` | ✅ | ✅ green |
| 27-04-02 | 04 | 4 | DEVX-01, ECAT-10 | T-27-10 | env-gated, in-place reads | CLI/env | `go test ./cmd/stc -run TestTestProject -count=1 && go test ./tests -run TestScenarioST301 -count=1` | ✅ | ✅ green |
| 27-04-03 | 04 | 4 | ECAT-09 | — | N/A | docs grep | see 27-04 Task 3 verify | ✅ | ✅ green |
| 27-04-04 | 04 | 4 | all | — | N/A | gate | `bash scripts/coverage-gate.sh` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Complete. Existing go test infrastructure covers the phase; every task's test file exists and every per-task command above was rerun green on 2026-10-06 after 27-04.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real ST301 scenario run | ECAT-10 | customer project is local-only | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run TestScenarioST301 -v` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06 (27-04 phase gate)

## Gate Results (2026-10-06, branch gsd/phase-27-scenarios)

| Check | Result |
|-------|--------|
| `go vet ./...` | clean |
| `go test ./... -count=1` | all packages pass (TestEmptyFBCall passes; no skip needed) |
| `bash scripts/coverage-gate.sh` | PASS, total 97.08% (min 85%) |
| `STC_SILD_DIR=... go test ./tests -run TestScenarioST301` | PASS, 4/4 assertions, removed EL1008 seen at cycle 1504, byte-identical JSON over two runs (51.6 s) |

| Package | Coverage | Min |
|---------|----------|-----|
| pkg/parser | 98.35% | 95% |
| pkg/lexer | 97.60% | 95% |
| pkg/checker | 98.66% | 94% |
| pkg/interp | 98.81% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.32% | 95% |
| pkg/symtree | 98.51% | 95% |
| pkg/scenario (go test -cover) | 98.6% | 95% (phase target) |
| pkg/ecat (go test -cover) | 99.4% | n/a |
| pkg/ecat/devices (go test -cover) | 100.0% | n/a |
| pkg/testing/runner_project.go | 96.8% to 100% per function | n/a |
