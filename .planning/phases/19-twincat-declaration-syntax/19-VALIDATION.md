---
phase: 19
slug: twincat-declaration-syntax
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-05
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.25) + `stc test` ST suites |
| **Config file** | .testcoverage.yml, .github/workflows/{ci,coverage,st-tests}.yml |
| **Quick run command** | `go test ./pkg/parser ./pkg/ast ./pkg/checker ./pkg/interp ./pkg/emit ./pkg/format -count=1` |
| **Full suite command** | `go test ./... -count=1 && go test -coverprofile=cov.txt -covermode=atomic -coverpkg=./... ./... -count=1 && go run github.com/vladopajic/go-test-coverage/v2@latest --config .testcoverage.yml --profile cov.txt` |
| **Estimated runtime** | per-task targeted `go test ./pkg/<x> -run <Pattern>`: under 30 seconds; coverage gate (`bash scripts/coverage-gate.sh`, full -coverpkg suite): about 120 seconds |

---

## Sampling Rate

- **After every task commit:** Run the task's targeted `<automated>` command (`go test ./pkg/<x> -run <Pattern> -count=1`), under 30 seconds
- **After every plan wave:** Run `bash scripts/coverage-gate.sh` (the full -coverpkg suite). It runs only as the last task's verify of plans 19-03, 19-05, 19-07, 19-08 and in 19-09 (coverage gate: pkg/parser, pkg/interp, pkg/types, pkg/emit, pkg/lexer >= 95%, pkg/checker >= 94%, total >= 85%)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds per task; about 120 seconds for the coverage gate once per wave

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | REQ-{XX} | T-19-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `{tests/test_file.py}` — stubs for REQ-{XX}
- [ ] `{tests/conftest.py}` — shared fixtures
- [ ] `{framework install}` — if no framework detected

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| {behavior} | REQ-{XX} | {reason} | {steps} |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Per-task feedback latency < 30s; coverage gate only at wave end (~120s)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** {pending / approved YYYY-MM-DD}
