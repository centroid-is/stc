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

## From the second v1.2 code review (.planning/v1.2-CODE-REVIEW-2.md)

- **R2-LO-02: `stc_sim_write` is not atomic and loses writes to explicit AT %I
  variables.** `Write` calls `rt.Set` before `plant.Apply`, so a failed force
  leaves the runtime value changed. An explicit `AT %IX…` variable takes the
  `runtime_set` route and the next scan's AT sync overwrites it. Fix: route
  every path through `plant.Check` and then `plant.Apply(ActSet)`, read the
  value back only on success, and fix the stale `Step` comment.
- **R2-LO-03: the AT %I guard matches only whole variables.** `Plant.set`
  compares `ROOT.VAR` exactly, so `SET('MAIN.aIn[1]', …)` or
  `GVL.stIn.bFlag` on an explicit `AT %IB` array or struct is accepted and
  then overwritten by the AT sync. Fix: reject any path whose `ROOT.VAR`
  prefix up to the first `.` or `[` after the variable name is in `atIn`.
- **R2-LO-04a: SIZEOF of LTIME, LDATE, LTOD and LDT reports 4 bytes.**
  `pkg/types` maps the long time types to the 32-bit kinds, so a value
  cannot tell LTIME from TIME. Fix: add 64-bit time kinds (or carry the
  declared type name) and return 8 for them.
- **R2-LO-04b: SIZEOF of a STRING(n) reports 81 bytes.** String values do not
  carry their declared length. Fix: carry it on the value or look it up
  from the declaration in `evalSizeof`, so `MEMCPY(..., SIZEOF(str))`
  copies the whole buffer.
- **R2-LO-04c: real-to-integer conversions out of range.** `LREAL_TO_ULINT`
  above 2^63 and NaN or Inf to any integer use Go's implementation-defined
  `int64(float)`. Fix: convert through `uint64` for unsigned targets and
  clamp or fail on out-of-range and non-finite values.
- **R2-LO-09: serve start-up text and early SIGINT.** `printServeInfo` prints
  "only 0.0.0.0:4840" for `0.0.0.0:4840` and `[::]:4840`; use
  `opcua.Exposed` for the "all interfaces" text. `signal.NotifyContext` is
  installed after scenario and OPC UA start-up, so a SIGINT during
  certificate generation skips the final `--persist` save; install it at
  the top of `runServe`.
- **R2-HI-01 follow-up: no collision diagnostic.** The scenario built-ins now
  resolve only in the TEST_CASE body, so a project FUNCTION named GET or
  SET works in project code. In the TEST_CASE body the built-in still
  wins, and the runner does not warn about the name clash.
