---
phase: 23-project-execution-runtime
plan: 04
subsystem: runtime
tags: [project-runtime, opcua, ethercat, st301, coverage, phase-gate]

requires:
  - phase: 23-03
    provides: sim project mode, --persist, stc serve skeleton
  - phase: 28-04
    provides: OPC UA serve (address space, RuntimeSource, security)
  - phase: 26-04
    provides: SIZEOF/SHL/X_TO_Y needed by ST301 (duplicates removed in favour of 23-01)
provides:
  - one stc serve (project runtime + opt-in OPC UA)
  - phase gate tests (fixtures always, ST301 env-gated)
  - --io member links resolved through library struct types
  - deterministic scope symbol order (stable diagnostics)
affects: [29 live hmi, opc ua serve, ethercat --io]

tech-stack:
  added: []
  patterns:
    - "serve hooks: projectRunner.BeforeTick runs on the scan goroutine between Ticks (OPC UA writes, tick count)"
    - "loadProjectAnalysis returns the analysis result next to the ProjectSpec for symtree/OPC UA"

key-files:
  created:
    - tests/runtime_project_test.go
    - cmd/stc/serve_opcua_test.go
  modified:
    - cmd/stc/serve_cmd.go
    - cmd/stc/project_run.go
    - cmd/stc/project_load.go
    - pkg/ecat/collect.go
    - pkg/symbols/scope.go
    - pkg/interp/interpreter.go
    - docs/CLI_REFERENCE.md
    - docs/OPCUA.md
    - docs/ETHERCAT_SIMULATION.md
    - docs/ARCHITECTURE.md
    - .planning/phases/23-project-execution-runtime/23-VALIDATION.md

key-decisions:
  - "stc serve --opcua is opt-in (empty default); 28-04 defaulted to :4840"
  - "serve: analysis errors fail start (23 loader); runtime errors with --opcua keep serving the last image"
  - "Kept 23-01 SIZEOF/SHL..ROR/X_TO_Y (with checker signatures); deleted Phase 26 stdlib_sizeof/bitshift/convert_any"
  - "ST301 gate asserts the configured Device_1_SlaveCount (22): the GVL predates the current export (23 slaves)"

requirements-completed: [RUNT-03, RUNT-04, RUNT-07, RUNT-09]

duration: ~75min
completed: 2026-10-06
---

# Phase 23 Plan 04: Phase gate, unified serve and ST301 run Summary

**ST301 runs 1000 x 1 ms with its four EtherCAT exports attached, error-free and byte-identical across runs; `stc serve` is one command that runs the Phase 23 project runtime and serves it over OPC UA.**

## Merge of main (Phases 25, 26, 28)

- **serve:** add/add conflict in `cmd/stc/serve_cmd.go`. The result is one command with the 23-03 flags (`--project`, `--io`, `--persist`, `--persist-interval`, `--cycle`, `--realtime`, `--duration` with `--run-for` alias, `-D`) and the 28-04 flags (`--opcua`, `--security`, `--cert`, `--key`, `--pki-dir`). The OPC UA `RuntimeSource` reads the Project's `Runtime`; queued writes are applied by a `BeforeTick` hook from `Project.Run`'s `OnTick`. The 28-04 ticker scan loop and its private loader were removed. Directories as inputs now expand to their `.st` files in the shared loader.
- **Built-ins:** `pkg/interp/stdlib_sizeof.go`, `stdlib_bitshift.go`, `stdlib_convert_any.go` deleted; 23-01's `stdlib_sys.go` and `types/builtin_sys.go` kept. Phase 26 tests kept where semantics agree (two cases dropped: untyped SHL defaulting to 32 bits and a negative-count error; SIZEOF of FB instances is now a size, not an error).
- **Tests:** 28-04 tests moved to `cmd/stc/serve_opcua_test.go` and adapted: the scan-stopped tests use a division by zero (runtime error) instead of analysis errors, a new test asserts analysis errors fail start, and the runtime-init test was removed because the analyzer now reports that initialiser error first.

## Gate results

