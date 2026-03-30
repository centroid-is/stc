FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Execute : BOOL;
    Position : LREAL;
END_VAR
VAR_OUTPUT
    Done : BOOL;
    Error : BOOL;
END_VAR
    Done := Execute;
    Error := FALSE;
END_FUNCTION_BLOCK
