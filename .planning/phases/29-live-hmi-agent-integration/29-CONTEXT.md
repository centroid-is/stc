# Phase 29: Live HMI & Agent Integration - Context

**Gathered:** 2026-10-06
**Status:** Ready for planning
**Mode:** Autonomous. Context written by the planner from orchestrator rulings. Worktree ../stc-wt-29, branch gsd/phase-29-integration. Execution starts only after Phases 23, 26, 27 and 28-04 are merged into main; every plan rebases first.

<domain>
## Phase Boundary

Make the running simulation usable live: OPC UA writes and subscriptions against the free-running scan of `stc serve`, a CLI to capture a real TF6100 browse snapshot plus a CI diff against it, a Flutter HMI acceptance procedure with an automated stand-in, four MCP tools for agents, and the docs for the whole v1.2 workflow.

Builds on (extend, never duplicate):
- Phase 23: `LoadProject`/Project with deterministic `Tick` and free-running `Run(ctx)`, PERSISTENT state file, shared CLI project loader (`cmd/stc/project_load.go`), `stc serve` skeleton.
- Phase 27: `pkg/scenario` Plant and executor (D-07 the executor owns the loop, D-12 inputs forced at the process image), `stc sim --io --scenario`.
- Phase 28 core: `pkg/opcua` Server/Build/Publish, NodeSource, MapSource, `opcuatest.Take/BrowseSnapshot` (nodes plus namespace-4 DataType definitions), golden `tests/opcua_golden/st301_shape.json`.
- Plan 28-04: `pkg/opcua/bind` (symtree to SymbolNode, `RuntimeSource` with snapshot reads and a pending write queue drained by `ApplyPending()` between scans), `stc serve --project --opcua --security --cycle --run-for --format json`, ST fixture `tests/opcua_golden/st301_shape/`.

Not in scope: ADS, RPC methods, AnalogItemType, username/password and trust lists, a Flutter build inside CI.
</domain>

<decisions>
## Implementation Decisions

### Live writes (OPCUA-07)
- **D-01** OPC UA writes go through `NodeSource.Write` of `bind.RuntimeSource`. They are queued and applied between scans by `ApplyPending()` under the runtime lock, coerced by Phase 22 `Runtime.Set`. A write is never applied in the middle of a Tick.
- **D-02** The `p_cmd_*` handshake is proven against a running Runtime: a client writes `p_cmd_Start := TRUE`, a later read returns TRUE or the FB has already acted, and within a bounded number of cycles the FB clears it back to FALSE and its effect is visible (for example `p_stat_Running`). The test must observe the cleared value through OPC UA, and must prove that the write was consumed by the FB, not lost.
- **D-03** The handshake is proven twice: in-process with a free-running Project (`Run(ctx)`) and end to end through the `stc serve` binary.

### Subscriptions (OPCUA-08)
- **D-04** Monitored items sample the live scan snapshot. awcullen's poll-based sampling through the read handler is acceptable. The executor first proves it with a test. If the library caches values and the read handler is not called per sample, add a post-scan push in pkg/opcua that writes changed values into the variable nodes, called from the serve loop after each Tick.
- **D-05** A test proves that a client subscription with a 100 ms publishing interval receives a data change for a changed value within a generous deadline (5 s), and receives no notification while the value is unchanged.
- **D-06** `stc serve` gains `--io <Device*.xml...>` (reusing the Phase 23/27 loader if 23 did not add it) and `--scenario <file.toml>`. In free-running mode the scenario steps fire as Tick counts elapse, using the Phase 27 executor's per-tick logic, not a second implementation. A subscription on a sensor `p_stat_*` delivers a data change when a scenario toggles the input.

### TF6100 fidelity diff (OPCUA-09)
- **D-07** New command `stc opcua snapshot <endpoint> --out file.json` with `--root` (default `ns=4;s=PLC1`), `--security none|basic256sha256`, `--cert/--key`, `--timeout`, `--format json`. It uses `opcuatest.Take` so the output schema equals the golden schema.
- **D-08** The capture targets a real TF6100. The user runs it on the plant network later. If the real root node differs from `ns=4;s=PLC1`, the user passes `--root`.
- **D-09** The diff compares node ids, node class, data type, value rank, array dimensions, access levels and struct definitions. DataType node ids differ between stc (`DT.<Name>`) and TF6100, so the diff normalizes DataType references to their BrowseName before comparing.
- **D-10** Test `tests/opcua_tf6100_diff_test.go` skips when `tests/opcua_golden/st301_real.json` (or the path in `STC_TF6100_SNAPSHOT`) is absent. When the file exists, a subset diff always runs. Every node of the emulated st301_shape fixture that also exists in the real snapshot must match, and every fixture node missing from the real snapshot fails. When `STC_SILD_DIR` also points at the ST301 sources, a full diff serves the real ST301 project and requires equality both ways. Any divergence fails with a readable list of at most 50 differences.
- **D-11** Whether `st301_real.json` is committed is the user's call, because it contains plant identifiers. The docs say so and describe the env var alternative.

