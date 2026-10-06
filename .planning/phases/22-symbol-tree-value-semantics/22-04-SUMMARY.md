---
phase: 22-symbol-tree-value-semantics
plan: 04
subsystem: interp
tags: [interpreter, initialisers, array-bounds, gvl, struct-defaults, runt-06]

requires:
  - phase: 22-01
    provides: "storeAs(dst, v) typed-store choke point"
provides:
  - "evalInit: type-directed ArrayInit (N(v) repetition), StructInit and scalar initialisers"
  - "instantiateVar: one instantiation path for program, GVL, FB (EXTENDS chain) and TEST_CASE variables"
  - "Constant-expression array bounds via ast.ConstIntValue + constLookup (G.C, C*2, scope constants)"
  - "Interpreter.InitErrors(): bound and initialiser failures, never silently dropped"
  - "Interpreter.TypeInits: TYPE-level defaults (alias/enum/struct TYPE := ...)"
  - "Value.ArrayLow and Value.Fields metadata (declared case and order)"
  - "RegisterGVLs: two-pass GVL registration (CONSTANT blocks first)"
affects: [22-05, symtree, runtime-get-set]

tech-stack:
  added: []
  patterns:
    - "typeCtx{resolve, consts, interp, env} drives zero-value construction"
    - "Every variable instantiation goes through instantiateVar -> zeroOf -> evalInit -> storeAs"

key-files:
  created:
    - pkg/interp/init_value.go
    - pkg/interp/init_value_test.go
    - tests/type_system/init_test.st
  modified:
    - pkg/interp/value.go
    - pkg/interp/interpreter.go
    - pkg/interp/fb_instance.go
    - pkg/interp/scan.go
    - pkg/interp/gvl.go
    - pkg/interp/gvl_test.go
    - pkg/interp/call_args.go
    - pkg/testing/runner.go

key-decisions:
  - "Bound failures keep a non-panicking array (1 slot or capped at 10000) and record an InitError; negative lower bounds are reported and treated as 0"
  - "An ArrayInit's element count (repetitions included) is checked against the array length before expansion (T-22-08)"
  - "Function/method locals use constant bounds and evalInit but do not record InitErrors (they would repeat on every call)"
  - "FB variable initialisers (fb : FB_X := (a := 1)) set user-FB members/inputs and stdlib FB inputs via SetInput"
  - "TYPE-level defaults live in a new Interpreter.TypeInits map because TypeDecls only holds TypeSpecs"

patterns-established:
  - "InitErrors is the place for instantiation failures; callers (Runtime in 22-05) surface them"

requirements-completed: []

duration: 13min
completed: 2026-10-06
---

# Phase 22 Plan 04: Initialisers and constant array bounds Summary

**Array, struct and struct-array initialisers now apply at instantiation over constant-expression bounds such as `ARRAY[1..EcDiagParam.MAX_EC_SLAVES]`, through one shared path for program, GVL, FB and TEST_CASE variables, with two-pass GVL registration and reported (not swallowed) failures.**

## Performance

- **Duration:** about 13 min
- **Started:** 2026-10-06T07:00:00Z
- **Completed:** 2026-10-06T07:13:01Z
- **Tasks:** 3
- **Files modified:** 11

## Accomplishments
- `zeroFromType` with a `typeCtx` sizes arrays from `ast.ConstIntValue` over GVL (`G.C*2`, `G.C - 1`) and scope constants. `Value.ArrayLow` and `Value.Fields` carry declared bounds and member order.
- Struct member defaults and TYPE-level defaults (`TYPE T_Speed : INT := 50`, enum `:= Run`) apply wherever the type is instantiated.
- `evalInit` handles `[1, 2, 3]`, `[3(0), 7]`, `N()`, nested `[[1,2],[3,4]]`, struct-arrays with omitted fields, and scalars through `storeAs`.
- `instantiateVar` replaces the duplicated loops in `initVarDecl`, `newUserFBInstanceDepth` and the test runner. Every FB instance, EXTENDS bases included, gets its own cloned aggregates.
- `RegisterGVLs` resolves `ECT_Diag` before `EcDiagParam` correctly. `RegisterGVL`, `SetGlobals` and the runner all go through it.
- `tests/type_system/init_test.st` adds 5 ST cases. The ST suite has 246 tests, all passing. pkg/interp coverage is 98.2% against a 95% gate.

## Task Commits

1. **Task 1: constant bounds, metadata, struct defaults** - `ccf5951` (test, RED), `12df01b` (feat)
2. **Task 2: evalInit and shared instantiateVar** - `fe12bc0` (test, RED), `3df1c79` (feat)
3. **Task 3: two-pass GVL registration and ST suite** - `311f936` (test, RED), `1594796` (feat), `204fed7` (test type fix)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The test runner had its own instantiation loop**
- **Found during:** Task 3
- **Issue:** `initializeTestEnv` duplicated variable creation and swallowed initialiser errors. Without changing it, `init_test.st` could not pass.
- **Fix:** It now calls the new exported `Interpreter.InstantiateVar`. The dead `typeNameFromSpec` helper was removed. The runner also passes TYPE defaults to the interpreter through `TypeInits`.
- **Files modified:** pkg/testing/runner.go, pkg/interp/init_value.go
- **Commit:** 1594796

**2. [Rule 2 - Missing functionality] TYPE defaults had no storage**
- **Issue:** `Interpreter.TypeDecls` maps names to TypeSpecs only, so `TYPE T : INT := 50` lost its default.
- **Fix:** Added an `Interpreter.TypeInits` map.
- **Commit:** 12df01b

**3. [Rule 2] FB variable initialisers were ignored**
- **Fix:** `initFB` now applies `fb : FB_X := (a := 1)` to user FBs and `t : TON := (PT := T#1S)` to stdlib FBs. Unknown members are reported.
- **Commit:** 3df1c79

**4. [Rule 2] Function locals ignored aggregate initialisers**
- **Fix:** `initialValue` now uses constant bounds and `evalInit`. It keeps the old silent fallback so InitErrors does not grow on every call.
- **Commit:** 3df1c79

**5. Unreachable-from-source branch**
- The parser only builds `N(v)` with a literal N, so `[K(1)]` is a call. The bad repetition count case is tested with a hand-built AST.

### TDD gate note
Each task has a test commit followed by a feat commit.

## Notes for 22-05
- Call `RegisterGVLs` with library GVLs first, then project GVLs. Set `TypeInits` beside `TypeDecls` when registering files.
- Surface `Interpreter.InitErrors()` from Runtime after initialisation.
- RUNT-06 is not marked complete here. 22-05 marks it once the Get check at cycle 0 passes.
- Only the first array dimension is modelled. Multi-dimensional initialisers are unchanged.

## Self-Check: PASSED
- Files exist: pkg/interp/init_value.go, pkg/interp/init_value_test.go, tests/type_system/init_test.st
- Commits exist: ccf5951, 12df01b, fe12bc0, 3df1c79, 311f936, 1594796, 204fed7
- `go test ./...` passes, `go run ./cmd/stc test tests/` reports 246 passed
