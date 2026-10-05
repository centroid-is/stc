# sildarvinnsla — OPC UA usage map (for TF6100 emulation in stc)

Scope: git-tracked `.TcPOU/.TcDUT/.TcGVL` only (224 files). `.claude/worktrees/track-stations` is a duplicate worktree and inflates raw grep counts ~35%; excluded. Root `hmi/`, `adslog/`, root `assets.json`, `foo.json`, `keymappings.json` are **untracked** local leftovers (still analysed).

## 1. Attribute forms and counts

| Attribute | Count | Values |
|---|---|---|
| `{attribute 'OPC.UA.DA' := '1'}` | 472 | |
| `{attribute 'OPC.UA.DA' := '0'}` | 2 | opt-out (legacy FB_Input TON/TOF) |
| `{attribute 'OPC.UA.DA.Access' := '1'}` | 391 | read-only |
| `{attribute 'OPC.UA.DA.Access' := '3'}` | 7 | read/write (p_cmd_ members) |
| `{attribute 'OPC.UA.DA.StructuredType' := '1'}` | 328 | always '1' |
| `{attribute 'OPC.UA.DA.Description' := '...'}` | 235 | free text, contains `''` escapes |

**Not used anywhere:** `TcRpcEnable`, `OPC.UA.DA.Method`, `.Property`, `.Unit`, `.DisplayName`, `.Deactivate`. METHODs exist (FB_ATV320, FB_Wagon, FB_Conveyor, FB_BatchConveyor, Baader FB_SerialFramer/FB_Fifo, legacy FB_EL1008/FB_EL2008/FB_EL3054) but none carry OPC attributes.
Other attributes in repo: `TcLinkTo` 675, `qualified_only` 73 (sometimes with double quotes: `{attribute "qualified_only"}`), `to_string` 17, `strict` 15.

### Attachment targets (parsed per declaration)

| Target | Count |
|---|---|
| GVL `VAR_GLOBAL` | 329 |
| DUT STRUCT member | 263 |
| FB `VAR` | 120 |
| GVL `VAR_GLOBAL PERSISTENT RETAIN` | 17 |
| POU header (before `FUNCTION_BLOCK X`) | 12 |
| FB `VAR PERSISTENT RETAIN` | 9 |
| DUT header (before `TYPE X :`) | 4 |
| FB `VAR_IN_OUT` | 3 |
| FB `VAR_OUTPUT` | 2 |
| DUT enum value | 2 |
| FB `VAR_INPUT` | 1 |
| PROGRAM MAIN | **0** |

### Attribute combinations per declaration

| Count | Combo | Typical use |
|---|---|---|
| 210 | DA=1 + StructuredType | struct/FB/array vars |
| 109 | Access=1 only | struct members |
| 102 | DA=1 + Access=1 | GVL scalars |
| 80 | DA=1 + Access=1 + Description | |
| 70 | Access=1 + Description | struct members |
| 59 | Description only | `p_cfg_` members (stay writable) |
| 58 | StructuredType only | FB instances in GVLs; POU/DUT headers |
| 20 | DA=1 + Access=1 + StructuredType | |
| 18 | DA=1 only | |
| 8 | DA=1 + Description | |
| 7 | Access=3 (+Description) | p_cmd_ |
| 3 | Access=1 + Description + StructuredType | |
| 2 | DA=0 | |

### Per project

| Project | DA=1 | StructuredType | Access | Description |
|---|---|---|---|---|
| SVNCoreComponents (shared lib) | 30 | 26 | 172 | 115 |
| ST101 | 127 | 65 | 65 | 43 |
| ST201 | 99 | 81 | 22 | 0 |
| ST301 | 106 | 91 | 33 | 19 |
| skammtalinur-legacy | 102 | 65 | 99 | 53 |
| Baader | 8 | 0 | 7 | 8 |
| SVN_Wagons_Temporary | 0 | 0 | 0 | 0 |

By file type: TcGVL DA=1 321 / StructuredType 279 (23 files); TcPOU 132 / 41 (37 files); TcDUT 19 / 8 (34 files).

### Representative quotes

**1. GVL scalar + GVL struct** — `ST301/ST301/GVLs/CVS03.TcGVL`
```
{attribute 'qualified_only'}
VAR_GLOBAL
	CN27_PH01 : BOOL;            // unexposed
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	OptimarInfeedPermitted : BOOL;
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	DomeLight : ST_DomeLight;
END_VAR
```