| Check | Result |
|-------|--------|
| TestProjectRuntimeGate SC1..SC4 (no env) | PASS |
| TestST301Runtime (STC_SILD_DIR set), run 3x | PASS |
| ST301 sim: cycles, sim_time_ns, PlcTask runs/overruns | 1000, 1000000000, 1000/0 |
| ST301 diagnostics | 278, none error (271 SEMA012 unused, 2 RUNT001, 1 VEND022 info) |
| ECT_Diag.Device_1_SlaveCount | 22 (configured); network reports 23 slaves |
| pkg/interp coverage (standalone / gate) | 98.7% / 98.75% |
| scripts/coverage-gate.sh | PASS, total 97.00% |
| go test ./... -race | PASS |

**serve smoke** (`stc serve "ST301 solution.tsproj" --io Device1..4 --opcua 127.0.0.1:48400 --realtime --duration 5s`):

```
OPC UA server listening on opc.tcp://...:48400 (all interfaces)
namespace 4, 13475 nodes, scan cycle 1ms
ECT_Diag.Device_1_SlaveCount = 22   (OPC UA read)
write 21 -> Good; read back 21      (write applied between Ticks)
Project: 613 cycles, sim time 613ms; exit 0
scan: 613 cycles, 613 overruns
```

The scan keeps ticking with no SIZEOF error. One ST301 scan takes about 8 ms in the interpreter, so a 1 ms realtime task overruns on every tick (about 120 ticks per second of wall time). Deterministic `stc sim` is unaffected.

## Task Commits

1. **Merge** - `74ddcc9` (merge)
2. **Task 1: Phase gate tests** - `ca7c78d` (fix), `717739d` (fix), `e1e8c0f` (test)
3. **Task 2: Validation contract and docs** - `c80c049` (docs)
4. **Task 3: Coverage gate** - no commit; the gate already passed (interp 98.75%), so `pkg/interp/project_cover_test.go` was not needed

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] --io member links ignored library struct types**
- **Found during:** Task 1 (ST301 run)
- **Issue:** 1328 "member ... is not declared by ECT.* (ST_EL9222_5500)" errors. The ECT terminal structs live in the SVNCoreComponents library, and the link collector indexed project files only.
- **Fix:** `ecat.CollectLinksWithLibraries(files, libs)` indexes library TYPE/FB declarations; the project `--io` path uses it.
- **Files modified:** pkg/ecat/collect.go, pkg/ecat/collect_test.go, cmd/stc/project_run.go
- **Commit:** ca7c78d

**2. [Rule 1 - Bug] Non-deterministic diagnostic order**
- **Found during:** Task 1 (two ST301 runs printed different JSON)
- **Issue:** `Scope.Symbols()` returned map order, so unused-variable warnings were shuffled.
- **Fix:** sort by declaration file, offset, then name.
- **Files modified:** pkg/symbols/scope.go, pkg/symbols/scope_test.go
- **Commit:** 717739d

**3. [Plan premise] SlaveCount assertion**
- The plan asked for `Device_1_SlaveCount` to equal the topology count. It is a generator constant (`UINT := 22`), and the ST301 GVLs were generated from an older export than the Device 1.xml now on disk (23 slaves). The test asserts the configured value parsed from ECT_Diag.TcGVL and logs the network count.

**4. SC4 IOBinder slot.** The gate's SC4 covers the wildcard slot, REAL, enum, struct AT members and the INT -5 output encode through `Project`. The IOBinder signed-INT decode is covered by the existing `pkg/interp` codec tests and the ST301 run, not by a separate gate subtest.

## Known Stubs

None.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: network | cmd/stc/serve_cmd.go | OPC UA listener now runs beside the project runtime; it remains opt-in via --opcua and binds all interfaces as in 28-04 |

No ST301 sources or outputs were committed (T-23-10).

## Self-Check: PASSED

- tests/runtime_project_test.go, cmd/stc/serve_opcua_test.go, 23-VALIDATION.md exist
- Commits 74ddcc9, ca7c78d, 717739d, e1e8c0f, c80c049 are in git log
