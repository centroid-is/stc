(* Common types used across Beckhoff TwinCAT 3 libraries *)
(* Stub declarations for stc type-checking. No implementation bodies. *)

(* AXIS_REF -- Opaque axis reference structure used by all MC_* blocks *)
TYPE AXIS_REF :
STRUCT
    NcToPlc     : DINT;
    PlcToNc     : DINT;
END_STRUCT
END_TYPE

(* MC_Direction -- Motion direction for MC_MoveAbsolute, MC_MoveVelocity *)
TYPE MC_Direction : (
    mcPositiveDirection,
    mcNegativeDirection,
    mcCurrentDirection,
    mcShortestWay
);
END_TYPE

(* T_AmsNetId -- AMS Net ID string for ADS communication *)
TYPE T_AmsNetId : STRING(23); END_TYPE

(* T_AmsPort -- AMS port number *)
TYPE T_AmsPort : UINT; END_TYPE

(* T_MaxString -- General-purpose maximum-length string *)
TYPE T_MaxString : STRING(255); END_TYPE

(* E_OpenPath -- File open path options for FB_FileOpen *)
TYPE E_OpenPath : (
    PATH_GENERIC,
    PATH_BOOTPATH,
    PATH_BOOTPRJPATH
);
END_TYPE

(* PVOID -- untyped address. TwinCAT defines PVOID as UXINT (ULINT on 64-bit,
   UDINT on 32-bit). stc declares it as POINTER TO BYTE (locked Phase 21
   decision). Phase 22 must make ADR(...) results and any POINTER TO <T>
   assignable to PVOID. *)
TYPE PVOID : POINTER TO BYTE; END_TYPE

(* Platform-width integer aliases (TwinCAT PLC manual 16.5.11), 64-bit sizes *)
TYPE __XWORD : LWORD; END_TYPE

TYPE XWORD : LWORD; END_TYPE

TYPE UXINT : ULINT; END_TYPE

TYPE __UXINT : ULINT; END_TYPE

TYPE XINT : LINT; END_TYPE

TYPE __XINT : LINT; END_TYPE

(* AMSPORT -- AMS port number (Tc2_System alias) *)
TYPE AMSPORT : UINT; END_TYPE
