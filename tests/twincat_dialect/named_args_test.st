(* TwinCAT dialect: named arguments in FUNCTION calls used as expressions.
   Inputs may be passed by name in any order, mixed with leading positional
   arguments, or omitted to take their declared default. `=>` binds a
   FUNCTION's VAR_OUTPUT, and TwinCAT allows a trailing comma.
*)

FUNCTION F_X : DINT
VAR_INPUT
    a : DINT;
    b : DINT;
END_VAR
    F_X := a * 10 + b;
END_FUNCTION

FUNCTION F_Scale : DINT
VAR_INPUT
    x : DINT;
    step : DINT := 3;
END_VAR
    F_Scale := x * step;
END_FUNCTION

FUNCTION F_DivMod : DINT
VAR_INPUT
    num : DINT;
    den : DINT;
END_VAR
VAR_OUTPUT
    rest : DINT;
END_VAR
    rest := num MOD den;
    F_DivMod := num / den;
END_FUNCTION

TEST_CASE 'Named arguments equal positional ones'
VAR
    n : DINT;
    m : DINT;
END_VAR
    n := F_X(a := 1, b := 2);
    m := F_X(1, 2);
    ASSERT_EQ(n, 12);
    ASSERT_EQ(n, m);
END_TEST_CASE

TEST_CASE 'Named arguments in a different order'
VAR
    n : DINT;
END_VAR
    n := F_X(b := 2, a := 1);
    ASSERT_EQ(n, 12);
END_TEST_CASE

TEST_CASE 'Positional then named arguments'
VAR
    n : DINT;
END_VAR
    n := F_X(4, b := 5);
    ASSERT_EQ(n, 45);
END_TEST_CASE

TEST_CASE 'Omitted input takes its default'
VAR
    n : DINT;
END_VAR
    n := F_Scale(x := 7);
    ASSERT_EQ(n, 21);
    n := F_Scale(x := 7, step := 2);
    ASSERT_EQ(n, 14);
END_TEST_CASE

TEST_CASE 'Output binding on a FUNCTION'
VAR
    q : DINT;
    r : DINT;
END_VAR
    q := F_DivMod(num := 17, den := 5, rest => r);
    ASSERT_EQ(q, 3);
    ASSERT_EQ(r, 2);
END_TEST_CASE

TEST_CASE 'Trailing comma in a call'
VAR
    n : DINT;
END_VAR
    n := F_X(a := 2, b := 3,);
    ASSERT_EQ(n, 23);
END_TEST_CASE
