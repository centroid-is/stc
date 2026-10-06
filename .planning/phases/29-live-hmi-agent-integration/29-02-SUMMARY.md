---
phase: 29-live-hmi-agent-integration
plan: 02
subsystem: opcua
tags: [opcua, tf6100, snapshot, diff, hmi, cli]
requires:
  - phase: 28-opc-ua-address-space
    provides: "opcuatest.Take/BrowseSnapshot, golden st301_shape.json, stc serve, bind adapters"
provides:
  - "opcuatest.Diff/FormatDiff/Load/DataTypeName with Subset and Full modes"
  - "stc opcua snapshot <endpoint> command"
  - "Skip-unless-captured TF6100 fidelity diff test"
  - "HMI keymappings stand-in and definition-driven struct/enum reader"
affects: [29-04 docs, OPCUA-09, OPCUA-10]
tech-stack:
  added: []
  patterns:
    - "Client-side ExtensionObject decoding by registering reflect.StructOf types built from served StructureDefinitions"
    - "tests package talks only to an exec'd stc serve (no in-process opcua.Server) to keep awcullen's global type registry collision-free"
key-files:
  created:
    - pkg/opcua/opcuatest/diff.go
    - pkg/opcua/opcuatest/diff_test.go
    - pkg/opcua/opcuatest/snapshot_test.go
    - cmd/stc/opcua_cmd.go
    - cmd/stc/opcua_cmd_test.go
    - tests/opcua_helpers_test.go
    - tests/opcua_tf6100_diff_test.go
    - tests/opcua_hmi_test.go
    - tests/opcua_golden/hmi_keymappings.json
  modified:
    - cmd/stc/main.go
key-decisions:
  - "DataType references normalize to the text after the last '.' or '>' of the identifier, because snapshots carry no DataType BrowseName"
  - "Diff ignores Description, Root, Parent, Reference and BrowseName"
  - "Tests in ./tests use an exec'd stc serve for both the diff and the HMI reads"
  - "STC_HMI_PROJECT selects the project for the real-keymappings run; hmi/keymappings.json targets the legacy project, not ST301"
requirements-completed: [OPCUA-09, OPCUA-10]
duration: 40min
completed: 2026-10-06
---

# Phase 29 Plan 02: TF6100 snapshot, fidelity diff and HMI stand-in Summary

**`stc opcua snapshot` captures any OPC UA server in the golden schema, `opcuatest.Diff` guards fidelity against a real TF6100 capture when present, and an HMI keymappings stand-in reads every HMI node id from `stc serve` with structs decoded by served field names and enums as `rdy(2)`.**

## Performance

- **Duration:** about 40 min
- **Completed:** 2026-10-06
- **Tasks:** 3
- **Files:** 9 created, 1 modified

## Accomplishments

- `opcuatest.Diff(emulated, real, Subset|Full)` reports node, node class, data type, value rank, array dimension, access level and DataType definition (kind, fields in order, enum values) divergences, sorted and deterministic. `FormatDiff` caps at 50 lines plus "... and N more".
- `stc opcua snapshot <endpoint> [--out] [--root] [--security none|basic256sha256] [--cert --key | --pki-dir] [--timeout]`. Output is byte-identical to `tests/opcua_golden/st301_shape.json` when run against the served fixture. A MapSource-backed test proves no writes reach the source (T-29-05).
- `tests/opcua_tf6100_diff_test.go` skips with the capture command when no capture exists, runs the subset diff when `st301_real.json` or `STC_TF6100_SNAPSHOT` exists, and the full ST301 diff with `STC_SILD_DIR`. The failure path runs in CI with a mutated golden copy.
- `tests/opcua_golden/hmi_keymappings.json` (tfc schema) and `TestHMIKeymappings`: 14 good, 0 bad, 3 skipped against `stc serve` on the fixture; `Line1.Motor1` decodes to 7 named fields with `p_stat_State` = `rdy(2)`.
- Coverage: pkg/opcua 95.4%, pkg/opcua/bind 100%, pkg/opcua/opcuatest 90.7%.

## Task Commits

1. **Task 1: opcuatest.Diff with DataType normalization** - `42e386f` (feat)
2. **Task 2: stc opcua snapshot command and TF6100 diff test** - `3001d70` (feat)
3. **Task 3: HMI keymappings stand-in** - `2916895` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] DataType normalization derives the name from the id**
- **Issue:** The snapshot's DataType table has no BrowseName field, so "map each DataType id to its BrowseName using the snapshot's own table" was impossible without changing the golden schema.
- **Fix:** `DataTypeName` keeps ns=0 ids and otherwise takes the identifier text after the last '.' or '>' (`DT.ST_X`, `<StructuredType>ST_X` and `Lib.ST_X` all become `ST_X`).
- **Commit:** 42e386f

**2. [Rule 3 - Blocking] Diff test serves via an exec'd stc serve, not in-process**
- **Issue:** An in-process `opcua.Server` registers Go types for its encoding ids in awcullen's global registry, and the HMI reader registers definition-built types for the same ids, so the two panicked in one test binary.
- **Fix:** All `./tests` OPC UA tests use an exec'd `stc serve`. The reader also reuses an already registered type.
- **Commit:** 2916895

**3. [Rule 2 - Missing functionality] STC_HMI_PROJECT override for the real-keymappings run**
- **Issue:** The real `hmi/keymappings.json` targets the legacy project (GVL_BatchLines, GVL_Baader...), so against ST301 it read 1 good and 319 bad.
- **Fix:** `STC_HMI_PROJECT` (absolute or relative to `STC_SILD_DIR`) selects the project. Against `skammtalinur-legacy/sildarvinnsla.tsproj`: 319 good, 1 bad, 108 skipped. Bad entries report the id plus status, never values.
- **Commit:** 2916895

**4. [Coverage] opcuatest Take/BrowseSnapshot test added**
- `snapshot_test.go` serves the fixture in-process inside the opcuatest package (separate test binary, no registry clash) to lift opcuatest coverage to 90.7%.

**TDD note:** Tests and implementation for each task were committed together in one `feat` commit, not as separate RED/GREEN commits.

## Deferred Issues

- `ns=4;s=GVL_BatchLines.recipes` (a variable-level StructuredType on `ARRAY [1..3] OF ST_LineRecipe`) is published as an Object, so the HMI's Value read fails. This needs a pkg/opcua Build change and TF6100 confirmation. Logged in `deferred-items.md`.

## Pending Phase 23

Nothing in this plan needs Project/LoadProject. `stc serve` from 28-04 was enough.

## User Setup Required

To activate the CI fidelity diff, capture the real PLC on the plant network:

```
stc opcua snapshot opc.tcp://<plc>:4840 --out tests/opcua_golden/st301_real.json
```

Committing the file is the user's call (D-11). `STC_TF6100_SNAPSHOT` is the alternative.

## Self-Check: PASSED
