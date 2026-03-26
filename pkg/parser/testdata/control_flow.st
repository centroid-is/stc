PROGRAM ControlTest
VAR
    i : INT;
    x : INT;
    state : INT;
    flag : BOOL;
END_VAR
IF x > 10 THEN
    x := 0;
ELSIF x > 5 THEN
    x := x - 1;
ELSE
    x := x + 1;
END_IF;

CASE state OF
    0: x := 0;
    1..5: x := 1;
    10, 20, 30: x := 2;
ELSE
    x := -1;
END_CASE;

FOR i := 0 TO 10 BY 2 DO
    x := x + i;
END_FOR;

WHILE flag DO
    flag := FALSE;
END_WHILE;

REPEAT
    x := x + 1;
UNTIL x >= 100
END_REPEAT;
END_PROGRAM
