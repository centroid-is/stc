(* Standard Library Comprehensive Test Suite
   Exercises every stdlib function block and function:
   - Timers: TON, TOF, TP
   - Counters: CTU, CTD, CTUD
   - Edge detectors: R_TRIG, F_TRIG
   - Bistables: SR, RS
   - Math: ABS, SQRT, SIN, COS, TAN, ASIN, ACOS, ATAN, LN, LOG, EXP, MIN, MAX, LIMIT, SEL, MUX, MOVE
   - String: LEN, LEFT, RIGHT, MID, CONCAT, FIND
   - Conversions: INT_TO_REAL, REAL_TO_INT, BOOL_TO_INT, INT_TO_BOOL, INT_TO_STRING, STRING_TO_INT, etc.
*)

(* ===== TIMERS ===== *)

TEST_CASE 'TON: output delayed by preset time'
VAR
    t : TON;
END_VAR
    t(IN := TRUE, PT := T#200ms);
    ASSERT_FALSE(t.Q);

    ADVANCE_TIME(T#100ms);
    t(IN := TRUE, PT := T#200ms);
    ASSERT_FALSE(t.Q);

    ADVANCE_TIME(T#150ms);
    t(IN := TRUE, PT := T#200ms);
    ASSERT_TRUE(t.Q);
END_TEST_CASE

TEST_CASE 'TON: resets when IN goes FALSE'
VAR
    t : TON;
END_VAR
    t(IN := TRUE, PT := T#100ms);
    ADVANCE_TIME(T#110ms);
    t(IN := TRUE, PT := T#100ms);
    ASSERT_TRUE(t.Q);

    (* Drop IN — need a tick for the FB to process the new input *)
    t(IN := FALSE, PT := T#100ms);
    ADVANCE_TIME(T#1ms);
    t(IN := FALSE, PT := T#100ms);
    ASSERT_FALSE(t.Q);

    (* Re-raise IN, timer restarts from zero *)
    t(IN := TRUE, PT := T#100ms);
    ADVANCE_TIME(T#1ms);
    t(IN := TRUE, PT := T#100ms);
    ASSERT_FALSE(t.Q);

    ADVANCE_TIME(T#50ms);
    t(IN := TRUE, PT := T#100ms);
    ASSERT_FALSE(t.Q);
END_TEST_CASE

TEST_CASE 'TOF: output stays TRUE for preset after IN drops'
VAR
    t : TOF;
END_VAR
    (* Raise IN: Q goes TRUE immediately *)
    t(IN := TRUE, PT := T#500ms);
    t(IN := TRUE, PT := T#500ms);
    ASSERT_TRUE(t.Q);

    (* Drop IN: Q stays TRUE during off-delay *)
    t(IN := FALSE, PT := T#500ms);
    ADVANCE_TIME(T#250ms);
    t(IN := FALSE, PT := T#500ms);
    ASSERT_TRUE(t.Q);

    (* After preset expires *)
    ADVANCE_TIME(T#300ms);
    t(IN := FALSE, PT := T#500ms);
    ASSERT_FALSE(t.Q);
END_TEST_CASE

TEST_CASE 'TP: pulse for exactly preset duration'
VAR
    t : TP;
END_VAR
    (* Trigger pulse *)
    t(IN := FALSE, PT := T#300ms);
    t(IN := TRUE, PT := T#300ms);
    ASSERT_TRUE(t.Q);

    (* During pulse *)
    ADVANCE_TIME(T#150ms);
    t(IN := TRUE, PT := T#300ms);
    ASSERT_TRUE(t.Q);

    (* After pulse duration *)
    ADVANCE_TIME(T#200ms);
    t(IN := TRUE, PT := T#300ms);
    ASSERT_FALSE(t.Q);
END_TEST_CASE

(* ===== COUNTERS ===== *)

TEST_CASE 'CTU: counts up on rising edges'
VAR
    c : CTU;
END_VAR
    c(CU := FALSE, R := FALSE, PV := 5);

    (* Count 3 rising edges *)
    c(CU := TRUE, R := FALSE, PV := 5);
    ASSERT_EQ(c.CV, 1);
    c(CU := FALSE, R := FALSE, PV := 5);

    c(CU := TRUE, R := FALSE, PV := 5);
    ASSERT_EQ(c.CV, 2);
    c(CU := FALSE, R := FALSE, PV := 5);

    c(CU := TRUE, R := FALSE, PV := 5);
    ASSERT_EQ(c.CV, 3);
    ASSERT_FALSE(c.Q);  (* Not yet at PV=5 *)
END_TEST_CASE

TEST_CASE 'CTU: Q goes TRUE at preset value'
VAR
    c : CTU;
    i : INT;
END_VAR
    c(CU := FALSE, R := FALSE, PV := 3);
    FOR i := 1 TO 3 DO
        c(CU := TRUE, R := FALSE, PV := 3);
        c(CU := FALSE, R := FALSE, PV := 3);
    END_FOR;
    ASSERT_EQ(c.CV, 3);
    ASSERT_TRUE(c.Q);
END_TEST_CASE

TEST_CASE 'CTU: reset clears counter'
VAR
    c : CTU;
END_VAR
    c(CU := FALSE, R := FALSE, PV := 10);
    c(CU := TRUE, R := FALSE, PV := 10);
    c(CU := FALSE, R := FALSE, PV := 10);
    c(CU := TRUE, R := FALSE, PV := 10);
    ASSERT_EQ(c.CV, 2);

    (* Reset *)
    c(CU := FALSE, R := TRUE, PV := 10);
    ASSERT_EQ(c.CV, 0);
    ASSERT_FALSE(c.Q);
END_TEST_CASE

TEST_CASE 'CTD: counts down on rising edges'
VAR
    c : CTD;
END_VAR
    (* Load PV *)
    c(CD := FALSE, LD := TRUE, PV := 3);
    ASSERT_EQ(c.CV, 3);
    ASSERT_FALSE(c.Q);

    c(CD := FALSE, LD := FALSE, PV := 3);

    (* Count down *)
    c(CD := TRUE, LD := FALSE, PV := 3);
    ASSERT_EQ(c.CV, 2);
    c(CD := FALSE, LD := FALSE, PV := 3);

    c(CD := TRUE, LD := FALSE, PV := 3);
    ASSERT_EQ(c.CV, 1);
    c(CD := FALSE, LD := FALSE, PV := 3);

    c(CD := TRUE, LD := FALSE, PV := 3);
    ASSERT_EQ(c.CV, 0);
    ASSERT_TRUE(c.Q);  (* CV <= 0 *)
END_TEST_CASE

TEST_CASE 'CTUD: counts up and down'
VAR
    c : CTUD;
END_VAR
    c(CU := FALSE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);

    (* Count up 3 times *)
    c(CU := TRUE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    c(CU := FALSE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    c(CU := TRUE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    c(CU := FALSE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    c(CU := TRUE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    c(CU := FALSE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    ASSERT_EQ(c.CV, 3);

    (* Count down 1 time *)
    c(CU := FALSE, CD := TRUE, R := FALSE, LD := FALSE, PV := 5);
    c(CU := FALSE, CD := FALSE, R := FALSE, LD := FALSE, PV := 5);
    ASSERT_EQ(c.CV, 2);
END_TEST_CASE

TEST_CASE 'CTUD: reset has priority over load'
VAR
    c : CTUD;
END_VAR
    (* Load PV=5 first *)
    c(CU := FALSE, CD := FALSE, R := FALSE, LD := TRUE, PV := 5);
    ASSERT_EQ(c.CV, 5);

    (* Reset wins over load *)
    c(CU := FALSE, CD := FALSE, R := TRUE, LD := TRUE, PV := 5);
    ASSERT_EQ(c.CV, 0);
END_TEST_CASE

(* ===== EDGE DETECTORS ===== *)

TEST_CASE 'R_TRIG: detects rising edge only'
VAR
    rt : R_TRIG;
END_VAR
    rt(CLK := FALSE);
    ASSERT_FALSE(rt.Q);

    rt(CLK := TRUE);
    ASSERT_TRUE(rt.Q);

    (* Held high: no edge *)
    rt(CLK := TRUE);
    ASSERT_FALSE(rt.Q);

    rt(CLK := FALSE);
    ASSERT_FALSE(rt.Q);

    (* New rising edge *)
    rt(CLK := TRUE);
    ASSERT_TRUE(rt.Q);
END_TEST_CASE

TEST_CASE 'F_TRIG: detects falling edge only'
VAR
    ft : F_TRIG;
END_VAR
    ft(CLK := FALSE);
    ASSERT_FALSE(ft.Q);

    ft(CLK := TRUE);
    ASSERT_FALSE(ft.Q);

    (* Falling edge *)
    ft(CLK := FALSE);
    ASSERT_TRUE(ft.Q);

    (* Held low: no edge *)
    ft(CLK := FALSE);
    ASSERT_FALSE(ft.Q);
END_TEST_CASE

(* ===== BISTABLES ===== *)

TEST_CASE 'SR: set-dominant bistable'
VAR
    sr : SR;
END_VAR
    (* Initial state: Q1=FALSE *)
    sr(S1 := FALSE, R := FALSE);
    ASSERT_FALSE(sr.Q1);

    (* Set *)
    sr(S1 := TRUE, R := FALSE);
    ASSERT_TRUE(sr.Q1);

    (* Hold: S=FALSE, R=FALSE, Q1 stays TRUE *)
    sr(S1 := FALSE, R := FALSE);
    ASSERT_TRUE(sr.Q1);

    (* Reset *)
    sr(S1 := FALSE, R := TRUE);
    ASSERT_FALSE(sr.Q1);

    (* Set-dominant: both S and R TRUE, Q1=TRUE *)
    sr(S1 := TRUE, R := TRUE);
    ASSERT_TRUE(sr.Q1);
END_TEST_CASE

TEST_CASE 'RS: reset-dominant bistable'
VAR
    rs : RS;
END_VAR
    (* Initial state: Q1=FALSE *)
    rs(S := FALSE, R1 := FALSE);
    ASSERT_FALSE(rs.Q1);

    (* Set *)
    rs(S := TRUE, R1 := FALSE);
    ASSERT_TRUE(rs.Q1);

    (* Hold *)
    rs(S := FALSE, R1 := FALSE);
    ASSERT_TRUE(rs.Q1);

    (* Reset *)
    rs(S := FALSE, R1 := TRUE);
    ASSERT_FALSE(rs.Q1);

    (* Reset-dominant: both S and R1 TRUE, Q1=FALSE *)
    rs(S := TRUE, R1 := TRUE);
    ASSERT_FALSE(rs.Q1);
END_TEST_CASE

(* ===== MATH FUNCTIONS ===== *)

TEST_CASE 'ABS: integer and real'
VAR
    ri : INT;
    rr : REAL;
END_VAR
    ri := ABS(-42);
    ASSERT_EQ(ri, 42);

    ri := ABS(10);
    ASSERT_EQ(ri, 10);

    rr := ABS(-3.14);
    ASSERT_NEAR(rr, 3.14, 0.001);
END_TEST_CASE

TEST_CASE 'SQRT: square root'
VAR
    r : REAL;
END_VAR
    r := SQRT(25.0);
    ASSERT_NEAR(r, 5.0, 0.001);

    r := SQRT(2.0);
    ASSERT_NEAR(r, 1.41421, 0.001);

    r := SQRT(0.0);
    ASSERT_NEAR(r, 0.0, 0.001);
END_TEST_CASE

TEST_CASE 'SIN and COS: trigonometric basics'
VAR
    s : REAL;
    c : REAL;
END_VAR
    (* sin(0) = 0, cos(0) = 1 *)
    s := SIN(0.0);
    ASSERT_NEAR(s, 0.0, 0.001);

    c := COS(0.0);
    ASSERT_NEAR(c, 1.0, 0.001);

    (* sin^2 + cos^2 = 1 for any angle *)
    s := SIN(1.0);
    c := COS(1.0);
    ASSERT_NEAR(s * s + c * c, 1.0, 0.001);
END_TEST_CASE

TEST_CASE 'TAN, ASIN, ACOS, ATAN'
VAR
    r : REAL;
END_VAR
    r := TAN(0.0);
    ASSERT_NEAR(r, 0.0, 0.001);

    r := ASIN(1.0);
    ASSERT_NEAR(r, 1.5708, 0.001);  (* pi/2 *)

    r := ACOS(1.0);
    ASSERT_NEAR(r, 0.0, 0.001);

    r := ATAN(1.0);
    ASSERT_NEAR(r, 0.7854, 0.001);  (* pi/4 *)
END_TEST_CASE

TEST_CASE 'LN and LOG: logarithms'
VAR
    r : REAL;
END_VAR
    r := LN(1.0);
    ASSERT_NEAR(r, 0.0, 0.001);

    r := LN(2.718281828);
    ASSERT_NEAR(r, 1.0, 0.001);

    r := LOG(100.0);
    ASSERT_NEAR(r, 2.0, 0.001);

    r := LOG(1.0);
    ASSERT_NEAR(r, 0.0, 0.001);
END_TEST_CASE

TEST_CASE 'EXP: exponential'
VAR
    r : REAL;
END_VAR
    r := EXP(0.0);
    ASSERT_NEAR(r, 1.0, 0.001);

    r := EXP(1.0);
    ASSERT_NEAR(r, 2.71828, 0.001);
END_TEST_CASE

TEST_CASE 'MIN and MAX: integer and real'
VAR
    ri : INT;
    rr : REAL;
END_VAR
    ri := MIN(10, 20);
    ASSERT_EQ(ri, 10);

    ri := MAX(10, 20);
    ASSERT_EQ(ri, 20);

    rr := MIN(3.14, 2.72);
    ASSERT_NEAR(rr, 2.72, 0.001);

    rr := MAX(3.14, 2.72);
    ASSERT_NEAR(rr, 3.14, 0.001);
END_TEST_CASE

TEST_CASE 'LIMIT: clamp value to range'
VAR
    r : INT;
    rr : REAL;
END_VAR
    (* Within range *)
    r := LIMIT(0, 50, 100);
    ASSERT_EQ(r, 50);

    (* Below minimum *)
    r := LIMIT(0, -10, 100);
    ASSERT_EQ(r, 0);

    (* Above maximum *)
    r := LIMIT(0, 150, 100);
    ASSERT_EQ(r, 100);

    (* Real values *)
    rr := LIMIT(0.0, 3.14, 10.0);
    ASSERT_NEAR(rr, 3.14, 0.001);
END_TEST_CASE

TEST_CASE 'SEL: binary selection'
VAR
    r : INT;
END_VAR
    r := SEL(FALSE, 10, 20);
    ASSERT_EQ(r, 10);

    r := SEL(TRUE, 10, 20);
    ASSERT_EQ(r, 20);
END_TEST_CASE

TEST_CASE 'MUX: multiplexer selection'
VAR
    r : INT;
END_VAR
    r := MUX(0, 100, 200);
    ASSERT_EQ(r, 100);

    r := MUX(1, 100, 200);
    ASSERT_EQ(r, 200);
END_TEST_CASE

TEST_CASE 'MOVE: pass-through'
VAR
    r : INT;
END_VAR
    r := MOVE(42);
    ASSERT_EQ(r, 42);
END_TEST_CASE

(* ===== STRING FUNCTIONS ===== *)

TEST_CASE 'LEN: string length'
VAR
    n : INT;
END_VAR
    n := LEN('Hello');
    ASSERT_EQ(n, 5);

    n := LEN('');
    ASSERT_EQ(n, 0);

    n := LEN('A B C');
    ASSERT_EQ(n, 5);
END_TEST_CASE

TEST_CASE 'LEFT and RIGHT: substring extraction'
VAR
    r : STRING;
END_VAR
    r := LEFT('ABCDEF', 3);
    ASSERT_EQ(r, 'ABC');

    r := RIGHT('ABCDEF', 3);
    ASSERT_EQ(r, 'DEF');

    (* Edge: request more chars than exist *)
    r := LEFT('AB', 10);
    ASSERT_EQ(r, 'AB');

    r := RIGHT('AB', 10);
    ASSERT_EQ(r, 'AB');
END_TEST_CASE

TEST_CASE 'MID: substring from position'
VAR
    r : STRING;
END_VAR
    r := MID('ABCDEF', 3, 2);  (* 3 chars starting at position 2 *)
    ASSERT_EQ(r, 'BCD');

    r := MID('ABCDEF', 2, 5);  (* 2 chars starting at position 5 *)
    ASSERT_EQ(r, 'EF');
END_TEST_CASE

TEST_CASE 'CONCAT: join strings'
VAR
    r : STRING;
END_VAR
    r := CONCAT('Hello', ' World');
    ASSERT_EQ(r, 'Hello World');

    r := CONCAT('', 'test');
    ASSERT_EQ(r, 'test');

    r := CONCAT('abc', '');
    ASSERT_EQ(r, 'abc');
END_TEST_CASE

TEST_CASE 'FIND: search for substring'
VAR
    pos : INT;
END_VAR
    pos := FIND('Hello World', 'World');
    ASSERT_EQ(pos, 7);

    pos := FIND('Hello World', 'Hello');
    ASSERT_EQ(pos, 1);

    pos := FIND('Hello World', 'xyz');
    ASSERT_EQ(pos, 0);

    pos := FIND('AABAA', 'AA');
    ASSERT_EQ(pos, 1);
END_TEST_CASE

(* ===== TYPE CONVERSIONS ===== *)

TEST_CASE 'INT_TO_REAL and REAL_TO_INT round-trip'
VAR
    n : INT := 42;
    f : REAL;
    n2 : INT;
END_VAR
    f := INT_TO_REAL(n);
    ASSERT_NEAR(f, 42.0, 0.001);

    n2 := REAL_TO_INT(f);
    ASSERT_EQ(n2, 42);
END_TEST_CASE

TEST_CASE 'REAL_TO_INT banker rounding'
VAR
    n : INT;
END_VAR
    (* 2.5 rounds to 2 (even), 3.5 rounds to 4 (even) *)
    n := REAL_TO_INT(2.5);
    ASSERT_EQ(n, 2);

    n := REAL_TO_INT(3.5);
    ASSERT_EQ(n, 4);

    n := REAL_TO_INT(4.5);
    ASSERT_EQ(n, 4);

    n := REAL_TO_INT(5.5);
    ASSERT_EQ(n, 6);
END_TEST_CASE

TEST_CASE 'BOOL_TO_INT: TRUE=1, FALSE=0'
VAR
    n : INT;
END_VAR
    n := BOOL_TO_INT(TRUE);
    ASSERT_EQ(n, 1);

    n := BOOL_TO_INT(FALSE);
    ASSERT_EQ(n, 0);
END_TEST_CASE

TEST_CASE 'INT_TO_BOOL: zero=FALSE, nonzero=TRUE'
VAR
    b : BOOL;
END_VAR
    b := INT_TO_BOOL(0);
    ASSERT_FALSE(b);

    b := INT_TO_BOOL(1);
    ASSERT_TRUE(b);

    b := INT_TO_BOOL(-1);
    ASSERT_TRUE(b);

    b := INT_TO_BOOL(999);
    ASSERT_TRUE(b);
END_TEST_CASE

TEST_CASE 'INT_TO_STRING and STRING_TO_INT round-trip'
VAR
    s : STRING;
    n : INT;
END_VAR
    s := INT_TO_STRING(12345);
    ASSERT_EQ(s, '12345');

    n := STRING_TO_INT('12345');
    ASSERT_EQ(n, 12345);

    (* Negative *)
    s := INT_TO_STRING(-99);
    ASSERT_EQ(s, '-99');

    n := STRING_TO_INT('-99');
    ASSERT_EQ(n, -99);
END_TEST_CASE

TEST_CASE 'BOOL_TO_STRING: TRUE and FALSE'
VAR
    s : STRING;
END_VAR
    s := BOOL_TO_STRING(TRUE);
    ASSERT_EQ(s, 'TRUE');

    s := BOOL_TO_STRING(FALSE);
    ASSERT_EQ(s, 'FALSE');
END_TEST_CASE

TEST_CASE 'DINT_TO_REAL and REAL_TO_DINT'
VAR
    f : REAL;
    n : DINT;
END_VAR
    f := DINT_TO_REAL(100000);
    ASSERT_NEAR(f, 100000.0, 0.1);

    n := REAL_TO_DINT(99999.7);
    ASSERT_EQ(n, 100000);
END_TEST_CASE

TEST_CASE 'BYTE_TO_INT and INT_TO_BYTE'
VAR
    n : INT;
    b : BYTE;
END_VAR
    n := BYTE_TO_INT(16#FF);
    ASSERT_EQ(n, 255);

    b := INT_TO_BYTE(256);
    ASSERT_EQ(b, 0);  (* 256 AND FF = 0 *)

    b := INT_TO_BYTE(170);
    ASSERT_EQ(b, 170);
END_TEST_CASE
