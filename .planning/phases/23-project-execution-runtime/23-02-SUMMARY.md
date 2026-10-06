---
phase: 23-project-execution-runtime
plan: 02
subsystem: interp, cmd/stc
tags: [runtime, io, codec, AT, wildcard, free-running, wall-clock, auto-stub]
requires:
  - 23-01 Project, LoadProject, Tick, TaskStats.Overruns, loadProjectSpec
  - Phase 24 IOBinder codec (bitCursor, decode/encode)
  - Phase 22 storeAs
provides:
  - ioCodec (decodeBytes, encodeBytes, bitSize) shared by IOBinder and scan.go
  - IOBinding.Spec declared-type AT sync
  - Project shared IOTable, IOSlot, SetIOBytes, IOBytes, wildcard slot allocation
  - WallClock, NewWallClock, RunOpts, Project.Run
  - Interpreter.Warnings, auto-stubs for undeclared types (RUNT001/RUNT002)
affects: [23-03 sim/serve CLI, 23-04 ST301 gate, 25/26 device models, 28 OPC UA]
tech-stack:
  added: []
  patterns: [one declared-type codec for every process-image path, sleep-until-deadline pacing with realign on overrun]
key-files:
  created:
    - pkg/interp/iocodec.go
    - pkg/interp/iocodec_test.go
    - pkg/interp/project_io.go
    - pkg/interp/project_io_test.go
    - pkg/interp/project_run_test.go
    - pkg/interp/scan_at_test.go
    - pkg/interp/autostub.go
    - pkg/interp/autostub_test.go
  modified:
    - pkg/interp/iobind.go
    - pkg/interp/scan.go
    - pkg/interp/project.go
    - pkg/interp/interpreter.go
    - pkg/interp/fb_instance.go
    - pkg/interp/init_value.go
    - pkg/interp/interp_coverage2_test.go
    - cmd/stc/project_load.go
    - cmd/stc/project_load_test.go
    - cmd/stc/project_load_sild_test.go
decisions:
  - "IOBinder embeds ioCodec; every decoded scalar finishes through storeAs (T-23-04)"
  - "A typed AT binding the codec cannot copy (STRING) is left alone; address-width typing remains only for bindings without a declared type"
  - "Project owns one IOTable shared by all task engines; engines sync explicit PROGRAM AT vars, the Project syncs GVL AT vars, AT struct members and wildcards once per Tick"
  - "Wildcards are allocated per area after the highest explicit address, aligned to 2/4/8 bytes; Q and M slots are seeded from initial values"
  - "Run realigns to the next due tick after an overrun (no catch-up), so virtual time falls behind wall time under overload"
  - "Undeclared types run as zero-output auto-stubs and undeclared member reads yield zero, each with a deduplicated warning; the sim/serve loader downgrades SEMA037/SEMA024 to warnings"
metrics:
  duration: ~50 min
  completed: 2026-10-06
  tasks: 3 (+1 required deviation)
  files: 18
---

# Phase 23 Plan 02: Declared-Type I/O Codec and Free-Running Run Summary

Every AT-bound variable now reads and writes by its declared IEC type through one codec shared by the EtherCAT IOBinder and the explicit-address scan sync. Wildcard `%I*`/`%Q*` variables get auto-assigned slots, `Project.Run` paces ticks against a monotonic wall clock with overrun counting, and the imported ST301 loads and runs by auto-stubbing its undeclared `FB_TwoWayConveyor`.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Extract the shared declared-type codec from IOBinder | bad4763 |
| 2 | Declared-type AT sync in scan.go and wildcard slots | 2735e78 |
| 3 | Free-running Project.Run with monotonic pacing and overruns | bbc32be |
| - | Auto-stub undeclared types so ST301 loads (orchestrator-required) | 6207b5a |
| - | Codec local naming for the key-link patterns | bacecd5 |

## API for 23-03 and later

