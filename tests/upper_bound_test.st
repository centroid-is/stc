TEST_CASE 'UPPER_BOUND follows the declaration'
VAR
    a : ARRAY[1..10] OF DINT;
    b : ARRAY[1..3] OF LREAL;
    n : DINT;
    total : DINT;
END_VAR
    ASSERT_EQ(UPPER_BOUND(a, 1), 10);
    ASSERT_EQ(UPPER_BOUND(b, 1), 3);
    total := 0;
    FOR n := 1 TO UPPER_BOUND(a, 1) DO
        total := total + 1;
    END_FOR;
    ASSERT_EQ(total, 10);
END_TEST_CASE
