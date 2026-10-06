# Phase 23: Project Execution Runtime - Context

**Gathered:** 2026-10-06 · **Status:** Ready for planning · **Mode:** Autonomous, speed-first (worktree ../stc-wt-23, branch gsd/phase-23-runtime, based on main with Phases 19-21 and 24 merged). Research folded into planning. EXECUTION STARTS ONLY AFTER PHASE 22 (symbol tree, Runtime Get/Set, initialisers, literal typing) IS MERGED INTO MAIN; plan against the Phase 22 API documented in /Users/jonb/Projects/stc-wt-22/.planning/phases/22-symbol-tree-value-semantics/*-SUMMARY.md.

<domain>
## Phase Boundary
A whole imported project runs on the host the way the PLC runs it: GVLs once, PROGRAMs per task at the configured cycle, retained state across restarts, wildcard I/O by declared type
Success criteria:
- `stc sim --project ST301.tsproj --cycles 1000` runs MAIN with all its user FBs, functions, methods and actions and advances simulated time by exactly 1 s at the 1 ms task cycle, identically on every run
- In free-running mode the scan is paced against wall-clock so 10 s of real time is approximately 10 000 cycles of a 1 ms task
- A `p_cfg_*` PERSISTENT value written in one run is restored from the state file on the next run
- An `AT %I*` INT reads back as -5 after its slot is set to 0xFFFB, and REAL, enum and struct-with-`AT %I*`-member bindings round-trip by declared type
Requirements: RUNT-03 (all GVLs instantiated once, PROGRAMs run per task with the configured cycle time; deterministic Tick plus a wall-clock-paced free-running mode), RUNT-04 (PERSISTENT/RETAIN load/save to a JSON state file), RUNT-07 (AT-bound variables read/write by declared type incl. sign extension, REAL, enums, structs with AT members; replaces address-width typing in scan.go), RUNT-09 (stc sim and stc serve run PROGRAMs that use user FBs, functions, methods, actions from the imported project; sim registers FBDecls/TypeDecls/EnumTypes/FuncDecls/TypeInits like the test runner does).
Not in scope: OPC UA (28), scenarios (27), device models (25/26).
</domain>

<decisions>
- New `pkg/interp/project.go` (or `pkg/runtime`): `LoadProject(model *twincat.Model or analyzer.AnalysisResult, cfg) (*Runtime, error)` that registers libraries then user files (GVLs two-pass first, TYPE/FB/FUNCTION decls, TypeInits), instantiates GVLs once, creates one ScanCycleEngine per task-program pair (tasks from the tsproj: name, cycle time, priority; a plcproj without tsproj gets one 10 ms task running MAIN), and exposes Phase 22's Runtime Get/Set/Snapshot plus `Tick()` (advance every task whose next due time ≤ now, in priority order, deterministic clock) and `Run(ctx)` free-running mode pacing with a monotonic wall clock (sleep until next due; never busy-wait; reports overruns as a counter).
- PERSISTENT/RETAIN: `--persist <file.json>` loads values by path at start (unknown paths warned, type-coerced via Set) and saves on stop and every N seconds; only variables declared PERSISTENT or RETAIN (VarBlock flags from Phase 19) are included.
- RUNT-07: IOBinder (Phase 24) and the explicit-address AT sync use the declared IEC type codec (BOOL/BYTE/WORD/DWORD/LWORD, signed ints with sign extension, REAL/LREAL bits, enums by base, struct with AT members member-wise, AMSADDR/NetId arrays) through one shared codec in pkg/interp.
- CLI: `stc sim <tsproj|plcproj|.st...> [--io Device*.xml ...] --cycles N | --realtime --duration 10s` runs the project (RUNT-09 proof: the imported ST301 runs N cycles without runtime errors and `--get ECT_Diag.Device_1_SlaveCount` shows the configured count); `stc serve` command skeleton (project load + free-running + --persist) so Phase 28-04 only adds the `--opcua` flag.
- Keep interp changes additive where possible; the interp gate is 95%.
</decisions>

<specifics>
Plans (4 max): 23-01 LoadProject + per-task engines + deterministic Tick (RUNT-03 half, RUNT-09); 23-02 declared-type AT codec shared with IOBinder (RUNT-07) + free-running Run (RUNT-03); 23-03 PERSISTENT state file (RUNT-04) + `stc sim <project>`/`stc serve` skeleton; 23-04 gate: env-gated STC_SILD_DIR run of imported ST301 for 1000 cycles with the EtherCAT network attached (--io Device*.xml), VALIDATION, docs, coverage gate.
</specifics>
