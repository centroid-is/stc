PROGRAM ExprTest
VAR
    a, b, c, d : INT;
    f : BOOL;
END_VAR
    a := 1 + 2 * 3;
    b := (1 + 2) * 3;
    c := 2 ** 3 ** 2;
    d := a MOD b;
    f := a > 0 AND b < 100 OR c = d;
    f := NOT f XOR TRUE;
END_PROGRAM
