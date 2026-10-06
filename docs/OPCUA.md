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

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--project` | | Project or ST sources (alternative to arguments) |
| `--opcua` | empty | Listen address such as `:4840`; the listener binds all interfaces. Empty starts no server |
| `--security` | `none` | `none` (SecurityPolicy None, Anonymous) or `basic256sha256` (secure only) |
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
{"endpoint":"opc.tcp://127.0.0.1:4840","namespace_index":4,"node_count":39,"cycle":"1ms","diagnostics":[...]}
```

`cycle` is the project's base tick. Runtime events (`scan_stopped`,
`write_error`) are then printed to stderr as `{"event": "...", "error": "..."}`,
and on stop the final project status object follows on stdout.

## Security

`stc serve` is a development emulator. With the default `--security none`
any client on the network can read and write published symbols, limited
only by `OPC.UA.DA.Access`. A loopback host in `--opcua` only
changes the advertised URL; the listener always binds all interfaces. Use
`--security basic256sha256` when the host is reachable by others. Client
certificates are not verified.

## Testing against the golden address space

`tests/opcua_golden/st301_shape/` is an ST project shaped like the ST301
production HMI symbols. `TestServeST301Parity` serves it and compares a
full browse (NodeClass, BrowseName, DataType, ValueRank, AccessLevel,
Description and custom DataType definitions) byte for byte with
`tests/opcua_golden/st301_shape.json`.
