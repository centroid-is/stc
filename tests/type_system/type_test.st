(* Type System Test Suite
   Validates all elementary types, arithmetic, conversions,
   string operations, arrays, and control flow constructs.
*)

TEST_CASE 'Integer arithmetic on all widths'
VAR
    nSint : SINT := 10;
    nInt  : INT := 1000;
    nDint : DINT := 100000;
    nLint : LINT := 1000000;
END_VAR
    nSint := nSint + 5;
    ASSERT_EQ(nSint, 15);

    nInt := nInt * 2;
    ASSERT_EQ(nInt, 2000);

    nDint := nDint - 50000;
    ASSERT_EQ(nDint, 50000);

    nLint := nLint + 500000;
    ASSERT_EQ(nLint, 1500000);
END_TEST_CASE

TEST_CASE 'Unsigned integer types'
VAR
    nUsint : USINT := 200;
    nUint  : UINT := 50000;
    nUdint : UDINT := 4000000;
END_VAR
    nUsint := nUsint + 55;
    ASSERT_EQ(nUsint, 255);

    nUint := nUint - 10000;
    ASSERT_EQ(nUint, 40000);

    nUdint := nUdint * 2;
    ASSERT_EQ(nUdint, 8000000);
END_TEST_CASE

TEST_CASE 'REAL arithmetic and ASSERT_NEAR'
VAR
    fReal  : REAL := 3.14;
    fLreal : LREAL := 2.718281828;
    result : REAL;
END_VAR
    result := fReal * 2.0;
    ASSERT_NEAR(result, 6.28, 0.001);

    fLreal := fLreal + 1.0;
    ASSERT_NEAR(fLreal, 3.718281828, 0.000001);

    (* Division *)
    result := 10.0 / 3.0;
    ASSERT_NEAR(result, 3.333333, 0.001);
END_TEST_CASE

TEST_CASE 'Boolean logic operators'
VAR
    a : BOOL := TRUE;
    b : BOOL := FALSE;
    r : BOOL;
END_VAR
    r := a AND b;
    ASSERT_FALSE(r);

    r := a OR b;
    ASSERT_TRUE(r);

    r := a XOR b;
    ASSERT_TRUE(r);

    r := NOT a;
    ASSERT_FALSE(r);

    r := a AND NOT b;
    ASSERT_TRUE(r);

    (* Double negation *)
    r := NOT NOT a;
    ASSERT_TRUE(r);
END_TEST_CASE

TEST_CASE 'Comparison operators'
VAR
    x : INT := 10;
    y : INT := 20;
END_VAR
    ASSERT_TRUE(x < y);
    ASSERT_TRUE(y > x);
    ASSERT_TRUE(x <= 10);
    ASSERT_TRUE(y >= 20);
    ASSERT_TRUE(x <> y);
    ASSERT_FALSE(x = y);
    ASSERT_TRUE(x = 10);
END_TEST_CASE

TEST_CASE 'Type conversions INT to REAL'
VAR
    n : INT := 42;
    f : REAL;
END_VAR
    f := INT_TO_REAL(n);
    ASSERT_NEAR(f, 42.0, 0.01);
END_TEST_CASE

TEST_CASE 'Type conversions REAL to INT'
VAR
    f : REAL := 3.7;
    n : INT;
