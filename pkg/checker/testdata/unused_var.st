PROGRAM Main
VAR
    unusedLocal : INT;
    usedLocal : INT;
END_VAR
VAR_INPUT
    unusedInput : INT;  (* Should NOT trigger warning *)
END_VAR

usedLocal := 42;

END_PROGRAM
