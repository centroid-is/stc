# Phase 29 deferred items

## From 29-02

- **Variable-level StructuredType on an ARRAY OF struct is published as an Object.**
  The legacy project (skammtalinur-legacy) declares
  `{attribute 'OPC.UA.DA.StructuredType' := '1'} recipes : ARRAY [1..3] OF ST_LineRecipe;`
  and the real hmi/keymappings.json reads `ns=4;s=GVL_BatchLines.recipes` (key LineRecipes)
  as one value. pkg/opcua Build treats the array as a container, so the Value read
  returns BadAttributeIdInvalid. TF6100 most likely serves a Variable of
  ExtensionObject[] here. Out of scope for 29-02 (pkg/opcua Build/convert change plus
  a golden update); confirm the TF6100 shape with `stc opcua snapshot` first.
  Found by TestHMIKeymappingsReal with STC_HMI_PROJECT=skammtalinur-legacy/sildarvinnsla.tsproj:
  319 good, 1 bad (this id), 108 skipped.

## From 29-01

- `gofmt -l` lists `pkg/opcua/model.go` and `cmd/stc/main.go`. Plan 29-01 did not touch either file, so they were left unformatted.
- Scenario wiring for `stc serve --scenario` and `stc-mcp --scenario` waits for the Phase 27 merge. `pkg/scenario` does not exist on main yet. See 29-01-SUMMARY "Pending".

## From the v1.2 code review (.planning/v1.2-CODE-REVIEW.md)

- **ME-04: explicit AT overlays use the bit-packed EtherCAT layout.** `allocIO`,
  `syncIOIn` and `syncIOOut` share the packed codec, so `stIn AT %IB0 : ST_Mix`
  puts a BOOL in one bit and `n : INT` at bit 2. TwinCAT gives BOOL one byte
  and aligns members to their size, capped at pack mode 8. Fix: give the
  `ioCodec` a layout mode, `packed` for TcLinkTo links and `memory` for AT
  slots. This changes wildcard slot sizes and offsets, so the
  project_io tests and any AT-based fixtures need review.
- **HI-04 follow-ups not taken.** The team lead kept `--opcua :4840` binding all
  interfaces and anonymous writes on by default, because the tfc-hmi app
  connects anonymously. Still open:
  - The server trusts any client certificate (`WithInsecureSkipVerify` plus an
    X509 identity authenticator that accepts every certificate). A
    trusted-client PKI directory would make `basic256sha256` an access
    control, not only encryption. docs/OPCUA.md lists this as a known
    limitation.
  - Consider defaulting `--opcua` to loopback, so binding all interfaces
    becomes an explicit `0.0.0.0:4840`.
- **ADR(arr[i]) buffer model (HI-02 fix).** A pointer to an array element
  now covers elements i to the end of that array. Pointer arithmetic past the
  array end, or into a neighbouring member of an enclosing struct, is still
  not modelled.
- **Vendored awcullen/opcua (HI-04 fix).** `third_party/awcullen-opcua` is a
  patched copy of v1.4.0 with one added option, `WithListenAddress`. Upstream
  the option, or re-apply the patch on upgrade (see STC_PATCHES.md there).
- `TestProjectRunRealClock` (wall-clock pacing, 150..210 ticks in 200 ms)
  failed once under a parallel `go test ./pkg/opcua/... ./pkg/interp/ ./cmd/...`
  run and passes in isolation. It is timing-sensitive under load.
