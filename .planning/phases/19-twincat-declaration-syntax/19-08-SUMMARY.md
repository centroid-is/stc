---
phase: 19-twincat-declaration-syntax
plan: 08
subsystem: interp, testing
tags: [interp, action, twincat, codesys, runtime, recursion-guard]

requires:
  - phase: 19-06
    provides: GVL runtime layer (gvlRoot, evalGVLMember, gvlEngine test helper)
  - phase: 19-07
    provides: ProgramDecl.Actions, FunctionBlockDecl.Actions, A1(); as AssignStmt{Target: CallExpr}
provides:
  - Env.DefineAction / Env.LookupAction; actions run against the owning POU env
  - A1(); inside a PROGRAM or FB, inst.A1(); from outside, inherited actions via EXTENDS
  - Interpreter.EnterCall/ExitCall with MaxCallDepth 256 on actions, methods and runner FUNCTIONs
  - Zero-argument FB instance calls b();, G.f();, outer.inner(); and s.fb();
  - FB instances built from FBDecls get inherited variables and ParentDecl
  - ST suites tests/twincat_dialect/action_test.st and empty_args_test.st
affects: [19-09, 20, 21]

tech-stack:
  added: []
  patterns:
    - "An action is registered on the env of its owner (program env or FB instance env) and executed with that env, not a child env"
    - "Nested ACTION/METHOD/FUNCTION execution goes through EnterCall/ExitCall"

key-files:
  created:
    - pkg/interp/action_test.go
    - pkg/testing/call_depth_test.go
    - tests/twincat_dialect/action_test.st
    - tests/twincat_dialect/empty_args_test.st
  modified:
    - pkg/interp/env.go
    - pkg/interp/interpreter.go
    - pkg/interp/scan.go
    - pkg/interp/fb_instance.go
    - pkg/testing/runner.go
    - .planning/phases/19-twincat-declaration-syntax/deferred-items.md

key-decisions:
  - "Action dispatch in evalCall comes before ADR/REF, LocalFunctions and stdlib, so an action shadows a library function of the same name inside its POU"
  - "An action called with arguments is a RuntimeError (action X takes no arguments)"
  - "The depth guard covers actions, methods and test-runner user FUNCTIONs, at 256 nested calls; the error names the callee"
  - "Zero-argument FB calls are resolved last in evalCall, after stdlib functions, so no existing function call changes meaning"
  - "newUserFBInstanceDepth walks the EXTENDS chain through FBDecls, base first, so inherited variables and actions exist in every instance env, not only in test-runner instances"

requirements-completed: [DIAL-08]

duration: 7min
completed: 2026-10-06
---

# Phase 19 Plan 08: ACTION runtime Summary

**ACTIONs now execute against their owning POU's variables. This works for `A1();` inside a PROGRAM or FB, for `inst.A1();` from outside, and for actions inherited through EXTENDS. A self-recursive action fails with a RuntimeError at call depth 256 instead of crashing.**

## Performance

- **Duration:** about 7 min
- **Started:** 2026-10-05T23:53:54Z
- **Completed:** 2026-10-06T00:01:00Z
- **Tasks:** 2 of 2
- **Files:** 4 created, 6 modified

## Accomplishments

- `Env` has an action table. The program env registers `ProgramDecl.Actions`, and each user FB instance env registers the actions of its FB and of every base FB.
- `evalCall` dispatches a bare call to an action before any function lookup and runs the body in the owner env. RETURN ends only the action.
- `evalMethodCall` falls back to the FB's actions when no method matches, so `inst.coe();` runs in the instance env.
- `MaxCallDepth` is 256. Actions, methods and test-runner user FUNCTIONs all go through `EnterCall`/`ExitCall`, and the depth unwinds after the error.
- Zero-argument FB calls now run, which fixes the 19-06 deferred bug. This covers `b();`, `G.f();` on a GVL instance, `outer.inner();` and `s.fb();`.
- action_inside.st runs as MAIN through ScanCycleEngine. The R_TRIG fires once, the TON is done after 1.2 s and not at 0.8 s, and `n` counts scans.

## Task Commits

1. **Task 1: Action dispatch in the interpreter with recursion guard**
   - `09a4caa` test(19-08): failing runtime tests (RED, build fails on `MaxCallDepth`)
   - `a2792f5` feat(19-08): implementation (GREEN), plus zero-arg FB calls and the runner FUNCTION guard
2. **Task 2: ST suites for FB actions and empty arguments**
   - `da60b84` test(19-08): action_test.st and empty_args_test.st
- `5851c6a` docs(19-08): deferred items

## Verification

