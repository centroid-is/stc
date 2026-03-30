(* Schneider Electric EcoStruxure -- System utility function blocks *)

FUNCTION GetBit : BOOL
VAR_INPUT
    Value       : DWORD;
    BitNum      : INT;
END_VAR
END_FUNCTION

FUNCTION SetBit : DWORD
VAR_INPUT
    Value       : DWORD;
    BitNum      : INT;
    BitValue    : BOOL;
END_VAR
END_FUNCTION

FUNCTION_BLOCK RTC
VAR_INPUT
    EN          : BOOL;
    PDT         : STRING(20);
END_VAR
VAR_OUTPUT
    Q           : BOOL;
    CDT         : STRING(20);
END_VAR
END_FUNCTION_BLOCK
