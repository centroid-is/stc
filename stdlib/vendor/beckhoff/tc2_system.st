(* Tc2_System -- System function blocks for Beckhoff TwinCAT 3 *)
(* Stub declarations for stc type-checking. No implementation bodies. *)
(* RTC is declared by Beckhoff in Tc2_Utilities (manual v2.18.2 section 3.79).
   It lives here because Tc2_Standard is built in to stc, and every stub
   closure includes tc2_system, so a project referencing only Tc2_Standard
   and Tc2_System still resolves RTC. *)

(* T_AmsNetIdArr -- AMS Net ID as six bytes (section 5, BYTE not USINT) *)
TYPE T_AmsNetIdArr : ARRAY[0..5] OF BYTE; END_TYPE

(* AMSNETID -- AMS Net ID byte array (TwinCAT type system, 6 x BYTE) *)
TYPE AMSNETID : ARRAY[0..5] OF BYTE; END_TYPE

(* AMSADDR -- AMS address; port is WORD in the TwinCAT type system *)
TYPE AMSADDR :
STRUCT
    netId       : AMSNETID;
    port        : WORD;
END_STRUCT
END_TYPE

(* ST_AmsAddr -- AMS address (section 5.11) *)
TYPE ST_AmsAddr :
STRUCT
    netId       : T_AmsNetIdArr;
    port        : T_AmsPort;
END_STRUCT
END_TYPE

VAR_GLOBAL CONSTANT
    DEFAULT_ADS_TIMEOUT : TIME := T#5S;     (* name cited; value assumed *)
    MAX_STRING_LENGTH   : UDINT := 255;     (* section 5.20 *)
END_VAR

(* F_CreateAmsNetId -- byte array to dotted Net ID string (section 4.2.4) *)
FUNCTION F_CreateAmsNetId : T_AmsNetId
VAR_INPUT
    nIds        : T_AmsNetIdArr;
END_VAR
END_FUNCTION

(* RTC -- software real-time clock (Tc2_Utilities section 3.79, see header) *)
FUNCTION_BLOCK RTC
VAR_INPUT
    EN          : BOOL;
    PDT         : DATE_AND_TIME;
END_VAR
VAR_OUTPUT
    Q           : BOOL;
    CDT         : DATE_AND_TIME;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK ADSREAD
VAR_INPUT
    NETID       : T_AmsNetId;
    PORT        : T_AmsPort;
    IDXGRP      : UDINT;
    IDXOFFS     : UDINT;
    LEN         : UDINT;
    DESTADDR    : PVOID;
    READ        : BOOL;
    TMOUT       : TIME := DEFAULT_ADS_TIMEOUT;
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
    SRCADDR     : PVOID;
    WRITE       : BOOL;
    TMOUT       : TIME := DEFAULT_ADS_TIMEOUT;
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
    destAddr    : PVOID;
    srcAddr     : PVOID;
    n           : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMSET : UDINT
VAR_INPUT
    destAddr    : PVOID;
    fillByte    : USINT;
    n           : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMMOVE : UDINT
VAR_INPUT
    destAddr    : PVOID;
    srcAddr     : PVOID;
    n           : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMCMP : DINT
VAR_INPUT
    pBuf1       : PVOID;
    pBuf2       : PVOID;
    n           : UDINT;
END_VAR
END_FUNCTION
