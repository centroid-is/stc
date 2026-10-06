---
phase: 29-live-hmi-agent-integration
plan: 01
subsystem: opcua
tags: [opcua, serve, handshake, subscriptions, live, tf6100]

requires:
  - phase: 23
    provides: "interp.Project Run(ctx, RunOpts{OnTick}), stc serve unified loop with BeforeTick hooks, --io"
  - phase: 28-04
    provides: "bind.RuntimeSource (Write queue, ApplyPending), bind.Root, opcua Build/Publish"
provides:
  - "tests/opcua_live: live ST fixture (FB_Conveyor p_cmd handshake, FB_Sensor with %I* input, mid-scan write probe)"
  - "Proof of OPCUA-07: p_cmd_* set-TRUE / FB-clears consumed within 10 cycles, in-process and through stc serve"
  - "Proof of OPCUA-08: monitored items sample the live source through the read handlers; no push needed"
affects: [27, 29-02, 29-03]

tech-stack:
  added: []
  patterns:
    - "Free-running in-process OPC UA tests wire Project.Run OnTick -> RuntimeSource.ApplyPending, the same composition stc serve uses"
    - "Handshake latency is measured exactly: the FB stamps GVL_Live.nCycle into p_stat_StartCycle when it consumes the command"
    - "Mid-scan write detection: MAIN samples a probe at the start and end of its body and counts mismatches"

key-files:
  created:
    - tests/opcua_live/live.st
    - tests/opcua_live/GVL_Live.st
    - pkg/opcua/bind/live_test.go
    - pkg/opcua/subscribe_test.go
    - cmd/stc/serve_live_test.go
  modified:
    - cmd/stc/serve_cmd.go
    - .planning/phases/29-live-hmi-agent-integration/deferred-items.md

key-decisions:
  - "No pkg/opcua/subscribe.go: awcullen's DataChangeMonitoredItem.Poll calls srv.readValue, which runs our read handler, so samples come from the live NodeSource"
  - "No new exported serve-loop helper: Phase 23 already made serve a Project.Run OnTick hook calling ApplyPending; the in-process test uses the same two calls"
  - "Scenario wiring landed after the Phase 27 merge: scenario.Live drives serve and stc-mcp scenarios"

requirements-completed: [OPCUA-07, OPCUA-08]

duration: 40min
completed: 2026-10-06
---

# Phase 29 Plan 01: Live OPC UA Writes and Subscriptions Summary

**The p_cmd_* set-TRUE / FB-clears handshake and OPC UA subscriptions are proven against the free-running scan, both in-process and through `stc serve`. Scenario wiring waits for Phase 27.**

## Performance

- Duration: about 40 minutes
- Completed: 2026-10-06
- Tasks: 2 of 3 complete, Task 3 partly complete (the end-to-end handshake and subscription are done, the scenario part is pending)
- Files: 5 created, 2 modified

## Accomplishments

- `tests/opcua_live` holds FB_Conveyor, whose HMI struct carries p_cmd_Start and p_cmd_Stop as writable members and p_stat_Running, p_stat_State (enum, rdy=2), p_stat_Starts and p_stat_StartCycle as read-only members. It also holds FB_Sensor with `xIn AT %I* : BOOL` copied to HMI.p_stat_xRaw, and a probe in MAIN that counts writes landing mid-scan.
- `TestLiveHandshake` runs the project free on a 10 ms cycle with ApplyPending between Ticks. A client writes p_cmd_Start TRUE. The FB consumes it within 10 cycles, counts exactly one start and clears the command. All of this is observed through OPC UA reads. p_cmd_Stop reverses it.
- `TestLiveWritesBetweenScans` toggles the probe 61 times during the scan. The mismatch count stays 0, and the probe is seen TRUE, so the writes were consumed.
- `TestLiveStatusNotWritable` checks that p_stat_* and Access '1' globals return BadNotWritable and keep their values.
- `TestSubscriptionSamplesSource` uses a 100 ms publishing interval, 50 ms sampling and a queue of 10. A BOOL item and a structured HMI item each get the initial value. Each gets exactly one data change per source change, and none during 500 ms windows while the value is constant. An unknown node returns BadNodeIdUnknown.
- `TestServeLiveHandshake` serves the fixture through `stc serve --format json`. A subscribed client receives p_stat_Running as [FALSE, TRUE] after its p_cmd_Start write. The FB clears the command within 10 cycles, and the mismatch probe stays 0.

