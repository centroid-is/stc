---
phase: 24-ethercat-topology-link-binding
plan: 03
subsystem: ecat, interp
tags: [ethercat, process-image, scan-cycle, pseudo-inputs, fault-injection]
requires:
  - "24-01 Topology, Images, ReadBits/WriteBits, pseudo-input slots"
  - "24-02 CollectLinks, Resolve, Binding"
provides:
  - "ecat.Network: per-master images, device registry, healthy Beckhoff pseudo-inputs, fault API"
  - "ecat.Device / Registry / DefaultRegistry / Passthrough for Phase 25/26 device models"
  - "interp.IOBinder: TcLinkTo bindings copied at scan boundaries"
affects: [24-04, 25, 26, 27]
tech-stack:
  added: []
  patterns: ["binder hooked into Tick through two nil-guarded calls", "codec driven by the zero value shape plus TypeDecls member order"]
key-files:
  created:
    - pkg/ecat/network.go
    - pkg/ecat/network_test.go
    - pkg/interp/iobind.go
    - pkg/interp/iobind_test.go
    - pkg/interp/iobind_e2e_test.go
  modified:
    - pkg/interp/scan.go
decisions:
  - "Device Step gets byte views over the slave's own PDO entry span per direction; pseudo-inputs are written by Network, never by devices"
  - "Network.Step runs inside IOBinder.preScan, so pseudo-inputs and device models update once per Tick before inputs are copied"
  - "The codec walks the variable's current (zero-initialised) value: width and sign from IECType, structs in TypeDecls declaration order, reads clamp at the slot end"
  - "Unknown integer widths (enum without resolved base) take the rest of the slot at top level and 16 bits inside aggregates"
  - "Bindings whose leaf is not BOOL/integer/real or an aggregate of those (STRING, TIME) are dropped with an error"
metrics:
  duration: "~30 min"
  completed: 2026-10-06
  tasks: 3
  files: 6
---

# Phase 24 Plan 03: Network, Device Registry and Scan-Boundary IOBinder Summary

A per-master `ecat.Network` now publishes Beckhoff healthy pseudo-inputs, accepts pluggable device models and fault overrides, and `interp.IOBinder` copies TcLinkTo-bound variables from the input image before each scan and to the output image after it.

## Tasks

| Task | Name | Commits |
| ---- | ---- | ------- |
| 1 | Network, Device registry and pseudo-inputs | c354350 (test), 0b360e6 (feat) |
| 2 | IOBinder path resolution and value codec | ab7339d (test), a2c7946 (feat) |
| 3 | Scan-boundary hooks and end-to-end test | 0c97f0a (test), b49329b (feat) |

## Exported API

pkg/ecat (for Phases 25/26/27 and 24-04):
- `type Device interface{ Init(s *Slave); Step(dt time.Duration, out, in []byte) }`. `out` and `in` are views over the slave's own PDO byte span. Either may be empty.
- `Passthrough` is the default no-op device.
- `NewRegistry()`, `(*Registry).Register(vendor, product uint32, factory func() Device)`, `(*Registry).New(s *Slave) Device`, package var `DefaultRegistry`.
- `NewNetwork(topo *Topology, reg *Registry) *Network` uses DefaultRegistry when reg is nil. `(*Network).Images()`, `Step(dt)`, `SetMasterNetID(master, [6]byte) error`.
- Fault API: `SetSlaveState(master string, slave int, state uint16) error`, `SetWcState(master string, slave int, bad bool) error`, `SetDevState(master string, bits uint16) error`, `ClearFaults()`. Slave is the bus index. Unknown master or slave returns an error.
- Constants `StateOP` (0x0008) and `FirstPort` (1001).

pkg/interp (for 24-04):
- `NewIOBinder(bindings []ecat.Binding, net *ecat.Network) *IOBinder`, `(*ScanCycleEngine).SetIOBinder(b)` (nil detaches), `(*IOBinder).Errors() []error`.
- Tick runs `preScan(dt)` after step 0. It resolves lazily, calls `net.Step(dt)`, then copies inputs. Tick runs `postScan()` after step 5.

## Verification

- `go test ./pkg/ecat -count=1 -cover`: pass, 99.8%. network.go is fully covered apart from the two empty Passthrough methods, which have no statements.
- `go test ./pkg/interp -count=1 -cover`: pass, 98.1%. iobind.go is at 100%.
- End-to-end on Demo Device 1/2: EL1008 Channel 1 reaches ECT.A1_01.I1 on the next Tick, and clearing it returns FALSE. CN01.q_uCMD writes 16#000F to the CMD slot. V1_C1 writes 0x5A to the CTEU C1 Output slot. SlaveCount reads 10, State 8, WcState FALSE, DevState/Frm0State/Frm0WcState 0, AmsNetId[0] 192, and CN01.amsaddr.port 1007. SetSlaveState, SetWcState and SetDevState show up after the next Tick.
- A nil binder leaves Tick unchanged. Existing scan tests pass.
- `git diff cab4035 --stat -- pkg/interp/scan.go` shows 12 changed lines. value.go, fb_instance.go and interpreter.go are untouched.
- `go build ./...` and `go vet` are clean. `go test ./...` fails only on the known pre-existing `pkg/checker` TestEmptyFBCall.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test assumption] EL1008 input span is eight bytes**
- **Found during:** Task 1
- **Issue:** The draft registry test expected a one-byte input region for the EL1008. The layout byte-aligns each of its eight 1-bit PDOs.
- **Fix:** The test now expects eight bytes and checks every channel through the topology.
- **Commit:** 0b360e6

**2. [Rule 3 - Test isolation] Task 2 tests call preScan/postScan directly**
- **Found during:** Task 2
- **Issue:** The Tick hooks only land in Task 3, so codec tests through Tick could not pass in Task 2.
- **Fix:** An `ioScan` helper initialises the engine and runs both binder copies.
- **Commit:** ab7339d

## Deferred Issues

- `pkg/checker` TestEmptyFBCall still fails. It is pre-existing and already logged in deferred-items.md.

## Threat Mitigations

- T-24-05: decoding is strictly by declared type and slot width. The bit cursor never reads or writes past the slot end, and ReadBits/WriteBits are bounds-safe. Unsupported shapes and unresolved paths drop the binding with an error, so nothing panics. Tests cover a short slot, a truncated AMSADDR, STRING leaves, nested STRING, and missing GVLs, variables and members.

## Known Stubs

None. Passthrough is the documented default device. Phase 25/26 register real models.

## Self-Check: PASSED
