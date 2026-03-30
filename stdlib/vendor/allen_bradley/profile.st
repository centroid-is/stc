(* Allen Bradley Logix 5000 -- AB-compatible type stubs *)
(* AB restrictions: no OOP, no POINTER TO, no REFERENCE TO, tag-based I/O *)
(* These stubs use standard IEC 61131-3 FUNCTION_BLOCK syntax. *)
(* The stc checker uses the AB vendor profile to restrict language features. *)

(* AB timer accumulator type used by TONR, TOFR, RTO *)
TYPE TIMER :
STRUCT
    PRE         : DINT;     (* Preset value in milliseconds *)
    ACC         : DINT;     (* Accumulated value in milliseconds *)
    EN          : BOOL;     (* Timer enable bit *)
    TT          : BOOL;     (* Timer timing bit *)
    DN          : BOOL;     (* Timer done bit *)
END_STRUCT
END_TYPE

(* AB counter type *)
TYPE COUNTER :
STRUCT
    PRE         : DINT;     (* Preset value *)
    ACC         : DINT;     (* Accumulated value *)
    CU          : BOOL;     (* Count up enable *)
    CD          : BOOL;     (* Count down enable *)
    DN          : BOOL;     (* Done bit *)
    OV          : BOOL;     (* Overflow bit *)
    UN          : BOOL;     (* Underflow bit *)
END_STRUCT
END_TYPE

(* AB CONTROL structure for block transfer and other instructions *)
TYPE CONTROL :
STRUCT
    LEN         : DINT;     (* Length *)
    POS         : DINT;     (* Position *)
    EN          : BOOL;     (* Enable *)
    EU          : BOOL;     (* Enable unload *)
    DN          : BOOL;     (* Done *)
    EM          : BOOL;     (* Empty *)
    ER          : BOOL;     (* Error *)
    FD          : BOOL;     (* Found *)
    IN_         : BOOL;     (* Inhibit *)
    UL          : BOOL;     (* Unload *)
END_STRUCT
END_TYPE

(* AB MSG instruction for peer-to-peer communication *)
FUNCTION_BLOCK MSG
VAR_INPUT
    Execute     : BOOL;
    MsgType     : DINT;
    TargetAddr  : STRING(80);
    LocalTag    : STRING(80);
    RemoteTag   : STRING(80);
END_VAR
VAR_OUTPUT
    EN          : BOOL;
    DN          : BOOL;
    ER          : BOOL;
    ErrorCode   : DINT;
END_VAR
END_FUNCTION_BLOCK
