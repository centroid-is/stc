---
phase: 28
slug: opc-ua-address-space
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 28-01-01 | 01 | 1 | REQ-{XX} | T-28-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

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
- [ ] Feedback latency < 28s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** {pending / approved YYYY-MM-DD}
