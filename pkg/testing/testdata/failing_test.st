TEST_CASE 'Intentional failure'
VAR
    x : INT := 1;
    y : INT := 2;
END_VAR
    ASSERT_EQ(x, y);
END_TEST_CASE
