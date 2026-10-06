---
phase: 26
slug: atv320-drive-ethercat-master-services
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
validated: 2026-10-06
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test ./pkg/ecat/... ./pkg/interp -count=1` |
| **Full suite command** | `go test ./... -count=1` (green after merging main; TestEmptyFBCall fixed there) |
| **Estimated runtime** | ~26 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./pkg/ecat/... ./pkg/interp -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 26 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 26-01-01 | 01 | 1 | ECAT-05 | — | Entry slots bound only to the slave's own entries | unit | `go test ./pkg/ecat -run 'Layout\|Network\|Registry' -count=1` | ✅ | ✅ green |
| 26-01-02 | 01 | 1 | ECAT-05 | — | N/A | unit | `go test ./pkg/ecat/devices -run CiA402 -count=1 -cover` | ✅ | ✅ green |
| 26-01-03 | 01 | 1 | ECAT-05 | — | N/A | unit | `go test ./pkg/ecat/... -count=1 -cover && go vet ./pkg/ecat/...` | ✅ | ✅ green |
| 26-02-01 | 02 | 2 | ECAT-08 | — | Unknown master or slave returns an error, never panics | unit | `go test ./pkg/ecat -count=1 -cover` | ✅ | ✅ green |
| 26-02-02 | 02 | 2 | ECAT-05 | — | Unknown objects and wrong sizes abort with CoE codes | unit | `go test ./pkg/ecat/devices -count=1 -cover` | ✅ | ✅ green |
| 26-02-03 | 02 | 2 | ECAT-05 | — | Process data gated below OP | unit | `go test ./pkg/ecat/... -count=1 -cover && go vet ./pkg/ecat/...` | ✅ | ✅ green |
| 26-03-01 | 03 | 3 | ECAT-08 | — | Bad buffer pointers end with ADS 0x706 | unit | `go test ./pkg/interp -run 'Ecat\|AmsNetId\|PtrBytes' -count=1` | ✅ | ✅ green |
| 26-03-02 | 03 | 3 | ECAT-08 | — | N/A | unit | `go test ./pkg/interp -run 'EcGet\|EcSet\|EcPhysical\|EcMaster\|EcCrc' -count=1` | ✅ | ✅ green |
| 26-03-03 | 03 | 3 | ECAT-08 | — | SDO buffers bounded by cbBufLen | unit | `go test ./pkg/interp -run 'CoESDo\|Ecat' -count=1 -cover && go vet ./pkg/interp` | ✅ | ✅ green |
| 26-04-01 | 04 | 4 | ECAT-05, ECAT-08 | T-26-09 | Every scan loop capped, fails with the current state | e2e | `go test ./pkg/interp -run TestEcatE2E -count=1 -v` | ✅ | ✅ green |
| 26-04-02 | 04 | 4 | ECAT-05, ECAT-08 | T-26-08, T-26-09 | External sources read-only; skipped without STC_SILD_DIR | e2e (env-gated) | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./pkg/interp -run TestEcatSildATV320Gate -count=1 -v` | ✅ | ✅ green |
| 26-04-03 | 04 | 4 | ECAT-05, ECAT-08 | — | N/A | coverage | `bash scripts/coverage-gate.sh` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real FB_ATV320 and FB_EcDeviceDiag on the simulator | ECAT-05, ECAT-08 | SVNCoreComponents sources are not in this repo, so CI skips the gate | Run the 26-04-02 command with the sildarvinnsla checkout present |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 26s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06