**2. FB instances in GVL: StructuredType without DA=1, some with nothing** — `ST301/ST301/GVLs/SPB03.TcGVL`
```
{attribute "qualified_only"}
VAR_GLOBAL
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	speedBatcher : FB_SpeedBatcher;
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	CN01_MA01_lift : FB_ConveyorLift;
	CN02_MA01_fb : FB_Conveyor;            // no attribute
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	EmgStop : BOOL;
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	BPM : FB_BPM;                          // FB instance with DA=1
```

**3. StructuredType on FB header; FB exposes retained HMI struct** — `SVNCoreComponents/SVNCoreComponents/DigitalSignals/Sensor/FB_Sensor.TcPOU`
```
{attribute 'OPC.UA.DA.StructuredType' := '1'}
FUNCTION_BLOCK FB_Sensor
VAR_INPUT  i_xRaw : BOOL; ...  END_VAR
VAR_OUTPUT Q : BOOL; ...       END_VAR
VAR PERSISTENT RETAIN
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Sensor_HMI;
END_VAR
```
`ST301/ST301/GVLs/sensors.TcGVL` declares ~40 `FB_Sensor` instances with **no** attributes — exposure is driven from inside the FB.

**4. HMI struct members** — `SVNCoreComponents/.../Sensor/ST_Sensor_HMI.TcDUT`
```
TYPE ST_Sensor_HMI :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	{attribute 'OPC.UA.DA.Description' := 'Unfiltered physical input'}
	p_stat_xRaw : BOOL;
	...
	{attribute 'OPC.UA.DA.Access' := '1'}
	{attribute 'OPC.UA.DA.Description' := 'Time q_xDetected has been TRUE (caps at 1 day)'}
	p_stat_tBlockedFor : TIME;
	{attribute 'OPC.UA.DA.Description' := 'On Delay'}
	p_cfg_tOnDelay : TIME;        // no Access => read/write
END_STRUCT
```

**5. FB_ATV320: PERSISTENT RETAIN + VAR_IN_OUT exposure** — `SVNCoreComponents/SVNCoreComponents/motors/ATV320/FB_ATV320.TcPOU`
```
{attribute 'OPC.UA.DA.StructuredType' := '1'}
FUNCTION_BLOCK FB_ATV320
VAR_INPUT
	amsaddr AT %I* : AMSADDR;
	i_eHMIS AT %I* : hmis_e;
...
VAR PERSISTENT RETAIN
	duRunMinutes : UDINT;
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Drive_HMI;
END_VAR
VAR_IN_OUT
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	param : ST_MotorParams;
END_VAR
```
`ST_Drive_HMI` (same dir): `p_cmd_JogFwd/JogBwd/FaultReset/Auto/Man/Reconfigure : BOOL` with **no** attribute (writable); `p_stat_*` each with `Access := '1'`, including enum-typed `p_stat_State : hmis_e`, `p_stat_LastFault : lft_e`, `p_stat_402_State : states`, `p_stat_RunMode : ATV320_RunMode`; REAL/UDINT/UINT/BYTE stats; `p_cfg_ManualFreq : REAL := 20.0` with Description only. Initial values in structs (`p_stat_xAuto : BOOL := TRUE`).

**6. AT %I*/%Q* struct members with Access** — `SVNCoreComponents/SVNCoreComponents/ECT/ST_EL9222_5500.TcDUT`
```
    {attribute 'OPC.UA.DA.Description' := 'Ch1 Enabled - output is switched on and supplying the load'}
    {attribute 'OPC.UA.DA.Access' := '1'}   // 1 = read-only (device status)
    p_stat_Enabled                 AT %I* : BOOL;
    {attribute 'OPC.UA.DA.Access' := '3'}   // 3 = read/write (operator command)
    p_cmd_Reset                    AT %Q* : BOOL;
```
`ST_EL1008` = `I1..I8 AT %I* : BOOL` each `Access := '1'`. Instantiated in generated `ST301/ST301/GVLs/ECT.TcGVL` (by `generate_gvl.py`):
```
	// ST301.A1.03 (EL1008)
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'TcLinkTo' := '.I1 := TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.03 (EL1008)^Channel 1^Input; .I2 := ...'}
	ST301_A1_03 : ST_EL1008;
```

