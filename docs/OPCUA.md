# OPC UA Server (`stc serve`)

`stc serve` runs a project's scan on the host and serves its symbols over
OPC UA with the address space of Beckhoff TwinCAT TF6100. An HMI or OPC UA
client written against a real TwinCAT PLC can connect to stc without
changes.

```bash
stc serve "ST301 solution.tsproj" --opcua :4840
stc serve src/ --opcua 127.0.0.1:4840 --cycle 5ms
stc serve app.st gvl.st --security basic256sha256 --cert server.der --key server.pem
```

## What is published

| Item | Value |
|------|-------|
| Root | Objects / DeviceSet (ns=2;i=5001) / PLC1 (`ns=4;s=PLC1`) |
| Namespace | index 4, `urn:BeckhoffAutomation:Ua:PLC1` |
| NodeIds | `ns=4;s=<GVL or PROGRAM>.<path>`, e.g. `ns=4;s=GVL_BatchLines.Drives_Line1[1].HMI` |
| Device properties | DeviceManual, DeviceRevision, DeviceState, Model, SerialNumber under PLC1 |
| Server state | `i=2259` reads Running (0) |

Symbols are published by the TF6100 attribute rules:

| Attribute | Effect |
|-----------|--------|
| `{attribute 'OPC.UA.DA' := '1'}` | Publish the symbol and everything below it |
| `{attribute 'OPC.UA.DA' := '0'}` | Prune the symbol and its subtree |
| `{attribute 'OPC.UA.DA' := '2'}` | Publish a struct as one Variable of its DataType |
| `{attribute 'OPC.UA.DA.Access' := '1'}` | Read-only (`'3'` read-write, the default) |
| `{attribute 'OPC.UA.DA.Description' := '...'}` | Node Description; IEC `$` escapes are decoded |
| `{attribute 'OPC.UA.DA.StructuredType' := '1'}` | On a STRUCT type: instances are one ExtensionObject Variable |

Type-level attributes (on the TYPE, the FUNCTION_BLOCK, or a member inside
it) apply to every instance; the instance declaration wins when both set
the same attribute. Enums get a DataType with EnumStrings, structs marked
StructuredType get a custom DataType with a binary encoding. POINTER and
REFERENCE symbols are never published (diagnostic OPCUA002).

## Scan and consistency

- The scan is the Phase 23 project runtime: every task's PROGRAMs run at
  their task cycles (priority order within a tick), paced by the monotonic
  wall clock. `--cycle` overrides the cycle of a single-task project or of
  the default 10 ms MAIN task; projects with several tasks keep theirs.
- `--io` attaches the EtherCAT network (`Device N.xml` exports) to the
  project's TcLinkTo links, and `--persist` restores and saves
  PERSISTENT/RETAIN variables, exactly as in `stc sim`.
- OPC UA reads come from one scan image: a struct Variable is composed from
  a single consistent snapshot.
- OPC UA writes are validated when submitted (unknown symbol, CONSTANT,
  read-only, type and range errors return Bad status codes) and applied
  between two Ticks, so a program never sees a value change mid-cycle.
- `--realtime` pins the scan to a dedicated OS thread and prints
  `scan: N cycles, M overruns` on exit (text format). Overruns are also
  counted per task in the final status.

Analysis errors stop serve before the server starts; undeclared library
types and members are tolerated as warnings and run as zero-output
auto-stubs. A runtime error stops the scan; the address space keeps
serving the last values until SIGINT/SIGTERM or `--duration`, and the
error is printed (`error: scan stopped: ...`).

## Live writes and subscriptions

A write from a client goes into a queue. The serve loop applies every queued write between two scans, under the runtime lock, through the same coercion as `stc sim --set`. A program therefore never sees a value change in the middle of a scan. A rejected write returns a Bad status code to the client, and with `--format json` a `write_error` event is printed.

TwinCAT HMI function blocks use a command handshake on `p_cmd_*` members:

1. The client writes `p_cmd_Start := TRUE`.
2. The write is applied before the next scan.
3. The function block acts on it and clears `p_cmd_Start` back to FALSE in the same or a later scan.
4. The client sees the effect on a `p_stat_*` member, such as `p_stat_Running`.

Write only `p_cmd_*` and `p_cfg_*` members. Status members are overwritten by the program every scan.

Every read and every subscription sample calls the node's read handler, which returns the value of the last completed scan. There is no cached copy that can go stale. Plan 29-01 adds the test that proves a subscription receives a data change for each changed value.

