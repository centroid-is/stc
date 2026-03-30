TEST_CASE 'SET_IO and GET_IO work'
VAR
    sensor : BOOL;
END_VAR
    SET_IO('I', 0, 3, TRUE);
    sensor := GET_IO('I', 0, 3);
    ASSERT_TRUE(sensor, 'Should read back TRUE from I0.3');

    SET_IO('Q', 2, 0, TRUE);
    ASSERT_TRUE(GET_IO('Q', 2, 0), 'Should read back TRUE from Q2.0');
END_TEST_CASE