END_VAR
    n := REAL_TO_INT(f);
    ASSERT_EQ(n, 4);  (* Banker's rounding: 3.7 rounds to 4 *)
END_TEST_CASE

TEST_CASE 'Type conversions BOOL to INT and back'
VAR
    b : BOOL := TRUE;
    n : INT;
    b2 : BOOL;
END_VAR
    n := BOOL_TO_INT(b);
    ASSERT_EQ(n, 1);

    b2 := INT_TO_BOOL(0);
    ASSERT_FALSE(b2);

    b2 := INT_TO_BOOL(42);
    ASSERT_TRUE(b2);
END_TEST_CASE

TEST_CASE 'INT to STRING conversion'
VAR
    n : INT := 123;
    s : STRING;
END_VAR
    s := INT_TO_STRING(n);
    ASSERT_EQ(s, '123');
END_TEST_CASE

TEST_CASE 'STRING to INT conversion'
VAR
    s : STRING := '456';
    n : INT;
END_VAR
    n := STRING_TO_INT(s);
    ASSERT_EQ(n, 456);
END_TEST_CASE

TEST_CASE 'String functions: LEN, LEFT, RIGHT, MID'
VAR
    s : STRING := 'Hello World';
    r : STRING;
    n : INT;
END_VAR
    n := LEN(s);
    ASSERT_EQ(n, 11);

    r := LEFT(s, 5);
    ASSERT_EQ(r, 'Hello');

    r := RIGHT(s, 5);
    ASSERT_EQ(r, 'World');

    r := MID(s, 5, 7);  (* 5 chars starting at position 7 *)
    ASSERT_EQ(r, 'World');
END_TEST_CASE

TEST_CASE 'String CONCAT and FIND'
VAR
    s1 : STRING := 'Foo';
    s2 : STRING := 'Bar';
    result : STRING;
    pos : INT;
END_VAR
    result := CONCAT(s1, s2);
    ASSERT_EQ(result, 'FooBar');

    pos := FIND(result, 'Bar');
    ASSERT_EQ(pos, 4);  (* 1-based position *)

    pos := FIND(result, 'xyz');
    ASSERT_EQ(pos, 0);  (* Not found *)
END_TEST_CASE

TEST_CASE 'FOR loop with accumulator'
VAR
    sum : INT := 0;
    i : INT;
END_VAR
    FOR i := 1 TO 10 DO
        sum := sum + i;
    END_FOR;
    ASSERT_EQ(sum, 55);
END_TEST_CASE

TEST_CASE 'FOR loop with BY step'
VAR
    sum : INT := 0;
    i : INT;
END_VAR
    FOR i := 0 TO 10 BY 2 DO
        sum := sum + i;
    END_FOR;
    (* 0+2+4+6+8+10 = 30 *)
    ASSERT_EQ(sum, 30);
END_TEST_CASE

TEST_CASE 'WHILE loop'
VAR
    count : INT := 0;
    value : INT := 1;
END_VAR
    WHILE value < 100 DO
        value := value * 2;
        count := count + 1;
    END_WHILE;
    (* 1->2->4->8->16->32->64->128, 7 iterations *)
    ASSERT_EQ(count, 7);
    ASSERT_EQ(value, 128);
END_TEST_CASE

TEST_CASE 'REPEAT-UNTIL loop'
VAR
    n : INT := 10;
    count : INT := 0;
END_VAR
    REPEAT
        n := n - 3;
        count := count + 1;
    UNTIL n <= 0
    END_REPEAT;
    (* 10->7->4->1->-2: 4 iterations *)
    ASSERT_EQ(count, 4);
    ASSERT_TRUE(n <= 0);
END_TEST_CASE

TEST_CASE 'CASE statement with integer labels'
VAR
    cmd : INT := 2;
    result : STRING := '';
END_VAR
    CASE cmd OF
        0: result := 'STOP';
        1: result := 'SLOW';
        2: result := 'RUN';
        3: result := 'FAST';
    ELSE
        result := 'UNKNOWN';
    END_CASE;
    ASSERT_EQ(result, 'RUN');
END_TEST_CASE

TEST_CASE 'CASE statement ELSE branch'
VAR
    cmd : INT := 99;
    result : STRING := '';
END_VAR
    CASE cmd OF
        0: result := 'STOP';
        1: result := 'RUN';
    ELSE
        result := 'UNKNOWN';
    END_CASE;
    ASSERT_EQ(result, 'UNKNOWN');
END_TEST_CASE

TEST_CASE 'Nested IF-ELSIF-ELSE'
VAR
    temp : REAL := 85.0;
    alarm : INT := 0;
END_VAR
    IF temp > 100.0 THEN
        alarm := 3;  (* Critical *)
    ELSIF temp > 80.0 THEN
        alarm := 2;  (* Warning *)
    ELSIF temp > 60.0 THEN
        alarm := 1;  (* Advisory *)
    ELSE
        alarm := 0;  (* Normal *)
    END_IF;
    ASSERT_EQ(alarm, 2);
END_TEST_CASE

TEST_CASE 'EXIT breaks out of loop'
VAR
    i : INT;
    last : INT := 0;
END_VAR
    FOR i := 1 TO 100 DO
        IF i > 5 THEN
            EXIT;
        END_IF;
        last := i;
    END_FOR;
    ASSERT_EQ(last, 5);
END_TEST_CASE

TEST_CASE 'Negative integer arithmetic'
VAR
    a : INT := -10;
    b : INT := 3;
    r : INT;
END_VAR
    r := a + b;
    ASSERT_EQ(r, -7);

    r := a * b;
    ASSERT_EQ(r, -30);

    r := a - b;
    ASSERT_EQ(r, -13);
END_TEST_CASE

TEST_CASE 'REAL precision with small values'
VAR
    a : REAL := 0.1;
    b : REAL := 0.2;
    sum : REAL;
END_VAR
    sum := a + b;
    (* Floating point: 0.1+0.2 is not exactly 0.3 *)
    ASSERT_NEAR(sum, 0.3, 0.001);
END_TEST_CASE

TEST_CASE 'Multiple assignments in sequence'
VAR
    x : INT;
    y : INT;
    z : INT;
END_VAR
    x := 1;
    y := x + 1;
    z := y + 1;
    ASSERT_EQ(x, 1);
    ASSERT_EQ(y, 2);
    ASSERT_EQ(z, 3);
END_TEST_CASE
