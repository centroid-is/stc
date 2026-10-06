{attribute 'qualified_only'}
{attribute 'strict'}
TYPE E_S : (a := 0, b := 1, c := 2) INT;
END_TYPE
TYPE E_T : (x, y);
END_TYPE
PROGRAM MAIN
VAR e : E_S; f : E_T; END_VAR
e := E_S.b;
f := y;
IF e = E_S.c THEN f := x; END_IF
END_PROGRAM
