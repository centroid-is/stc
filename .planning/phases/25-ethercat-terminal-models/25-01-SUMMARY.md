---
phase: 25-ethercat-terminal-models
plan: 01
subsystem: ecat
tags: [ethercat, device-models, layout, registry, diagnostics, digital-io]
requires:
  - "24-03 Network, Device, Registry, DefaultRegistry, Passthrough, slaveSpans"
provides:
  - "ecat.Layout / Field / NewLayout / Get / Put / Binder: name-addressed PDO entry positions relative to a slave's views"
  - "Registry.RegisterModel / RegisterPassive / Lookup (exact (vendor, product) beats vendor-scoped model-name pattern)"
  - "ECAT010 (ecat.CodeNoModel) warning per unmodelled slave via Network.Diagnostics()"
  - "Network.Device / DeviceByName / Layout accessors"
  - "pkg/ecat/devices: Base (Set/Get/ChannelField/stepIO/setField/read), Passive, DigitalIO, product constants, Register(r) + init()"
affects: [25-02, 25-03, 25-04, 26]
tech-stack:
  added: []
  patterns: ["device models embed devices.Base and call stepIO from Step", "input stimulus is a persistent override reapplied every Step", "models registered by exact id plus anchored vendor-scoped name fallback"]
key-files:
  created:
    - pkg/ecat/layout.go
    - pkg/ecat/layout_test.go
    - pkg/ecat/devices/base.go
    - pkg/ecat/devices/base_test.go
    - pkg/ecat/devices/ids.go
    - pkg/ecat/devices/passive.go
    - pkg/ecat/devices/digital.go
    - pkg/ecat/devices/digital_test.go
  modified:
    - pkg/ecat/network.go
    - cmd/stc/ecat.go
decisions:
  - "Field.Bit is relative to the slave's span in its direction, so models index the in/out views passed to Step directly"
  - "Base.Set only accepts input entries; outputs are owned by the PLC and read with Get"
  - "Coupler name fallback is ^EK1[0-2]\\d\\d$ so EK1200 with a non-registered product code still gets the Passive model"
  - "DigitalIO.Output falls back to bit indexing across multi-bit output entries (Festo CTEU) when no 1-bit channel matches"
metrics:
  duration: "~10 min"
  completed: 2026-10-06
  tasks: 3
  files: 10
---

# Phase 25 Plan 01: Layout, Model Selection, ECAT010 and Digital/Passive Models Summary

Device models now resolve their PDO entries by (PDO, entry) name through `ecat.Layout`. `NewNetwork` picks a model by exact id or model-name fallback and reports every unmodelled slave as an ECAT010 warning. The new `pkg/ecat/devices` package ships the shared `Base`, the passive coupler models and the `DigitalIO` model.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 | Layout, Binder, model-name fallback and ECAT010 | 598a7e4 (test), be4f9bd (feat) |
| 2 | devices Base, ids and passive models | 024a819 (test), 3eaa5f3 (feat) |
| 3 | DigitalIO models and CLI registration | e533602 (test), 49bd6e5 (feat) |

## API for 25-02 and 25-03

- **Layout.** `l.Field(pdo, entry)`, `l.Fields(dir)` and `l.FindEntry(dir, entry)` return `ecat.Field{Pdo, Entry, Dir, Bit, BitLen, DataType}`. `ecat.Get(view, f)` and `ecat.Put(view, f, v)` are bounds-safe.
- **Base.** Embed `devices.Base` and call `b.stepIO(out, in)` first in `Step`. Then use `b.read(f)`, `b.setField(f, v)` for persistent input overrides, `b.ChannelField(dir, n)`, `b.Layout()`, `b.Slave()` and `b.name()`. Write computed inputs with `ecat.Put(b.in, f, v)`.
- **Registration.** Add entries to `knownIDs` with `{name, vendor, product, factory}` and model-name patterns to `beckhoffFallbacks` in `ids.go`. `Register(r)` and `init()` pick them up.
- **Network.** `Network.DeviceByName(name)`, `Device(master, i)`, `Layout(master, i)` and `Diagnostics()`.

## Verification

- `go test ./pkg/ecat/... ./cmd/stc -count=1 -cover` passes. Coverage is 99.6% for `pkg/ecat` and 100% for `pkg/ecat/devices`.
- `go vet ./pkg/ecat/... ./cmd/stc` is clean.
- All product codes in `ids.go` except EL1018 were confirmed in the reference exports. EL1018 is absent from them, as the plan notes.
- On Demo Device 1, ECAT010 fires only for ATV320 EtherCAT, EL2912, EL9222-5500 and PS2001-2410. The test asserts these by model.
- The full `go test ./...` run shows only the known pre-existing `pkg/checker` `TestEmptyFBCall` failure.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Coupler fallback pattern widened to `^EK1[0-2]\d\d$`**
- **Found during:** Task 2
- **Issue:** The plan's `^EK1[01]\d\d$` does not match EK1200. The Demo Device 1 fixture uses synthetic product codes for its EK1200 (0x04b03052) and EK1110 (0x04563052), so the EK1200 would have received ECAT010.
- **Fix:** Widened the pattern, which is still anchored and vendor-scoped.
- **Commit:** 3eaa5f3

**2. [Rule 2 - Missing functionality] Additions beyond the plan text**
- `Network.Layout(master, i)` exposes a slave's layout to tests and later models.
- `ecat.NewLayout(fields)` lets model tests build layouts without a topology.
- `ecat.CodeNoModel = "ECAT010"` follows the existing `Code*` constants.
- `Passive.Step` calls `stepIO`, so the generic `Set`/`Get` stimulus also works on couplers. With no stimulus it still writes nothing.
- **Commits:** be4f9bd, 3eaa5f3

**3. Test correction during Task 1.** Phase 24 lays out each PDO byte-aligned unless a ProcessImage gives BitOffs. The packed EP2338 test therefore supplies a ProcessImage with packed output BitOffs, as TwinCAT exports do. It asserts that outputs share one byte while inputs without a ProcessImage are byte-aligned. This was a test-only change before the commit.

**Note.** `ids.go` registers via `func init() { Register(ecat.DefaultRegistry) }`. The plan's key-link regex `DefaultRegistry\.Register` therefore does not match literally, but the link exists and is tested by `TestRegistryKnowsEveryProduct`.

## Known Stubs

None.

## Self-Check: PASSED

All created files exist, and commits 598a7e4, be4f9bd, 024a819, 3eaa5f3, e533602 and 49bd6e5 are present on gsd/phase-25-terminal-models.
