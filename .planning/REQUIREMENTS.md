# Requirements: STC v1.1 -- Vendor Libraries & I/O

**Defined:** 2026-03-30
**Core Value:** Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor -- no hardware required for development and testing.

## v1.1 Requirements

Requirements for vendor library support, I/O mapping, and mock framework.

### Vendor Library Loading

- [x] **VLIB-01**: User can declare vendor FBs in .st stub files (declarations without bodies) and reference them from production code
- [x] **VLIB-02**: User configures library paths via `[build.library_paths]` in stc.toml
- [x] **VLIB-03**: `stc check` resolves vendor FB types from stubs and validates input/output parameter usage
- [x] **VLIB-04**: LSP provides completion, hover, and go-to-definition for vendor FB inputs and outputs
- [x] **VLIB-05**: Single-vendor enforcement -- project targets one vendor, stubs from other vendors produce warnings

### I/O Address Mapping

- [x] **IO-01**: Parser handles AT %IX0.0, %QX0.0, %IW0, %QW0, %MW0, %MD0 address syntax in VAR blocks
- [x] **IO-02**: Interpreter maintains a mock I/O table mapping addresses to values
- [x] **IO-03**: I/O values sync at scan cycle boundaries (inputs copied before execution, outputs copied after)
- [x] **IO-04**: Tests can inject I/O values via the mock I/O table before assertions
- [x] **IO-05**: Address overlap detection warns when byte and bit addresses conflict

### Mock Framework

- [x] **MOCK-01**: User can write ST mock FBs with full bodies that override vendor stubs by name
- [x] **MOCK-02**: Mock paths configured via `[test.mock_paths]` in stc.toml
- [x] **MOCK-03**: FBs without explicit mocks auto-generate zero-value instances (accept inputs, return zeros)
- [x] **MOCK-04**: Mock signatures validated against stub signatures (parameter count and types must match)
- [x] **MOCK-05**: Zero-value auto-stubs emit fidelity warnings in test output

### Shipped Stubs -- Beckhoff

- [ ] **STUB-01**: Tc2_MC2 stubs shipped (MC_Power, MC_MoveAbsolute, MC_MoveRelative, MC_MoveVelocity, MC_Stop, MC_Home, MC_Reset, MC_ReadActualPosition, MC_ReadActualVelocity, MC_ReadStatus)
- [ ] **STUB-02**: Tc2_System stubs shipped (ADSREAD, ADSWRITE, FB_FileOpen, FB_FileClose, FB_FileRead, FB_FileWrite, MEMCPY, MEMSET, MEMMOVE)
- [ ] **STUB-03**: Tc2_Utilities stubs shipped (FB_FormatString, CRC16, CRC32)
- [ ] **STUB-04**: Tc3_EventLogger stubs shipped (FB_TcEventLogger, FB_TcAlarm)
- [ ] **STUB-05**: Common types shipped (AXIS_REF, MC_Direction, T_AmsNetId, T_AmsPort, E_OpenPath)
- [ ] **STUB-06**: Common EtherCAT terminal I/O patterns documented with example GVL stubs

### Shipped Stubs -- Schneider

- [ ] **STUB-07**: Schneider motion stubs shipped (MC_Power, MC_MoveAbsolute, MC_Stop with Schneider-specific parameters)
- [ ] **STUB-08**: Schneider communication stubs shipped (READ_VAR, WRITE_VAR, SEND_REQ, RCV_REQ)
- [ ] **STUB-09**: Schneider system stubs shipped (GetBit, SetBit, RTC)

### Shipped Stubs -- Allen Bradley

- [ ] **STUB-10**: AB type-check profile stubs (no OOP, no POINTER TO, no REFERENCE TO, tag-based I/O)
- [ ] **STUB-11**: AB timer stubs (TONR, TOFR, RTO -- different names from IEC)
- [ ] **STUB-12**: AB common instructions stubs (ADD, SUB, MUL, DIV, MOV, CMP, EQU, NEQ, GRT, LES, GEQ, LEQ)

