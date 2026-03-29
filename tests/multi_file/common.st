(* Common definitions shared across test files.
   This file defines FUNCTION_BLOCK and FUNCTION declarations
   that are used by main_test.st in the same directory.

   NOTE: The current test runner parses each _test.st file independently.
   Cross-file references require all declarations in the same file.
   This file is provided as a parse-validation target.
*)

FUNCTION_BLOCK FB_PID
VAR_INPUT
    SetPoint : REAL;
    ProcessValue : REAL;
    Kp : REAL;
END_VAR
VAR_OUTPUT
    ControlOutput : REAL;
END_VAR
VAR
    error : REAL;
END_VAR
    error := SetPoint - ProcessValue;
    ControlOutput := error * Kp;
END_FUNCTION_BLOCK

FUNCTION Clamp : REAL
VAR_INPUT
    val : REAL;
    lo : REAL;
    hi : REAL;
END_VAR
    IF val < lo THEN
        Clamp := lo;
    ELSIF val > hi THEN
        Clamp := hi;
    ELSE
        Clamp := val;
    END_IF;
END_FUNCTION