- `go test ./... -count=1` passes.
- `go test ./pkg/interp -run TestAction -v` shows 11 passing subtests. They cover each behaviour line in the plan, including the depth-256 guard for actions and methods. `TestZeroArgFBCall` adds 8 more.
- `go run ./cmd/stc test tests/twincat_dialect` gives 12 of 12 passing. That is 4 cases in action_test.st and 2 in empty_args_test.st.
- `go run ./cmd/stc test tests/` gives 216 of 216 passing.
- Both new suites were formatted with `stc fmt` into a temp dir and still pass, 6 of 6.
- The CI step for tests/twincat_dialect already exists in .github/workflows/st-tests.yml.
- `bash scripts/coverage-gate.sh` passes:

| package | covered | percent | min |
|---------|---------|---------|-----|
| pkg/parser | 1006/1039 | 96.82% | 95% |
| pkg/lexer | 228/234 | 97.44% | 95% |
| pkg/checker | 837/873 | 95.88% | 94% |
| pkg/interp | 1673/1732 | 96.59% | 95% |
| pkg/types | 124/124 | 100.00% | 95% |
| pkg/emit | 571/586 | 97.44% | 95% |
| total | 7989/8474 | 94.28% | 85% |

## Requirements

- **DIAL-08** is complete. Actions parse, check and print (19-07) and now execute.
- **DIAL-05** was already complete in 19-04. This plan adds the end-to-end TON suite and does not re-mark it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Zero-argument FB instance calls failed at runtime**
- **Found during:** Task 1 (logged in 19-06)
- **Issue:** `b();` reported `undefined function: B` and `G.f();` reported `method 'f' not found`, because both reach evalCall/evalMethodCall instead of execCallStmt.
- **Fix:** evalCall runs an FB instance named by the callee when nothing else matches and there are no arguments. evalMethodCall handles GVL roots, struct members and nested FB members the same way.
- **Files modified:** pkg/interp/interpreter.go
- **Commit:** a2792f5

**2. [Rule 3 - Blocking] FB instances outside the test runner had no inherited variables**
- **Found during:** Task 1, on the "derived FB calls a base action" behaviour
- **Issue:** Only pkg/testing initialised base FB variables and set ParentDecl. In ScanCycleEngine, a GVL or a nested FB, a derived FB failed with `undefined variable: n` on the base's variable.
- **Fix:** `fbExtendsChain` walks EXTENDS through FBDecls, base first, with a cycle check and the existing depth bound. Variables and actions of the whole chain go into the instance env, and ParentDecl is set. The runner's own base-variable loop still runs and skips names that already exist.
- **Files modified:** pkg/interp/fb_instance.go
- **Commit:** a2792f5

**3. [Rule 2 - Critical] Recursive user FUNCTIONs in the test runner had no depth bound**
- **Issue:** Under threat T-19-16, only actions and methods were guarded inside pkg/interp. A recursive FUNCTION in a test suite would still overflow the Go stack.
- **Fix:** The runner's FUNCTION wrapper calls `EnterCall`/`ExitCall`. Covered by pkg/testing/call_depth_test.go.
- **Files modified:** pkg/testing/runner.go
- **Commit:** a2792f5

### Other notes

- pkg/interp/env.go, scan.go and fb_instance.go were not gofmt-clean before this plan. They were gofmt'd as part of the edit, so the diff has a few whitespace-only lines.
- The method recursion test recurses through a GVL instance, `G.r.M()`. `THIS^` does not parse yet, and unqualified method calls do not run (see below).

## Issues Encountered

- Unqualified METHOD calls inside an FB, such as `Inc();`, fail at runtime with `undefined function`. This predates the plan, but now matters more: the 19-07 checker accepts an action that calls its FB's method, and that action fails when run. Logged in deferred-items.md.
- `Env.LookupAction` walks the whole parent chain. An FB body could therefore reach an action that only the enclosing PROGRAM defines. The checker reports SEMA010 for that call. Logged.

## Notes for Later Plans

- **19-09:** dispatch order in evalCall is now: action, ADR, REF, LocalFunctions, stdlib, zero-argument FB instance, then the `undefined function` error. Any new nested execution path should use `interp.EnterCall(name, pos)` with a deferred `ExitCall()`. `stc test` still never runs PROGRAMs, so PROGRAM actions are tested through ScanCycleEngine using the `gvlEngine`/`probeEngine` helpers in pkg/interp.
- Orphan top-level ActionDecls are never registered on any env, so they never run.

## Known Stubs

None.

## Threat Flags

None. T-19-16 is mitigated and tested for actions, methods and runner FUNCTIONs. T-19-17 is accepted; MaxLoopIterations still applies inside action bodies.

## Self-Check: PASSED
