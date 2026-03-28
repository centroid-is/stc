TEST_CASE 'Multiple assertions collected'
VAR
    x : INT := 10;
END_VAR
    ASSERT_TRUE(x > 5);
    ASSERT_EQ(x, 99);
    ASSERT_TRUE(x < 100);
END_TEST_CASE
