# Phase 27: Plant Scenarios & Simulation CLI - Context

**Gathered:** 2026-10-06 · **Status:** Ready for planning · **Mode:** Autonomous, written by the planner from orchestrator rulings (no discuss phase). Worktree ../stc-wt-27, branch gsd/phase-27-scenarios, based on the Phase 25 branch (which includes Phase 24). EXECUTION STARTS ONLY AFTER PHASES 23 AND 26 ARE MERGED INTO MAIN; every plan rebases first and re-checks the Phase 23/26 API listed below.

<domain>
## Phase Boundary

Engineers and CI drive the simulated plant through deterministic scripted scenarios, from a TOML file on the `stc sim` CLI or from ST unit tests under `stc test`.

Success criteria (ROADMAP):
1. `stc sim --project ST301.tsproj --io "Device*.xml" --scenario jam.toml --cycles N --format json` runs the project against the simulator and reports outputs and diagnostics, with identical output on repeated runs
2. A scenario can set an input by variable path or link path, set an analog value, trip an EL9222 channel, pull a slave (not present / link error), raise an ATV320 fault with an LFT code and ramp a value over time, each at a chosen scan
3. A `*_test.st` using `SET`, `GET`, `SIM_SET_LINK`, `SIM_TRIP`, `SIM_SLAVE_STATE` and `RUN_CYCLES` passes under `stc test` with the project and I/O config loaded, e.g. a removed slave shows up in `ECT_Diag` after N cycles

Requirements: ECAT-09 (scenario TOML + ST built-ins for every stimulus kind, deterministic against the scan clock), ECAT-10 (`stc sim --project --io --scenario --cycles` reports outputs and diagnostics in text and JSON), DEVX-01 (ST test built-ins in `*_test.st` when a project and I/O config are loaded).

Not in scope: OPC UA (28/29), MCP sim tools (29), new device models (25/26 own them), wall-clock/realtime scenario timing (scenarios run on the deterministic Tick path only).
</domain>

<decisions>
## Implementation Decisions

### Scenario file (TOML)
- **D-01** Format is TOML via the existing `github.com/BurntSushi/toml` dependency. Unknown keys are errors (use `MetaData.Undecoded`). Optional header table `[scenario]` with `name`, `description`, `cycles`. Steps are `[[step]]` array entries.
- **D-02** Each step has exactly one trigger: `at = "<duration>"` (Go duration string, sim clock) or `cycle = N` (N completed Ticks). Neither or both is an error.
- **D-03** Each step has at most one action and optionally one `expect`. An expect-only step is allowed. Action keys (inline tables):
  - `set = {path, value}`: variable path (Phase 22 grammar) or, if the path contains `^`, a TcLinkTo link path
  - `link = {path, value}`: link path only (`TIID^...`)
  - `analog = {slave, channel, ma | volts | raw}`: exactly one of the three units
  - `trip = {slave, channel}`: EL9222 channel trip
  - `slave_state = {slave, state}`: state is a name (`init`, `preop`, `safeop`, `op`, `not_present`, `link_error`, `ok`) or an integer
  - `drive_fault = {slave, lft}`: ATV320 InjectFault(lft); `lft = 0` calls ClearFault
  - `ramp = {path, from, to, over}` or `ramp = {slave, channel, unit = "mA"|"V", from, to, over}`: linear ramp over sim time
  - `serial_peer = {slave, script}`: script is `baader`, `loopback` or `none`
- **D-04** `expect = {path, value, within = N, tol = x}`: path is a variable path or link path. `within` (default 0) is the cycle tolerance: the expect passes if the value matches after any of the Ticks k0..k0+within, where k0 is the Tick the step fires before. `tol` (default 1e-6) applies to REAL/LREAL comparisons; integers and BOOL compare exactly; enums compare by name, case-insensitive.
- **D-05** Value types: TOML bool, integer, float and string. Strings for variable paths go to Phase 22 `Runtime.Set` unchanged (IEC literals, enum names, `T#5s`). Strings for link paths accept IEC integer literals only (`16#FF`, `2#1010`, `8#17`, decimal).
- **D-06** Slave names match `Slave.Name` exactly (`ST301.A1.02 (EL9222-5500)`), then case-insensitively, then as the unique prefix before ` (`. An ambiguous or unknown name is an error listing up to 5 candidates. A name found on several masters is an error naming the masters.