**7. Arrays with constant-expression bounds + initialisers** — `ST301/ST301/GVLs/ECT_Diag.TcGVL` (generated)
```
	{attribute 'OPC.UA.DA' := '1'}
	Device_1_SlaveCount : UINT := 22;
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	Device_1_SlaveInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [
		(p_stat_sName := 'ST301.A1.01 (EL6070)', p_stat_sModel := 'EL6070', p_stat_nPhysAddr := 1001, ...), ...];
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	Device_1_Diag : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveDiag;
```
`ST_EcSlaveDiag.TcDUT`: `{attribute 'OPC.UA.DA.Access' := '3'} p_cmd_resetLinkLostCounter : BOOL; // HMI sets TRUE -> FB zeroes ..., then resets the flag` — HMI-write / PLC-clear command pattern.

**8. VAR_IN_OUT array + StructuredType on TYPE header** — `ST301/ST301/POUs/wagon/FB_Wagon.TcPOU`, `ST_WagonStation.TcDUT`
```
VAR_IN_OUT
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	station : ARRAY [1..10] OF ST_WagonStation;
---
{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_WagonStation :
STRUCT
	{attribute 'OPC.UA.DA.Description' := 'Indicate if station is source or destination'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	                                   // NB: blank line between attributes and decl
	p_stat_etType : ET_WagonStationType;
```

**9. Flat FB-internal alarm vars** — `ST101/ST101/BER/FB_BER01ScadaPoll.TcPOU` (137 OPC attrs, largest single file)
```
VAR
	{attribute 'OPC.UA.DA.Description' := 'Modbus TCP polling succeeds AND the PLC heartbeat (Puls) is toggling'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	{attribute 'OPC.UA.DA' := '1'}
	p_stat_xModbusHealthy : BOOL;
	... p_stat_xAlmEstop, p_stat_xAlmDriveM101..M105, p_stat_cErrorCount : UDINT ...
```

**10. Opt-out** — `skammtalinur-legacy/sildarvinnsla_plc/POUs/FB_Input.TcPOU`
```
VAR
	{attribute 'OPC.UA.DA' := '0'}
	TON_Filter : TON;
	{attribute 'OPC.UA.DA' := '0'}
	TOF_Filter : TOF;
```

**11. Oddity: attribute on enum value** — `SVNCoreComponents/.../ATV320/hmis_e.TcDUT`
```
{attribute 'qualified_only'}
{attribute 'to_string'}
TYPE hmis_e :
(
  tun := 0,
  {attribute 'OPC.UA.DA' := '1'}
  {attribute 'OPC.UA.DA.Description' := 'Ready'}
  rdy := 2,
  ...
) UINT;
```
TwinCAT propagates it into `ST301/_Config/ST301/ST301 Instance.xtv` `<EnumInfo><Properties><Property><Name>OPC.UA.DA</Name>...`; almost certainly no server effect.

**12. Legacy GVL actually used by the HMI** — `skammtalinur-legacy/sildarvinnsla_plc/GVLs/GVL_BatchLines.TcGVL`
```
{attribute "qualified_only"}
VAR_GLOBAL
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	Drives_Line1 : ARRAY [1..2] OF FB_ATV320;
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	speedBatchers : ARRAY [1..3] OF FB_SpeedBatcher;
	{attribute 'OPC.UA.DA.StructuredType' := '1'}
	air_1_71 : FB_AirCabinet;
	Line1_Conveyor2_fb : FB_Conveyor;
	{attribute 'OPC.UA.DA' := '1'}
	{attribute 'OPC.UA.DA.Access' := '1'}
	line1Run : BOOL;
```
HMI reads `GVL_BatchLines.Drives_Line1[1].HMI` — i.e. array element of FB (StructuredType only) → inner `HMI` (DA=1).

## 2. Where exposed vars live, naming, types

- **Location:** GVLs (almost all `qualified_only`) and inside FB instances declared in GVLs. **No PROGRAM MAIN exposes anything** (checked ST101/201/301/legacy/Baader MAIN).
- **Nesting:** real paths are `GVL.fbOrArray[i].HMI.p_stat_X`. Intermediate FB instances carry only `StructuredType` or nothing; only the inner var has `DA=1`. Rule for emulator: expose a var if it or any descendant has `DA=1`; create parent nodes for FB instances/array elements on the path. Struct members of a DA=1 struct are exposed without their own DA.
- **Prefix convention** (declarations only):

