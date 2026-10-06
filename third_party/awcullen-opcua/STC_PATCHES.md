# Patched copy of github.com/awcullen/opcua v1.4.0

stc uses this copy through a `replace` directive in the top-level `go.mod`.
Only the `client`, `server` and `ua` packages are kept; upstream tests,
commands and schema files are dropped. The code is MIT licensed (see
`LICENSE`).

## Patches

- `server/option.go`, `server/server.go`: new `WithListenAddress(addr)`
  option. Upstream `ListenAndServe` always listens on `":"+port`, that is on
  every interface, whatever host the endpoint URL names. With the option,
  `stc serve --opcua 127.0.0.1:4840` binds loopback only (v1.2 review
  HI-04).

To upgrade, copy the three packages from the new upstream release and
re-apply the patches above.
