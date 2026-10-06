---
phase: 28-opc-ua-address-space
plan: 02
subsystem: opcua
tags: [opcua, awcullen, plcopen, datatypes, structuredefinition, reflect-structof, enums, arrays, access-level]

requires:
  - phase: 28-01
    provides: "Server core, NodeSource/MapSource contract, test helpers"
provides:
  - "typemap.go: uaDataType, uaGoType, arrayShape, arrayLen (PLCopen OPC 30000 mapping, OPCUA-06)"
  - "convert.go: toUA / fromUA between canonical IEC Go values and awcullen wire values"
  - "datatypes.go: (*Server).ensureEnum / ensureStruct, structDT, process-global goTypeCache, enumProperty"
  - "space.go: NodeClass, AccessLevel, NodeSpec, Space, (*Space).Find"
  - "publish.go: (*Server).Publish(sp *Space, src NodeSource) error, rootID() (ObjectsFolder for now), statusFor"
affects: [28-03, 28-04, 29]

tech-stack:
  added: []
  patterns:
    - "Runtime struct Go types via reflect.StructOf with X<Type>__<i>_<member> field names, registered once per namespace+name+layout"
    - "A changed layout of an already registered struct name gets encoding id TE.<Name>.v<fnv32>.DefaultBinary"
    - "Structured Variable reads compose from one NodeSource.Snapshot of all leaf member paths"
    - "Whole-struct writes convert every member first, then write only members whose own NodeSpec is writable"
    - "Registry-only tests use New without Start, so they skip the 3 s Close grace"

key-files:
  created:
    - pkg/opcua/typemap.go
    - pkg/opcua/typemap_test.go
    - pkg/opcua/convert.go
    - pkg/opcua/convert_test.go
    - pkg/opcua/datatypes.go
    - pkg/opcua/datatypes_test.go
    - pkg/opcua/genericdecode_test.go
    - pkg/opcua/space.go
    - pkg/opcua/publish.go
    - pkg/opcua/publish_test.go
  modified:
    - pkg/opcua/server.go

key-decisions:
  - "Every array Variable and array struct field is ValueRank 1 with the flattened element count; awcullen rejects flat slices on rank>1 nodes"
  - "fromUA returns int64 for signed kinds, uint64 for unsigned and bit kinds, time.Duration for TIME/TOD and an int64 ordinal for enums"
  - "An enum with ordinals 0..n-1 gets EnumStrings; any other enum gets EnumValues, on both the DataType and each enum Variable"
  - "A struct member without its own NodeSpec inherits the writability of its enclosing node during a whole-struct write"
  - "Publish refuses node ids that already exist, because awcullen's AddNodes silently replaces nodes"

requirements-completed: [OPCUA-04, OPCUA-05, OPCUA-06]

duration: 35min
completed: 2026-10-06
---

# Phase 28 Plan 02: OPC UA Value Layer Summary

**PLCopen type mapping, IEC value conversion, served Enumeration and Structure DataTypes backed by runtime StructOf types, and `(*Server).Publish` for a hand-built Space with access levels, descriptions, enums, arrays and structured ExtensionObjects.**

## Performance

- Duration: about 35 minutes
- Completed: 2026-10-06
- Tasks: 3 of 3
- Files: 10 created, 1 modified

## Accomplishments

- Every OPCUA-06 type reads over the awcullen client with the expected DataType and Go value type. TIME is Int64 ms, TOD is UInt32 ms and DT is DateTime.
- The enum `E_State{Idle=0, rdy=2, Run=3}` is served as `DT.E_State` under Enumeration, with an EnumDefinition and an EnumValues property. Contiguous enums get EnumStrings.
- `ST_Drive_HMI` and the nested `ST_Outer` serve StructureDefinitions with a DefaultEncodingId and a Default Binary encoding object. A generic test decoder decodes a value by field name using only served definitions. That includes a nested struct, an array of REAL and an array of structs.
- A structured `GVL_Test.fbMotor.HMI` reads as one ExtensionObject from exactly one Snapshot call and zero Read calls. Its members stay individually readable and writable.
- A whole-struct write only changes the writable `p_cmd_JogFwd` member. The read-only `p_stat_*` members stay unchanged.
- `go test -race -count=2 ./pkg/opcua` passes in about 29 s with 94.9% statement coverage. `go vet` is clean and `go test ./...` is green.

