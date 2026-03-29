(* Advanced Control Flow Test Suite
   Exercises nested loops, EXIT/CONTINUE in various loop types,
   RETURN from functions, CASE with ranges and multiple labels,
   REPEAT..UNTIL with complex conditions, and empty statements.
*)

FUNCTION Clamp : INT
VAR_INPUT
    val : INT;
    lo : INT;
    hi : INT;
END_VAR
    IF val < lo THEN
        Clamp := lo;
        RETURN;
    END_IF;
    IF val > hi THEN
        Clamp := hi;
        RETURN;
    END_IF;
    Clamp := val;
END_FUNCTION

FUNCTION SumUpTo : INT
VAR_INPUT
    n : INT;
END_VAR
VAR
    i : INT;
    total : INT := 0;
END_VAR
    FOR i := 1 TO n DO
        total := total + i;
    END_FOR;
    SumUpTo := total;
END_FUNCTION

(* ===== RETURN FROM FUNCTION ===== *)

TEST_CASE 'RETURN from middle of function'
VAR
    r : INT;
END_VAR
    r := Clamp(50, 0, 100);
    ASSERT_EQ(r, 50);

    r := Clamp(-10, 0, 100);
    ASSERT_EQ(r, 0);

    r := Clamp(200, 0, 100);
    ASSERT_EQ(r, 100);
END_TEST_CASE

TEST_CASE 'Function with FOR loop'
VAR
    r : INT;
END_VAR
    r := SumUpTo(10);
    ASSERT_EQ(r, 55);

    r := SumUpTo(0);
    ASSERT_EQ(r, 0);

    r := SumUpTo(1);
    ASSERT_EQ(r, 1);
END_TEST_CASE

(* ===== NESTED LOOPS ===== *)

TEST_CASE 'Nested FOR loops'
VAR
    i : INT;
    j : INT;
    count : INT := 0;
END_VAR
    FOR i := 1 TO 5 DO
        FOR j := 1 TO 3 DO
            count := count + 1;
        END_FOR;
    END_FOR;
    ASSERT_EQ(count, 15);  (* 5 * 3 *)
END_TEST_CASE

TEST_CASE 'IF inside FOR inside WHILE'
VAR
    outer : INT := 0;
    i : INT;
    total : INT := 0;
END_VAR
    WHILE outer < 3 DO
        outer := outer + 1;
        FOR i := 1 TO 4 DO
            IF i MOD 2 = 0 THEN
                total := total + i;
            END_IF;
        END_FOR;
    END_WHILE;
    (* Each FOR iteration adds 2+4=6, outer runs 3 times: 3*6=18 *)
    ASSERT_EQ(total, 18);
END_TEST_CASE

(* ===== EXIT FROM NESTED LOOPS ===== *)

TEST_CASE 'EXIT from inner loop only'
VAR
    i : INT;
    j : INT;
    outerCount : INT := 0;
    innerSum : INT := 0;
END_VAR
    FOR i := 1 TO 5 DO
        outerCount := outerCount + 1;
        FOR j := 1 TO 100 DO
            IF j > 3 THEN
                EXIT;
            END_IF;
            innerSum := innerSum + j;
        END_FOR;
    END_FOR;
    ASSERT_EQ(outerCount, 5);  (* Outer loop runs fully *)
    ASSERT_EQ(innerSum, 30);   (* Each inner: 1+2+3=6, times 5 = 30 *)
END_TEST_CASE

TEST_CASE 'EXIT from WHILE loop'
VAR
    i : INT := 0;
END_VAR
    WHILE TRUE DO
        i := i + 1;
        IF i >= 10 THEN
            EXIT;
        END_IF;
    END_WHILE;
    ASSERT_EQ(i, 10);
END_TEST_CASE

(* ===== CONTINUE IN LOOPS ===== *)

TEST_CASE 'CONTINUE in FOR loop skips even numbers'
VAR
    i : INT;
    sum : INT := 0;
END_VAR
    FOR i := 1 TO 10 DO
        IF i MOD 2 = 0 THEN
            CONTINUE;
        END_IF;
        sum := sum + i;
    END_FOR;
    (* 1+3+5+7+9 = 25 *)
    ASSERT_EQ(sum, 25);
END_TEST_CASE

TEST_CASE 'CONTINUE in WHILE loop'
VAR
    i : INT := 0;
    sum : INT := 0;
