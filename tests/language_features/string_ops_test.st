(* String Operations Test Suite
   Tests string concatenation, comparison, and built-in string functions.
*)

TEST_CASE 'String concatenation'
VAR
    s1 : STRING := 'Hello';
    s2 : STRING := ' World';
    result : STRING;
END_VAR
    result := CONCAT(s1, s2);
    ASSERT_EQ(result, 'Hello World');
    ASSERT_EQ(LEN(result), 11);
END_TEST_CASE

TEST_CASE 'String comparison'
VAR
    a : STRING := 'ABC';
    b : STRING := 'DEF';
END_VAR
    ASSERT_TRUE(a < b);
    ASSERT_FALSE(a = b);
END_TEST_CASE
