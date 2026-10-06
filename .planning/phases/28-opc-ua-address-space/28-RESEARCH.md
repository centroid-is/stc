# OPC UA Server Library Spike: awcullen/opcua for a TF6100 Emulator (stc Phase 28)

Date: 2026-10-06
Prototype: `/private/tmp/claude-501/-Users-jonb-Projects-stc/3a97ec58-502a-4334-8f74-c078fd46a32a/scratchpad/opcua-spike/` (scratch, not in any repo)

## Verdict: GO for awcullen/opcua

Every requested item works with running code, including the key unknown (item 3).
- A structured DataType can be created **at runtime** with no compile-time Go struct.
- Its `DataTypeDefinition` (StructureDefinition) is served as a native attribute.
- Two independent generic clients decode the value by field name using only that definition: a hand-written gopcua walker and python-asyncua's `load_data_type_definitions()`. asyncua uses the same mechanism as the open62541 client the sildarvinnsla HMI runs on.

The NodeSet2 fallback is **not** needed. It would also not have worked, because awcullen's NodeSet loader drops `<Definition>` (see Gotchas).

The main risks are maintenance cadence and a process-global type registry. Both are manageable and covered below. No blockers.

### Results (`go build -o bin/spike ./cmd/spike && ./bin/spike`)

```
[PASS] 1. connect None/Anonymous                        opc.tcp://localhost:48400
[PASS] 1. NamespaceArray[4] == PLC1 urn                 [".../UA/" "urn:stc:tf6100-emu" ".../UA/DI/" "http://PLCopen.org/OpcUa/IEC61131-3/" "urn:BeckhoffAutomation:Ua:PLC1"]
[PASS] 2. browse Objects/GVL_Test/fbMotor/HMI           HMI=ns=4;s=GVL_Test.fbMotor.HMI members=[p_cmd_JogFwd p_stat_State p_stat_Frequency]
[PASS] 2. p_stat_State Int32 enum + EnumStrings         value=2 dt=ns=4;s=DT.hmis_e label="Running" (5 strings)
[PASS] 2. p_stat_Frequency Float                        float32 1.5
[PASS] 2. p_cmd_JogFwd AccessLevel = Read|Write (3)     3
[PASS] 2. GVL_Test.arr Int16[] + ArrayDimensions        value=[1 2 3 4 5] ([]int16) dims=[5] rank=1
[PASS] 2. Description attribute                         Text:Output frequency [Hz]
[PASS] 3. generic decode ST_Drive_HMI by name           map[p_cmd_JogFwd:false p_stat_Frequency:1.5 p_stat_State:2]
[PASS] 3. generic decode nested STRUCT/ARRAY/STRING     map[inner:map[i_Count:7 s_Name:conveyor] state:4 vals:[1.25 2.5 3.75]]
[PASS] 3. generic whole-struct write + read back        write=StatusGood readback=map[inner:map[i_Count:99 s_Name:written] state:1 vals:[9 8 7]]
[PASS] 4. data-change notifications (2s @100ms)         18 notifications, first=1.5 last=10
[PASS] 5. write p_cmd_JogFwd=true + read back           write=StatusGood readback=true
[PASS] 5. struct view reflects member write             map[p_cmd_JogFwd:true p_stat_Frequency:11.5 p_stat_State:2]
[PASS] 5. write read-only -> BadNotWritable             StatusBadNotWritable (0x803B0000)
[PASS] 5. write wrong type -> BadTypeMismatch           StatusBadTypeMismatch (0x80740000)
[PASS] 6. i=2259 ServerStatus.State                     0 (int32) (0=Running)
[PASS] 6. ServerStatus.BuildInfo.ProductName            TF6100 emulator
ALL CHECKS PASSED
```

Independent client check with python-asyncua 2.0.1 (`asyncua_check.py`, run against `bin/server`):

```
loaded types: ['Date', 'ST_Drive_HMI', 'ST_Inner', 'ST_InnerTwin', 'ST_Outer', 'Time', 'hmis_e']
HMI: ST_Drive_HMI(p_cmd_JogFwd=False, p_stat_State=<hmis_e.Running: 2>, p_stat_Frequency=7.5)
outer: ST_Outer(inner=ST_Inner(i_Count=7, s_Name='conveyor'), vals=[1.25, 2.5, 3.75], state=<hmis_e.Fault: 4>)
outer after write: ST_Outer(inner=ST_Inner(i_Count=123, s_Name='from-asyncua'), ...)
read-only write rejected: BadNotWritable
ServerStatus.State: 0
```