| Setting | Recommendation |
|---------|----------------|
| Publishing interval | 100 ms or more; a faster interval only repeats the same scan image |
| Sampling interval | Equal to or a multiple of the scan cycle |
| Scan cycle | `--cycle 10ms` for production projects; one ST301 scan takes about 8 ms, so a 1 ms task overruns every scan |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | | Project or ST sources (alternative to arguments) |
| `--opcua` | empty | Listen address `host:port`. The listener binds only `host`; `:4840` or `0.0.0.0:4840` binds all interfaces. Empty starts no server |
| `--security` | `none` | `none` (SecurityPolicy None, Anonymous) or `basic256sha256` (secure only, client certificate identity) |
| `--allow-anonymous-write` | `true` | Let anonymous clients write. `false` limits them to browse, read and subscribe |
| `--allow-anonymous` | `true`, `false` with `basic256sha256` | Accept anonymous clients. Can only be turned off with `basic256sha256` |
| `--cert`, `--key` | generated | Server key pair; a self-signed pair is created once in the PKI dir |
| `--pki-dir` | user cache dir | Where generated certificates live |
| `--cycle` | task cycles, else 10ms | Override the single task cycle |
| `--realtime` | false | Dedicated OS thread, cycle and overrun report |
| `--duration`, `--run-for` | 0 | Stop after a wall-clock duration; 0 runs until SIGINT/SIGTERM |
| `--io` | | EtherCAT exports to attach (repeatable) |
| `--persist`, `--persist-interval` | | State file for PERSISTENT/RETAIN, saved every interval (default 10s) and on stop |
| `-D` | | Preprocessor defines (`STC_SIM` is always defined) |

With `--format json` one line is printed once the server listens:

```json
{"endpoint":"opc.tcp://127.0.0.1:4840","namespace_index":4,"node_count":39,"cycle":"1ms","listen":"127.0.0.1:4840","anonymous_write":true,"diagnostics":[...]}
```

`cycle` is the project's base tick. `listen` is the bound address, `:4840`
for all interfaces. `anonymous_write` tells whether anonymous clients can
write. Runtime events (`warning`, `scan_stopped`,
`write_error`) are then printed to stderr as `{"event": "...", "error": "..."}`,
and on stop the final project status object follows on stdout.

## Security

`stc serve` is a development emulator. With the default `--security none`
any client that can reach the listener can read and write published
symbols, limited only by `OPC.UA.DA.Access`.

- **Binding:** the listener binds only the host in `--opcua`. Use
  `127.0.0.1:4840` to keep the server on this machine. `:4840` and
  `0.0.0.0:4840` bind all interfaces, and stc prints a warning when
  anonymous clients can write there.
- **Anonymous writes:** `--allow-anonymous-write` defaults to `true`
  because the tfc-hmi app connects anonymously and writes commands. Pass
  `--allow-anonymous-write=false` for a read-only anonymous server.
- **Secure mode:** `--security basic256sha256` serves SignAndEncrypt only
  and requires an X509 certificate user identity. Anonymous clients are
  refused unless `--allow-anonymous` is given.

**Known limitation:** client certificates are not verified. The server
trusts any client certificate, for the secure channel and as a user
identity, so `basic256sha256` encrypts traffic but does not restrict who
connects. Run the emulator on an isolated network or behind a host
firewall.

## Testing against the golden address space

`tests/opcua_golden/st301_shape/` is an ST project shaped like the ST301
production HMI symbols. `TestServeST301Parity` serves it and compares a
full browse (NodeClass, BrowseName, DataType, ValueRank, AccessLevel,
Description and custom DataType definitions) byte for byte with
`tests/opcua_golden/st301_shape.json`.

## Connect the Flutter HMI

This is the manual acceptance procedure for the tfc-hmi Flutter app. It needs the app and the proprietary ST301 project, so it runs on your machine, not in CI. `TestHMIKeymappings` is its automated stand-in: it serves the `st301_shape` fixture and reads every node of `tests/opcua_golden/hmi_keymappings.json` with an OPC UA client.

1. Start the emulator on the machine that has the ST301 sources and the EtherCAT exports:

```bash
stc serve "<sild>/ST301/ST301 solution.tsproj" --io "<sild>/ethercat/ST301/Device 1.xml" --io "<sild>/ethercat/ST301/Device 2.xml" --opcua 0.0.0.0:4840 --cycle 10ms --realtime
```

2. Wait for the line `OPC UA server listening on opc.tcp://0.0.0.0:4840 (all interfaces)`. The warning that anonymous clients can write is expected here, because the HMI writes anonymously.
3. Open the tfc-hmi app and go to **Server Config**.
4. Add a server with the endpoint `opc.tcp://<host>:4840`, where `<host>` is the machine running stc.
5. Set the security policy to **None** and authentication to **Anonymous**. Leave the client certificate empty.
6. Save and connect.

