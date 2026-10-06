---
phase: 28-opc-ua-address-space
plan: 03
subsystem: opcua
tags: [opcua, tf6100, address-space, exposure-rules, deviceset, golden, st301]

requires:
  - phase: 28-02
    provides: "Space/NodeSpec, Publish, ensureStruct/ensureEnum, typemap"
provides:
  - "Build(root SymbolNode, src NodeSource) (*Space, []diag.Diagnostic) with TF6100 exposure rules"
  - "attr (last occurrence, case-insensitive) and unescapeIEC"
  - "Diagnostic codes OPCUA001-OPCUA008 as exported Code* constants"
  - "structEligible(*types.StructType) error, pure twin of ensureStruct's rules"
  - "Objects/DeviceSet (ns=2;i=5001)/PLC1 (ns=4;s=PLC1) with DI properties; rootID() is PLC1"
  - "pkg/opcua/opcuatest: BrowseSnapshot(ctx, client, root) ([]byte, error) and Take"
  - "tests/opcua_golden/st301_shape.json golden browse snapshot"
affects: [28-04, 29]

tech-stack:
  added: []
  patterns:
    - "Build returns each subtree as a slice with the container first, so parents always precede children without index juggling"
    - "Exposure: '0' prunes, '1'/'2' expose and inherit, Access never inherits"
    - "Members of a structured Variable force nested structs and array-of-struct elements to be structured"
    - "Golden snapshot omits values and encoding ids, so it depends only on the address-space shape"

key-files:
  created:
    - pkg/opcua/attrs.go
    - pkg/opcua/attrs_test.go
    - pkg/opcua/build.go
    - pkg/opcua/build_test.go
    - pkg/opcua/deviceset.go
    - pkg/opcua/deviceset_test.go
    - pkg/opcua/fixture_test.go
    - pkg/opcua/st301_test.go
    - pkg/opcua/golden_test.go
    - pkg/opcua/opcuatest/snapshot.go
    - tests/opcua_golden/st301_shape.json
  modified:
    - pkg/opcua/datatypes.go
    - pkg/opcua/publish.go
    - pkg/opcua/publish_test.go
    - pkg/opcua/server.go
    - pkg/opcua/datatypes_test.go

key-decisions:
  - "'2' on an FB instance or array exposes it like '1'; '2' on a struct that cannot be a DataType is skipped with OPCUA005"
  - "POINTER/REFERENCE (OPCUA002) and unknown bounds (OPCUA003) are only reported when the symbol would be exposed"
  - "A struct's node class follows StructuredType even when only a descendant is exposed"
  - "DI properties use BrowseNames in ns=2 and NodeIds ns=4;s=PLC1.<Property>; Model is a String"
  - "The golden snapshot excludes encoding ids because they are process-global and layout-versioned"

requirements-completed: [OPCUA-02, OPCUA-03, OPCUA-04, OPCUA-05]

duration: 30min
completed: 2026-10-06
---

# Phase 28 Plan 03: TF6100 Address-Space Builder Summary

**Build turns a SymbolNode tree into a TF6100-shaped Space under Objects/DeviceSet/PLC1, following the OPC.UA.DA exposure, Access, Description and StructuredType rules, with OPCUA001-008 diagnostics and a golden ST301 browse snapshot.**

## Performance

- Duration: about 30 minutes
- Completed: 2026-10-06
- Tasks: 3 of 3
- Files: 11 created, 5 modified

## Accomplishments

- Build covers every locked exposure rule with a named table case. These include inheritance, '0' pruning, '2' flattening, type-level marks, the silent MAIN, Access mapping, StructuredType and all eight diagnostic codes.
- The ST301-shaped fixture reads `ns=4;s=GVL_BatchLines.Drives_Line1[1].HMI` as one ExtensionObject. Decoded by field name, it has p_stat_State 2, which the served E_DriveState names "rdy".
- p_stat_* members are read-only and reject writes with BadNotWritable. p_cmd_* writes show up in the struct read.
- `sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw` is published because of the FB-type mark, even though the instance is unmarked.
- `go test -race ./pkg/opcua` passes in about 15 s with 95.4% statement coverage. The golden test passes three times in a row and `go test ./...` is green.

## Task Commits

1. **Task 1 RED: attribute and Build tests, fixture** - `0764b5c` (test)
2. **Task 1 GREEN: attrs, Build, structEligible** - `2289369` (feat)
3. **Task 2: DeviceSet/PLC1 and root attachment** - `cf28123` (feat)
4. **Task 3: ST301 tests, opcuatest snapshot, golden** - `5dbc7a3` (test)

## API for 28-04

- Call `Build(root, src)`, then `srv.Publish(space, src)`. Diagnostics have zero positions and are in walk order.
- The adapter's root must be `KindRoot`, and its Children must be GVL and PROGRAM nodes in declaration order.
- Array elements need Name `arr[i]` and Path `<arrayPath>[i]`.
- Attributes must list type-level attributes first: the TYPE header, then the member declaration inside the FB or STRUCT. The instance declaration comes last, because the last one wins.
- Struct nodes need Type `*types.StructType` and FB nodes `*types.FunctionBlockType`.
- Unexposed symbols may stay in the tree, since Build prunes them.
- The exec test can call `opcuatest.BrowseSnapshot(ctx, c, ua.NewNodeIDString(4, "PLC1"))` and compare the result with `tests/opcua_golden/st301_shape.json`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Order-dependent encoding id assertion**
- **Found during:** Task 3
- **Issue:** The fixture's 7-member ST_Drive_HMI shares the process-global goTypeCache with the 3-member ST_Drive_HMI in the 28-02 tests. Whichever registers second gets a versioned `TE.ST_Drive_HMI.v<hash>.DefaultBinary`, and TestEnsureStruct failed under parallel ordering.
- **Fix:** TestEnsureStruct now accepts the optional `.v<hash>` part. The golden snapshot leaves encoding ids out.
- **Commit:** 5dbc7a3

**2. [Rule 1 - Bug] '2' on an ineligible struct**
- **Issue:** The plan says "a Variable of the struct DataType anyway". Publish cannot serve that, so it would reject the whole Space.
- **Fix:** Build skips the node and reports OPCUA005.
- **Commit:** 2289369

### Other adjustments

- server.go gained `dsOnce` and `dsErr` fields for the DeviceSet. The plan did not list this file.
- The DeviceSet test lives in deviceset_test.go instead of build_test.go.
- The TestPublish browse check now browses PLC1 instead of ObjectsFolder.
- `opcuatest.Take` is exported as well, for structured comparisons.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-28-09 is enforced by default-deny exposure and tested with the silent MAIN, '0' and unmarked internals. T-28-11 is enforced by the depth cap of 64 (OPCUA008) and case-insensitive duplicate path detection (OPCUA007).

## Next Phase Readiness

- 28-04 needs the symtree adapter to satisfy the SymbolNode expectations above. It can then reuse the golden file to prove that the real ST301 import produces the same shape.

## Self-Check: PASSED