## Pinned versions and footprint

| Item | Value |
|---|---|
| Server | `github.com/awcullen/opcua v1.4.0` (released 2024-12-31, last push 2026-02-08, 114 stars, 11 open issues, not archived) |
| License | MIT (Converter Systems LLC) |
| Test client | `github.com/gopcua/opcua v0.9.1` (MIT). Test-only for stc. awcullen pulls v0.6.1 for its own benchmark test, and `go mod tidy` silently picks v0.6.1 unless you `go get github.com/gopcua/opcua@v0.9.1` explicitly. |
| awcullen runtime deps | djherbis/buffer, gammazero/deque, gammazero/workerpool, google/uuid, pkg/errors, golang.org/x/crypto. All small and pure Go, with no cgo. |
| Go version | awcullen needs go 1.22; stc is on go 1.25.0, so they are compatible. The spike ran on go1.26.0. |
| Binary size | hello-world 2.48 MB, emulator 12.79 MB. Stripped (`-s -w`) that is 1.66 MB vs 9.73 MB. The delta is about +10 MB unstripped and +8 MB stripped. It includes the 2.9 MB embedded ns0 `nodeset_1_04.xml`. The marginal cost inside the stc binary will be lower because stdlib crypto/net are shared. |
| Startup / memory | 162 ms until the port accepts connections, mostly parsing the ns0 NodeSet. Idle RSS is about 30 MB. |

## API recipe per item

All snippets come from `emu/server.go` and `demo/model.go` in the prototype.

### 1. Server, SecurityPolicy None + Anonymous, namespace at index 4

```go
perms := []ua.RolePermissionType{ // Anonymous has no Write by default!
    {RoleID: ua.ObjectIDWellKnownRoleAnonymous, Permissions: ua.PermissionTypeBrowse | ua.PermissionTypeRead | ua.PermissionTypeWrite | ua.PermissionTypeReceiveEvents},
    {RoleID: ua.ObjectIDWellKnownRoleAuthenticatedUser, Permissions: ua.PermissionTypeBrowse | ua.PermissionTypeRead | ua.PermissionTypeWrite | ua.PermissionTypeReceiveEvents},
}
srv, err := server.New(ua.ApplicationDescription{
        ApplicationURI: "urn:stc:tf6100-emu", ApplicationType: ua.ApplicationTypeServer,
        ApplicationName: ua.NewLocalizedText("stc TF6100 emulator", "en"),
        DiscoveryURLs: []string{"opc.tcp://localhost:48400"}},
    certPath, keyPath,                 // REQUIRED even for None (tls.LoadX509KeyPair)
    "opc.tcp://localhost:48400",
    server.WithAnonymousIdentity(true),
    server.WithSecurityPolicyNone(true),
    server.WithInsecureSkipVerify(),
    server.WithRolePermissions(perms),
    server.WithServerDiagnostics(false))
nm := srv.NamespaceManager()           // ns0 = UA, ns1 = ApplicationURI (fixed)
nm.Add("http://opcfoundation.org/UA/DI/")        // ns2 filler
nm.Add("http://PLCopen.org/OpcUa/IEC61131-3/")   // ns3 filler
ns := nm.Add("urn:BeckhoffAutomation:Ua:PLC1")   // == 4; Add returns the index
go srv.ListenAndServe()                // blocks; srv.Close() to stop
```

`emu/cert.go` generates a self-signed RSA-2048 cert with the ApplicationURI as a URI SAN on first run. `NamespaceArray` (i=2255) reflects `Add` automatically.

### 2. Objects, Variables, string NodeIds, enum, array, Description

