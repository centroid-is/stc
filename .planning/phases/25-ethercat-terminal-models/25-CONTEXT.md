# Phase 25: EtherCAT Terminal Models - Context

**Gathered:** 2026-10-06 · **Status:** Ready for planning · **Mode:** Autonomous, speed-first (worktree ../stc-wt-25, branch gsd/phase-25-terminal-models, based on the Phase 24 branch at 0f48edb). Research folded into planning.

<domain>
## Phase Boundary
Device behaviour models for every non-drive slave in the sildarvinnsla hardware list, registered in `pkg/ecat`'s `Registry` by (VendorId, ProductCode) and selected automatically by `NewNetwork`: digital in/out (EL1008/EL1018, EL2008, EP2338-0002/-1002 8 DI + 8 DO, Festo CTEU outputs), analog in (EL3054 4-20 mA, EL3064 0-10 V: per channel Status WORD + INT), EL9222-5500 (per channel Enabled/Tripped/Hardware Protection/Current Level Warning/Cool Down Lock/Diag/TxPDO State/Input cycle counter + Reset/Switch controls, trip injection and recovery only after Reset pulse), PS2001-2410 (Warning/Error/DC OK/Output voltage/current/Input undervoltage), EL2912/EP1918/EL1904 standard diagnostics (field-voltage bits only), couplers/passive (EK1100/EK1110/EK1200/EL6070/EL9011: no PDO behaviour), unknown devices → Passthrough with a diagnostic, and an EL6001 serial model (22-byte mode: Ctrl/Status WORD + 22 data bytes each way, handshake bits per Beckhoff docs) with a pluggable byte-stream peer so a scripted Baader peer answers `md`/`mt1` with `{ 1 0 0 369 0 }\r\n+`. Each model exposes a Go "stimulus" API (SetInput(ch, bool), SetCurrent(ch, mA), SetVoltage, Trip(ch), SetPSU(...), SerialPeer) that Phase 27 scenarios will drive. Not in scope: ATV320 and Tc2_EtherCAT mocks (Phase 26), scenario files/CLI (27).
</domain>

<decisions>
## Implementation Decisions
- Package `pkg/ecat/devices` with one file per family; `func init()` registers into `ecat.DefaultRegistry` (vendor 2 = Beckhoff; product codes from the real exports: EL1008 0x03f03052, EL2008 0x07d83052, EL9222-5500 0x24063052, PS2001-2410 0x7d13082, EP2338-0002/-1002, EL3054 0x0bee3052, EL3064 0x0bf83052, EL6001 0x17713052, EL2912, EP1918, EL1904, EK1100 0x044c2c52, EK1110, EK1200, EL6070, EL9011; Festo vendor 0x1d product 572556; verify each against the exports under /Users/jonb/Projects/sildarvinnsla/IO List from ethercat/*/Device *.xml and the per-box .xti; where the export's model name is present prefer matching by (vendor, product) and fall back to the model-name regex for unknown revisions).
- Models implement `ecat.Device { Init(*Slave); Step(dt, out, in []byte) }` using the slave's PDO/entry layout (bit offsets from `Slave.Pdos` as laid out by Phase 24) rather than hard-coded offsets, so EL1008 with 8 byte-aligned 1-bit PDOs and EP2338 with packed bits both work. Shared helpers for bit/word access within the slave's own in/out slices.
- Analog: EL3054 4-20 mA → INT 0..32767 over 4..20 mA (Underrange below 4 mA/Overrange above 20 mA, Error bit on open wire = 0 mA); EL3064 0-10 V → 0..32767; status WORD bits: 0 Underrange, 1 Overrange, 2-3 Limit1, 4-5 Limit2, 6 Error, 14 TxPDO State, 15 TxPDO Toggle (toggles each step).
- EL9222-5500: per channel state machine Enabled/Tripped; `Trip(ch)` sets Tripped and clears Enabled; Reset rising edge on the control bit clears Tripped and re-enables after `Switch` is on; Current Level Warning/Cool Down Lock settable via stimulus API; Input cycle counter increments mod 4 each step.
- EL6001 22-byte mode per Beckhoff: output Ctrl WORD (TransmitRequest toggle bit 0, ReceiveAccepted bit 1, InitRequest bit 2, SendContinuous bit 3; OutputLength bits 8-15) + 22 data bytes; input Status WORD (TransmitAccepted bit 0, ReceiveRequest bit 1, InitAccepted bit 2, BufferFull bit 3, ParityError/FramingError/OverrunError bits 4-6, InputLength bits 8-15) + 22 data bytes. Peer interface `SerialPeer { Write(tx []byte); Read() []byte }`; a `ScriptedPeer` maps request strings to responses with the `\r\n+` terminator and optional delay in steps; the Baader `md`/`mt1` example is a test fixture.
- Diagnostics for unknown devices go through `Network` creation (collect `[]diag.Diagnostic` with code ECAT010 "no device model for X (vendor, product); using passthrough").
- Coverage: pkg/ecat not gated, target >= 95% for devices; OS-agnostic; deterministic.
</decisions>

<code_context>
Phase 24 API (worktree already contains it): `ecat.Device`, `NewRegistry`, `Register(vendor, product, func() Device)`, `New(*Slave)`, `DefaultRegistry`, `Passthrough`, `NewNetwork(topo, reg)`, `Network.Step/Images/SetSlaveState/SetWcState/SetDevState/ClearFaults`, `Slave{Vendor, Product, Model, Pdos}`, `Pdo{Dir, Entries}`, `Entry{BitLen, DataType}`, `ReadBits/WriteBits`, fixtures under tests/ecat_fixtures/ (Demo Device 1.xml has EL1008, EL2008, EL9222-5500, EL2912, EK1110, ATV320, EP2338, PS2001, CTEU). Beckhoff docs mirror: /Users/jonb/Projects/beckhoff-docs/ (pdf-text/ for EL30xx, EL60xx, EL92xx). Known pre-existing failure on this branch base: pkg/checker TestEmptyFBCall (fixed on main; ignore).
</code_context>

<specifics>
Plans: 25-01 digital + couplers + registry wiring + diagnostics; 25-02 analog + EL9222 + PSU + safety diag; 25-03 EL6001 serial + ScriptedPeer + Baader fixture; 25-04 gate (every ST301/Baader slave gets a model: env-gated STC_SILD_DIR test over all Device*.xml of ST101/ST201/ST301/baader listing unmatched products; docs; VALIDATION sign-off; coverage gate tolerant of the known failing test).
</specifics>

<deferred>
ATV320/CoE/Tc2_EtherCAT mocks → 26; scenarios/CLI → 27; EL40xx/EL70xx/EL72xx → v2.
</deferred>