| Prefix | Access | Count |
|---|---|---|
| p_stat_ | '1' | 219 |
| p_stat_ | none | 20 |
| p_cmd_ | none | 30 |
| p_cmd_ | '3' | 6 |
| p_cfg_ | none | 14 |
| p_cfg_ | '1' | 2 |

  `p_stat_` = read-only status, `p_cmd_` = HMI-writable command (often HMI sets TRUE, FB clears), `p_cfg_` = writable config (often persisted with `PERSISTENT RETAIN`). Not enforced: 22 exceptions (e.g. `ST_PButton_HMI.p_stat_feedback` no Access; `ST_PS2001_2410_HMI.p_stat_* AT %I*` no Access; Baader `FB_HeightMeasureToDigital.p_cfg_onDelay` Access=1). Plain GVL scalars use Hungarian/no prefix (`Run`, `EmgStop`, `xErrorActive`, `line1Run`); FB I/O uses `i_`/`q_` (`q_xOk`, `q_rRPM`, `i_xRoeSwitchEnabled` exposed directly).
- **Types exposed** (declaration sites with OPC attrs): BOOL 289, UINT 40, REAL 36, UDINT 18, TIME 15, BYTE 4, WORD 3, INT 2, STRING/STRING(1)/(32)/(80) 5, DT 1; enums `hmis_e`, `lft_e`, `states`, `ATV320_RunMode`, `ecfg_e`, `dir_e`, `E_EcSlaveState`, `ET_WagonStationType`, `ET_WagonLocation`; STRUCTs `ST_EL9222_5500` 65, `ST_EL1008` 34, `ST_EP2338_0002` 34, `ST_EL2008` 31, `ST_PS2001_2410` 16, `ST_Conveyor` 13, `ST_*_HMI`, `ST_LineRecipe`, `ST_DomeLight`, `ST_MotorParams`, `ST_CounterRatio`, `ST_BPM`; ARRAYs 33 (of structs, of FB_ATV320, `ARRAY[1..10] OF ST_Item`); FB instances FB_ConveyorLift, FB_AirCabinet, FB_SpeedBatcher, FB_BatchPacking, FB_BPM, FB_ATV320, FB_MovingUnit, FB_Sensor, FB_EL1008/2008/3054, even TON/TOF. Structs can contain FB instances (`ST_StrappingLine_HMI.p_stat_LastSensor : FB_Sensor`, `p_stat_StrappingMachines : ARRAY[1..2] OF ST_StrapX`).

## 3. HMI (Flutter) client

- Root `hmi/` is untracked: no `pubspec.yaml`, only `lib/foo-main.dart` (imports `package:tfc/...` — tfc-hmi pages: page_editor, alarm_view, history_view, server_config, dbus_login), build dirs, `hmi/keymappings.json` (428 entries), `hmi/page-editor.json`. Pods include `open62541_libs` 0.0.5.
- Tracked app: `skammtalinur-legacy/hmi` (Flutter, name `sildarvinnslan`, "Connects to OPC UA servers and PostgreSQL databases"). `pubspec.yaml` deps:
  ```yaml
  open62541:
    git: { url: https://github.com/centroid-is/open62541_dart.git, ref: main }
  tfc:
    git: { url: https://github.com/centroid-is/tfc-hmi.git, ref: main }
  ```
  → **OPC UA client via open62541 C library through Dart FFI.** No ADS in the HMI.
- Deployment `skammtalinur-legacy/docker-compose.yml`: `ghcr.io/centroid-is/sv-hmi` under Weston/Wayland, mounts `./opcua_certs:/certs:ro` (client certs ⇒ secured endpoint likely), TimescaleDB for history (`collect`). File contains plaintext passwords (not reproduced).
- No endpoint URL / port 4840 / security policy in repo; server list configured at runtime in tfc "Server Config" page (shared preferences). Server aliases seen: default (PLC), `speedbatcher1..3`, `__aggregate`.
- **Node id format:** numeric namespace + string identifier = `ns=4;s=<GVL>.<var>[idx].<member>`. No PLC-name prefix (e.g. no `PLC1.`), no `nsu=` form, no MAIN paths. Case as declared (old keymappings had `conv_1_09_1.p_stat_state` vs current `p_stat_State` — identifiers are case-sensitive on the wire).

### keymappings.json schemas