```go
ref := func(t ua.NodeID, inv bool, target ua.NodeID) ua.Reference { return ua.NewReference(t, inv, ua.NewExpandedNodeID(target)) }

// Object (parent link is an INVERSE reference on the child; AddNode adds the forward one)
obj := server.NewObjectNode(srv, ua.NewNodeIDString(ns, "GVL_Test"), ua.NewQualifiedName(ns, "GVL_Test"),
    ua.NewLocalizedText("GVL_Test", ""), ua.LocalizedText{}, nil,
    []ua.Reference{ref(ua.ReferenceTypeIDOrganizes, true, ua.ObjectIDObjectsFolder),
                   ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.ObjectTypeIDBaseObjectType)}, 0)

// Variable: description, dataType, valueRank, arrayDimensions, accessLevel are ctor args
v := server.NewVariableNode(srv, ua.NewNodeIDString(ns, "GVL_Test.arr"), ua.NewQualifiedName(ns, "arr"),
    ua.NewLocalizedText("arr", ""), ua.NewLocalizedText("ARRAY[0..4] OF INT", ""), nil,
    []ua.Reference{ref(ua.ReferenceTypeIDHasComponent, true, gvlID),
                   ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.VariableTypeIDBaseDataVariableType)},
    ua.DataValue{}, ua.DataTypeIDInt16, ua.ValueRankOneDimension, []uint32{5},
    ua.AccessLevelsCurrentRead /* |ua.AccessLevelsCurrentWrite for p_cmd_ */, 0, false, nil)
nm.AddNodes(obj, v)
```

Enumeration DataType with EnumStrings + EnumDefinition. The variable's DataType is the enum node, and its value is `int32`. awcullen resolves Enumeration to Int32 for write type checks.

```go
dt := server.NewDataTypeNode(srv, ua.NewNodeIDString(ns, "DT.hmis_e"), ua.NewQualifiedName(ns, "hmis_e"), ..., nil,
    []ua.Reference{ref(ua.ReferenceTypeIDHasSubtype, true, ua.DataTypeIDEnumeration),
                   ref(ua.ReferenceTypeIDHasProperty, false, enumStringsID)},
    false, ua.EnumDefinition{Fields: []ua.EnumField{{Value: 0, Name: "Idle", DisplayName: ua.NewLocalizedText("Idle", "")}, ...}})
es := server.NewVariableNode(srv, enumStringsID, ua.NewQualifiedName(0, "EnumStrings"), ..., 
    []ua.Reference{ref(ua.ReferenceTypeIDHasProperty, true, dtID), ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.VariableTypeIDPropertyType)},
    ua.NewDataValue([]ua.LocalizedText{...}, ua.Good, now, 0, now, 0),
    ua.DataTypeIDLocalizedText, ua.ValueRankOneDimension, []uint32{n}, ua.AccessLevelsCurrentRead, 0, false, nil)
```

### 3. Runtime structured DataType with a served DataTypeDefinition (the key unknown)

awcullen exposes DataTypeDefinition through the **last argument of `server.NewDataTypeNode`**, typed `any`. You pass a `ua.StructureDefinition` or a `ua.EnumDefinition`. `Read(AttributeIDDataTypeDefinition)` returns it encoded as an ExtensionObject (`server_service_set.go:6221`).

Value encoding is the harder half. `ua.ExtensionObject` is just `any`. The binary encoder looks up the value's **Go `reflect.Type`** in a process-global registry (`ua.RegisterBinaryEncodingID`) to find the encoding NodeId. It then serializes the struct fields in order by reflection. The fix is to build that Go type at runtime with `reflect.StructOf`:

