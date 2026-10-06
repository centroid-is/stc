---
phase: 25-ethercat-terminal-models
plan: 03
subsystem: ecat
tags: [ethercat, device-models, el6001, serial, baader]
requires:
  - "25-01 ecat.Layout, devices.Base, knownIDs/beckhoffFallbacks registration"
provides:
  - "devices.SerialPeer {Write([]byte); Read() []byte}, devices.Ticker {Tick()}"
  - "devices.ScriptedPeer (Rules []ScriptRule{Request, Reply, Delay}, Terminator, RequestEnd, Default, Requests())"
  - "devices.BaaderPeer(), BaaderMdReply, BaaderMt1Reply, BaaderMaReply, LoopbackPeer"
  - "devices.EL6001: SetPeer, Peer, SetErrors(parity, framing, overrun), DataLen"
  - "tests/ecat_fixtures/Demo Serial.xml"
affects: [25-04, 27]
tech-stack:
  added: []
  patterns: ["serial peers are pure step-driven state machines: no goroutines, delay counted in Tick calls"]
key-files:
  created:
    - pkg/ecat/devices/serial_peer.go
    - pkg/ecat/devices/serial_peer_test.go
    - pkg/ecat/devices/el6001.go
    - pkg/ecat/devices/el6001_test.go
    - tests/ecat_fixtures/Demo Serial.xml
  modified:
    - pkg/ecat/devices/ids.go
decisions:
  - "TransmitRequest and ReceiveAccepted are edge-detected against their previous value; InitRequest resets TransmitAccepted/ReceiveRequest to 0 as in the Beckhoff init example"
  - "InitRequest also discards bytes the peer sends during init, so FB_SerialFramer.Reinit really yields a clean line"
  - "8-bit Ctrl/Status (small/medium image) carry lengths in bits 4-6 and no error bits; chunks cap at 7 bytes there"
  - "Send continuous (Ctrl bit 3) is not modelled; FB_SerialFramer holds it FALSE"
  - "The mt1 fixture is synthetic, padded to 60 bytes so it spans three 22-byte chunks; protocol.md has no captured mt1 line"
metrics:
  duration: "~20 min"
  completed: 2026-10-06
  tasks: 3
  files: 6
---

# Phase 25 Plan 03: EL6001 Serial Terminal and Scripted Baader Peer Summary

The EL6001 serial terminal now has a model with the Beckhoff toggle handshake and a pluggable byte-stream peer. A scripted Baader peer answers `md` and `mt1` through the 22-byte PDO, driven by a Go port of FB_SerialFramer that touches only the master images.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 | SerialPeer and ScriptedPeer | 3815fa7 (test), 22b4bde (feat) |
| 2 | EL6001 model with the 22-byte handshake | 26c427b (test), 8b676d5 (feat) |
| 3 | Baader md/mt1 exchange through the PDO | e5fd818 (test) |

## Serial API for Phase 27

- **SerialPeer.** `Write(tx []byte)` receives what the terminal put on the wire. `Read() []byte` returns and drains the peer's reply bytes. A peer that also implements `Ticker` gets `Tick()` once per Step, after Write and before Read.
- **ScriptedPeer.** Rules match the whitespace-trimmed request text exactly, in order. Requests end at `RequestEnd`, default CR. Replies get `Terminator`, default `\r\n+`, unless they already end with it. `Delay` counts Ticks, and replies leave in request order. Unknown requests get `Default` or nothing. A lone CR gets nothing. `Requests()` lists what was received.
- **BaaderPeer().** Answers md with `{ 1 0 0 369 0 }\r\n+`, mt1 with a 60-byte synthetic reply and ma with a status line. `LoopbackPeer` echoes.
- **EL6001.** `SetPeer(p)`, `Peer()`, `SetErrors(parity, framing, overrun)` and `DataLen()`. With no peer, transmitted bytes are dropped. Find the device with `Network.DeviceByName`.

**Note for Phase 27.** This plan proves the PDO contract that FB_BaaderSerial relies on, using a Go port of FB_SerialFramer. Running the real FB_BaaderSerial and FB_SerialFramer ST through the interpreter needs the Phase 27 scenario harness.

## Behaviour

- **Layouts.** The model resolves a 16-bit Ctrl/Status word, an 8-bit word for the small/medium image, or the split `Ctrl__*`/`Status__*` entries of the COM PDOs. The data length is the number of `Data Out n`/`Data In n` entries. The real baader.xml export uses the COM PDOs. The export spells the Ctrl bit 3 entry "Send continues", and the model ignores it.
- **Init.** While InitRequest is set, both FIFOs, both toggles, the latched overrun and any bytes from the peer are cleared, and InitAccepted is 1.
- **Transmit.** Each TransmitRequest change sends OutputLength bytes to the peer and toggles TransmitAccepted once. OutputLength is clamped to the Data Out entries (T-25-07).
- **Receive.** Peer bytes go into a 128-byte FIFO. Excess bytes are dropped and latch Overrun, and BufferFull is set at 128 (T-25-08). The next chunk is presented only after the PLC toggles ReceiveAccepted. A 60-byte reply arrives as 22, 22 and 16 bytes.
- **Peer buffer.** ScriptedPeer caps its pending request buffer at 4096 bytes and drops the oldest bytes (T-25-09).

## Verification

- `go test ./pkg/ecat/... -count=1 -cover` passes. Coverage is 99.6% for `pkg/ecat` and 100% for `pkg/ecat/devices`.
- `go vet ./pkg/ecat/...` is clean, `gofmt -l` is empty and `go build ./...` succeeds.
- `TestEL6001BaaderMdMt1` runs over `ecat.NewNetwork` on Demo Serial.xml for both PDO layouts, with and without a 5-step peer delay.

| Exchange | Steps without delay | Steps with 5-step delay | Asserted upper bound |
|----------|---------------------|-------------------------|----------------------|
| md | 3 | 7 | 6 + delay |
| mt1 | 5 | 9 | 10 + delay |

- The full `go test ./...` run shows only the known pre-existing `pkg/checker` `TestEmptyFBCall` failure.
- EL6001 product code 393293906 (0x17713052) was confirmed in baader.xml.
- ECAT-06 is proved at the process-image level.

## Deviations from Plan

**1. [Rule 2 - Correctness] Init discards bytes the peer sends meanwhile**
- **Found during:** Task 2
- **Issue:** Without this, peer bytes queued during init would surface right after it. That defeats FB_SerialFramer.Reinit, which exists to drop stale bytes.
- **Fix:** The init branch drains the peer's Read and discards it.
- **Commit:** 8b676d5

**2. Fixture has two EL6001 slaves.** Demo Serial.xml maps the real COM PDOs on DEMO.A3.01 and the 16-bit-word 22-byte PDOs on DEMO.A3.02, so both status layouts are tested at master image slots. The 8-bit small image is tested on a synthetic Layout.

**3. Task 3 has no RED commit.** It is test-only. The model it exercises was committed in Task 2, so the test passed on first run.

## Known Stubs

None.

## Self-Check: PASSED
