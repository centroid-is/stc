(* Tc3_EventLogger -- Event logging for Beckhoff TwinCAT 3 *)
(* Stub declarations for stc type-checking. No implementation bodies. *)

FUNCTION_BLOCK FB_TcEventLogger
VAR_INPUT
    bExecute    : BOOL;
    nEventId    : UDINT;
    eSeverity   : INT;
    sMessage    : T_MaxString;
END_VAR
VAR_OUTPUT
    bDone       : BOOL;
    bBusy       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_TcAlarm
VAR_INPUT
    bExecute    : BOOL;
    nEventId    : UDINT;
    eSeverity   : INT;
    sMessage    : T_MaxString;
    bConfirm    : BOOL;
    bReset      : BOOL;
END_VAR
VAR_OUTPUT
    bDone       : BOOL;
    bBusy       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
    bActive     : BOOL;
    bConfirmed  : BOOL;
END_VAR
END_FUNCTION_BLOCK