```go
// 1. dynamic Go type, field order == StructureDefinition order
sfs := []reflect.StructField{
    {Name: "XST_Drive_HMI__0_p_cmd_JogFwd",     Type: reflect.TypeOf(false)},
    {Name: "XST_Drive_HMI__1_p_stat_State",     Type: reflect.TypeOf(int32(0))},    // enum -> int32
    {Name: "XST_Drive_HMI__2_p_stat_Frequency", Type: reflect.TypeOf(float32(0))},
}   // nested STRUCT -> the nested dynamic type (encoded inline); ARRAY -> reflect.SliceOf(elem)
goType := reflect.StructOf(sfs)
ua.RegisterBinaryEncodingID(goType, ua.ExpandedNodeID{
    NamespaceURI: "urn:BeckhoffAutomation:Ua:PLC1",            // must be URI + ns-less id,
    NodeID:       ua.NewNodeIDString(0, "TE.ST_Drive_HMI.DefaultBinary")}) // not ns=4

// 2. DataType node with StructureDefinition + HasEncoding
dtID, encID := ua.NewNodeIDString(ns, "DT.ST_Drive_HMI"), ua.NewNodeIDString(ns, "TE.ST_Drive_HMI.DefaultBinary")
dt := server.NewDataTypeNode(srv, dtID, ua.NewQualifiedName(ns, "ST_Drive_HMI"), ..., nil,
    []ua.Reference{ref(ua.ReferenceTypeIDHasSubtype, true, ua.DataTypeIDStructure),
                   ref(ua.ReferenceTypeIDHasEncoding, false, encID)},
    false,
    ua.StructureDefinition{DefaultEncodingID: encID, BaseDataType: ua.DataTypeIDStructure,
        StructureType: ua.StructureTypeStructure,
        Fields: []ua.StructureField{
            {Name: "p_cmd_JogFwd", DataType: ua.DataTypeIDBoolean, ValueRank: -1},
            {Name: "p_stat_State", DataType: enumDTID, ValueRank: -1},
            {Name: "p_stat_Frequency", DataType: ua.DataTypeIDFloat, ValueRank: -1}}})
// 3. "Default Binary" encoding object
enc := server.NewObjectNode(srv, encID, ua.NewQualifiedName(0, "Default Binary"), ..., nil,
    []ua.Reference{ref(ua.ReferenceTypeIDHasEncoding, true, dtID),
                   ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.ObjectTypeIDDataTypeEncodingType)}, 0)
nm.AddNodes(dt, enc)

// 4. Variable ns=4;s=GVL_Test.fbMotor.HMI with DataType=dtID; its read handler returns
v := reflect.New(goType).Elem(); v.Field(i).Set(...)       // from member symbols
return ua.NewDataValue(v.Interface(), ua.Good, now, 0, now, 0) // Variant encoder wraps structs in ExtensionObject
```

Incoming struct writes decode to the same dynamic type because it is registered. The write handler checks `reflect.TypeOf(w.Value.Value) == goType` and splits the fields back into member symbols with `compose`/`decompose` in `emu/server.go`. Nesting and arrays were verified: a STRUCT containing a STRUCT, `ARRAY[..] OF REAL`, STRING and an enum.

**Modelling note.** `GVL_Test` and `fbMotor` are Objects. `HMI` is a **Variable** of DataType `ST_Drive_HMI` that also has `HasComponent` child Variables for each member. A node cannot be both an Object and a Variable, and the HMI's keymappings read `...HMI` as a value and `...HMI.p_stat_X` as members. Whether TF6100 models `StructuredType`-only FB instances as Objects or as structured Variables still needs confirming on a real TF6100.

**NodeId naming caveat.** The `DT.<name>` and `TE.<name>.DefaultBinary` NodeIds are spike inventions. Clients discover them by browsing, so they do not need to match TwinCAT for the HMI to work. Mirroring TF6100's real DataType and encoding NodeIds would still be safer for cached client configs. Confirm by browsing a real TF6100 with UaExpert.

### 4. Subscriptions

Nothing needs to be done server-side. awcullen's `DataChangeMonitoredItem` is **poll-based**. A `PollGroup` goroutine per distinct sampling interval calls the same `readValue` path as a Read, including the per-node read handler, then deduplicates by value. A backend that only changes values (here `MemSource.Write` every 100 ms) therefore produces notifications. The client requested 100 ms publishing and got 18 increasing notifications in 2 s. awcullen clamps `minPublishingInterval` to 125 ms.

### 5. Writes and BadNotWritable

The write path is `AccessLevel & CurrentWrite`, then `UserAccessLevel` (role permissions), then a builtin type check, then the write handler (`server_service_set.go:5588-6036`).
- **Read-only node**: a node without `AccessLevelsCurrentWrite` returns `BadNotWritable`. This was verified.
- **Wrong type**: an `int32` written to a Boolean returns `BadTypeMismatch`. This was verified.
- **Anonymous role**: without the `WithRolePermissions` override above, every anonymous write returns `BadUserAccessDenied`.

### 6. Standard Server object

The standard Server object comes from the embedded ns0 NodeSet. `i=2259` reads `int32(0)` (Running), and BuildInfo reflects `WithBuildInfo`.

### 7. Dynamic data source hooks

```go
n.SetReadValueHandler(func(s *server.Session, r ua.ReadValueID) ua.DataValue {
    v, err := src.Read(path)
    if err != nil { return ua.NewDataValue(nil, ua.BadNoData, time.Time{}, 0, time.Now(), 0) }
    return ua.NewDataValue(v, ua.Good, time.Now(), 0, time.Now(), 0)
})
n.SetWriteValueHandler(func(s *server.Session, w ua.WriteValue) (ua.DataValue, ua.StatusCode) {
    if w.IndexRange != "" { return ua.DataValue{}, ua.BadWriteNotSupported }
    if err := src.Write(path, w.Value.Value); err != nil { return ua.DataValue{}, ua.BadInternalError }
    return ua.NewDataValue(w.Value.Value, ua.Good, time.Now(), 0, time.Now(), 0), ua.Good
})
```

