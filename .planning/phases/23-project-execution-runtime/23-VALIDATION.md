---
phase: 23
slug: project-execution-runtime
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (testify) |
| **Config file** | `.testcoverage.yml` (interp threshold 95%) |
| **Quick run command** | `go test ./pkg/interp ./cmd/stc -count=1` |
| **Full suite command** | `go test ./... -race -count=1` |
| **Estimated runtime** | ~23 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/interp ./cmd/stc -count=1`
- **After every plan wave:** Run `go test ./... -race -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 23 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 23-01-01 | 01 | 1 | RUNT-03 | T-23-01 | Malformed task specs and unknown programs are rejected at load | unit | `cd /Users/jonb/Projects/stc-wt-23 && go test ./pkg/interp -run 'TestLoadProject' -count=1` | ✅ | ✅ green |
| 23-01-02 | 01 | 1 | RUNT-03 | T-23-02 | Tick is deterministic and serialised with Get/Set under the Runtime mutex | unit | `cd /Users/jonb/Projects/stc-wt-23 && go test ./pkg/interp -run 'TestProjectTick|TestProjectAdvance|TestProjectIOBinder' -race -count=1` | ✅ | ✅ green |
| 23-01-03 | 01 | 1 | RUNT-09 | T-23-03 | Project paths are read only; analysis errors fail the load | integration | `cd /Users/jonb/Projects/stc-wt-23 && go test ./cmd/stc -run 'TestLoadProjectSpec' -count=1` | ✅ | ✅ green |
| 23-02-01 | 02 | 2 | RUNT-07 | T-23-04 | Codec bounds-checks slot widths | unit | `cd /Users/jonb/Projects/stc-wt-23 && go test ./pkg/interp -run 'TestIOCodec|TestIOBinder' -count=1` | ✅ | ✅ green |
| 23-02-02 | 02 | 2 | RUNT-07 | T-23-05 | Wildcard slots never overlap explicit addresses | unit | `cd /Users/jonb/Projects/stc-wt-23 && go test ./pkg/interp -run 'TestScanATDeclaredType|TestProjectWildcard|TestScan' -count=1` | ✅ | ✅ green |
| 23-02-03 | 02 | 2 | RUNT-03 | T-23-06 | Run never busy-waits; overruns counted, no catch-up burst | unit | `cd /Users/jonb/Projects/stc-wt-23 && go test ./pkg/interp -run 'TestProjectRun' -race -count=1` | ✅ | ✅ green |
| 23-03-01 | 03 | 3 | RUNT-04 | T-23-07 | Corrupt or foreign-version state files apply nothing | unit | `cd /Users/jonb/Projects/stc-wt-23 && go test ./pkg/interp -run 'TestPersist' -count=1` | ✅ | ✅ green |
| 23-03-02 | 03 | 3 | RUNT-09 | T-23-08 | Unresolved --io links fail the load with diagnostics | integration | `cd /Users/jonb/Projects/stc-wt-23 && go test ./cmd/stc -run 'TestSimProject|TestSim' -count=1` | ✅ | ✅ green |
| 23-03-03 | 03 | 3 | RUNT-03 | T-23-09 | serve stops cleanly on SIGINT/SIGTERM and saves state | integration | `cd /Users/jonb/Projects/stc-wt-23 && go test ./cmd/stc -run 'TestServe' -count=1` | ✅ | ✅ green |
| 23-04-01 | 04 | 4 | RUNT-03, RUNT-04, RUNT-07, RUNT-09 | T-23-10 | ST301 read from STC_SILD_DIR only; asserts counts and values | e2e | `cd /Users/jonb/Projects/stc-wt-23 && go test ./tests -run 'TestProjectRuntimeGate|TestST301Runtime' -count=1 -v` | ✅ | ✅ green |
| 23-04-02 | 04 | 4 | RUNT-03, RUNT-04 | — | N/A (documentation) | doc check | `cd /Users/jonb/Projects/stc-wt-23 && grep -q "nyquist_compliant: true" .planning/phases/23-project-execution-runtime/23-VALIDATION.md && grep -q "stc serve" docs/CLI_REFERENCE.md` | ✅ | ✅ green |
| 23-04-03 | 04 | 4 | RUNT-03, RUNT-07 | T-23-11 | .testcoverage.yml unchanged | coverage | `cd /Users/jonb/Projects/stc-wt-23 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements (go test, testify, `.testcoverage.yml`).

---

## Manual-Only Verifications

All phase behaviors have automated verification. `TestST301Runtime` needs the local-only sildarvinnsla checkout (`STC_SILD_DIR`); it was run on 2026-10-06 and passed. The `stc serve` ST301 OPC UA smoke run was performed by hand and is recorded in 23-04-SUMMARY.md.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 23s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06