The HMI's keymappings need no change. They address nodes in namespace index 4 with string identifiers in the declared case, for example:

| Key | Node |
|-----|------|
| Drive HMI struct | `ns=4;s=GVL_BatchLines.Drives_Line1[1].HMI` |
| Drive state | `ns=4;s=GVL_BatchLines.Drives_Line1[1].HMI.p_stat_State` |
| Sensor raw input | `ns=4;s=sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw` |
| Conveyor speed | `ns=4;s=GVL_BatchLines.Conveyor.rSpeed` |
| Server state | `ns=0;i=2259` |

Checklist:

- [ ] The server shows as connected in Server Config.
- [ ] A drive view shows the HMI struct with live fields, and `p_stat_State` displays as `rdy(2)`.
- [ ] A sensor view shows `p_stat_xRaw`.
- [ ] A conveyor view shows its HMI struct.
- [ ] The server status reads Running from `ns=0;i=2259`.
- [ ] A jog or start button changes the drive's status within a second.

The test passes when every item is checked.

| Symptom | Cause | Fix |
|---------|-------|-----|
| Connection refused or certificate error | The HMI asks for a secured endpoint | Select SecurityPolicy None, or start stc with `--security basic256sha256` and trust its certificate |
| `BadNodeIdUnknown` on some keys | The identifier case differs from the declaration | Use the declared case; NodeIds are case-sensitive |
| Every key under one server fails | The keymapping has a `server_alias` for another server | Point that alias at the stc endpoint |
| Values freeze | A runtime error stopped the scan | Read the `error: scan stopped:` line from stc and fix the program |
| Overruns climb | The task cycle is shorter than the scan | Add `--cycle 10ms` |

## Capture a TF6100 snapshot

The fidelity test compares stc's address space with a real TwinCAT TF6100 server. It needs a snapshot of the real server, which you capture once on the plant network. The command only browses and reads attributes. It never writes values or calls methods, and the file holds no process values.

```bash
stc opcua snapshot opc.tcp://<plc>:4840 --out tests/opcua_golden/st301_real.json
```

- Use `--root` when the PLC node is not `ns=4;s=PLC1`. Browse the server with any client and pass the NodeId of the PLC object.
- Use `--security basic256sha256` when the PLC only offers secured endpoints. stc generates a client certificate in `--pki-dir`, and the PLC must trust it. Pass `--cert` and `--key` to use your own pair.
- Raise `--timeout` for a large address space on a slow link.

The snapshot contains node names and structure from the plant. Committing it is your decision. To keep it out of git, store it elsewhere and point the test at it:

```bash
STC_TF6100_SNAPSHOT=/secure/st301_real.json go test -count=1 -run TestTF6100 ./tests/
```

Without a snapshot the test skips and prints the capture command. With one, every node of the emulated `st301_shape` fixture must exist in the real snapshot with the same node class, data type, value rank, array dimensions, access level and struct definition. When `STC_SILD_DIR` also points at the ST301 sources, a full diff serves the real project and requires equality in both directions.

A failure lists at most 50 differences, one per line, then `... and N more`. Each line has the form `<NodeId> <attribute>: emulated <value>, real <value>`. DataType ids are compared by name, because TF6100 and stc number them differently. Descriptions and browse names are not compared.

## MCP tools

`stc-mcp` lets an LLM agent drive the same project step by step. Start it with the project and, optionally, the exports and an OPC UA listener:

```bash
stc-mcp --project cmd/stc-mcp/testdata/live --io "tests/ecat_fixtures/Demo Device 1.xml" --opcua 127.0.0.1:4840
```

The session is stepped, not free-running, so the same calls always give the same results. With `--opcua`, an HMI or client can watch while the agent steps. Writes from OPC UA clients are applied at the start of the next `stc_sim_step`.

| Tool | Example arguments |
|------|-------------------|
| `stc_sim_step` | `{"cycles": 100}` |
| `stc_sim_read` | `{"paths": ["GVL_Live.nRunCycles", "GVL_Live.conveyor.HMI"]}` |
| `stc_sim_write` | `{"path": "GVL_Live.xSensor", "value": true}` |
| `stc_opcua_browse` | `{"node": "ns=4;s=PLC1", "depth": 2}` or `{"endpoint": "opc.tcp://<plc>:4840"}` |

`stc_sim_write` on a TcLinkTo-bound input forces the slot in the EtherCAT input image, so the value survives the next scan. Remote browse is read-only and stops after 10 seconds. See [CLI_REFERENCE.md](CLI_REFERENCE.md#stc-mcp) for every flag.
