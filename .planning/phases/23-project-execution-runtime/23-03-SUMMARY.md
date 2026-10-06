---
phase: 23-project-execution-runtime
plan: 03
subsystem: interp, cmd/stc
tags: [runtime, persistent, retain, state-file, sim, serve, ethercat, free-running]
requires:
  - 23-01 Project, LoadProject, Tick, Tasks, loadProjectSpec
  - 23-02 Project.Run, RunOpts, WallClock, Interpreter.Warnings
  - Phase 22 Runtime Get/Set/ToJSON
  - Phase 24 ecat.LoadProject/CollectLinks/Resolve/NewNetwork, interp.NewIOBinder
provides:
  - Project.PersistPaths, SaveState, LoadState (state file version 1)
  - cmd/stc project_run.go shared runner (projectSetup, attachECat, runFree, status)
  - stc sim project mode (--project, --io, --persist, --persist-interval, --realtime, --duration)
  - stc serve (runProjectServe loop, serveExtensions hook)
affects: [23-04 ST301 gate, 28-04 OPC UA serve merge]
tech-stack:
  added: []
  patterns: [one project runner shared by sim and serve, atomic temp-file-and-rename state writes, flag-compatible command skeleton for a concurrent branch]
key-files:
  created:
    - pkg/interp/persist.go
    - pkg/interp/persist_test.go
    - cmd/stc/project_run.go
    - cmd/stc/sim_project_test.go
    - cmd/stc/serve_cmd.go
    - cmd/stc/serve_cmd_test.go
  modified:
    - pkg/interp/project.go
    - cmd/stc/sim_cmd.go
    - cmd/stc/project_mode_test.go
    - cmd/stc/main.go
decisions:
  - "State file stores TIME as decimal milliseconds with ns precision (json.Number), because ToJSON's {ms, iso} object is not accepted by Runtime.Set"
  - "LoadState only applies paths in PersistPaths; anything else is an 'unknown path' warning, so a stale or edited file cannot write non-persistent variables"
  - "Project mode is selected by a tsproj/plcproj, several .st files, or any of --project/--io/--persist/--realtime/--duration; --wave and --dt are usage errors there"
  - "stc serve uses the 28-04 flag names (--project slice, --cycle, --realtime, --run-for, --define); --duration is primary and --run-for its alias"
metrics:
  duration: ~45 min
  completed: 2026-10-06
  tasks: 3
  files: 10
---

# Phase 23 Plan 03: Persistence, sim Project Mode and serve Summary

PERSISTENT and RETAIN variables now survive restarts through a sorted, timestamp-free JSON state file. `stc sim` runs whole projects with their task schedule, deterministically or free-running, with the EtherCAT network and persistence attached. `stc serve` runs a project free-running until a signal or `--duration`.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | PERSISTENT/RETAIN state file load and save (RED) | ddb045d |
| 1 | PERSISTENT/RETAIN state file load and save (GREEN) | 8734893 |
| 2 | stc sim project mode with --io, --realtime and --persist | 80348ff |
| 3 | stc serve skeleton | 8fdc998 |

## API for 23-04 and the 28-04 merge

- `(*interp.Project).PersistPaths() []string` lists persistent paths, sorted. It covers GVL and PROGRAM blocks and FB instances reachable from them, including EXTENDS bases, nested FBs, struct fields and array elements.
- `SaveState(path) error` writes `{"version":1,"values":{...}}` to `path.tmp`, syncs it and renames it over `path`.
- `LoadState(path) ([]string, error)` treats a missing file as a first run. A malformed file or another version is an error.
- `cmd/stc/project_run.go` holds the shared runner.
  - `projectSetup(cmd, inputs, defines, projectSetupOpts{Sets, Cycle}, errOut)` loads the project, attaches `--io`, loads `--persist` and applies `--set`.
  - `(*projectRunner).runFree(ctx, duration, clock)` runs free-running with periodic saves and saves once more on stop.
  - `status(gets)` and `writeProjectStatus` print the result.
- `cmd/stc/serve_cmd.go` provides `runProjectServe(ctx, r, duration, realtime)` and the hook `serveExtensions(cmd, p *interp.Project) error`.

### Merging with 28-04 serve on main

