(* Expression Test Suite
   Exercises operator precedence, boolean logic, typed literals,
   string operations, power operator, MOD, and unary operators.
*)

(* ===== OPERATOR PRECEDENCE ===== *)

TEST_CASE 'Multiplication before addition'
VAR
    r : INT;
END_VAR
    r := 2 + 3 * 4;
    ASSERT_EQ(r, 14);
END_TEST_CASE

TEST_CASE 'Parentheses override precedence'
VAR
    r : INT;
END_VAR
    r := (2 + 3) * 4;
    ASSERT_EQ(r, 20);
END_TEST_CASE

TEST_CASE 'Division before subtraction'
VAR
    r : INT;
END_VAR
    r := 20 - 10 / 2;
    ASSERT_EQ(r, 15);
END_TEST_CASE

TEST_CASE 'Nested parentheses'
VAR
    r : INT;
END_VAR
    r := ((2 + 3) * (4 - 1)) + 1;
    ASSERT_EQ(r, 16);  (* (5 * 3) + 1 = 16 *)
END_TEST_CASE

TEST_CASE 'Mixed integer and real arithmetic'
VAR
    r : REAL;
END_VAR
    r := 10.0 / 3.0;
    ASSERT_NEAR(r, 3.333, 0.01);
END_TEST_CASE

(* ===== POWER OPERATOR ===== *)

TEST_CASE 'Power operator integer'
VAR
    r : INT;
END_VAR
    r := 2 ** 10;
    ASSERT_EQ(r, 1024);
END_TEST_CASE

TEST_CASE 'Power of zero'
VAR
    r : INT;
END_VAR
    r := 5 ** 0;
    ASSERT_EQ(r, 1);
END_TEST_CASE

TEST_CASE 'Power of one'
VAR
    r : INT;
END_VAR
    r := 7 ** 1;
    ASSERT_EQ(r, 7);
END_TEST_CASE

(* ===== MOD OPERATOR ===== *)

TEST_CASE 'MOD basic'
VAR
    r : INT;
END_VAR
    r := 17 MOD 5;
    ASSERT_EQ(r, 2);
END_TEST_CASE

TEST_CASE 'MOD with zero remainder'
VAR
    r : INT;
END_VAR
    r := 20 MOD 5;
    ASSERT_EQ(r, 0);
END_TEST_CASE

TEST_CASE 'MOD with small dividend'
VAR
    r : INT;
END_VAR
    r := 3 MOD 7;
    ASSERT_EQ(r, 3);
END_TEST_CASE

(* ===== BOOLEAN EXPRESSIONS ===== *)

TEST_CASE 'Boolean AND has higher precedence than OR'
VAR
    r : BOOL;
END_VAR
    (* TRUE OR (FALSE AND TRUE) = TRUE OR FALSE = TRUE *)
    r := TRUE OR FALSE AND TRUE;
    ASSERT_TRUE(r);
END_TEST_CASE

TEST_CASE 'Boolean NOT has highest precedence'
VAR
    r : BOOL;
END_VAR
    r := NOT FALSE AND TRUE;
    ASSERT_TRUE(r);
END_TEST_CASE

TEST_CASE 'Boolean XOR'
VAR
    r : BOOL;
END_VAR
    r := TRUE XOR FALSE;
    ASSERT_TRUE(r);

    r := TRUE XOR TRUE;
    ASSERT_FALSE(r);

    r := FALSE XOR FALSE;
    ASSERT_FALSE(r);
END_TEST_CASE

TEST_CASE 'Complex boolean expression'
VAR
    x : INT := 50;
    r : BOOL;
END_VAR
    r := x > 0 AND x < 100;
    ASSERT_TRUE(r);

    r := x > 0 AND x < 10;
    ASSERT_FALSE(r);

    r := x < 0 OR x > 40;
    ASSERT_TRUE(r);
END_TEST_CASE

(* ===== COMPARISON OPERATORS ===== *)

TEST_CASE 'All comparison operators'
VAR
    a : INT := 10;
    b : INT := 20;