END_VAR
    WHILE i < 10 DO
        i := i + 1;
        IF i MOD 3 = 0 THEN
            CONTINUE;
        END_IF;
        sum := sum + i;
    END_WHILE;
    (* Skip 3,6,9: 1+2+4+5+7+8+10 = 37 *)
    ASSERT_EQ(sum, 37);
END_TEST_CASE

(* ===== CASE WITH RANGES ===== *)

TEST_CASE 'CASE with range labels'
VAR
    x : INT;
    result : STRING;
END_VAR
    x := 3;
    CASE x OF
        1..5: result := 'low';
        6..10: result := 'high';
    ELSE
        result := 'other';
    END_CASE;
    ASSERT_EQ(result, 'low');

    x := 8;
    CASE x OF
        1..5: result := 'low';
        6..10: result := 'high';
    ELSE
        result := 'other';
    END_CASE;
    ASSERT_EQ(result, 'high');

    x := 99;
    CASE x OF
        1..5: result := 'low';
        6..10: result := 'high';
    ELSE
        result := 'other';
    END_CASE;
    ASSERT_EQ(result, 'other');
END_TEST_CASE

TEST_CASE 'CASE with multiple labels'
VAR
    x : INT;
    result : STRING;
END_VAR
    x := 3;
    CASE x OF
        1, 3, 5: result := 'odd';
        2, 4, 6: result := 'even';
    ELSE
        result := 'other';
    END_CASE;
    ASSERT_EQ(result, 'odd');

    x := 4;
    CASE x OF
        1, 3, 5: result := 'odd';
        2, 4, 6: result := 'even';
    ELSE
        result := 'other';
    END_CASE;
    ASSERT_EQ(result, 'even');
END_TEST_CASE

TEST_CASE 'CASE with mixed ranges and values'
VAR
    grade : INT := 85;
    letter : STRING;
END_VAR
    CASE grade OF
        90..100: letter := 'A';
        80..89: letter := 'B';
        70..79: letter := 'C';
        60..69: letter := 'D';
    ELSE
        letter := 'F';
    END_CASE;
    ASSERT_EQ(letter, 'B');
END_TEST_CASE

(* ===== REPEAT..UNTIL ===== *)

TEST_CASE 'REPEAT UNTIL with compound condition'
VAR
    x : INT := 1;
    y : INT := 100;
END_VAR
    REPEAT
        x := x * 2;
        y := y - 10;
    UNTIL x >= 32 OR y <= 0
    END_REPEAT;
    ASSERT_EQ(x, 32);  (* 1->2->4->8->16->32, 5 iterations *)
    ASSERT_EQ(y, 50);
END_TEST_CASE

TEST_CASE 'REPEAT executes at least once'
VAR
    count : INT := 0;
END_VAR
    REPEAT
        count := count + 1;
    UNTIL TRUE
    END_REPEAT;
    ASSERT_EQ(count, 1);  (* Body runs once even though condition is immediately true *)
END_TEST_CASE

(* ===== EMPTY STATEMENT ===== *)

TEST_CASE 'Empty statement is valid'
VAR
    x : INT := 42;
END_VAR
    ;  (* Empty statement *)
    ;
    ASSERT_EQ(x, 42);
END_TEST_CASE

(* ===== FOR WITH NEGATIVE STEP ===== *)

TEST_CASE 'FOR loop counting down with BY -1'
VAR
    i : INT;
    sum : INT := 0;
END_VAR
    FOR i := 5 TO 1 BY -1 DO
        sum := sum + i;
    END_FOR;
    ASSERT_EQ(sum, 15);  (* 5+4+3+2+1 *)
END_TEST_CASE

TEST_CASE 'FOR loop with BY step of 3'
VAR
    i : INT;
    sum : INT := 0;
END_VAR
    FOR i := 0 TO 12 BY 3 DO
        sum := sum + i;
    END_FOR;
    (* 0+3+6+9+12 = 30 *)
    ASSERT_EQ(sum, 30);
END_TEST_CASE

(* ===== DEEPLY NESTED IF ===== *)

TEST_CASE 'Deeply nested IF-ELSIF-ELSE'
VAR
    a : INT := 1;
    b : INT := 2;
    c : INT := 3;
    result : INT := 0;
END_VAR
    IF a = 1 THEN
        IF b = 2 THEN
            IF c = 3 THEN
                result := 100;
            ELSE
                result := 200;
            END_IF;
        ELSE
            result := 300;
        END_IF;
    ELSE
        result := 400;
    END_IF;
    ASSERT_EQ(result, 100);
END_TEST_CASE