main's `serve_cmd.go` defines its own `newServeCmd`, with a Runtime `scanLoop` and an OPC UA server. Both files define `newServeCmd`, so the merge must pick one body. Recommended union:

1. Keep main's OPC UA flags (`--opcua`, `--security`, `--cert`, `--key`, `--pki-dir`).
2. Add `addProjectRunFlags(cmd)` and `--duration` to them.
3. Replace `loadServeProject` and `scanLoop` with `projectSetup` and `runProjectServe`.
4. Build the OPC UA source from `r.P.Runtime()`.

Queued OPC UA writes need a per-tick call. Wrap `runFree`'s OnTick, or add an OnTick option to `runProjectServe`. The `newServeCmd()` line in `main.go` is identical on both sides.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TIME values would not round-trip**
- **Found during:** Task 1
- **Issue:** ToJSON writes TIME as `{"ms","iso"}`, which `Runtime.Set` rejects. It also drops sub-millisecond precision.
- **Fix:** the state file uses its own `stateJSON` encoder, which writes TIME as decimal milliseconds with nanosecond precision. Every other kind uses the ToJSON form.
- **Commit:** 8734893

**2. [Rule 2 - Missing] --dt is refused in project mode**
- **Found during:** Task 2
- **Issue:** the plan ignores `--dt` in project mode. Ignoring it silently would mislead users.
- **Fix:** project mode now rejects `--dt`, the same way it rejects `--wave`.
- **Commit:** 80348ff

**3. [Rule 3 - Blocking] LoadProject and NewIOBinder are called from project_run.go, not sim_cmd.go**
- **Found during:** Task 2
- **Issue:** the plan wants one shared setup for sim and serve.
- **Fix:** both commands call `projectSetup`. The key-link patterns match in `cmd/stc/project_run.go`.
- **Commit:** 80348ff

**4. [Rule 3 - Merge] serve flag set aligned with 28-04**
- **Found during:** Task 3
- **Issue:** main already has a 28-04 `serve_cmd.go`.
- **Fix:** this branch uses the same flag names: `--project` as a slice, `--cycle`, `--realtime`, `--run-for` as an alias of `--duration`, and `--define`.
  - `--cycle` overrides the cycle of a single task or of the default task. With several tasks it is an error.
  - `--opcua` and `--security` are left to 28-04.
- **Commit:** 8fdc998

### Behaviour changes

- `stc sim x.tsproj` now prints the project status JSON instead of the waveform `SimResult`. The three earlier tsproj sim tests were rewritten for the new output.
- A task PouCall naming a missing PROGRAM now fails the load, as decided in 23-01. Before, sim fell back to the first PROGRAM.
- `loadSimProject` was deleted.

## Verification

- This command passes:

  ```
  go test ./pkg/interp ./cmd/stc -run 'TestPersist|TestSimProject|TestSim|TestServe' -count=1
  ```

- The serve tests pass under `-race`.
- `go test ./... -count=1` passes.
- pkg/interp coverage is 98.5%, above the 95% gate.
- `stc sim "Demo solution.tsproj" --cycles 1000` reports 1000 runs of the 1 ms PlcTask and a sim time of 1s. The JSON output is byte-identical between the positional form and `--project`.
- With both Demo Device exports, `--io` reads `ECT.Dev1_SlaveCount = 10`. Without Device 2, the unresolved D2_I1 link exits non-zero with its diagnostic.
- Persistence works end to end:
  - Run 1 sets `p_cfg_Speed` to 42.5 and saves it, and run 2 restores it.
  - A RETAIN counter advances across runs.
  - Stopping serve by SIGINT or ctx cancel saves the state.

## Known Issues / Deferred

- The IEC TIME literal parser drops microseconds, for example in `T#1m2s3ms4us`. This was pre-existing and is out of scope. Numeric millisecond values keep full precision.
- `docs/CLI_REFERENCE.md` does not describe the new sim project flags or serve yet. main has uncommitted serve docs from 28-04, so this was left to the merge.
- `cmd/stc/main.go` was already not gofmt-clean before this plan. It was left as is to keep the merge diff to one line.

## Self-Check: PASSED

- All created files exist.
- Commits ddb045d, 8734893, 80348ff and 8fdc998 are on gsd/phase-23-runtime.
