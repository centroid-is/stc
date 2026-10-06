---
phase: 20
slug: twincat-expression-semantics
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-10-06
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.25) + `stc test` ST suites + tests/twincat_probes_test.go classification gate |
| **Config file** | .testcoverage.yml, scripts/coverage-gate.sh, .github/workflows/{ci,coverage,st-tests}.yml |
| **Quick run command** | targeted `go test ./pkg/<pkg> -run <Pattern> -count=1` (< 30 s) |
| **Full suite command** | `go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` (gate: parser/lexer/interp/types/emit >= 95%, checker >= 94%, total >= 85%) |
| **Estimated runtime** | ~20 seconds |

---

## Sampling Rate

- **After every task commit:** Run the targeted quick command for the touched package
- **After every plan wave:** Run the full suite command above; the coverage gate runs in the last task of each wave-closing plan and in the final gate plan
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 20-01-01 | 01 | 1 | REQ-{XX} | T-20-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

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
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** {pending / approved YYYY-MM-DD}
