---
phase: 20
slug: twincat-expression-semantics
status: approved
nyquist_compliant: true
wave_0_complete: true
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
| 20-01-01 | 01 | 1 | DIAL-04, DIAL-06, DIAL-07, DIAL-09, DIAL-10 (enabling) | T-20-01, T-20-03 | EnumOrdinals overflow or malformed literal gives Known=false, never a panic | unit | `go test ./pkg/ast -run 'TestPhase20Nodes\|TestEnumOrdinals' -count=1` | ✅ | ✅ green |
| 20-01-02 | 01 | 1 | DIAL-04, DIAL-06, DIAL-07, DIAL-09 (enabling) | T-20-02 | Every new node has a printer case and an exact-substring test, so fmt/emit cannot drop syntax | unit | `go test ./pkg/format ./pkg/emit ./pkg/lint -run 'TestPhase20Nodes\|Lint' -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 20-02-01 | 02 | 2 | DIAL-04, DIAL-09 | T-20-04, T-20-05 | Postfix bit-access loop always consumes a token; bit index kept as literal text | unit | `go test ./pkg/parser -run 'TestBitAccess\|TestThisSuper' -count=1` | ✅ | ✅ green |
| 20-02-02 | 02 | 2 | DIAL-06 | T-20-04, T-20-06 | Call-argument loops terminate on `f(,,,)` and EOF; statement-head FB calls unchanged | unit | `go test ./pkg/parser -run 'TestNamedArgsExpr\|TestCallArg\|TestThisSuper' -count=1` | ✅ | ✅ green |
| 20-02-03 | 02 | 2 | DIAL-07, DIAL-09, DIAL-10 | T-20-04 | `r REF=` at EOF finishes with a diagnostic; qualified CASE labels round-trip | unit | `go test ./pkg/parser -run 'TestRefAssign\|TestCaseQualifiedLabels' -count=1 && go test ./pkg/format -run TestPhase20ExprRoundTrip -count=1` | ✅ | ✅ green |
| 20-03-01 | 03 | 2 | DIAL-07, DIAL-10 | T-20-09 | Based-literal scan always advances; `BYTE#16#` at EOF is a diagnostic | unit | `go test ./pkg/lexer -run TestTypedBasedLiteral -count=1 && go test ./pkg/parser -run 'TestEnumBaseType\|TestNamespaceType\|TestStraySemicolon' -count=1` | ✅ | ✅ green |
| 20-03-02 | 03 | 2 | DIAL-10 | T-20-07, T-20-08 | Initialiser nesting above 64 levels is a parse error; repetition counts are never expanded | unit | `go test ./pkg/parser -run TestInitializer -count=1 && go test ./pkg/format -run TestPhase20DeclRoundTrip -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 20-04-01 | 04 | 3 | DIAL-10 | T-20-10 | Alias fixpoint sweep stops when nothing new resolves; self and mutual aliases terminate | unit | `go test ./pkg/checker -run 'TestResolveForwardRef\|TestGVL\|TestResolve' -count=1` | ✅ | ✅ green |
| 20-04-02 | 04 | 3 | RUNT-08, DIAL-09 | T-20-10, T-20-11 | EXTENDS walk has a visited set and depth bound; user FBs override standard FBs by design | unit | `go test ./pkg/checker -run 'TestStdFB\|TestInheritedScope\|TestAction' -count=1` | ✅ | ✅ green |
| 20-04-03 | 04 | 3 | RUNT-08, DIAL-10 | T-20-12 | One SEMA037 per TypeSpec; types.Invalid suppresses cascades | unit | `go test ./pkg/checker -run 'TestUndeclaredType\|TestResolveForwardRef\|TestStdFB\|TestGVL' -count=1 && go test -cover ./pkg/checker -count=1` | ✅ | ✅ green |
| 20-05-01 | 05 | 3 | DIAL-04 | T-20-13 | Bit index range-checked before shifting; 64, -1 and huge indices are RuntimeErrors | unit | `go test ./pkg/interp -run TestBitAccess -count=1` | ✅ | ✅ green |
| 20-05-02 | 05 | 3 | DIAL-07 | T-20-14, T-20-15 | Enum lookup never shadows a variable or GVL; inline enum registration bounded by source | unit | `go test ./pkg/interp -run 'TestEnumRuntime\|TestTimeLiteral\|TestTypedLiteral' -count=1 && go test ./pkg/testing -run TestEnumRegister -count=1` | ✅ | ✅ green |
| 20-05-03 | 05 | 3 | RUNT-08 | — | N/A | unit | `go test ./pkg/interp -run 'TestStdFBAliases\|TestCTU\|TestCTD\|TestCTUD\|TestSR\|TestRS' -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 20-06-01 | 06 | 4 | DIAL-07 | T-20-25 | Each known ordinal range-checked against the base type; `(a := 300) USINT` reported | unit | `go test ./pkg/checker -run 'TestEnumResolve\|TestFunctionOutputs' -count=1 && go test ./pkg/types -count=1` | ✅ | ✅ green |
| 20-06-02 | 06 | 4 | DIAL-04 | T-20-17, T-20-26 | 20-digit bit index treated as out of range; constant-index reading only for integer objects | unit | `go test ./pkg/checker -run TestBitAccess -count=1` | ✅ | ✅ green |
| 20-06-03 | 06 | 4 | DIAL-06 | — | N/A | unit | `go test ./pkg/checker -run 'TestNamedArgs\|TestAction' -count=1 && go test -cover ./pkg/checker -count=1` | ✅ | ✅ green |
| 20-07-01 | 07 | 4 | DIAL-06 | T-20-19 | FUNCTION calls go through EnterCall/ExitCall (MaxCallDepth 256) | unit | `go test ./pkg/interp -run 'TestNamedArgs\|TestCallFunction\|TestVarInOut\|TestEmptyArg' -count=1 && go test ./pkg/testing -count=1` | ✅ | ✅ green |
| 20-07-02 | 07 | 4 | DIAL-09 | T-20-19 | Self-recursive SUPER^() and methods hit the call depth limit | unit | `go test ./pkg/interp -run 'TestThisSuper\|TestUnqualifiedMethod\|TestAction\|TestMethod' -count=1` | ✅ | ✅ green |
| 20-07-03 | 07 | 4 | DIAL-09 | T-20-20, T-20-21 | Dangling reference paths are RuntimeErrors; aggregate write-back keeps the reference | unit | `go test ./pkg/interp -run 'TestRefAssign\|TestPointer\|TestReference' -count=1 && bash scripts/coverage-gate.sh` | ✅ | ✅ green |
| 20-08-01 | 08 | 6 | DIAL-04, DIAL-06, DIAL-07, DIAL-09 | — | N/A | ST suite | `go run ./cmd/stc test tests/twincat_dialect --format json` | ✅ | ✅ green |
| 20-08-02 | 08 | 6 | DIAL-10, RUNT-08 | T-20-22, T-20-23, T-20-24 | Oracle logs counts and templated messages only; only synthetic probes committed; allowance map deleted | integration | `go test ./tests -run 'TestTwinCATProbeFixtures\|TestTwinCATDialectCheck\|TestOwnedLine' -count=1 && go test ./pkg/checker ./pkg/interp -run TestAction -count=1 && STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests -run TestTwinCATProbeOracle -count=1 -v` | ✅ | ✅ green |
| 20-08-03 | 08 | 6 | all | — | N/A | full gate | `go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh && grep -q "nyquist_compliant: true" .planning/phases/20-twincat-expression-semantics/20-VALIDATION.md` | ✅ | ✅ green |
| 20-09-01 | 09 | 5 | DIAL-07 | T-20-18 | Non-strict enum relaxations follow ruling A4; strict enums keep full checking | unit | `go test ./pkg/checker -run 'TestEnumAttr\|TestEnumResolve' -count=1 && go test ./pkg/types -count=1` | ✅ | ✅ green |
| 20-09-02 | 09 | 5 | DIAL-09 | T-20-27 | REF= accepts only Ident/Member/Index/Deref paths of the exact base type | unit | `go test ./pkg/checker -run 'TestRefAssign\|TestThisSuper\|TestRefAutoDeref\|TestAction' -count=1` | ✅ | ✅ green |
| 20-09-03 | 09 | 5 | DIAL-10 | T-20-16 | Repetition counts use saturating arithmetic and are never expanded | unit | `go test ./pkg/checker -run TestInitializerCheck -count=1 && go test -cover ./pkg/checker -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure (go test, `stc test`, the probe gate and the coverage gate) covers all phase requirements. Every TDD task created its own failing test file first:

- [x] 20-01: `pkg/ast/phase20_nodes_test.go`, `pkg/ast/enum_ordinals_test.go`, `pkg/format/phase20_nodes_test.go`, `pkg/emit/phase20_nodes_test.go`, `pkg/lint/phase20_nodes_test.go`
- [x] 20-02: `pkg/parser/bit_access_test.go`, `pkg/parser/named_args_expr_test.go`, `pkg/parser/ref_this_super_test.go`, `pkg/parser/case_qualified_test.go`, `pkg/format/phase20_expr_roundtrip_test.go`
- [x] 20-03: `pkg/lexer/typed_based_literal_test.go`, `pkg/parser/initializer_test.go`, `pkg/parser/enum_base_test.go`, `pkg/parser/namespace_type_test.go`, `pkg/format/phase20_decl_roundtrip_test.go`
- [x] 20-04: `pkg/checker/stdlib_fb_test.go`, `pkg/checker/inherited_scope_test.go`, `pkg/checker/undeclared_type_test.go`, `pkg/checker/testdata/forward_ref_members.st`
- [x] 20-05: `pkg/interp/bit_access_test.go`, `pkg/interp/enum_runtime_test.go`, `pkg/interp/stdlib_alias_test.go`, `pkg/testing/enum_register_test.go`
- [x] 20-06: `pkg/checker/enum_resolve_test.go`, `pkg/checker/function_outputs_test.go`, `pkg/checker/bit_access_test.go`, `pkg/checker/named_args_test.go`
- [x] 20-07: `pkg/interp/call_args_test.go`, `pkg/interp/this_super_test.go`, `pkg/interp/ref_path_test.go`, `pkg/testing/runner_functions_test.go`
- [x] 20-09: `pkg/checker/enum_attr_test.go`, `pkg/checker/ref_this_test.go`, `pkg/checker/initializer_check_test.go`, `pkg/interp/stdlib_convert_test.go`
- [x] 20-08: `tests/twincat_dialect/{bit_access,named_args,enum,ref_this_super}_test.st`, `tests/twincat_dialect_check_test.go`, `tests/twincat_probes/{case,enum,fcall,ptr}.st`

---

## Manual-Only Verifications

All phase behaviors have automated verification.

The zero-parse-error oracle (`TestTwinCATProbeOracle`) is automated but local-only: it reads the uncommitted customer sources through `STC_PROBES_DIR` and skips in CI. The committed synthetic probes and `TestTwinCATDialectCheck` run everywhere.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 20s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-10-06