- `(*Project).Run(ctx, interp.RunOpts{Duration, Clock WallClock, OnTick func(sim time.Duration)}) error` returns nil after Duration of wall time, `ctx.Err()` on cancel, or the first Tick error.
- `interp.WallClock` is an interface with `Now() time.Duration` and `Sleep(ctx, d) error`. `interp.NewWallClock()` is the monotonic default.
- `TaskStats.Overruns` is now counted by Run.
- `(*Project).IOSlot(path) (iomap.IOAddress, size int, ok bool)` reports GVL AT vars, wildcard PROGRAM vars and AT struct members. Explicit PROGRAM AT vars are not listed.
- `(*Project).SetIOBytes(area byte, off int, b []byte) error` and `IOBytes(area byte, off, n int) ([]byte, error)` take area 'I', 'Q' or 'M'. Both are bounds-checked and never panic.
- `(*Interpreter).Warnings() []diag.Diagnostic` lists auto-stub warnings (code `RUNT001`) and undeclared-member warnings (code `RUNT002`). 23-03 should print them through the normal diagnostic output.
- `Project.SetIOBinder` re-runs wildcard allocation, so variables linked by TcLinkTo lose their auto slot.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Typed but unsupported AT bindings were clobbered**
- **Found during:** Task 2
- **Issue:** the plan kept the width path as a fallback. With that fallback, a `STRING AT %IB20` was overwritten by a BYTE integer on every scan.
- **Fix:** a binding with a declared type the codec cannot copy is left unchanged. The width path now runs only when no declared type is known.
- **Commit:** 2735e78

**2. [Rule 2 - Missing] Shared process image and initial values for Q and M**
- **Found during:** Task 2
- **Issue:** each engine had its own IOTable, so `Project.SetIOBytes` had no single image to write. A `%MW2 := 9` variable was also zeroed by its first input copy.
- **Fix:** the Project creates one IOTable and every engine uses it. It also syncs explicit GVL AT variables and AT struct members, which nothing synced before. Output and memory slots are seeded from the variables' initial values.
- **Commit:** 2735e78

**3. [Orchestrator-required] Undeclared types and members no longer abort sim/serve**
- **Found during:** post-plan ST301 run
- **Issue:** ST301's `SPB03.TcGVL` declares `FB_TwoWayConveyor` instances, but the type is never declared. `SVNCoreComponents` also uses an undeclared `ST_Batch`. `loadProjectSpec` failed on the checker's SEMA037 error. Bypassing the checker made the instance a DINT, and the first call stopped the scan with "undefined function".
- **Fix:**
  - An unknown type name now instantiates a zero-output auto-stub, following the Phase 14 pattern. Calls and method calls on it do nothing. Members the program writes read back, and every other member reads as zero.
  - Each undeclared type gets one `RUNT001` warning with its source position.
  - Reading a struct member that is not declared yields zero plus a `RUNT002` warning that names the path, for example `rs[1].missing`. This replaces the runtime error.
  - The sim/serve loader downgrades SEMA037 (undeclared type) and SEMA024 (no such member) to warnings. Other analysis errors still fail the load.
  - Two coverage tests that expected the old member error now assert the warning instead.
- **Files:** pkg/interp/autostub.go, interpreter.go, fb_instance.go, init_value.go, cmd/stc/project_load.go
- **Commit:** 6207b5a

## Verification

- `go test ./pkg/interp -run 'TestIOCodec|TestIOBind|TestScan|TestProjectWildcard|TestProjectRun|TestAutoStub' -race -count=1` passes.
- `go test ./... -count=1` passes.
- The real-clock Run test passed 8 repeated runs under -race.
- pkg/interp coverage is 98.8%, above the 95% gate.
- `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./cmd/stc -run TestLoadProjectSild` loads ST101 and ST301 and runs 1000 ticks of each without errors. ST301 logs two RUNT001 warnings, for `FB_TwoWayConveyor` and `ST_Batch`.
- Acceptance greps match. `type ioCodec struct` is in iocodec.go, there is no `func (b *IOBinder) decode(`, and the `Run` signature is in project.go.

## Known Issues / Deferred

- An auto-stub replaces a whole undeclared type. A non-FB alias from a missing library, such as an undeclared `T_MaxString`, therefore also becomes a stub until something assigns a plain value to it.
- Wildcard offsets are stable for a given source and binder, but they are not TwinCAT's real link addresses. Use IOSlot to find them.

## Known Stubs

None. The auto-stubs are runtime behaviour for undeclared types, not placeholder code.

## Threat Flags

None. SetIOBytes and IOBytes are the planned trust-boundary entry points (T-23-05) and are bounds-checked.

## TDD Gate Compliance

Each task's tests and implementation were committed together, with no separate RED commit. Task 1 is a pure move, so its tests passed from the start.

## Self-Check: PASSED

- All created files exist.
- Commits bad4763, 2735e78, bbc32be, 6207b5a and bacecd5 are in git log.
