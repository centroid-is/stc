TYPE E_X : (a, b, c);
END_TYPE
PROGRAM MAIN
VAR e : E_X; n : INT; END_VAR
CASE e OF
	E_X.a: n := 1;
	E_X.b, E_X.c: n := 2;
END_CASE
END_PROGRAM