`hmi/keymappings.json` (current tfc schema): `{"nodes": {<key>: {opcua_node, m2400_node, io, collect}}}`
```json
"Line1.Motor1": {
  "opcua_node": {"namespace": 4, "identifier": "GVL_BatchLines.Drives_Line1[1].HMI",
                 "array_index": null, "server_alias": null},
  "m2400_node": null, "io": null,
  "collect": {"key": "Line1.Motor1", "name": "Line1.Motor1",
              "retention": {"drop_after_min": 525600, "schedule_interval_min": null},
              "sample_interval_us": 5000000, "sample_expression": null}
},
"Line1.Motor1.Error": {"opcua_node": {"namespace": 4,
   "identifier": "GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error", ...}, "collect": null},
"SB1.Checkweigher1.CurrentWeight": {"opcua_node": {"namespace": 2,
   "identifier": "Checkweigher1.CurrentWeight", "server_alias": "speedbatcher1"}, ...},
"__agg_default_connected": {"opcua_node": {"namespace": 1,
   "identifier": "Servers/Status/OpcUa/default/connected", "server_alias": "__aggregate"}},
"Server.State": {"opcua_node": {"namespace": 0, "identifier": "2259"}},
"Subdevice.3": {"opcua_node": {"namespace": 4, "identifier": "GVL_BatchLines.Internal_Bus_3.hmi"}, "io": true},
"weigher1v.weight": {"opcua_node": null,
   "m2400_node": {"record_type": "recStat", "field": "weight", "server_alias": "weigher1v", "status_filter": null}, ...}
```
Stats: 319 ns=4 PLC (289 unique ids; roots GVL_BatchLines 191, GVL_Baader 58, GVL_Roe 57, GVL_GLOBAL 13 — all the **legacy** project); 60 ns=2 on speedbatcher1/2/3 (separate OPC UA servers); 8 ns=1 `__aggregate` (tfc aggregating proxy status); 1 ns=0 i=2259 (ServerStatus.State); 40 m2400 weigher nodes; 11 `io:true`; 140 with `collect`. Note `identifier: "2259"` is a numeric id stored as string for ns=0.

Sample PLC identifiers:
```
GVL_BatchLines.Drives_Line1[1].HMI                 (whole struct, read as one ExtensionObject)
GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error    (member of same struct)
GVL_BatchLines.speedBatchers[3].hmi.p_stat_EmgGateTripped
GVL_BatchLines.packing[3].hmi.p_stat_Run
GVL_BatchLines.Internal_Bus_8.processed_state
GVL_Baader.baaderCounters[10].slot_counter.Minute1
GVL_Baader.Baader10_1.hmi.descriptions
GVL_Roe.distributionAugerRpm.q_rRPM
GVL_BatchLines.air_1_72.p_cmd_button
GVL_GLOBAL.missing_one_frame
```

Root `keymappings.json` (older schema, 73 entries): `{"nodes": {key: {"nodeId": {"namespace": 4, "identifier": "..."}, "collectSize": 1000, "pollIntervalUs": 100000, "io"?: ...}}}`. `keymappings.json.before`: 19 entries, same shape.

Root `assets.json`: `{"assets": [{asset_name: "SpeedBatcherConfig"|"ConveyorConfig"|..., page_name, coordinates{x,y,angle}, size{width,height}, label, key: "SB1", gateKey: "SB1.EmgGateTripped", ...}]}` — keys reference keymapping keys, not node ids (37 keys). `hmi/page-editor.json` same idea (102 keys, with `$current_baader_machine.*` templating).

`foo.json`: debug dump of `CollectedSample(value: HMI Hmi bits {p_cmd_JogFwd: false, ..., p_stat_State: rdy(2), p_stat_LastFault: cnf(7), p_stat_402_State: switched_on(4), p_stat_Frequency: 0.0, p_stat_SlaveId: 1014, p_cfg_AutoFreq: 20.0}, time: ...)` — proves the client reads StructuredType vars as one ExtensionObject decoded by field name, and resolves **enum value names** from server type metadata.

## 4. adslog / Baader (ADS, plus OPC UA in adslog)

