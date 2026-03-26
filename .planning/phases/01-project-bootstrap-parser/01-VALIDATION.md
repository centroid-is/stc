---
phase: 1
slug: project-bootstrap-parser
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-26
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test -v -race -count=1 ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test -v -race -count=1 ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | PARS-01 | unit | `go test ./pkg/ast/...` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | PARS-07 | unit | `go test ./pkg/lexer/...` | ❌ W0 | ⬜ pending |
| 01-03-01 | 03 | 2 | PARS-01..10 | integration | `go test ./pkg/parser/...` | ❌ W0 | ⬜ pending |
| 01-04-01 | 04 | 3 | CLI-01..05 | integration | `go test ./cmd/stc/...` | ❌ W0 | ⬜ pending |
| 01-05-01 | 05 | 3 | PARS-05 | unit | `go test ./pkg/parser/ -run TestErrorRecovery` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` — initialize module github.com/centroid-is/stc
- [ ] `go.sum` — dependency lock file
- [ ] `Makefile` — build, test, lint, install targets
- [ ] `.github/workflows/ci.yml` — multi-platform CI (macOS, Windows, Linux)
- [ ] `.golangci.yml` — linter configuration

*Test infrastructure (go test) is built into Go — no additional framework installation needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `stc --version` outputs correct version | CLI-03 | Binary version embedding varies | Build binary, run `stc --version`, verify output |
| `stc.toml` project manifest parsing | CLI-04 | Config format usability | Create sample stc.toml, run `stc parse` in project dir |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
