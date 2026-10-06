# Phase 21 Deferred Items

## From 21-02

- **PVOID typing (Phase 22).** `common_types.st` declares `PVOID : POINTER TO BYTE`. TwinCAT defines PVOID as UXINT. Phase 22 must make `ADR(...)` results and any `POINTER TO <T>` assignable to PVOID, otherwise MEMCPY/ADSREAD/FB_EcCoESdoRead calls with typed pointers will report SEMA001 once ADR is typed.
- **gofmt drift (pre-existing, out of scope).** `stdlib/vendor/allen_bradley/stubs_test.go` is not gofmt-clean on main before 21-02.

## From 21-06 (phase gate)

- **PVOID typed as POINTER TO BYTE (Phase 22).** The PVOID alias stays as locked (orchestrator ruling). Phase 22 must accept any `POINTER TO <T>` and `ADR(...)` results where PVOID is expected, and must support pointer comparison with `0` and pointer indexing (Baader: 33 SEMA003/SEMA023 in the oracle). See research Pitfall 8.
- **A2 double-quoted attribute assumption (user confirmation).** stc treats `{attribute "qualified_only"}` (double quotes) as ignored by TwinCAT and warns with SEMA039 instead of enforcing qualified_only. If TwinCAT does honour it, ST101/ST201/ST301 have 23 real SEMA033 findings. The user should confirm.
- **`stc sim <project>` does not register user FBs or methods from other project files (Phase 23).** Only the task program file's declarations run under sim.
- **Method call as a statement with arguments reports "no member" (pre-existing checker bug).** `f.Push(p := x);` reports SEMA024 `type FB_F has no member "Push"`, while `ok := f.Push(p := x);` and `f.Flush();` resolve. Baader hits it 10 times (FB_Fifo.Configure/Push, FB_SerialFramer.SendBytes). Repro:
  ```
  FUNCTION_BLOCK FB_F
  METHOD Push : BOOL
  VAR_INPUT p : INT; END_VAR
  Push := TRUE;
  END_METHOD
  END_FUNCTION_BLOCK
  PROGRAM P
  VAR f : FB_F; END_VAR
  f.Push(p := 1);   (* SEMA024 *)
  END_PROGRAM
  ```
  Owner: Phase 22 or 23 (method calls). Allowlisted as `deferred` in tests/twincat_import_test.go.
- **External read of an FB's internal VAR.** Baader MAIN reads `GVL_Pipeline.rtFifo.nElemSize`, a plain `VAR` of FB_Fifo, and stc reports SEMA024 (3 errors). Decide whether TwinCAT accepts read access to internal FB variables and align the checker (Phase 22).
- **LEN typed as STRING (pre-existing checker bug).** `n := LEN(s);` reports `cannot assign STRING to INT`. `types.BuiltinFunctions["LEN"]` returns INT, but generic candidate resolution in `checker.checkBuiltinCall` returns the argument type. One Baader error (F_ParseBraceNumbers). Owner: Phase 22 built-ins.
- **Genuine sildarvinnsla drift at HEAD (report to user, not stc bugs).**
  - ST201 `SPB02` and ST301 `SPB03` declare `FB_TwoWayConveyor`, which SVNCoreComponents renamed to `FB_BatchConveyor` (SEMA037).
  - ST201 and ST301 MAIN write `ST_LineRecipe.stopDistanceFromEnd`; SVNCoreComponents FB_Conveyor uses `stopDistanceFromEnd` and `drivePastForDelivery`; neither member exists on ST_LineRecipe any more (SEMA024).
  - SVNCoreComponents FB_Conveyor uses `ST_Batch`, which is declared nowhere (SEMA037).
- **Baader IO-generated types (Phase 24).** `MDP5001_600_*`, `Status_FBD35181_Plc` and `Ctrl_903458A3_Plc` come from per-box `_Config/IO` xti `<DataTypes>`; 16 SEMA037 plus 36 "type Invalid" cascades.
