(* Control Flow Test Suite
   Tests FOR, WHILE, REPEAT, CASE with edge cases including EXIT, CONTINUE, and ranges.
*)

TEST_CASE 'FOR with EXIT'
VAR
    i : INT;
    sum : INT := 0;
END_VAR
    FOR i := 1 TO 100 DO
        IF i > 10 THEN EXIT; END_IF;
        sum := sum + i;
    END_FOR;
    ASSERT_EQ(sum, 55);
END_TEST_CASE

TEST_CASE 'WHILE with CONTINUE'
VAR
    i : INT := 0;
    sum : INT := 0;
END_VAR
    WHILE i < 10 DO
        i := i + 1;
        IF i MOD 2 = 0 THEN CONTINUE; END_IF;
        sum := sum + i;
    END_WHILE;
    ASSERT_EQ(sum, 25);  (* 1+3+5+7+9 *)
END_TEST_CASE

TEST_CASE 'REPEAT UNTIL'
VAR
    i : INT := 0;
END_VAR
    REPEAT
        i := i + 1;
    UNTIL i >= 5
    END_REPEAT;
    ASSERT_EQ(i, 5);
END_TEST_CASE

TEST_CASE 'Nested CASE with ranges'
VAR
    x : INT := 7;
    result : STRING;
END_VAR
    CASE x OF
        1..3: result := 'low';
        4..6: result := 'mid';
        7..9: result := 'high';
    ELSE
        result := 'other';
    END_CASE;
    ASSERT_EQ(result, 'high');
END_TEST_CASE