**Baader collector** — `Baader/collect/ads.py`, `baader_service.py`: **ADS via pyads** (no OPC UA). `requirements-windows.txt`: pyads, pyodbc, pywin32. Writes to Postgres/TimescaleDB or MSSQL.
```python
if self.local_net_id:
    pyads.set_local_address(self.local_net_id)
c = pyads.Connection(self.net_id, self.port, self.ip)
c.open()
value = self._conn.read_structure_by_name(name, structure_def)   # struct streams
self._conn.read_by_name(name, plc_type)
self._conn.write_by_name(args.rt_ack_var, rt_committed, pyads.PLCTYPE_UINT)  # seq/ack
```
`baader.env.example`: `BAADER_AMS_NET_ID=5.170.142.46.1.1`, `BAADER_AMS_PORT=851`, optional `BAADER_TARGET_IP`, `BAADER_LOCAL_AMS_NET_ID`; symbols `register.realTimeToRegister`/`...Ack`, `register.statToRegister`/`...Ack`, `register.statusToRegister`/`...Ack`. Struct layouts hand-declared as pyads tuples (`("ok_trays", pyads.PLCTYPE_UDINT, 1)` …). Exactly-once seq/ack handshake (PLC holds sample until acked UINT).

**adslog/** — untracked, only `.pyc` (no source). Modules: `client`/`stream` (pyads, `read_structure_by_name`, StreamSpec seq/ack streams), `tags` (polled tags from JSON: `"symbol": "GVL_Baader.Machine[1].okTrays"`, `sample_interval_ms`), `opcua` (`asyncua.sync.Client`, lazy import; "Read one tag by its string node id (ns=N;s=... / ns=N;i=...)"), `sources.MultiSource` ("Dispatches reads by binding kind ("ads", "opcua", ...)" so one tags.json mixes protocols), `db`, `runner`, `service`. Config flags `--ams-net-id`, `--ams-port`, `--ip`, `--local-ams-net-id`, `--db-backend postgres|mssql`.

No `*.sql` files tracked; only YAML is legacy docker-compose + `.github/workflows/skammtalina-hmi.yml` (CI, no PLC addressing).

## 5. TF6100 server configuration

None in repo: no `TcUaServer`, `TF6100`, `TcOpcUaServer`, `.tcopcuaserverconfig`, `UaServer`, `opc.tcp` hits. Server runs with Beckhoff defaults. PLC project names (determine TF6100 PLC namespace URI): `ST101`, `ST201`, `ST301`, legacy `sildarvinnsla_plc`, `gagnasofnun` (Baader), `wagon`. HMI uses namespace index **4** for the PLC. Attributes are persisted by TwinCAT into instance files, e.g. `ST301/_Config/ST301/ST301 Instance.xtv`: `<Properties><Property><Name>OPC.UA.DA</Name><Value>1</Value></Property>…`. ST301 target NetId `10.1.177.66.1.1`, PLC task cycle 10 ms (`ST301 solution.tsproj`).

## 6. Methods / RPC / StructuredType

- **No RPC methods**: zero `TcRpcEnable` / `OPC.UA.DA.Method`. Server is data-only.
- **StructuredType heavily used (328)**: on variables (struct/FB/array types), and as type-level pragma before `FUNCTION_BLOCK` (FB_Sensor, FB_Section, FB_ATV320, FB_ConveyorLift, FB_Reposition, FB_Wagon, FB_BER01ScadaPoll; legacy FB_EL1008, FB_EL2008, FB_OutputRpm, FB_ConveyorLift, FB_ATV320) and before `TYPE` (ST_WagonStation; legacy ST_EL10XX_HMI, ST_EL20XX_HMI, ST_EL3054_HMI — legacy ones with both StructuredType and DA=1 on header).

## Implications for an stc TF6100 emulator (inference; not verified against live TF6100)

1. Namespace index 4, string NodeIds `<GVL>.<path>` with `[i]` array indices, declared case; also serve `ns=0;i=2259`.
2. Exposure: DA=1 on var, inherited by members of exposed struct; parent FB/array nodes created when a descendant is exposed; honour DA=0.
3. AccessLevel: missing or '3' → read/write; '1' → read-only. Description → node Description.
4. StructuredType: struct node value = ExtensionObject with DataTypeDefinition; members also individually addressable; enums must expose EnumStrings/EnumValues so clients decode names (`rdy(2)`).
5. Parser must accept: attributes in STRUCT bodies, on enum values, before FUNCTION_BLOCK/TYPE headers, with blank lines between pragma and decl, double-quoted attribute names, `''` escapes, `VAR PERSISTENT RETAIN`, `VAR_GLOBAL PERSISTENT RETAIN`, `VAR_IN_OUT` with attributes, `AT %I*/%Q*` inside struct members, multi-member `TcLinkTo`, constant-expression array bounds with struct-array initialisers.
