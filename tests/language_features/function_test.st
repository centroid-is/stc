(* Function Declaration Test Suite
   Tests FUNCTION declarations, calls, and recursion.
*)

FUNCTION Add : INT
VAR_INPUT
    a : INT;
    b : INT;
END_VAR
    Add := a + b;
END_FUNCTION

FUNCTION Factorial : DINT
VAR_INPUT
    n : DINT;
END_VAR
    IF n <= 1 THEN
        Factorial := 1;
    ELSE
        Factorial := n * Factorial(n - 1);
    END_IF;
END_FUNCTION

TEST_CASE 'Function call'
    ASSERT_EQ(Add(3, 4), 7);
END_TEST_CASE

TEST_CASE 'Recursive function'
    ASSERT_EQ(Factorial(5), 120);
END_TEST_CASE
