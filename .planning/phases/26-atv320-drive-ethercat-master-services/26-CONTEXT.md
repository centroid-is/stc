# Phase 26: ATV320 Drive & EtherCAT Master Services - Context

**Gathered:** 2026-10-06 · **Status:** Ready for planning · **Mode:** Autonomous, speed-first (worktree ../stc-wt-26, branch gsd/phase-26-atv320, based on the Phase 24 branch at 0f48edb). Research folded into planning.

<domain>
## Phase Boundary
(1) An ATV320 device model in `pkg/ecat/devices` (vendor 0x0800005a, product 0x389): CiA402 state machine on CMD (0x6040) / ETA (0x6041): Not ready → Switch on disabled (0x40) → Ready to switch on (0x21) → Switched on (0x23) → Operation enabled (0x27) → Quick stop active (0x07) → Fault (0x08/0x0F) → Fault reaction; transitions on CMD bits (Switch on, Enable voltage, Quick stop, Enable operation, Fault reset bit 7, Halt bit 8); LFR (0x2037:03, 0.1 Hz) → RFR ramps toward LFR at ACC/DEC (0x203C:02/03, 0.1 s to reach HSP); LCR current model (0x2002:05, 0.1 A: idle 0, proportional to |RFR|); HMIS (0x2002:29: 0 tun, 1 rdy, 2 nst, 3 run, 4 acc, 5 dec, 6 cli, 7 fst, 8 fl, 9 nlp, 10 ctl, 11 obr, 12 soc, 13 usa, 14 tc, 15 st; use the sildarvinnsla `hmis_e` enum values in SVNCore as the source of truth), LFT fault codes (0x2029:16 per `lft_e`), DI (0x2016:03 logic inputs) and OL1R (0x2016:0D relay/logic outputs) passthrough; a CoE object dictionary (0x6040/0x6041/0x6060/0x6061, 0x2002:xx, 0x2016:xx, 0x2029:xx, 0x2032:01 EEPROM save with a step delay, 0x2037:03, 0x203C:02/03, plus every parameter FB_ATV320's FB_Parameter instances write: NPR, UNS, NCR, FRS, DCF, NSP, COS, ITH, CLI, HSP, LSP, LFA, RSA, TRA, SSB; look up indices in the ATV320 CoE map inside SVNCore's FB_Parameter/FB_ReadParameter and the custom ESI at /Users/jonb/Projects/sildarvinnsla/custom esi files/); stimulus API: `InjectFault(lft)`, `ClearFault()`, `SetDI(bits)`, `SetSTO(bool)`. (2) Tc2_EtherCAT behavioural mocks (`pkg/interp` StandardFB implementations or ST-based mocks under stdlib/mocks/beckhoff) backed by the simulated `ecat.Network`: FB_EcGetSlaveState / FB_EcGetAllSlaveStates (ST_EcSlaveState{deviceState, linkState}), FB_EcSetSlaveState (transition with configurable step delay; PreOp→OP), FB_EcGetMasterState (DevState), FB_EcGetAllSlaveCrcErrors / FB_EcGetSlaveCrcErrorEx (counters), FB_EcPhysicalWriteCmd (clears CRC registers 0x0300..0B), FB_EcCoESDoRead / FB_EcCoESDoWrite (route to the addressed slave's object dictionary by AMS port = slave address; bBusy for N scans then bDone/bError with nErrId), ADSREAD/ADSWRITE unchanged. Acceptance: SVNCore's unmodified FB_ATV320 bound to the model walks cfgWaitForPreOp → cfgWriteParams → cfgCommitEEPROM → cfgPromoteOp → cfgReady; a run command drives ETA to Operation enabled, RFR ramps, HMIS = run; InjectFault sets Fault with the LFT code and FB_ATV320 surfaces q_xError/p_stat_LastFault; FB_EcDeviceDiag fills ECT_Diag.Device_N_Diag over several scans. Not in scope: scenario files/CLI (27), OPC UA (28).
</domain>

<decisions>
- Slave "Final State = PreOp" semantics: the ATV320 model starts in PreOp (InfoData.State 0x2) and only reaches OP via FB_EcSetSlaveState, matching the real slave configuration FB_ATV320 requires; other slaves start in OP (Phase 24 default).
- Tc2_EtherCAT mocks live in Go as `interp.StandardFB`s registered when a `Network` is attached (`engine.SetNetwork(net)`), with the slave resolved from `nSlaveAddr` (EtherCAT address 1001+i) and the master from `sNetId`; async completion after a fixed `MockLatency` of 2 scans; errors use Beckhoff ADS codes (0x745 timeout, 0x1 etc.) where known.
- CoE SDO reads of unknown objects return error 0x06020000 (object does not exist); writes store values; the parameter store starts with sane defaults (NPR etc.) and `0x2032:01 := 1` completes after 5 steps and resets to 0.
- Tests: Go unit tests for the state machine and ramps; an ST test suite under tests/twincat_dialect or tests/ecat/ that instantiates a trimmed FB_ATV320-like configurator is acceptable, but the gate must run the REAL FB_ATV320 (env-gated STC_SILD_DIR: parse SVNCore's FB_ATV320.TcPOU + DUTs via the Phase 21 converter if available on this branch, otherwise via the flattened probe svncorecomponents.st) bound to the model through Phase 24's IOBinder, stepping the scan until cfgReady, then commanding i_xFwd and asserting HMIS/ETA/RFR.
- Coverage: pkg/ecat/devices and new interp files >= 95%; interp gate 95%.
</decisions>

<code_context>
Phase 24 API in this worktree: `ecat.Device`, `Register`, `DefaultRegistry`, `NewNetwork`, `Network.Step/Images/SetSlaveState/SetWcState`, `StateOP`, `FirstPort`, `interp.NewIOBinder(bindings, net)`, `engine.SetIOBinder`, Tick hooks (net.Step then inputs copied, outputs copied after). Stubs for Tc2_EtherCAT signatures: on main (Phase 21, stdlib/vendor/beckhoff/tc2_ethercat.st) — copy the signatures from /Users/jonb/Projects/stc/stdlib/vendor/beckhoff/tc2_ethercat.st (read-only) so the mocks match. FB_ATV320 source: /Users/jonb/Projects/sildarvinnsla/SVNCoreComponents/SVNCoreComponents/motors/ATV320/*.TcPOU and *.TcDUT (read-only). Known pre-existing failure on this branch base: pkg/checker TestEmptyFBCall (fixed on main; ignore).
</code_context>

<specifics>
Plans: 26-01 ATV320 model (CiA402 + ramps + HMIS/LFT + DI/OL1R + stimulus API); 26-02 CoE object dictionary + parameter defaults + EEPROM save + PreOp start; 26-03 Tc2_EtherCAT mocks on Network (state, master, CRC, SDO, physical write) with async latency; 26-04 gate: real FB_ATV320 + FB_EcDeviceDiag end-to-end (env-gated), docs, VALIDATION, coverage gate.
</specifics>

<deferred>
Scenario files and `stc sim --io` → 27; OPC UA → 28; ADS server → v2.
</deferred>
