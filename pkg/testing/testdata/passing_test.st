TEST_CASE 'Addition works'
VAR
    x : INT := 5;
    y : INT := 10;
    result : INT;
END_VAR
    result := x + y;
    ASSERT_EQ(result, 15);
    ASSERT_TRUE(result > 0);
END_TEST_CASE

TEST_CASE 'Boolean logic'
VAR
    a : BOOL := TRUE;
    b : BOOL := FALSE;
END_VAR
    ASSERT_TRUE(a);
    ASSERT_FALSE(b);
    ASSERT_EQ(a OR b, TRUE);
END_TEST_CASE
