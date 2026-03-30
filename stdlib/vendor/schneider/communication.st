(* Schneider Electric EcoStruxure -- Communication function blocks *)
(* Modbus and general communication stubs *)

FUNCTION_BLOCK READ_VAR
VAR_INPUT
    ADR         : STRING(32);
    OBJ         : STRING(8);
    NUM         : INT;
    Req         : BOOL;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Error       : BOOL;
    Status      : UDINT;
    DataOut     : INT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK WRITE_VAR
VAR_INPUT
    ADR         : STRING(32);
    OBJ         : STRING(8);
    NUM         : INT;
    DataIn      : INT;
    Req         : BOOL;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Error       : BOOL;
    Status      : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK SEND_REQ
VAR_INPUT
    ADR         : STRING(32);
    DataIn      : STRING(255);
    LenIn       : INT;
    Req         : BOOL;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Error       : BOOL;
    Status      : UDINT;
    DataOut     : STRING(255);
    LenOut      : INT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK RCV_REQ
VAR_INPUT
    Enable      : BOOL;
    ADR         : STRING(32);
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Error       : BOOL;
    Status      : UDINT;
    DataOut     : STRING(255);
    LenOut      : INT;
END_VAR
END_FUNCTION_BLOCK
