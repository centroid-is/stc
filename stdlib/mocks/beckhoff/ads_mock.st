(* Behavioral mock for ADSREAD with configurable response *)
(* Sets BUSY for 1 cycle after READ rising edge, then completes *)
(* Configure error behavior by setting ERR/ERRID before execution *)

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
VAR
    prevRead        : BOOL;
    cycleCount      : INT;
    simulateError   : BOOL;
    simulateErrId   : UDINT;
END_VAR

(* Detect rising edge of READ *)
IF READ AND NOT prevRead THEN
    BUSY := TRUE;
    ERR := FALSE;
    ERRID := 0;
    cycleCount := 0;
END_IF;

IF BUSY THEN
    cycleCount := cycleCount + 1;
    IF cycleCount >= 1 THEN
        BUSY := FALSE;
        IF simulateError THEN
            ERR := TRUE;
            ERRID := simulateErrId;
        ELSE
            ERR := FALSE;
            ERRID := 0;
        END_IF;
    END_IF;
END_IF;

prevRead := READ;
END_FUNCTION_BLOCK
