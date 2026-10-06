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

(* TIMESTRUCT -- Windows SYSTEMTIME layout *)
TYPE TIMESTRUCT :
STRUCT
    wYear           : WORD;
    wMonth          : WORD;
    wDayOfWeek      : WORD;
    wDay            : WORD;
    wHour           : WORD;
    wMinute         : WORD;
    wSecond         : WORD;
    wMilliseconds   : WORD;
END_STRUCT
END_TYPE

(* E_TimeZoneID -- time zone state reported by FB_LocalSystemTime *)
TYPE E_TimeZoneID : (
    eTimeZoneID_Invalid := -1,
    eTimeZoneID_Unknown := 0,
    eTimeZoneID_Standard := 1,
    eTimeZoneID_Daylight := 2
);
END_TYPE

(* FB_LocalSystemTime -- local Windows time, synchronised cyclically (3.47) *)
FUNCTION_BLOCK FB_LocalSystemTime
VAR_INPUT
    sNetID      : T_AmsNetId := '';
    bEnable     : BOOL;
    dwCycle     : DWORD(1..86400) := 5;
    dwOpt       : DWORD := 1;
    tTimeout    : TIME := DEFAULT_ADS_TIMEOUT;
END_VAR
VAR_OUTPUT
    bValid      : BOOL;
    systemTime  : TIMESTRUCT;
    tzID        : E_TimeZoneID := eTimeZoneID_Invalid;
END_VAR
END_FUNCTION_BLOCK

(* SYSTEMTIME_TO_DT -- TIMESTRUCT to DATE_AND_TIME (4.1.18) *)
FUNCTION SYSTEMTIME_TO_DT : DT
VAR_INPUT
    TIMESTR     : TIMESTRUCT;
END_VAR
END_FUNCTION