### Timing and determinism
- **D-07** The executor owns the loop. Before each `Project.Tick`, it fires every due step in (due Tick, file order) order. A `cycle = N` step is due when the completed Tick count k equals N; `at = d` is due at the first k where k*BaseTick >= d. After each Tick it evaluates pending expects. No wall-clock reads anywhere in pkg/scenario (`grep -c "time.Now" == 0`).
- **D-08** Ramps interpolate `from + (to-from) * min(1, (clock - t0) / over)` with float64 math, applied before every Tick from the firing Tick through the Tick where the fraction reaches 1, then removed. Integer targets round half away from zero. A new ramp or set on the same target cancels the running ramp.
- **D-09** Run length: CLI `--cycles` overrides `[scenario] cycles`; if neither is given, run to the last due Tick plus the largest `within` plus 1. Expects still pending at the end fail with "run ended at cycle N". Steps due after the end produce warning SCN010.
- **D-10** Diagnostics codes: SCN001 TOML syntax (with line), SCN002 unknown key, SCN003 trigger error, SCN004 action count/shape error, SCN005 bad duration or out-of-range number, SCN006 unknown or wrong-model slave / no network loaded, SCN007 unknown or unsettable path, SCN008 action failed at run time, SCN009 expect failed, SCN010 step never fired. Every step is validated against the target before the first Tick; any error stops the run before it starts.
- **D-11** Limits: scenario file at most 1 MiB, at most 10 000 steps, `over` in (0, 24h], `within` <= 1 000 000, channel 1..64.