## Task Commits

1. **Task 1 RED: type mapping and conversion tests** - `f0e5330` (test)
2. **Task 1 GREEN: typemap and convert** - `2228f5f` (feat)
3. **Task 2: enum and struct DataTypes, generic decode** - `67228bc` (feat)
4. **Task 3: Space, Publish and handlers** - `2cde271` (feat)

## API for 28-03

- Build produces `*Space{Nodes []NodeSpec}` with parents before children, then calls `srv.Publish(space, src)`.
- `NodeSpec{Path, Name, Parent, Class, Type, Structured, Access, Description}`. Use `Structured: true` only for Variables whose Type is `*types.StructType`. Struct-typed Variables without it are rejected, so Build publishes those as Objects.
- Publish returns an error for types it cannot serve. These are POINTER, REFERENCE, FB, arrays with non-constant bounds, arrays of structs as standalone Variables, and structs with such members. Build should check for these first or fall back to an Object.
- `rootID()` in publish.go returns ObjectsFolder. 28-03 changes it to the PLC1 object.
- Member leaf paths use `<struct>.<member>`, and array-of-struct elements use `<member>[i]` or `[i,j]`, starting at each Low bound.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Multi-dimensional arrays fall back to rank 1**
- **Found during:** Task 3
- **Issue:** awcullen's write type check rejects a flat slice on a node with ValueRank greater than 1. A flattened value with rank 2 would be readable but never writable.
- **Fix:** `arrayShape` still reports the true rank and dimensions. Publish and struct fields use rank 1 with the flattened element count, as the plan's fallback clause allows.
- **Commit:** 2cde271

**2. [Rule 1 - Bug] Re-publishing a node id silently replaced the node**
- **Found during:** Task 3
- **Issue:** awcullen's AddNodes overwrites an existing node without an error.
- **Fix:** Publish checks `FindNode` for every path before it adds anything.
- **Commit:** 2cde271

**3. [Rule 2 - Correctness] Encoding cache keyed by namespace URI too**
- **Issue:** The registered ExpandedNodeId contains the PLC namespace URI, so two Servers with different PLCNamespace values must not share a cache entry.
- **Fix:** The goTypeCache key is namespaceURI, name and layout.
- **Commit:** 67228bc

### Process notes

- Task 2 and Task 3 were committed as single feat commits containing tests and code. There is no separate RED commit for them. Task 1 has the full RED and GREEN pair.

## Notes

- **LTIME has no input.** pkg/types has no LTIME, LDATE, LDT or LTOD kind, so the CONTEXT rule mapping LTIME to Int64 ns cannot be applied yet.
- **CHAR and WCHAR** accept an integer code or a one-character string. They are published as Byte and UInt16.
- **Read errors** return BadOutOfRange for ErrOutOfRange and BadNoData for everything else. Conversion failures on read also give BadNoData.
- **Read IndexRange** is not implemented. The custom read handler returns the whole value, as noted in Gotcha 7.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-28-05 is covered by awcullen's AccessLevel gate, the fromUA checks and IndexRange rejection. T-28-06 is covered by the writable-member filter. T-28-07 is covered by fromUA's element count check. T-28-08 is covered by the mutex-guarded goTypeCache and the versioned encoding ids.

## Next Phase Readiness

- 28-03 can build Spaces from SymbolNode trees and switch `rootID()` to the PLC1 object.

## Self-Check: PASSED
