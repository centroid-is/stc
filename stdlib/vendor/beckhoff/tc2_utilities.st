(* Tc2_Utilities -- Utility function blocks for Beckhoff TwinCAT 3 *)
(* Stub declarations for stc type-checking. No implementation bodies. *)

FUNCTION_BLOCK FB_FormatString
VAR_INPUT
    sFormat     : T_MaxString;
    arg1        : T_MaxString;
    arg2        : T_MaxString;
    arg3        : T_MaxString;
    arg4        : T_MaxString;
    arg5        : T_MaxString;
    arg6        : T_MaxString;
    arg7        : T_MaxString;
    arg8        : T_MaxString;
    arg9        : T_MaxString;
    arg10       : T_MaxString;
    bExecute    : BOOL;
END_VAR
VAR_OUTPUT
    bDone       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
    sOut        : T_MaxString;
END_VAR
END_FUNCTION_BLOCK

FUNCTION CRC16 : UINT
VAR_INPUT
    pData       : UDINT;
    cbLen       : UDINT;
    nInit       : UINT;
    nPoly       : UINT;
END_VAR
END_FUNCTION

FUNCTION CRC32 : UDINT
VAR_INPUT
    pData       : UDINT;
    cbLen       : UDINT;
    nInit       : UDINT;
    nPoly       : UDINT;
END_VAR
END_FUNCTION
