FUNCTION_BLOCK FB_Timer
VAR_INPUT
    enable : BOOL;
    preset : INT;
END_VAR
VAR_OUTPUT
    done : BOOL;
    elapsed : INT;
END_VAR
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    timer1 : FB_Timer;
    result : BOOL;
    counter : INT;
END_VAR
    result := timer1.done;
    counter := timer1.elapsed;
END_PROGRAM
