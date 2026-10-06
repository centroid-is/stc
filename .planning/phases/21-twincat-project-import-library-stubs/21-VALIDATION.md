---
phase: 21
slug: twincat-project-import-library-stubs
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.25) + `stc test` ST suites + env-gated sildarvinnsla import test (STC_SILD_DIR) |
| **Config file** | .testcoverage.yml, scripts/coverage-gate.sh, .github/workflows/{ci,coverage,st-tests}.yml |
| **Quick run command** | targeted `go test ./pkg/<pkg> -run <Pattern> -count=1` (< 30 s) |
| **Full suite command** | `go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` |
| **Estimated runtime** | ~21 seconds |

---

## Sampling Rate

- **After every task commit:** Run the targeted quick command for the touched package
- **After every plan wave:** Run the full suite command above; coverage gate in the last task of wave-closing plans and the gate plan
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 21 seconds

---

## Per-Task Verification Map

Commands use the current package paths. Plan 21-04 moved `pkg/vendor/twincat` to `pkg/twincat` and the embedded stubs to `stdlib/beckhoff`; all rows were re-run green on 2026-10-06.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 21-01-01 | 01 | 1 | IMPT-01 | T-21-01 | PROPERTY PUBLIC headers parse; no crash on malformed modifiers | unit | `go test ./pkg/parser -count=1` | ✅ | ✅ green |
| 21-01-02 | 01 | 1 | IMPT-04 | T-21-02 | `fb();` checked as an FB call; no cascade on Invalid types | unit | `go test ./pkg/checker -count=1` | ✅ | ✅ green |
| 21-01-03 | 01 | 1 | IMPT-04 | T-21-01 | Double-quoted attributes warn (SEMA039) and round-trip through fmt | unit | `go test ./pkg/parser ./pkg/ast ./pkg/checker ./pkg/format -count=1` | ✅ | ✅ green |
| 21-02-01 | 02 | 1 | IMPT-04 | T-21-03 | Stub types match Beckhoff layouts (AMSADDR, T_AmsNetIdArr) | unit | `go test ./stdlib/... ./pkg/vendor/... -count=1` | ✅ | ✅ green |
| 21-02-02 | 02 | 1 | IMPT-04 | T-21-03 | Every shipped stub parses | unit | `go test ./stdlib/beckhoff ./stdlib/vendor/beckhoff -count=1` | ✅ | ✅ green |
| 21-02-03 | 02 | 1 | IMPT-04 | T-21-04 | Embedded stubs load with dependency closure and check clean | unit | `go test ./stdlib/beckhoff ./stdlib/vendor/beckhoff -count=1` | ✅ | ✅ green |
| 21-03-01 | 03 | 2 | IMPT-01, IMPT-03 | T-21-05 | Model and codes stable; fixtures synthetic only | unit | `go test ./pkg/twincat -run TestModel -count=1` | ✅ | ✅ green |
| 21-03-02 | 03 | 2 | IMPT-03 | T-21-05, T-21-06 | XML readers reject bad XML (VEND027); paths normalised | unit | `go test ./pkg/twincat -run 'TestTsproj|TestPlcproj|TestTcTTO|TestMergeTasks' -count=1` | ✅ | ✅ green |
| 21-03-03 | 03 | 2 | IMPT-01 | T-21-07 | Converter preserves TcPOU line/column | unit | `go test ./pkg/twincat -cover -count=1` | ✅ | ✅ green |
| 21-04-01 | 04 | 3 | IMPT-02 | T-21-08 | Ordered resolver; unresolved reported as VEND020 | unit | `go test ./pkg/twincat -run TestResolve -count=1` | ✅ | ✅ green |
| 21-04-02 | 04 | 3 | IMPT-01, IMPT-02 | T-21-09 | Import + AnalyzeProject deterministic | unit | `go test ./pkg/twincat ./pkg/analyzer -count=1` | ✅ | ✅ green |
| 21-04-03 | 04 | 3 | IMPT-05 | T-21-10 | Extract renders all kinds in plcproj order; nothing skipped silently | unit+exec | `go test ./pkg/vendor/... ./cmd/stc -run 'Extract|Vendor|Load' -count=1` | ✅ | ✅ green |
| 21-05-01 | 05 | 4 | IMPT-01, IMPT-05 | T-21-11 | --out targets validated before any write | unit+exec | `go test ./pkg/twincat -run TestWriteOut -count=1 && go test ./cmd/stc -run 'TestVendorImport|TestVendorExtract' -count=1` | ✅ | ✅ green |
| 21-05-02 | 05 | 4 | IMPT-02 | T-21-12 | stc check project mode reports positions in TcPOU files | exec | `go test ./cmd/stc -run 'TestCheck' -count=1` | ✅ | ✅ green |
| 21-05-03 | 05 | 4 | IMPT-03 | T-21-12 | test --project and sim <project> use task program and cycle | unit+exec | `go test ./pkg/testing ./cmd/stc -run 'TestProjectFiles|TestTestProject|TestSimProject|TestRun|TestSim' -count=1` | ✅ | ✅ green |
| 21-06-01 | 06 | 5 | IMPT-01..05 | T-21-13, T-21-14 | Oracle reads only STC_SILD_DIR; owner buckets, phase 21 bucket zero, unmatched errors fail | unit (env-gated) | `go test ./tests -run 'TestSildarvinnsla|TestClassifySild' -count=1` | ✅ | ✅ green |
| 21-06-02 | 06 | 5 | IMPT-01..05 | — | Docs and deferred items present | grep | `grep -q "vendor import" docs/CLI_REFERENCE.md && grep -q Tc2_ModbusSrv docs/VENDOR_LIBRARIES.md && grep -q PVOID .planning/phases/21-twincat-project-import-library-stubs/deferred-items.md` | ✅ | ✅ green |
| 21-06-03 | 06 | 5 | IMPT-01..05 | — | Full suite and coverage gates hold | full | `go vet ./... && go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `pkg/twincat/testdata/` synthetic solution (Demo, DemoLib, broken fixtures); no customer files
- [x] `stdlib/beckhoff/testdata/usage.st` usage fixture across the stub closures
- [x] New test files: pkg/twincat/*_test.go, stdlib/beckhoff/beckhoff_test.go, cmd/stc/vendor_import_test.go, cmd/stc/check_project_test.go, cmd/stc/project_mode_test.go, pkg/testing/project_files_test.go, pkg/analyzer/project_test.go
- [x] `tests/twincat_import_test.go` STC_SILD_DIR oracle with owner buckets and the 65-object extract check

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real sildarvinnsla projects import and check with zero Phase-21-owned errors; extract yields 65 parsing objects | IMPT-01..05 | Customer code is local-only; CI skips the test without STC_SILD_DIR | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run 'TestSildarvinnsla' -count=1 -v` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 21s
- [x] Nyquist compliance flag set to true in frontmatter

**Approval:** approved 2026-10-06
