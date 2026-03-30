(* Allen Bradley Logix 5000 -- Timer instruction stubs *)
(* AB uses different timer names and behavior from IEC standard *)

(* TONR -- Timer On Delay Retentive *)
(* Unlike IEC TON, TONR retains accumulated time when disabled *)
FUNCTION_BLOCK TONR
VAR_INPUT
    TimerEnable : BOOL;
    PRE         : DINT;     (* Preset value in milliseconds *)
END_VAR
VAR_OUTPUT
    ACC         : DINT;     (* Accumulated value in milliseconds *)
    EN          : BOOL;     (* Timer enable bit *)
    TT          : BOOL;     (* Timer timing bit *)
    DN          : BOOL;     (* Timer done bit *)
END_VAR
END_FUNCTION_BLOCK

(* TOFR -- Timer Off Delay Retentive *)
FUNCTION_BLOCK TOFR
VAR_INPUT
    TimerEnable : BOOL;
    PRE         : DINT;     (* Preset value in milliseconds *)
END_VAR
VAR_OUTPUT
    ACC         : DINT;     (* Accumulated value *)
    EN          : BOOL;
    TT          : BOOL;
    DN          : BOOL;
END_VAR
END_FUNCTION_BLOCK

(* RTO -- Retentive Timer On *)
(* Accumulates time while enabled, retains value when disabled *)
(* Must be explicitly reset with RES instruction *)
FUNCTION_BLOCK RTO
VAR_INPUT
    TimerEnable : BOOL;
    PRE         : DINT;     (* Preset value in milliseconds *)
END_VAR
VAR_OUTPUT
    ACC         : DINT;     (* Accumulated value *)
    EN          : BOOL;
    TT          : BOOL;
    DN          : BOOL;
END_VAR
END_FUNCTION_BLOCK