After a `Good` return, the server also calls `n.SetValue(result)`. This is harmless, but it means the node keeps a stale copy that is never read while a read handler is installed. The `*Session` argument identifies the client, which is useful for audit logging.

## Client-side generic decode procedure

This is what the HMI's open62541 client, asyncua, and the spike's gopcua walker in `cmd/spike/decode.go` all do. No compile-time types are involved.

1. Browse to the variable, or address it directly by `ns=4;s=GVL_Test.fbMotor.HMI`.
2. Read the variable's `DataType` attribute (14). The result is `ns=4;s=DT.ST_Drive_HMI`.
3. Read that node's `DataTypeDefinition` attribute (23). The result is a `StructureDefinition` carrying `DefaultEncodingId` and an ordered `Fields[]` of {Name, DataType, ValueRank, ArrayDimensions}.
4. Resolve each field's DataType to a builtin by browsing inverse `HasSubtype` (i=45) until you reach ns=0. Enumeration (i=29) resolves to Int32. Structure (i=22) means you recurse into that type's definition, because nested structs are encoded inline with no ExtensionObject header.
5. Read the `Value`. The result is an ExtensionObject whose `TypeId == DefaultEncodingId`, with a binary body.
6. Walk the body in field order. Each scalar is encoded as its builtin type. Arrays are an Int32 length followed by the elements. String is an Int32 length followed by UTF-8.
7. Enum labels come from the enum DataType's `EnumStrings` property (HasProperty) or from its `EnumDefinition`.

gopcua-specific trick: gopcua, like awcullen's own client, **silently discards bodies of unregistered encoding ids**. Register a catch-all type that keeps raw bytes under each discovered `DefaultEncodingId` before reading:

```go
type rawBody struct{ B []byte }
func (r *rawBody) Decode(b []byte) (int, error) { r.B = append([]byte(nil), b...); return len(b), nil }
func (r *rawBody) Encode() ([]byte, error)      { return r.B, nil }   // enables generic struct writes
ua.RegisterExtensionObject(def.DefaultEncodingID, new(rawBody))       // gopcua v0.9.1
```

## Gotchas

1. **The cert/key are mandatory** even for SecurityPolicy None, because `server.New` loads them unconditionally. Generate them on first run. The production HMI mounts client certs, so a secured endpoint (Basic256Sha256) is likely later. awcullen supports it, but this spike did not test it.
2. **Anonymous cannot write by default.** `DefaultRolePermissions` gives Anonymous only Browse|Read. Override it with `WithRolePermissions`, or pass per-node `rolePermissions`.
3. **The anonymous PolicyId is `Anonymous_<n>`**, not `"Anonymous"`. gopcua's bare `AuthAnonymous()` fails with `BadIdentityTokenInvalid`. Clients must take the token policy from GetEndpoints, and gopcua does that with `SecurityFromEndpoint(ep, ua.UserTokenTypeAnonymous)`. open62541 and UaExpert already do this. The advertised endpoint URL uses the host from the `endpointURL` argument.
4. **The ExtensionObject type registry is process-global, append-only, and panics on conflict.** Two consequences follow.
   - (a) Two ST structs with identical layouts would get the *same* `reflect.StructOf` type and panic on the second registration. The spike avoids this with type-qualified Go field names such as `X<TypeName>__<i>_<field>`, and `ST_Inner`/`ST_InnerTwin` prove it works.
   - (b) There is no unregister. Reloading a changed STRUCT layout in the same process needs a fresh encoding NodeId, for example with a version suffix, or a process restart. `go test` packages that build several servers must register each type once, for example behind `sync.Once` or a cache keyed by name+layout hash.
