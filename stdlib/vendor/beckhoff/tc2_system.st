(* Tc2_System -- System function blocks for Beckhoff TwinCAT 3 *)
(* Stub declarations for stc type-checking. No implementation bodies. *)

FUNCTION_BLOCK ADSREAD
VAR_INPUT
    NETID       : T_AmsNetId;
    PORT        : T_AmsPort;
    IDXGRP      : UDINT;
    IDXOFFS     : UDINT;
    LEN         : UDINT;
    DESTADDR    : UDINT;
    READ        : BOOL;
    TMOUT       : TIME := T#5S;
END_VAR
VAR_OUTPUT
    BUSY        : BOOL;
    ERR         : BOOL;
    ERRID       : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK ADSWRITE
VAR_INPUT
    NETID       : T_AmsNetId;
    PORT        : T_AmsPort;
    IDXGRP      : UDINT;
    IDXOFFS     : UDINT;
    LEN         : UDINT;
    SRCADDR     : UDINT;
    WRITE       : BOOL;
    TMOUT       : TIME := T#5S;
END_VAR
VAR_OUTPUT
    BUSY        : BOOL;
    ERR         : BOOL;
    ERRID       : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileOpen
VAR_INPUT
    sNetId      : T_AmsNetId;
    sPathName   : T_MaxString;
    nMode       : DWORD;
    ePath       : E_OpenPath := PATH_GENERIC;
    bExecute    : BOOL;
    tTimeout    : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
    hFile       : UINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileClose
VAR_INPUT
    sNetId      : T_AmsNetId;
    hFile       : UINT;
    bExecute    : BOOL;
    tTimeout    : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileRead
VAR_INPUT
    sNetId      : T_AmsNetId;
    hFile       : UINT;
    pReadBuff   : UDINT;
    cbReadLen   : UDINT;
    bExecute    : BOOL;
    tTimeout    : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
    cbRead      : UDINT;
    bEOF        : BOOL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileWrite
VAR_INPUT
    sNetId      : T_AmsNetId;
    hFile       : UINT;
    pWriteBuff  : UDINT;
    cbWriteLen  : UDINT;
    bExecute    : BOOL;
    tTimeout    : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy       : BOOL;
    bError      : BOOL;
    nErrId      : UDINT;
    cbWrite     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION MEMCPY : UDINT
VAR_INPUT
    destAddr    : UDINT;
    srcAddr     : UDINT;
    n           : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMSET : UDINT
VAR_INPUT
    destAddr    : UDINT;
    fillByte    : BYTE;
    n           : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMMOVE : UDINT
VAR_INPUT
    destAddr    : UDINT;
    srcAddr     : UDINT;
    n           : UDINT;
END_VAR
END_FUNCTION
