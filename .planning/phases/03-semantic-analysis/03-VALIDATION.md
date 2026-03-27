---
phase: 3
slug: semantic-analysis
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-27
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify |
| **Config file** | none — existing infrastructure |
| **Quick run command** | `go test ./pkg/types/... ./pkg/symbols/... ./pkg/checker/... ./pkg/analyzer/...` |
| **Full suite command** | `go test -v -race -count=1 ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/{package}/...` for the package being modified
- **After every plan wave:** Run `go test -v -count=1 ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | SEMA-02 | unit | `go test ./pkg/types/... -count=1 -v` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | SEMA-02 | unit | `go test ./pkg/types/... -count=1 -run TestBuiltin` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | SEMA-04,05 | unit | `go test ./pkg/symbols/... -count=1 -v` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | SEMA-04,05 | unit | `go test ./pkg/symbols/... -count=1 -v` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03 | 2 | SEMA-01,03 | unit | `go test ./pkg/checker/... -count=1 -run TestResolve -v` | ❌ W0 | ⬜ pending |
| 03-03-02 | 03 | 2 | SEMA-01,03 | unit | `go test ./pkg/checker/... -count=1 -v` | ❌ W0 | ⬜ pending |
| 03-04-01 | 04 | 2 | SEMA-07 | unit | `go test ./pkg/checker/... -count=1 -run TestVendor -v` | ❌ W0 | ⬜ pending |
| 03-04-02 | 04 | 2 | SEMA-04 | unit | `go test ./pkg/checker/... -count=1 -run TestUsage -v` | ❌ W0 | ⬜ pending |
| 03-05-01 | 05 | 3 | SEMA-05,06 | integration | `go test ./pkg/analyzer/... -count=1 -v` | ❌ W0 | ⬜ pending |
| 03-05-02 | 05 | 3 | SEMA-06 | integration | `go test ./cmd/stc/... -count=1 -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `pkg/types/` — new package (tests created with implementation)
- [ ] `pkg/symbols/` — new package (tests created with implementation)
- [ ] `pkg/checker/` — new package (tests created with implementation)
- [ ] `pkg/analyzer/` — new package (tests created with implementation)

*Test infrastructure (go test) is built into Go — no additional framework installation needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none) | — | — | All phase behaviors have automated verification |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
