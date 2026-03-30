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
