---
phase: 22
slug: symbol-tree-value-semantics
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-10-06
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.25) + stc test ST suites |
| **Config file** | .testcoverage.yml, scripts/coverage-gate.sh, CI workflows |
| **Quick run command** | targeted `go test ./pkg/<pkg> -run <Pattern> -count=1` |
| **Full suite command** | `go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` |
| **Estimated runtime** | ~22 seconds |

---

## Sampling Rate

- **After every task commit:** Run the targeted quick command
- **After every plan wave:** Run the full suite command (gate in final plan)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 22 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | RUNT-05 | T-22-02 | Wrap is the specified IEC semantics | unit | `go test ./pkg/interp -run 'TestWrapInt|TestStoreAs|Enum' -count=1` | ✅ | ✅ green |
| 22-01-02 | 01 | 1 | RUNT-05 | — | N/A | unit | `go test ./pkg/interp -run 'TestInitialisedIECType|TestStore|Ref|Bit|For' -count=1` | ✅ | ✅ green |
| 22-01-03 | 01 | 1 | RUNT-05 | T-22-01 | Division and MOD by zero stay runtime errors | unit + ST | `go test ./pkg/interp -count=1 && go run ./cmd/stc test tests/type_system/` | ✅ | ✅ green |
| 22-02-01 | 02 | 1 | RUNT-05 | T-22-03 | Constant folding never panics; overflow leaves the value unknown | unit | `go test ./pkg/checker -run 'TestUntyped' -count=1` | ✅ | ✅ green |
| 22-02-02 | 02 | 1 | RUNT-05 | — | N/A | unit | `go test ./pkg/checker -count=1` | ✅ | ✅ green |
| 22-02-03 | 02 | 1 | RUNT-05 | T-22-04 | Oracle counts by message only | integration + oracle | `go test ./tests -run 'TestTwinCATDialect' -count=1` (oracle: `STC_PROBES_DIR=... go test ./tests -run TestTwinCATProbeOracle`) | ✅ | ✅ green |
| 22-03-01 | 03 | 1 | RUNT-01 | T-22-06 | Type-name stack cycle guard | unit | `go test ./pkg/analyzer ./pkg/symtree -run 'TestBuild|TestAnalyze' -count=1` | ✅ | ✅ green |
| 22-03-02 | 03 | 1 | RUNT-01 | T-22-05 | 1024-byte path cap, no panic (fuzzed) | unit + fuzz | `go test ./pkg/symtree -count=1 && go test ./pkg/symtree -run XXX -fuzz FuzzParsePath -fuzztime 10s` | ✅ | ✅ green |
| 22-03-03 | 03 | 1 | RUNT-01 | T-22-07 | Prints only user declarations | CLI exec | `go test ./cmd/stc -run 'TestCheck' -count=1` | ✅ | ✅ green |
| 22-04-01 | 04 | 2 | RUNT-06 | T-22-08 | Array slot cap and bound checks | unit | `go test ./pkg/interp -run 'TestConstBounds|TestStructMemberDefaults|TestClone' -count=1` | ✅ | ✅ green |
| 22-04-02 | 04 | 2 | RUNT-06 | T-22-09 | FB and TYPE nesting depth caps | unit | `go test ./pkg/interp -count=1` | ✅ | ✅ green |
| 22-04-03 | 04 | 2 | RUNT-06 | — | N/A | unit + ST | `go test ./pkg/interp ./pkg/testing -count=1 && go run ./cmd/stc test tests/` | ✅ | ✅ green |
| 22-05-01 | 05 | 3 | RUNT-02 | — | N/A | unit | `go test ./pkg/interp -run 'TestNewRuntime|TestRuntimeTick|TestRegisterFiles' -count=1` | ✅ | ✅ green |
| 22-05-02 | 05 | 3 | RUNT-02 | T-22-12 | 1024-byte path cap, bounds checked before indexing, no panic | unit | `go test ./pkg/interp -run 'TestRuntimeGet|TestToJSON|TestRuntimePath' -count=1` | ✅ | ✅ green |
| 22-05-03 | 05 | 3 | RUNT-02 | T-22-10, T-22-11, T-22-13 | CONSTANT, pointer and stdlib-output writes rejected; mutex serialises Tick/Get/Set | unit + race | `go test ./pkg/interp -run 'TestRuntimeSet|TestCoerce' -count=1 && go test -race ./pkg/interp -run TestRuntimeConcurrent -count=1` | ✅ | ✅ green |
| 22-05-04 | 05 | 3 | RUNT-02 | — | N/A | CLI exec | `go test ./pkg/sim ./cmd/stc -run 'TestSim' -count=1` | ✅ | ✅ green |
| 22-05-05 | 05 | 3 | RUNT-01, RUNT-02, RUNT-05, RUNT-06 | T-22-14 | Oracle subtest counts by message, logs templated text | integration + oracle + gate | `go test ./... -count=1 && go run ./cmd/stc test tests/ && STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests -run 'TwinCAT|TestRuntimeFixture' -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `pkg/interp/store_test.go` — wrapInt table and storeAs (22-01)
- [x] `tests/type_system/wrap_test.st` — ST wrap suite (22-01)
- [x] `pkg/checker/untyped_test.go` — untyped literal adoption and range errors (22-02)
- [x] `pkg/symtree/symtree_test.go` and fuzz target — tree, paths, JSON (22-03)
- [x] `pkg/interp/init_value_test.go`, `tests/type_system/init_test.st` — initialisers and constant bounds (22-04)
- [x] `pkg/interp/runtime_test.go` — Runtime Get/Set/ToJSON/coercion/race (22-05)
- [x] `tests/runtime/st301_shape/` and `tests/runtime_fixture_test.go` — end-to-end SC1-SC4 (22-05)

Existing infrastructure (go test, testify, stc test, coverage gate) covers the framework.


---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| none | — | — | — |

All phase behaviors have automated verification. The oracle subtest needs the local-only STC_PROBES_DIR and skips without it.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 22s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06