### Behavioral Mocks

- [ ] **BMOCK-01**: Shipped behavioral mock for MC_MoveAbsolute (simulates motion with cycle counting)
- [ ] **BMOCK-02**: Shipped behavioral mock for MC_Power (simulates enable/disable with status)
- [ ] **BMOCK-03**: Shipped behavioral mock for MC_Home (simulates homing sequence)
- [ ] **BMOCK-04**: Shipped behavioral mock for MC_Stop (simulates deceleration)
- [ ] **BMOCK-05**: Shipped behavioral mock for ADSREAD (configurable response data)

### Test Integration

- [ ] **TEST-08**: `stc test` auto-defines STC_TEST preprocessor symbol
- [ ] **TEST-09**: `stc sim` auto-defines STC_SIM preprocessor symbol

### Tooling

- [ ] **TOOL-01**: `stc vendor extract <path.plcproj>` extracts FB stubs from TwinCAT project XML files

## Future Requirements

### Community & Ecosystem
- **COMM-01**: Community stub repository (DefinitelyTyped model)
- **COMM-02**: `stc vendor install` command for downloading stubs

### Advanced Mocking
- **AMOCK-01**: Recording mocks that capture call history for assertion
- **AMOCK-02**: Mock expectations (assert FB was called N times with specific params)

### Allen Bradley Emission
- **AB-01**: `stc emit --target allen_bradley` with AOI generation
- **AB-02**: Tag-based variable model mapping for AB output

## Out of Scope

| Feature | Reason |
|---------|--------|
| EtherCAT PDO configuration | Exists outside ST code -- hardware config, not compiler |
| Distributed clocks | EtherCAT infrastructure, not compilable |
| Beckhoff .library parsing | Proprietary binary format, undocumented |
| AB emission in v1.1 | Dialect differences too deep for this milestone; type-checking only |
| VAR_CONFIG remapping | Complex feature, defer to v2 |
| Tc3_JsonXml stubs | Requires METHOD declarations in stubs -- needs design work |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| IO-01 | Phase 12 | Complete |
| IO-02 | Phase 12 | Complete |
| IO-03 | Phase 12 | Complete |
| IO-05 | Phase 12 | Complete |
| VLIB-01 | Phase 13 | Complete |
| VLIB-02 | Phase 13 | Complete |
| VLIB-03 | Phase 13 | Complete |
| VLIB-04 | Phase 13 | Complete |
| VLIB-05 | Phase 13 | Complete |
| MOCK-01 | Phase 14 | Complete |
| MOCK-02 | Phase 14 | Complete |
| MOCK-03 | Phase 14 | Complete |
| MOCK-04 | Phase 14 | Complete |
| MOCK-05 | Phase 14 | Complete |
| IO-04 | Phase 14 | Complete |
| STUB-01 | Phase 15 | Pending |
| STUB-02 | Phase 15 | Pending |
| STUB-03 | Phase 15 | Pending |
| STUB-04 | Phase 15 | Pending |
| STUB-05 | Phase 15 | Pending |
| STUB-06 | Phase 15 | Pending |
| STUB-07 | Phase 16 | Pending |
| STUB-08 | Phase 16 | Pending |
| STUB-09 | Phase 16 | Pending |
| STUB-10 | Phase 16 | Pending |
| STUB-11 | Phase 16 | Pending |
| STUB-12 | Phase 16 | Pending |
| BMOCK-01 | Phase 17 | Pending |
| BMOCK-02 | Phase 17 | Pending |
| BMOCK-03 | Phase 17 | Pending |
| BMOCK-04 | Phase 17 | Pending |
| BMOCK-05 | Phase 17 | Pending |
| TEST-08 | Phase 18 | Pending |
| TEST-09 | Phase 18 | Pending |
| TOOL-01 | Phase 18 | Pending |

**Coverage:**
- v1.1 requirements: 35 total
- Mapped to phases: 35
- Unmapped: 0

---
*Requirements defined: 2026-03-30*
*Last updated: 2026-03-30 after roadmap creation*
