---
phase: 19-twincat-declaration-syntax
plan: 06
subsystem: interp, testing, sim, ci
tags: [interpreter, gvl, test-runner, sim, twincat, ci]

requires:
  - phase: 19-01
    provides: twincat probe fixtures
  - phase: 19-05
    provides: ast.GVLDecl from top-level VAR_GLOBAL, qualified_only enforced by the checker
provides:
  - Interpreter.RegisterGVL, Interpreter.GlobalParent, Env.GetLocal
  - GVL.x / GVL.s.a / GVL.fb(...) / q => GVL.flag at runtime
  - ScanCycleEngine.SetGlobals and the shared Interpreter.initVarDecl helper
  - GVL registration per TEST_CASE in pkg/testing
  - sim.SimConfig.GVLs and GVL support in `stc sim`
  - tests/twincat_dialect ST suite and its CI step
affects: [19-07, 19-08, 19-10, 22, 23]

tech-stack:
  added: []
  patterns:
    - "One Env per GVL; GVL.x reads the GVL's own scope only (Env.GetLocal), so chained parents never leak into qualified access"
    - "Non qualified_only GVL envs form a parent chain; program, test and FUNCTION envs are created with Interpreter.GlobalParent() as parent"
    - "A local variable with the GVL's name shadows the GVL for member access"

key-files:
  created:
    - pkg/interp/gvl.go
    - pkg/interp/gvl_test.go
    - pkg/testing/gvl_runner_test.go
    - pkg/sim/gvl_test.go
    - cmd/stc/sim_gvl_test.go
    - tests/twincat_dialect/gvl_test.st
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/scan.go
    - pkg/interp/env.go
    - pkg/testing/runner.go
    - pkg/sim/engine.go
    - cmd/stc/sim_cmd.go
    - .github/workflows/st-tests.yml
    - .planning/phases/19-twincat-declaration-syntax/deferred-items.md

key-decisions:
  - "Writing an undeclared GVL member is a RuntimeError, not an implicit define, so a typo cannot create a global"
  - "Every GVL env gets the current unqualified chain as parent so initialisers can read earlier GVLs; member access ignores the parent"
  - "CallStmt accepts a MemberAccessExpr callee (GVL.t(...), s.fb(...)) and => bindings to member targets"
  - "User FUNCTION envs in the test runner also chain to GlobalParent so bare GVL names work inside functions"

patterns-established:
  - "Program and GVL variables are created by one helper, Interpreter.initVarDecl"

requirements-completed: []  # DIAL-02 stays pending until 19-10 (--gvl-name)

duration: 8min
completed: 2026-10-05
---

# Phase 19 Plan 06: GVL runtime env layer Summary

**The interpreter now runs `GVL.x`, nested `GVL.s.a`, `GVL.fb(...)` and bare access to non-qualified_only GVL variables. GVLs work in `stc test` with fresh state per TEST_CASE and in `stc sim`, and a new twincat_dialect ST suite runs in CI on all three OSes.**

## Performance

- **Duration:** about 8 min
- **Started:** 2026-10-05T23:26:30Z
- **Completed:** 2026-10-05T23:34:30Z
- **Tasks:** 3 of 3
- **Files:** 6 created, 8 modified

## Accomplishments

- `Interpreter.RegisterGVL` builds one Env per GVL with the same variable setup as a program. Stdlib FBs and user FBs from `FBDecls` become live instances. User TYPEs resolve through `TypeDecls`.
- `evalMemberAccess` and `execAssignMember` fall back to the GVL env when the root identifier is not a variable in scope. Nested struct and array writes go through the shared Go map or slice.
- GVLs without `qualified_only`, on the GVL or on any of its blocks, form a parent chain. `GlobalParent()` returns its innermost env, and program, test and FUNCTION envs use it as their parent.
- `ScanCycleEngine.SetGlobals` registers GVLs before the program env is built. `initializeEnv` now uses the shared `initVarDecl`, which also clones aggregate initial values per name.
- `execCallStmt` accepts member callees, so `G.t(IN := TRUE, PT := T#10MS);` runs a TON inside a GVL. `Q => G.q` output bindings write member targets.
- The test runner collects GVLDecls and registers them on each TEST_CASE's fresh interpreter, so state never leaks between cases.
- `sim.SimConfig.GVLs` is passed to `SetGlobals`. `stc sim` collects the file's GVLs.
- `tests/twincat_dialect/gvl_test.st` has 6 cases: qualified, unqualified, nested struct, isolation, FB body and shadowing. `stc check` on the file reports 0 errors and 0 warnings. st-tests.yml has a new "TwinCAT dialect" step.

## Task Commits

1. **Task 1: Interpreter GVL env layer**
   - `87bdcdb` test(19-06): failing tests (RED)
   - `6b7a5e4` feat(19-06): implementation (GREEN)
