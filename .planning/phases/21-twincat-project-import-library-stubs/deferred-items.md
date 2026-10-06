# Phase 21 Deferred Items

## From 21-02

- **PVOID typing (Phase 22).** `common_types.st` declares `PVOID : POINTER TO BYTE`. TwinCAT defines PVOID as UXINT. Phase 22 must make `ADR(...)` results and any `POINTER TO <T>` assignable to PVOID, otherwise MEMCPY/ADSREAD/FB_EcCoESdoRead calls with typed pointers will report SEMA001 once ADR is typed.
- **gofmt drift (pre-existing, out of scope).** `stdlib/vendor/allen_bradley/stubs_test.go` is not gofmt-clean on main before 21-02.
