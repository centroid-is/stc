---
phase: 28-opc-ua-address-space
plan: 01
subsystem: opcua
tags: [opcua, awcullen, tf6100, server, certificates, namespace, symbolnode, nodesource]

requires: []
provides:
  - "github.com/awcullen/opcua v1.4.0 pinned in go.mod (sum.golang.org verified; no gopcua)"
  - "pkg/opcua contract: Kind (+String), SymbolNode, NodeSource, ErrUnknownSymbol/ErrNotWritable/ErrTypeMismatch/ErrOutOfRange"
  - "MapSource goroutine-safe NodeSource fake: NewMapSource, Read, Write, Snapshot, FailWrite, Set, Get, Writes, WriteRecord"
  - "EnsureCert(dir, appURI): self-signed RSA-2048, 10 years, URI/DNS/IP SANs, key 0600, dir 0700"
  - "Config, DefaultConfig, New, (*Server).Start/Stop/Endpoint/NamespaceIndex; unexported uaServer() and namespaceManager() for 28-02"
  - "Test helpers in testutil_test.go: freeAddr, testConfig, startServer, dial, dialAnon, readValue, readAttr, browseForward, browseNames, errContains"
affects: [28-02, 28-03, 28-04, 29]

tech-stack:
  added: ["github.com/awcullen/opcua v1.4.0 (MIT) + djherbis/buffer, gammazero/deque, gammazero/workerpool, google/uuid, pkg/errors (indirect)"]
  patterns:
    - "PLC namespace forced to index 4 by adding urn:stc:filler:2 and urn:stc:filler:3 first"
    - "Anonymous write via WithRolePermissions (Browse|Read|Write|ReceiveEvents for Anonymous and AuthenticatedUser); per-node AccessLevel still gates writes"
    - "Start pre-binds :port to fail fast on a busy port, then polls TCP readiness with a deadline"
    - "Network tests are t.Parallel so awcullen's 3 s Close grace periods overlap"

key-files:
  created:
    - pkg/opcua/doc.go
    - pkg/opcua/model.go
    - pkg/opcua/mapsource.go
    - pkg/opcua/mapsource_test.go
    - pkg/opcua/cert.go
    - pkg/opcua/server.go
    - pkg/opcua/server_test.go
    - pkg/opcua/testutil_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Endpoint port 0 is rejected: awcullen advertises the configured URL verbatim, so ':0' would be unreachable"
  - "An empty Endpoint host advertises os.Hostname() and probes 127.0.0.1 for readiness"
  - "PKIDir is resolved lazily in New (UserCacheDir/stc/opcua/pki, else a temp dir) and never created when CertFile/KeyFile are given"
  - "EnsureCert regenerates when either the cert or the key file is missing"
  - "Stop waits up to 5 s for ListenAndServe after Close; Start after Stop and a second Start are errors"

requirements-completed: [OPCUA-01, OPCUA-02]

duration: 25min
completed: 2026-10-06
---

# Phase 28 Plan 01: OPC UA Server Core Summary

**awcullen/opcua v1.4.0 server with the Beckhoff PLC namespace at index 4, anonymous writes, self-signed certificates for None and Basic256Sha256, and the SymbolNode/NodeSource contract with a MapSource fake.**

## Performance

- Duration: about 25 minutes
- Completed: 2026-10-06
- Tasks: 3 of 3
- Files: 8 created, 2 modified

## Accomplishments

- The dependency is pinned at v1.4.0. `go list -deps ./pkg/opcua` contains no pkg/symtree, pkg/interp or pkg/vendor.
- Client tests prove that NamespaceArray[4] is `urn:BeckhoffAutomation:Ua:PLC1` and that i=2259 reads int32 0 (Running).
- Basic256Sha256 SignAndEncrypt connects with a self-signed client certificate. A None dial is refused when AllowNone is false.
- The doubled race run of the package takes about 26 s with 89.9% statement coverage. `go test ./...` is green.

## Task Commits

1. **Task 1: pin dependency, contract, MapSource** - `5737ad0` (feat)
2. **Task 2 RED: cert and server core tests** - `9685e11` (test)
3. **Task 2 GREEN: EnsureCert and Server** - `698673c` (feat)
4. **Task 3: client integration tests and helpers** - `3c8785c` (test)

## Library limitations recorded

- **Secured policies are always advertised.** awcullen v1.4.0 advertises Basic256Sha256 and its other secured policies whenever a certificate loads. Only None can be toggled. `EnableBasic256Sha256` records intent, and with `AllowNone=false` it yields a secure-only server.
- **ListenAndServe binds all interfaces.** It calls `net.Listen("tcp", ":"+port)`, so the host in Endpoint only shapes the advertised URL. Tests use free ports reserved on 127.0.0.1.
- **Close sleeps three seconds** while running, a client grace period. Every Stop of a started server therefore takes at least 3 s.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] go.mod listed the module as indirect in the Task 1 commit**
- **Found during:** Task 1
- **Issue:** `go mod tidy` drops a module that no package imports yet, and Task 1 has no awcullen import.
- **Fix:** Task 1 committed the `go get` result, marked `// indirect`. The tidy in the Task 2 RED commit made it a direct requirement.
- **Commits:** 5737ad0, 9685e11

**2. [Rule 1 - Bug] Busy-port detection in Start**
- **Found during:** Task 2
- **Issue:** The readiness probe can connect to a foreign listener on the same port before ListenAndServe reports its bind failure.
- **Fix:** Start pre-binds `:port` and closes it before launching ListenAndServe. It also checks the listener error after a successful probe. TestStartPortInUse covers this.
- **Commit:** 698673c

**3. [Rule 2 - Correctness] Extra validation in New**
- **Issue:** Some invalid configs produced late or confusing failures.
- **Fix:** New rejects an empty ApplicationURI, an empty PLCNamespace, and port 0 or out-of-range ports. It also rejects a PLCNamespace that does not land at index 4, for example a filler URN.
- **Commit:** 698673c

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-28-01 and T-28-02 are documented on the Config doc comment. T-28-04 is enforced and tested with key mode 0600 and directory mode 0700, skipped on Windows.

## Next Phase Readiness

- 28-02 builds nodes through `s.uaServer()` and `s.namespaceManager()` with `ns = s.NamespaceIndex()`.
- Tests use `startServer`, `dialAnon`, `readValue`/`readAttr`, `browseForward` and `MapSource`.

## Self-Check: PASSED
