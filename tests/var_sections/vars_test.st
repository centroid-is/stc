(* VAR Section Types Test Suite
   Exercises all VAR block types: VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT,
   VAR_TEMP, VAR CONSTANT, multiple VAR blocks, and initial values.
*)

(* FB with all VAR section types *)
FUNCTION_BLOCK FB_AllSections
VAR_INPUT
    SetPoint : REAL;
    Enable : BOOL;
END_VAR
VAR_OUTPUT
    Output : REAL;
    IsActive : BOOL;
END_VAR
VAR
    gain : REAL := 2.0;
    offset : REAL := 0.0;
END_VAR
    IF Enable THEN
        Output := SetPoint * gain + offset;
        IsActive := TRUE;
    ELSE
        Output := 0.0;
        IsActive := FALSE;
    END_IF;
END_FUNCTION_BLOCK

(* FB with multiple VAR blocks *)
FUNCTION_BLOCK FB_MultiVar
VAR_INPUT
    x : INT;
END_VAR
VAR_OUTPUT
    y : INT;
END_VAR
VAR
    temp1 : INT := 0;
END_VAR
VAR
    temp2 : INT := 0;
END_VAR
    temp1 := x * 2;
    temp2 := temp1 + 10;
    y := temp2;
END_FUNCTION_BLOCK

(* Function with VAR_INPUT and local VAR *)
FUNCTION Scale : REAL
VAR_INPUT
    raw : INT;
    minVal : REAL;
    maxVal : REAL;
    rawMin : INT;
    rawMax : INT;
END_VAR
VAR
    span : REAL;
    normalized : REAL;
END_VAR
    span := maxVal - minVal;
    normalized := INT_TO_REAL(raw - rawMin) / INT_TO_REAL(rawMax - rawMin);
    Scale := minVal + normalized * span;
END_FUNCTION

(* ===== VAR_INPUT / VAR_OUTPUT TESTS ===== *)

TEST_CASE 'FB with VAR_INPUT and VAR_OUTPUT'
VAR
    fb : FB_AllSections;
END_VAR
    fb(SetPoint := 50.0, Enable := TRUE);
    ASSERT_NEAR(fb.Output, 100.0, 0.001);  (* 50 * 2.0 + 0.0 *)
    ASSERT_TRUE(fb.IsActive);
END_TEST_CASE

TEST_CASE 'FB disabled returns zero'
VAR
    fb : FB_AllSections;
END_VAR
    fb(SetPoint := 50.0, Enable := FALSE);
    ASSERT_NEAR(fb.Output, 0.0, 0.001);
    ASSERT_FALSE(fb.IsActive);
END_TEST_CASE

(* ===== MULTIPLE VAR BLOCKS ===== *)

TEST_CASE 'FB with multiple VAR blocks'
VAR
    fb : FB_MultiVar;
END_VAR
    fb(x := 5);
    ASSERT_EQ(fb.y, 20);  (* 5*2 + 10 = 20 *)

    fb(x := 10);
    ASSERT_EQ(fb.y, 30);  (* 10*2 + 10 = 30 *)
END_TEST_CASE

(* ===== VARIABLES WITH INITIAL VALUES ===== *)

TEST_CASE 'Variables with initial values'
VAR
    a : INT := 42;
    b : REAL := 3.14;
    c : BOOL := TRUE;
    d : STRING := 'hello';
    e : INT := 0;
END_VAR
    ASSERT_EQ(a, 42);
    ASSERT_NEAR(b, 3.14, 0.001);
    ASSERT_TRUE(c);
    ASSERT_EQ(d, 'hello');
    ASSERT_EQ(e, 0);
END_TEST_CASE

TEST_CASE 'Variables default to zero without init'
VAR
    a : INT;
    b : REAL;
    c : BOOL;
    d : STRING;
END_VAR
    ASSERT_EQ(a, 0);
    ASSERT_NEAR(b, 0.0, 0.001);
    ASSERT_FALSE(c);
    ASSERT_EQ(d, '');
END_TEST_CASE

(* ===== FUNCTION WITH MULTIPLE VAR SECTIONS ===== *)

TEST_CASE 'Function with VAR_INPUT and local VAR'
VAR
    r : REAL;
END_VAR
    (* Scale 500 from range 0..1000 to range 0.0..100.0 *)
    r := Scale(500, 0.0, 100.0, 0, 1000);
    ASSERT_NEAR(r, 50.0, 0.001);

    (* Scale 250 from range 0..1000 to range 0.0..100.0 *)
    r := Scale(250, 0.0, 100.0, 0, 1000);
    ASSERT_NEAR(r, 25.0, 0.001);
END_TEST_CASE

(* ===== VAR CONSTANT ===== *)

TEST_CASE 'Constant variables used in expressions'
VAR CONSTANT
    MAX_SPEED : INT := 1500;
    PI_APPROX : REAL := 3.14159;
END_VAR
VAR
    speed : INT;
    area : REAL;
END_VAR
    speed := MAX_SPEED;
    ASSERT_EQ(speed, 1500);

    area := PI_APPROX * 5.0 * 5.0;
    ASSERT_NEAR(area, 78.53975, 0.001);
END_TEST_CASE

(* ===== COMPUTED INITIAL VALUES ===== *)

TEST_CASE 'Initial value from expression'
VAR
    base : INT := 10;
    doubled : INT := 10 * 2;
END_VAR
    ASSERT_EQ(base, 10);
    ASSERT_EQ(doubled, 20);
END_TEST_CASE

(* ===== MULTIPLE VARIABLE NAMES PER DECLARATION ===== *)

TEST_CASE 'Multiple names in one declaration'
VAR
    a, b, c : INT := 0;
END_VAR
    a := 1;
    b := 2;
    c := 3;
    ASSERT_EQ(a + b + c, 6);
END_TEST_CASE