### Plant target (input routing)
- **D-12** Scenario inputs are forced at the process image, not written to variables, because the IOBinder copies input slots into linked variables every scan. New `ecat.Network` force overrides (`ForceInput(path, bits)`, `ReleaseInput(path)`, `ReleaseAll()`) are applied at the end of `Network.Step`, after device models and pseudo-inputs, and persist until released. Forcing an output-direction slot is an error.
- **D-13** `set` on a variable path: if the path is a TcLinkTo-bound input leaf (IOBinder binding, case-insensitive), the value is encoded by the slot's data type (BOOL/BIT, two's-complement integers truncated to BitLen, REAL/LREAL IEEE bits) and forced on that slot. A variable with an explicit `AT %I` address and no TcLinkTo is an SCN007 error (its value would be overwritten by the AT sync). Everything else goes to `Runtime.Set`.
- **D-14** `slave_state` presets: `not_present` sets link state `ecat.LinkNotPresent`, InfoData.State 0x0011 (Init + error bit) and WcState bad; `link_error` sets `ecat.LinkWithoutComm`, 0x0011 and WcState bad; `init`/`preop`/`safeop`/`op` set the state nibble 1/2/4/8 only; `ok` restores OP, link 0 and WcState good for that slave; an integer sets InfoData.State verbatim. These values are what FB_EcDeviceDiag decodes (state nibble, 16#10 error bit, linkState <> 0).
- **D-15** Analog uses the Phase 25 model API (`SetCurrent`, `SetVoltage`, `SetRaw`); trip uses `EL9222.Trip`; drive faults use the Phase 26 `ATV320.InjectFault/ClearFault`; serial peers use `EL6001.SetPeer(devices.BaaderPeer())`, `devices.LoopbackPeer` or nil. A slave whose model does not have the API is SCN006 naming the actual model.

### CLI and ST built-ins
- **D-16** `stc sim <project> --io Device*.xml --scenario x.toml --cycles N [--get path...] [--format json]` extends the Phase 23 project mode; no second loader. `--io` values containing glob metacharacters are expanded with `filepath.Glob` (quoted patterns work); no match is an error. JSON adds `scenario` (steps, assertions, passed), `outputs` (every TcLinkTo output leaf, path to value), `ethercat` (slaves not healthy at the end) and `diagnostics` to the Phase 23 output. Exit code 1 when any assertion fails or any error diagnostic exists. Two runs produce byte-identical JSON.
- **D-17** ST built-ins: `SET(path, value)`, `GET(path)`, `SIM_SET_LINK(link, value)`, `SIM_TRIP(slave, ch)`, `SIM_SLAVE_STATE(slave, state)` (STRING name or integer), `RUN_CYCLES(n)`, plus `SIM_ANALOG(slave, ch, value, unit)`, `SIM_DRIVE_FAULT(slave, lft)`, `SIM_RAMP(path, from, to, over)` and `SIM_SERIAL_PEER(slave, script)` so every ECAT-09 stimulus is available from ST. All call the same Plant methods as the scenario executor. Ramps started from ST advance during RUN_CYCLES.
- **D-18** `stc test [dir] --project <tsproj|plcproj|.st...> --io <Device*.xml...>` enables project mode. Each TEST_CASE gets a fresh Project and Network built from one parsed spec (isolation as today). The test body runs on the project's interpreter, so GVL paths also resolve directly. In project mode a test file may contain only TEST_CASEs; other declarations are an error telling the user to put them in the project. `ADVANCE_TIME(d)` runs d / BaseTick Ticks. Without `--io`, SET/GET/RUN_CYCLES work and SIM_* fail with "no --io network loaded".

### Claude's Discretion
- Package name and file split inside `pkg/scenario`; Plant lives there (imports interp, ecat, devices; nothing imports it except cmd/stc and pkg/testing).
- Text output layout for `stc sim` scenario mode.
- Exact fixture ST for the diag program, as long as it uses the Phase 26 FB_EcGetAllSlaveStates mock and fills an `ECT_Diag.Device_1_Diag` array with `p_stat_nDeviceState`, `p_stat_nLinkState` and `p_stat_bOk` like FB_EcDeviceDiag.
</decisions>

<canonical_refs>
## Canonical References
- `.planning/phases/24-ethercat-topology-link-binding/24-03-SUMMARY.md`: Network, IOBinder, fault API
- `.planning/phases/25-ethercat-terminal-models/25-01..03-SUMMARY.md`: Layout, devices Base, Analog/EL9222/PSU/EL6001 stimulus APIs, `Network.DeviceByName`
- `/Users/jonb/Projects/stc-wt-26/.planning/phases/26-atv320-drive-ethercat-master-services/26-01-SUMMARY.md`, `26-02-SUMMARY.md`, `26-03-PLAN.md`: ATV320 InjectFault/ClearFault, `SetLinkState`, `LinkNotPresent/LinkWithoutComm`, `(*ScanCycleEngine).SetNetwork`
- `/Users/jonb/Projects/stc/.planning/phases/22-symbol-tree-value-semantics/22-05-SUMMARY.md`: Runtime Get/Set/ToJSON, path grammar
- `/Users/jonb/Projects/stc-wt-23/.planning/phases/23-project-execution-runtime/23-01..03-PLAN.md`: ProjectSpec, LoadProject, Project.Tick/Clock/BaseTick/SetIOBinder, `loadProjectSpec`, `stc sim` project mode, `attachECat`
- `/Users/jonb/Projects/sildarvinnsla/SVNCoreComponents/SVNCoreComponents/ECT/Diag/FB_EcDeviceDiag.TcPOU`: how deviceState/linkState become p_stat_bOk (read-only, local only)
- `pkg/testing/runner.go` registerIOFunctions: built-in registration pattern
</canonical_refs>

<specifics>
## Example scenario (fixture tests/ecat_fixtures/scenario/jam.toml)

    [scenario]
    name = "jam"

    [[step]]
    cycle = 0
    set = { path = "ECT.A1_01.I1", value = true }

    [[step]]
    at = "100ms"
    trip = { slave = "DEMO.A1.03 (EL9222-5500)", channel = 1 }
    expect = { path = "ECT.A1_03.p_stat_Enabled", value = false, within = 2 }

    [[step]]
    at = "200ms"
    slave_state = { slave = "DEMO.A1.01 (EL1008)", state = "not_present" }
    expect = { path = "ECT_Diag.Device_1_Diag[2].p_stat_bOk", value = false, within = 20 }

Plans (exactly 4, sequential):
- 27-01 scenario model, parser, validation and deterministic executor against a fake target (ECAT-09)
- 27-02 Plant target: Network force overrides, slave lookup, routing, fixture project, end-to-end jam scenario on LoadProject (ECAT-09)
- 27-03 `stc sim --scenario` with JSON/text report, plus the ST built-in library (ECAT-10, DEVX-01)
- 27-04 `stc test --project --io` project mode, docs, env-gated ST301 run, validation and coverage gate (DEVX-01, ECAT-09, ECAT-10)

Known baseline failure on this branch base: `pkg/checker` TestEmptyFBCall (fixed on main). Gates tolerate it until the rebase removes it.
</specifics>

<deferred>
## Deferred Ideas
- Wall-clock (`--realtime`) scenario timing
- Inline scripted serial peer rules in TOML
- MCP tools for scenarios (Phase 29)
</deferred>
