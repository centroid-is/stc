---
phase: 29-live-hmi-agent-integration
plan: 03
subsystem: mcp
tags: [mcp, simulation, opcua, ethercat, agents]
requires:
  - phase: 23-project-execution-runtime
    provides: "interp.LoadProject/Project.Tick, project loader, attachECat"
  - phase: 28-opc-ua-address-space
    provides: "opcua.Build/Publish/Server, bind.RuntimeSource, opcuatest.Take"
provides:
  - "pkg/projectload: importable Load and AttachECat (cmd/stc delegates to it)"
  - "stc-mcp --project/--io/--scenario/--opcua with one long-lived stepped simulation"
  - "MCP tools stc_sim_step, stc_sim_read, stc_sim_write, stc_opcua_browse"
affects: [29-01 scenario wiring, 29-04 docs, DEVX-02]
tech-stack:
  added: []
  patterns:
    - "Linked-input force: raw bits written into the master's input image before every Tick, so the IOBinder copies them like a terminal"
    - "Lazy session host: the simulation loads on the first sim tool call; load errors are IsError tool results"
key-files:
  created:
    - pkg/projectload/load.go
    - pkg/projectload/ecat.go
    - cmd/stc-mcp/sim.go
    - cmd/stc-mcp/sim_test.go
    - cmd/stc-mcp/tools_sim_test.go
    - cmd/stc-mcp/testdata/live/GVL_Live.st
    - cmd/stc-mcp/testdata/live/fbs.st
    - cmd/stc-mcp/testdata/live/main.st
    - cmd/stc-mcp/testdata/live/types.st
  modified:
    - cmd/stc/project_load.go
    - cmd/stc/project_run.go
    - cmd/stc/opcua_cmd_test.go
    - tests/runtime_project_test.go
    - cmd/stc-mcp/main.go
    - cmd/stc-mcp/tools.go
    - cmd/stc-mcp/tools_mcp_test.go
    - cmd/stc-mcp/main_test.go
    - cmd/stc-mcp/tools_test.go
    - cmd/stc-mcp/tools_coverage_test.go
key-decisions:
  - "Loader extracted to pkg/projectload so stc-mcp can import it; cmd/stc keeps thin wrappers and identical behavior"
  - "Without Phase 27's Plant, stc_sim_write forces TcLinkTo-bound inputs at the input-image level; other paths use Runtime.Set"
  - "--scenario is accepted but returns a clear error until the Phase 27 scenario package is merged and 29-01 wires it"
  - "stc_sim_step reports total session cycles; parallel calls serialize on the session mutex"
  - "Remote browse takes the full opcuatest.Take snapshot and cuts it at depth (read-only, 10 s deadline)"
requirements-completed: [DEVX-02]
duration: 55min
completed: 2026-10-06
---

# Phase 29 Plan 03: MCP live simulation tools Summary

**`stc-mcp --project <p> --io <xml> [--opcua addr]` hosts one stepped, deterministic simulation that agents drive through `stc_sim_step`, `stc_sim_read`, `stc_sim_write` and `stc_opcua_browse`. Linked inputs are forced in the EtherCAT input image, and the optional OPC UA server shares the runtime.**

## Performance

- **Duration:** about 55 min
- **Completed:** 2026-10-06
- **Tasks:** 3 of 3, plus a rebase repair and a loader refactor
- **Coverage:** cmd/stc-mcp at 91.3% of statements

## Accomplishments

- `pkg/projectload` holds the Phase 23 loader and EtherCAT attach. `AttachECat` also returns the network and resolved bindings.
- `simSession` holds the loaded project, an optional network, a `bind.RuntimeSource` and an optional OPC UA server. One mutex guards it.
- `Step(n)` accepts 1 to 1 000 000 cycles. Before each Tick it drains the OPC UA write queue and re-applies input forces.
- `Read` uses `Runtime.Get` and `ToJSON`. An unknown path gets its own error entry.
- `Write` uses `Runtime.Set` coercion, so CONSTANT and unknown paths are rejected. A TcLinkTo-bound input is also forced in the image.
- `stc_opcua_browse` walks `opcua.Space` from `ns=4;s=PLC1` without a network. Depth defaults to 2 and is capped at 10. With `endpoint` it browses a remote server read-only through `opcuatest.Take`.
- Every sim tool without `--project` returns IsError with "start stc-mcp with --project <path> [--io ...]". `callToolResult` now carries IsError.
- Protocol tests use the in-memory transport. They cover the tool list, write, step, read and browse, serialized parallel steps, and remote browse of the session's own server. An OPC UA client reads a value written through MCP.

## Task Commits

1. **Rebase repair** - `775764a` (fix)
2. **Loader extraction** - `8a6e9cb` (refactor)
3. **Task 1: simSession and flags** - `faea6db` (feat)
4. **Task 2: four tools** - `9e6e82b` (feat)
5. **Task 3: MCP protocol tests** - `41e3422` (test)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Test build broken after the rebase on Phase 23**
- **Found during:** rebase
- **Issue:** `cmd/stc/opcua_cmd_test.go` from 29-02 called `loadServeProject`, which the unified serve removed. `tests/` declared `stcBinary` twice, once from Phase 23 and once from 29-02.
- **Fix:** The test now uses `loadProjectAnalysis`. The Phase 23 duplicate helper is gone.
- **Commit:** 775764a

**2. [Rule 3 - Blocking] Phase 27 Plant routing and the 29-01 tests/opcua_live fixture do not exist yet**
- **Issue:** The plan reuses Phase 27 D-13 force routing and 29-01's fixture. Neither is on main.
- **Fix:** Bound inputs are forced in the input image, which a plain `Runtime.Set` would lose on the next scan. A test proves that. The plan adds its own fixture in `cmd/stc-mcp/testdata/live`, which links to `tests/ecat_fixtures/Demo Device 1.xml`.

**3. [Rule 2 - Missing functionality] IsError on tool results**
- **Issue:** `callToolResult` had no error flag.
- **Fix:** Added `IsError`, `textResult` and `errorResult`, and `toMCPResult` propagates the flag.

## Known Stubs

- `--scenario` in `cmd/stc-mcp/sim.go`: `newSimSession` rejects it with a clear error, and `scenario_failures` is always empty. 29-01 must wire Phase 27 `pkg/scenario`. That means BeforeTick before each Tick, AfterTick after it, and reporting failures in `stepResult`.
- `stc_sim_write` should route forced inputs through the Phase 27 Plant force once it is merged, replacing the image-level force.

## Threat Flags

None beyond the plan's register. Steps are capped at 1 000 000 and depth at 10. Writes go only through `Runtime.Set`. Remote browse is read-only with a 10 s deadline.

## Self-Check: PASSED

- All files in key-files exist, and commits 775764a, 8a6e9cb, faea6db, 9e6e82b and 41e3422 are in git log.
- `go test -race -cover ./cmd/stc-mcp/` passes at 91.3%, and `go test ./...` passes.