## Task Commits

1. **Task 1: live fixture and in-process handshake** - `2be139e` (test)
2. **Task 2: subscriptions sample the live source** - `a85cbe8` (test)
3. **Task 3 (partial): end-to-end handshake and subscription through stc serve, help text** - `876d2cd` (test)

## Verification

| Command | Result |
|---------|--------|
| `go test -race -count=3 -run 'TestLive' ./pkg/opcua/bind/` | ok |
| `go test -race -count=2 -run 'TestSubscri' ./pkg/opcua/` | ok |
| `go test -race -count=2 -run 'TestServeLive' ./cmd/stc/` | ok |
| `go test -race ./pkg/opcua/... ./cmd/stc/` | ok |
| `go test ./tests/ -run Doc` | ok |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The probe moved from a separate PROGRAM into MAIN**
- **Found during:** Task 1
- **Issue:** The interpreter cannot call one PROGRAM from another, and the scan stopped with `undefined function: PRG_CONSISTENCY`.
- **Fix:** MAIN samples GVL_Live.xProbe at the start and end of its own body, with work and both FB calls in between.
- **Files modified:** tests/opcua_live/live.st
- **Commit:** 2be139e

**2. [Rule 3 - Blocking] The GVL lives in its own file**
- **Found during:** Task 1
- **Issue:** The GVL name comes from the file name, so `GVL_Live` needs `GVL_Live.st`.
- **Fix:** Added tests/opcua_live/GVL_Live.st next to live.st.
- **Commit:** 2be139e

**3. [Rule 2 - Correctness] Exact cycle bound**
- **Issue:** Polling reads cannot bound "within 10 cycles" precisely.
- **Fix:** The FB stamps the cycle in which it consumes the command into p_stat_StartCycle. The tests compare it with nCycle read just before the write.
- **Commit:** 2be139e

### Plan items not created

- `pkg/opcua/subscribe.go` was not created because the subscription test passed against the existing read handlers (D-04).
- No exported serve-loop helper was added. Serve already calls `Project.Run` with an OnTick hook that runs ApplyPending (Phase 23-04), and the in-process test uses the same composition.
- The tests are proof tests of behaviour that already existed after Phases 23 and 28-04. A RED commit that fails was therefore not possible. Each task has one `test(29-01)` commit.

## Scenario wiring (completed after the Phase 27 merge)

- `pkg/scenario/stepper.go`: `Executor.Start` returns a `Live` with BeforeTick, AfterTick and Finish. `Executor.Run` now drives the same `Live`, so the per-tick logic exists once (D-06).
- `stc serve --scenario`: validated before the OPC UA server starts. Between Ticks the expects are evaluated, pending OPC UA writes applied, then the next steps fire. The report prints on stop and failures are warnings. `TestServeScenario` subscribes to `GVL_Live.sensor.HMI.p_stat_xRaw` and sees [FALSE, TRUE] from `tests/opcua_live/toggle.toml` (cycle 100, 1 s at 10 ms, for subscription margin).
- `stc-mcp --scenario` (D-14): the session is a `scenario.Plant`; steps fire in `stc_sim_step`, failed expects return in `scenario_failures`. `stc_sim_write` forces linked inputs through the Plant network force instead of an image-level force map.
- Scenario diagnostics now carry the scenario file name.

## Known Stubs

None.

## Threat Flags

None. T-29-01 is tested by TestLiveWritesBetweenScans and TestLiveStatusNotWritable. T-29-02 needed no push mechanism and keeps the library limits. T-29-03 is covered by TestServeScenarioInvalid and TestSimSessionScenarioInvalid: a scenario that does not validate stops before anything runs.

## Self-Check: PASSED
