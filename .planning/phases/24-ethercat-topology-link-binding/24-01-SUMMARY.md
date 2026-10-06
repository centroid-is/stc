---
phase: 24-ethercat-topology-link-binding
plan: 01
subsystem: ecat
tags: [ethercat, twincat, topology, process-image, tclinkto]
requires: []
provides:
  - "pkg/ecat LoadConfig/LoadProject (EtherCATConfig v1.3 loader)"
  - "generate_gvl.py IO-tree rules: ClassifyRole, assignParents, SlaveBasePath, ModuleSegment, LinkPath, IECType"
  - "Per-master process image layout: Slot, Master.Slot/Slots, Topology.Slot/Paths, Images, ReadBits/WriteBits"
affects: [24-02, 24-03, 24-04, 25, 26, 27]
tech-stack:
  added: []
  patterns: ["encoding/xml mirror structs with ,any capture for SmN", "ProcessImage BitOffs authoritative, computed bus-order layout as fallback"]
key-files:
  created:
    - pkg/ecat/topology.go
    - pkg/ecat/topology_test.go
    - pkg/ecat/tree.go
    - pkg/ecat/tree_test.go
    - pkg/ecat/image.go
    - pkg/ecat/image_test.go
    - tests/ecat_fixtures/Demo Device 1.xml
    - tests/ecat_fixtures/Demo Device 2.xml
  modified: []
decisions:
  - "Without Physics, EL terminals classify as plain (generator fallback has no terminal case), so they stay master-level; followed the generator over the plan bullet"
  - "Entries flattened with __ resolve through TwinCAT's grouped ProcessImage variable (e.g. '<pdo>.Status') plus their bit offset inside the group"
  - "Pseudo-input slots are byte-aligned single-item groups; slot DataType carries the EtherCAT type (BIT, UINT, AMSADDR, AMSNETID)"
metrics:
  duration: "~20 min"
  completed: 2026-10-06
  tasks: 3
  files: 8
---

# Phase 24 Plan 01: EtherCAT Topology Loader and Process Image Layout Summary

`pkg/ecat` loads TwinCAT EtherCATConfig exports, reproduces generate_gvl.py's E-bus nesting, Module segments and TcLinkTo link paths, and maps every linkable path (PDO entries plus slave and master pseudo-inputs) to a stable (master, dir, byte, bit, bitLen) slot.

## Tasks

| Task | Name | Commits |
| ---- | ---- | ------- |
| 1 | Fixtures and EtherCATConfig loader | e9fce94 (test), ff99efc (feat) |
| 2 | IO-tree nesting, module segments, link paths | 046e31f (test), 396986f (feat) |
| 3 | Process image layout, pseudo slots, Topology.Slot | 0863624 (test), b187f02 (feat) |

## Exported API (for 24-02 / 24-03)

- Loader: `LoadConfig(path) (*Master, error)`, `LoadProject(paths ...string) (*Topology, error)`, `Topology.Master(name) *Master`, `DefaultNetID`.
- Types: `Topology{Masters}`, `Master{Name, Slaves, NetID [6]byte, InBytes, OutBytes}`, `Slave{Index, Name, Model, Vendor, Product, Revision, HasVendor, HasProduct, Phys, HasPhys, Physics, PrevPhys, HasPrevPhys, PrevPort, Pdos, Parent, IsEBus}`, `Pdo{Name, Index, Dir, Entries}`, `Entry{Name, Index, SubIndex, BitLen, DataType}` with `Padding()`.
- Tree: `Role` (`RoleOpen`, `RoleTerminal`, `RoleClose`, `RolePlain`), `ClassifyRole`, `SlaveBasePath`, `ModuleSegment`, `LinkPath`, `IECType`.
- Image: `Dir` (`DirIn`, `DirOut`), `Slot{Master, Dir, Byte, Bit, BitLen, DataType, Path}`, `Master.Slot(path)`, `Master.Slots()`, `Topology.Slot(path)`, `Topology.Paths()`, `Image{In, Out}`, `Images{ByMaster}` with `NewImages`, `Get`, `Masters`, `ReadBits`, `WriteBits`.
- Pseudo paths: `<SlaveBasePath>^WcState^WcState` (1), `^InfoData^State` (16), `^InfoData^AdsAddr` (64); `TIID^<master>^Inputs^DevState|SlaveCount|Frm0State|Frm0WcState` (16), `^InfoData^AmsNetId` (48), `^InfoData^ChangeCount` (16).

## Verification

- `go test ./pkg/ecat -count=1 -cover`: pass, 99.6% coverage; `go vet` and `gofmt` clean; `go build ./...` ok.
- Read-only smoke check on the real ST301 exports, not committed: Device 1 to 4 load with 23, 49, 34 and 16 slaves. EL1008, EL9222 grouped status, EL2912 Module 3 and InfoData^AmsNetId paths all resolve.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The plan's Device 2 nesting bullet contradicted the generator**
- **Found during:** Task 2
- **Issue:** The plan says the EL1008 in Demo Device 2, which has no Physics, nests under the EK1100 "via fallback". The generator's `classify_role` fallback only returns close, open or plain, so the EL1008 is plain and stays at master level. The EK1110 still closes the EK1100 segment.
- **Fix:** Ported the generator exactly, as the ECAT-01 decision requires, and asserted the generator's behaviour in `TestAssignParentsDemo2Fallback`.
- **Files modified:** pkg/ecat/tree_test.go
- **Commit:** 046e31f

**2. [Rule 2 - Missing functionality] Grouped ProcessImage variables**
- **Found during:** Task 3, while reading the real export
- **Issue:** TwinCAT's ProcessImage lists `Status__Enabled`-style entries as one struct variable, `<slave>.<pdo>.Status`. An exact-name lookup would ignore the real offsets for EL9222 status bits.
- **Fix:** When the exact name is missing, the lookup falls back to the group variable's BitOffs plus the entry's offset within the group. `TestLayoutGroupedEntriesUseParentVariable` covers this.
- **Files modified:** pkg/ecat/image.go, pkg/ecat/image_test.go
- **Commit:** b187f02

**3. [Rule 2] Hex VendorId/ProductCode**
- The loader accepts both decimal and `#x` integers, so the fixture's ATV320 uses `#x0800005A`. The generator's `_int` is decimal-only, so this is a superset.

## Threat Mitigations

- T-24-01: The loader rejects BitLen values outside 0..4096, ProcessImage BitOffs above 1<<24 and ByteSize above 2 MiB with an error that names the path. Tests cover each case.
- T-24-02: ReadBits and WriteBits bounds-check every access. An out-of-range read returns 0, an out-of-range write is ignored, and neither panics. Tests cover these cases.

## Known Stubs

None.

## Self-Check: PASSED