END_VAR
    ASSERT_TRUE(a < b);
    ASSERT_TRUE(a <= b);
    ASSERT_TRUE(b > a);
    ASSERT_TRUE(b >= a);
    ASSERT_TRUE(a <> b);
    ASSERT_FALSE(a = b);
    ASSERT_TRUE(a = 10);
    ASSERT_TRUE(a <= 10);
    ASSERT_TRUE(a >= 10);
END_TEST_CASE

TEST_CASE 'String comparison'
VAR
    s1 : STRING := 'ABC';
    s2 : STRING := 'ABC';
    s3 : STRING := 'XYZ';
END_VAR
    ASSERT_TRUE(s1 = s2);
    ASSERT_TRUE(s1 <> s3);
END_TEST_CASE

(* ===== UNARY OPERATORS ===== *)

TEST_CASE 'Unary NOT'
VAR
    r : BOOL;
END_VAR
    r := NOT TRUE;
    ASSERT_FALSE(r);

    r := NOT FALSE;
    ASSERT_TRUE(r);

    r := NOT NOT TRUE;
    ASSERT_TRUE(r);
END_TEST_CASE

TEST_CASE 'Unary minus'
VAR
    r : INT;
END_VAR
    r := -42;
    ASSERT_EQ(r, -42);

    r := -(-42);
    ASSERT_EQ(r, 42);
END_TEST_CASE

(* ===== TYPED LITERALS ===== *)

TEST_CASE 'Typed literal INT#'
VAR
    r : INT;
END_VAR
    r := INT#42;
    ASSERT_EQ(r, 42);
END_TEST_CASE

TEST_CASE 'Typed literal BOOL#'
VAR
    r : BOOL;
END_VAR
    r := BOOL#TRUE;
    ASSERT_TRUE(r);

    r := BOOL#FALSE;
    ASSERT_FALSE(r);
END_TEST_CASE

TEST_CASE 'Hex literal'
VAR
    r : INT;
END_VAR
    r := 16#FF;
    ASSERT_EQ(r, 255);
END_TEST_CASE

TEST_CASE 'Time literal'
VAR
    t : TIME;
END_VAR
    t := T#5s;
    ASSERT_TRUE(t = T#5s);
END_TEST_CASE

(* ===== STRING EXPRESSIONS ===== *)

TEST_CASE 'CONCAT multiple strings'
VAR
    r : STRING;
END_VAR
    r := CONCAT('Hello', ' World');
    ASSERT_EQ(r, 'Hello World');
END_TEST_CASE

TEST_CASE 'Nested CONCAT'
VAR
    r : STRING;
END_VAR
    r := CONCAT(CONCAT('A', 'B'), CONCAT('C', 'D'));
    ASSERT_EQ(r, 'ABCD');
END_TEST_CASE

TEST_CASE 'String length of concatenated result'
VAR
    s : STRING;
    n : INT;
END_VAR
    s := CONCAT('Hello', ' World');
    n := LEN(s);
    ASSERT_EQ(n, 11);
END_TEST_CASE

(* ===== COMPLEX EXPRESSION COMBINATIONS ===== *)

TEST_CASE 'Arithmetic in function arguments'
VAR
    r : INT;
END_VAR
    r := MAX(3 * 4, 2 + 9);
    ASSERT_EQ(r, 12);

    r := MIN(100 - 50, 30 + 10);
    ASSERT_EQ(r, 40);
END_TEST_CASE

TEST_CASE 'Comparison result used in SEL'
VAR
    x : INT := 42;
    r : STRING;
END_VAR
    (* SEL(FALSE, a, b) = a; SEL(TRUE, a, b) = b *)
    r := SEL(x > 50, 'small', 'big');
    ASSERT_EQ(r, 'small');  (* x=42, 42>50 is FALSE, returns first *)

    r := SEL(x < 50, 'small', 'big');
    ASSERT_EQ(r, 'big');  (* x=42, 42<50 is TRUE, returns second *)
END_TEST_CASE

TEST_CASE 'Chained arithmetic operations'
VAR
    r : INT;
END_VAR
    r := 1 + 2 + 3 + 4 + 5;
    ASSERT_EQ(r, 15);

    r := 100 - 10 - 20 - 30;
    ASSERT_EQ(r, 40);
END_TEST_CASE
