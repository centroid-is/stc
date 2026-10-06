// Package devices holds behavioural EtherCAT slave models that plug into
// ecat.Network through ecat.DefaultRegistry. Importing the package registers
// every model; unknown slaves stay ecat.Passthrough.
//
// # ATV320
//
// ATV320 models a Schneider Electric Altivar 320 with the EtherCAT option,
// registered for VendorId 0x0800005A and ProductCode 0x389. It reads and
// writes its process data by entry name through ecat.LayoutAware, so it
// follows the master's real layout (tests/ecat_fixtures/atv320_device.xml
// carries the ST301 mapping: CMD, LFR, OL1R, ACC, DEC out; ETA, RFR, LCR,
// DI, LFT, HMIS in).
//
// EtherCAT state: the drive boots in PreOp, like a slave whose Final State
// is PreOp, and reaches a requested state (RequestState, or
// FB_EcSetSlaveState through the network services) after
// ecat.Network.StateDelay steps. Below OP all input entries read zero, the
// CiA402 machine holds and the motor stops.
//
// CiA402: CMD drives the CiA402 state machine (shutdown 16#06, switch on
// 16#07, enable operation 16#0F, fault reset on a rising bit 7, quick stop,
// halt on bit 8). ETA reports the standard status words, for example
// 16#0250 switch on disabled, 16#0231 ready, 16#0233 switched on, 16#0237
// operation enabled and 16#0218 fault; masked with 16#6F that is 16#50,
// 16#21, 16#23, 16#27.
//
// Motion: units are the drive's own, 0.1 Hz for LFR, RFR, HSP and LSP,
// 0.1 s for ACC and DEC and 0.1 A for LCR. In operation enabled RFR ramps
// toward LFR clamped to [LSP, HSP] at HSP per ACC (or DEC) seconds, in
// integer math with a carried remainder so runs are deterministic. ACC and
// DEC come from the PDO when nonzero, else from the object dictionary.
// HMIS reports RDY, NST, RUN, ACC, DEC, FST, FLT or STO from the CiA402
// state and the ramp; LCR follows a load estimate from NCR and RFR/FRS.
//
// Object dictionary (SDO, ecat.CoEDevice): the CiA402 objects
// 16#6040/16#6041/16#6060/16#6061, live mirrors of RFR, LCR, HMIS, LFT and
// DI, the PDO outputs, ACC 16#203C:02 and DEC 16#203C:03, HSP 16#2001:05,
// LSP 16#2001:06, the FB_ATV320 motor block parameters under 16#2042,
// 16#2052 and 16#203E, and the EEPROM save object 16#2032:01 (a nonzero
// write commits after a few steps; SaveCount counts commits). Unknown
// objects abort with ecat.AbortNoObject.
//
// Stimulus API for tests: InjectFault(lft) shows fault reaction active on
// the next step, then Fault, with LFT set to lft; ClearFault removes the
// cause, after which a CMD bit 7 rising edge resets the drive. SetDI sets
// the digital input word and SetSTO(true) forces Safe Torque Off (HMIS
// STO, power stage disabled). Getters RFR, ETA, HMIS, LFT, LCR, OL1R,
// State and EcState expose the current values.
//
// # Tc2_EtherCAT mocks
//
// The models are reached from ST through the Tc2_EtherCAT function blocks
// that pkg/interp mocks on the same network. Call
// (*interp.ScanCycleEngine).SetNetwork(net) before the first Tick (and
// before registering files that instantiate FBs), and pass the same net to
// interp.NewIOBinder so the network steps once per scan. Every mocked
// request (FB_EcGetSlaveState, FB_EcSetSlaveState, FB_EcCoESdoRead and
// FB_EcCoESdoWrite, the CRC and master state blocks, ...) is busy for
// interp.MockLatency scans, 2, before it reports done or an ADS or CoE
// abort code. With that wiring the unmodified SVNCoreComponents FB_ATV320
// walks its configurator from PreOp to cfgReady and runs the drive, and
// FB_EcDeviceDiag fills its diagnostics.
package devices