5. **`reflect.StructOf` needs exported field names**, which is why the `X` prefix is used. The OPC UA field names live only in the StructureDefinition.
6. **The NodeSet2 import drops `<Definition>`.** `LoadNodeSetFromBuffer` calls `NewDataTypeNode(..., nil)` for every UADataType (`namespace_manager.go:605-615`). The "generate NodeSet2 and import" fallback would therefore serve **no** DataTypeDefinition, and the standard ns0 structures carry none either. This was established by code inspection. The programmatic `NewDataTypeNode` path is the one to use.
7. **A custom read or write handler bypasses IndexRange.** The built-in `readRange`/`writeRange` are unexported. Implement IndexRange for arrays in the handler, or reject it as the spike does for writes. The HMI's keymappings use `array_index: null`, so this is low priority.
8. **The concurrency model** has several layers. NodeSource must be goroutine-safe and cheap, and Read is called at every sampling tick.
   - There is one goroutine per TCP connection.
   - Service requests are dispatched on a `gammazero/workerpool`, which has 4 workers by default and is tunable with `WithMaxWorkerThreads`.
   - There is one `PollGroup` goroutine per distinct sampling interval.
   - The session and subscription managers each run a housekeeping goroutine.
   - Read/write handlers run concurrently on worker and poll goroutines.
   - Under stc's deterministic scan cycle, serve reads from the snapshot of the last completed cycle, and queue writes so they apply at the next cycle boundary.
9. **There is no push notify API.** `VariableNode.SetValue` does not trigger notifications. Everything is sampled, so the notification rate is bounded by sampling and publishing intervals with a 125 ms publishing minimum.
10. **The `ua` decoder** in awcullen's client also drops unknown ExtensionObjects, so use gopcua or asyncua for tests.
11. **Maintenance is slow.** The last tag is v1.4.0 from Dec 2024. Pin it, and be ready to vendor or fork. The code is plain Go, about 44k non-test lines across `ua/` and `server/`, much of it generated.
12. **Struct writes are all-or-nothing on members.** A whole-struct write to `HMI` also overwrites the `p_stat_*` fields, even though each member node is read-only. TF6100 behaviour for this case is unknown. The emulator should probably reject or ignore writes to members whose own access level is read-only.

## Recommended internal interface for stc

```go
package opcuasrv // e.g. pkg/opcua

// Path is a TwinCAT symbol path without namespace: "GVL.fb[1].HMI.p_stat_X".
// Values use OPC UA builtin Go types: bool, int8..uint64, float32/64, string,
// time.Time, []T for arrays; enums as int32; STRUCTs are NOT passed whole --
// the server composes/decomposes them from member paths using the type model.
type NodeSource interface {
    // Read returns the value from the last completed scan cycle.
    Read(path string) (any, error)
    // Write enqueues a value to apply at the next cycle boundary. It returns
    // ErrNotWritable / ErrTypeMismatch / ErrUnknownSymbol for status mapping.
    Write(path string, v any) error
    // Subscribe is optional for awcullen (it polls Read), kept so a future
    // push-based server or a websocket bridge can avoid polling.
    Subscribe(path string, fn func(v any)) (cancel func())
}

// Model is derived once from the ST symbol table plus OPC.UA.* attributes.
type Model struct {
    Enums   []EnumType   // Name, Values []string
    Structs []StructType // Name, Fields []Field{Name, Type TypeRef, ArrayDims []uint32}
    Nodes   []NodeSpec   // Path, Kind (Object|Variable|StructVariable), Type TypeRef,
                         // Writable (from OPC.UA.DA.Access), Description
}

func Serve(ctx context.Context, endpoint string, m Model, src NodeSource, opts ...Option) error
```

Keep awcullen types out of the interface so the library could be swapped later. The adapter layer is about 370 lines in the spike: `emu/server.go` plus `emu/source.go`.

## Prototype layout

| Path | Purpose |
|---|---|
| `emu/server.go` | awcullen adapter: `New`, `AddObject`, `AddEnumType`, `AddStructType` (StructOf + registry + DataType/encoding nodes), `AddVariable`, `AddStructVariable` |
| `emu/source.go` | `NodeSource` interface and the thread-safe `MemSource` |
| `emu/cert.go` | self-signed cert generation |
| `demo/model.go` | the GVL_Test model, including nested `ST_Outer`, `ST_Inner` and `ST_InnerTwin` |
| `cmd/spike` | server plus gopcua client checks; exits non-zero on failure |
| `cmd/server` | standalone server on :48400 for external clients such as UaExpert or asyncua |
| `asyncua_check.py` | independent decode check, run with `../venv/bin/python asyncua_check.py` |
| `cmd/baseline` | hello-world binary used for the size comparison |