2. **Task 2: Register GVLs in the ST test runner and stc sim**
   - `86caa84` test(19-06): failing tests (RED)
   - `346ee77` feat(19-06): implementation (GREEN)
   - `b0296ad` fix(19-06): sim command test decodes outputs as plain numbers. This edit was made before 346ee77 but was left out of that commit, so 346ee77 alone fails that one test.
3. **Task 3: tests/twincat_dialect GVL suite and CI step**
   - `dc3c6c1` test(19-06): suite and CI step

## Verification

- `go test ./... -count=1` passes.
- `go run ./cmd/stc test tests/twincat_dialect` reports 6 passed.
- `go run ./cmd/stc test tests/` reports 210 passed, 0 failed.
- st-tests.yml parses as YAML and lists the new step after stdlib_comprehensive.
- `bash scripts/coverage-gate.sh` passes. Every function in gvl.go is at 100%.

| package | covered | percent | min |
|---------|---------|---------|-----|
| pkg/parser | 960/994 | 96.58% | 95% |
| pkg/lexer | 228/234 | 97.44% | 95% |
| pkg/checker | 777/814 | 95.45% | 94% |
| pkg/interp | 1592/1649 | 96.54% | 95% |
| pkg/types | 124/124 | 100.00% | 95% |
| pkg/emit | 564/579 | 97.41% | 95% |
| total | 7736/8238 | 93.91% | 85% |

## Requirements

- **DIAL-02** stays pending. Its `--gvl-name` part is plan 19-10.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] CallStmt only accepted an identifier callee**
- **Found during:** Task 1
- **Issue:** `G.t(IN := TRUE, PT := T#10MS);` parses as a CallStmt whose callee is a MemberAccessExpr. `execCallStmt` rejected it with "unsupported call target".
- **Fix:** A member callee is evaluated with `evalMemberAccess` and must yield an FB instance. `=>` output bindings to a member target go through `execAssignMember`.
- **Files modified:** pkg/interp/interpreter.go
- **Commit:** 6b7a5e4

**2. [Rule 2 - Correctness] Member lookup must not walk the GVL chain**
- **Issue:** GVL envs have parents, so `G2.a` would have found `a` from G1 through `Env.Get`.
- **Fix:** Added `Env.GetLocal`. GVL member reads and writes use only the GVL's own scope.
- **Commit:** 6b7a5e4

**3. [Rule 2 - Correctness] User FUNCTION bodies in the test runner could not see bare GVL names**
- **Fix:** `callUserFunction` creates its env with `GlobalParent()` as parent. This is covered by the runner test.
- **Commit:** 346ee77

**4. [Rule 2] SimConfig was named Config in the plan**
- The field went on the existing `sim.SimConfig`. A pkg/sim unit test was added beside the command test.

### Other notes

- `initVarDecl` replaced the inline loop in `ScanCycleEngine.initializeEnv`. Behaviour for programs without GVLs is unchanged. The engine's `FBDecls` and `TypeDecls` stay nil unless a caller sets them. Aggregate initial values are now cloned per name.
- `stc sim` still registers no TYPE or FUNCTION_BLOCK declarations. That is pre-existing. A struct-typed GVL member is therefore a RuntimeError there, which is logged in deferred-items.md.

## Issues Encountered

- **Zero-argument FB calls such as `b();` fail at runtime with `undefined function: B`.** This happens without any GVL, so it predates this plan. The test FBs take an input instead. Logged in deferred-items.md for 19-08, which edits the same evalCall branch.

## Known Stubs

None.

## Threat Flags

None. T-19-12 is mitigated: unknown GVL members return a RuntimeError on read, write, call and output binding, and tests check every case. T-19-13 is mitigated: GVL envs are rebuilt for each TEST_CASE, and both the runner test and the ST suite check isolation.

## Notes for Later Plans

- **19-08 (actions):** zero-argument calls reach `evalCall` and `evalMethodCall`, never `execCallStmt`. `G.f();` on a GVL FB instance currently reports "method 'f' not found". A fallback that runs the FB instance there would also fix plain `b();`.
- **Phase 23:** GVL state lives in `Interpreter.gvls`. Per-task instantiation can either share one Interpreter across tasks or move the map out of it. PERSISTENT and RETAIN have no runtime effect yet.
- **Library and mock GVLs** are not registered by the test runner. Only GVLs in the `*_test.st` file are.

## Self-Check: PASSED

- Created files exist: pkg/interp/gvl.go, pkg/interp/gvl_test.go, pkg/testing/gvl_runner_test.go, pkg/sim/gvl_test.go, cmd/stc/sim_gvl_test.go, tests/twincat_dialect/gvl_test.st.
- Commits 87bdcdb, 6b7a5e4, 86caa84, 346ee77, b0296ad and dc3c6c1 are in git log.
