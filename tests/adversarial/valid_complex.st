PROGRAM p
VAR
    x : INT;
    y : INT;
    z : REAL;
    b : BOOL;
    s : STRING;
    i : INT;
END_VAR

x := 42;
y := x * 2 + 10 - 3;
z := 3.14;
b := TRUE AND (x > 10) OR FALSE;
s := 'hello' + ' ' + 'world';

IF x > 0 THEN
    y := 1;
ELSIF x < 0 THEN
    y := -1;
ELSE
    y := 0;
END_IF;

FOR i := 0 TO 10 BY 2 DO
    x := x + i;
END_FOR;

WHILE y > 0 DO
    y := y - 1;
END_WHILE;

x := 0;
REPEAT
    x := x + 1;
UNTIL x >= 5;
END_REPEAT;

CASE x OF
    1: y := 10;
    2: y := 20;
    3, 4, 5: y := 50;
ELSE
    y := 0;
END_CASE;

END_PROGRAM
