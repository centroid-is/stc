---
phase: 19
slug: twincat-declaration-syntax
status: approved
nyquist_compliant: true
wave_0_complete: true
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
| 19-01-01 | 01 | 1 | DIAL-05 (enabling) | — | N/A | unit | `go test ./pkg/interp -run 'TestStdlibErr\|TestScanIOSizes' -count=1` | ✅ | ✅ green |
| 19-01-02 | 01 | 1 | all (enabling) | T-19-02 | Gate replica only reads the repo; accepted | script | `bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 19-01-03 | 01 | 1 | all (fixtures) | T-19-01 | Only one-construct probes and synthetic shapes committed; customer sources stay local | integration | `ls tests/twincat_probes/*.st \| wc -l \| grep -q 11 && go test ./tests/ -count=1 && go run ./cmd/stc test tests/` | ✅ | ✅ green |
| 19-02-01 | 02 | 1 | DIAL-01, DIAL-02, DIAL-03, DIAL-08 | T-19-03 | SanitizeGVLName turns hostile filenames into a valid identifier | unit | `go test ./pkg/ast -run 'TestAttribute\|TestSanitizeGVLName\|TestSetGVLName\|TestNodeKind' -count=1` | ✅ | ✅ green |
| 19-02-02 | 02 | 1 | DIAL-01, DIAL-02, DIAL-03, DIAL-05, DIAL-08 | T-19-04 | Attribute.String escapes quotes so printed pragmas cannot break out | unit (tdd) | `go test ./pkg/ast -run TestMarshal -count=1` | ✅ | ✅ green |
| 19-03-01 | 03 | 2 | DIAL-01 | T-19-05 | Attribute scanner is bounded and never panics on malformed text | unit | `go test ./pkg/parser -run 'TestAttributeText\|TestCollectPragmas' -count=1` | ✅ | ✅ green |
| 19-03-02 | 03 | 2 | DIAL-01 | T-19-06 | collectPragmas always advances, so parser loops terminate | unit (tdd) | `go test ./pkg/parser -run 'TestPragmaAttach\|TestAttribute\|TestCollectPragmas' -count=1` | ✅ | ✅ green |
| 19-03-03 | 03 | 2 | DIAL-01 | — | N/A | unit + CLI (tdd) | `go test ./pkg/emit ./pkg/format ./cmd/stc -run 'Attribute\|Attr' -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 19-04-01 | 04 | 3 | DIAL-03, DIAL-05 | — | N/A | unit | `go test ./pkg/parser -run 'TestStructAT\|TestEmptyArg' -count=1` | ✅ | ✅ green |
| 19-04-02 | 04 | 3 | DIAL-03, DIAL-05 | — | N/A | unit (tdd) | `go test ./pkg/emit ./pkg/format -run 'StructAT\|EmptyArg' -count=1` | ✅ | ✅ green |
| 19-04-03 | 04 | 3 | DIAL-03, DIAL-05 | T-19-08 | AT addresses validated for format only; accepted | unit (tdd) | `go test ./pkg/checker -run 'TestATWildcard\|TestEmptyArgCheck\|TestATAddress' -count=1` | ✅ | ✅ green |
| 19-04-04 | 04 | 3 | DIAL-05 | T-19-07 | execCallStmt skips nil argument values instead of dereferencing them | unit (tdd) | `go test ./pkg/interp -run TestEmptyArg -count=1` | ✅ | ✅ green |
| 19-05-01 | 05 | 4 | DIAL-02, DIAL-01 | T-19-09 | GVL name comes from the sanitised file basename | unit | `go test ./pkg/parser ./pkg/emit ./pkg/format -run GVL -count=1` | ✅ | ✅ green |
| 19-05-02 | 05 | 4 | DIAL-02 | T-19-11 | qualified_only bare access reported as SEMA033; accepted | unit (tdd) | `go test ./pkg/checker -run TestGVL -count=1 && go test ./pkg/incremental -run TestDepGraph -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 19-06-01 | 06 | 5 | DIAL-02 | T-19-12 | GVL member lookup is case-insensitive and bounded to declared members | unit | `go test ./pkg/interp -run TestGVL -count=1` | ✅ | ✅ green |
| 19-06-02 | 06 | 5 | DIAL-02 | T-19-13 | GVL state is fresh per TEST_CASE, so tests stay isolated | unit + CLI (tdd) | `go test ./pkg/testing ./pkg/sim ./cmd/stc -run GVL -count=1` | ✅ | ✅ green |
| 19-06-03 | 06 | 5 | DIAL-02 | — | N/A | ST suite (tdd) | `go run ./cmd/stc test tests/twincat_dialect && go run ./cmd/stc test tests/` | ✅ | ✅ green |
| 19-07-01 | 07 | 5 | DIAL-08 | T-19-14 | POU parse loops always advance on ACTION recovery | unit | `go test ./pkg/parser -run TestAction -count=1` | ✅ | ✅ green |
| 19-07-02 | 07 | 5 | DIAL-08 | — | N/A | unit (tdd) | `go test ./pkg/emit ./pkg/format -run Action -count=1` | ✅ | ✅ green |
| 19-07-03 | 07 | 5 | DIAL-08 | T-19-15 | Orphan ACTION reported, never silently attached | unit (tdd) | `go test ./pkg/checker -run TestAction -count=1 && go test ./pkg/symbols -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 19-08-01 | 08 | 6 | DIAL-08 | T-19-16 | Call depth guard turns runaway action recursion into a runtime error | unit | `go test ./pkg/interp -run TestAction -count=1` | ✅ | ✅ green |
| 19-08-02 | 08 | 6 | DIAL-08, DIAL-05 | T-19-17 | Infinite loops in action bodies are the author's responsibility; accepted | ST suite (tdd) | `go run ./cmd/stc test tests/twincat_dialect && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 19-10-01 | 10 | 5 | DIAL-02 | T-19-10 | --gvl-name value sanitised to a valid identifier | CLI | `go test ./cmd/stc -run TestGVLNameFlag -count=1` | ✅ | ✅ green |
| 19-10-02 | 10 | 5 | DIAL-02 | T-19-20 | --gvl-name with more than one file rejected before any parsing | CLI (tdd) | `go test ./cmd/stc -run 'TestParseJSONECT\|TestGVLNameFlag' -count=1` | ✅ | ✅ green |
| 19-09-01 | 09 | 7 | DIAL-01, DIAL-02, DIAL-03, DIAL-05, DIAL-08 | T-19-18 | Oracle sources read only via STC_PROBES_DIR; test logs counts and line numbers, skips in CI | integration | `go test ./tests/ -run TestTwinCATProbe -count=1 && STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests/ -run TestTwinCATProbeOracle -count=1 -v` | ✅ | ✅ green |
| 19-09-02 | 09 | 7 | DIAL-01, DIAL-02, DIAL-08 | — | N/A | unit (tdd) | `go test ./pkg/lsp -run TwinCAT -count=1` | ✅ | ✅ green |
| 19-09-03 | 09 | 7 | all | T-19-19 | .testcoverage.yml unchanged versus main | full gate | `go vet ./... && go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh && grep -q "nyquist_compliant: true" .planning/phases/19-twincat-declaration-syntax/19-VALIDATION.md` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `scripts/coverage-gate.sh` — local replica of the CI coverage gate (19-01)
- [x] `tests/twincat_probes/*.st` and `tests/twincat_probes/README.md` — 11 committed TwinCAT probe fixtures (19-01)
- [x] `pkg/interp/stdlib_errors_test.go` — interp coverage headroom so later plans keep the 95% gate (19-01)
- [x] Framework: go test and `stc test` already present; no install needed

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
All phase behaviors have automated verification.

The oracle half of 19-09-01 needs the local customer sources. It is automated through `STC_PROBES_DIR` and skips in CI by design.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Per-task feedback latency < 30s; coverage gate only at wave end (~120s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06