### HMI acceptance (OPCUA-10)
- **D-12** Manual procedure in docs/OPCUA.md, section "Connect the Flutter HMI". It covers the endpoint `opc.tcp://<host>:4840`, SecurityPolicy None with Anonymous, namespace index 4, example node ids, `ns=0;i=2259`, enum display like `rdy(2)`, and the tfc "Server Config" page steps.
- **D-13** Automated stand-in: a committed `tests/opcua_golden/hmi_keymappings.json` in the current tfc schema (`{"nodes": {key: {"opcua_node": {"namespace", "identifier", ...}}}}`). Every entry must exist in the st301_shape fixture, including whole HMI structs, single `p_stat_*` members, the sensor `p_stat_xRaw` and `Server.State` at ns 0 identifier 2259. The test starts `stc serve` on the fixture, reads every node with the awcullen client and requires Good status. It decodes structs by field name from served definitions and formats enums as `name(value)`. `STC_HMI_KEYMAPPINGS` optionally points at the real `hmi/keymappings.json` for a local run against ST301.

### MCP tools (DEVX-02)
- **D-14** `stc-mcp` gains flags `--project`, `--io` (repeatable or glob), `--scenario` and `--opcua <addr>`. They start one long-lived simulation per MCP server process. Without `--project`, the sim tools return a clear error result, not a crash.
- **D-15** Tools:
  - `stc_sim_step {cycles}` runs N deterministic Ticks. It applies pending writes before each Tick and fires scenario steps, then returns the cycle count and sim time.
  - `stc_sim_read {paths[]}` returns values as JSON using Phase 22 ToJSON forms.
  - `stc_sim_write {path, value}` applies a value through Runtime.Set or a forced input, using the same routing as Phase 27 D-13. It is visible on the next read.
  - `stc_opcua_browse {node?, depth?, endpoint?}` browses the session's address space from `ns=4;s=PLC1`, or browses a remote endpoint when one is given.
- **D-16** The session is stepped, not free-running, so agent runs stay deterministic. When `--opcua` is set, the OPC UA server shares the session runtime through `bind.RuntimeSource`, and `stc_sim_step` drains its write queue.
- **D-17** Tool descriptions stay under 100 tokens (MCP-07). Tests cover handlers, the MCP in-memory transport and every error path.

### Docs (DEVX-03)
- **D-18** New `docs/TWINCAT_IMPORT.md`. Extend `docs/ETHERCAT_SIMULATION.md` (scenarios, serve with --io) and `docs/OPCUA.md` (writes, subscriptions, snapshot capture procedure, HMI section, MCP tools). Also update `docs/CLI_REFERENCE.md` and the README docs index if it exists.
- **D-19** Fix these stale claims:
  - `stdlib/vendor/beckhoff/ethercat_io.md` says "stc does not model specific terminal hardware". It now has EtherCAT topology, link binding and device models.
  - `docs/TESTING_GUIDE.md` covers I/O mocking only by explicit address. Add the project mode, `--io`, scenarios, the SIM_* built-ins and wildcard `%I*` binding.
  - `docs/ST_LANGUAGE_SUPPORT.md` Known Limitations and the pragma table must reflect Phases 19-22. Attributes now drive OPC UA exposure, and TwinCAT declaration syntax is supported.
  - Every other claim the executor finds contradicted by the merged code.
- **D-20** A docs test checks that every `stc` subcommand and flag named in the new docs exists in `--help` output. Examples must come from real command output.

### Claude's Discretion
- File layout inside cmd/stc (opcua_cmd.go), cmd/stc-mcp (sim.go) and pkg/opcua (subscribe.go only if D-04 needs it).
- The cycle bound for the handshake. Default: at most 10 cycles at 10 ms.
- Diff output format.
- Coverage: the last plan runs the repo coverage gate (`.testcoverage.yml`: total 85, interp/symtree 95). pkg/opcua and cmd/stc-mcp target at least 85%.
</decisions>

<code_context>
## Existing Code Insights

- `cmd/stc-mcp/tools.go`: arg structs with `jsonschema` tags, `desc*` constants, `handle*` returning `*callToolResult`, `wrap*` adapters, `registerTools` with `mcp.AddTool`, `allToolDefinitions()` for metadata tests. `main.go` currently takes no flags.
- `pkg/opcua/opcuatest/snapshot.go`: `Take(ctx, client, root) (*Snapshot, error)` and `BrowseSnapshot`. Snapshot has Root, Nodes (NodeClass, BrowseName, DataType, ValueRank, ArrayDimensions, AccessLevel, Description) and DataTypes (fields and enum values).
- `pkg/opcua/testutil_test.go` has helpers (freeAddr, dial, dialAnon, readValue, browseForward). They are unexported, so tests outside pkg/opcua need their own small helpers.
- Env-gated proprietary inputs: `STC_SILD_DIR`, `STC_PROBES_DIR`. They are never committed, and tests log counts only.
- Coverage CI: `.github/workflows/coverage.yml` and `.testcoverage.yml`.
</code_context>

<specifics>
## Specific Ideas

- HMI node ids: `GVL_BatchLines.Drives_Line1[1].HMI` (ExtensionObject), `GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error`, `sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw`, `ns=0;i=2259`. Case is significant on the wire.
- ST301 PLC task cycle is 10 ms. The HMI collect interval is 5 s, and the older schema polls every 100 ms.
</specifics>

<deferred>
## Deferred Ideas

- Building and driving the Flutter app in CI.
- Username/password auth, trust lists, RPC methods, ADS → v2.
</deferred>
