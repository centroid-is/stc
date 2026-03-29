(* OOP Test Suite
   Tests METHOD, PROPERTY, EXTENDS, IMPLEMENTS for FUNCTION_BLOCK declarations.
   Also tests FB body execution, state persistence, and VAR_INPUT/VAR_OUTPUT.
*)

FUNCTION_BLOCK FB_Base
VAR
    baseVal : INT := 10;
END_VAR
METHOD GetValue : INT
    GetValue := baseVal;
END_METHOD
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Child EXTENDS FB_Base
METHOD GetDouble : INT
    GetDouble := baseVal * 2;
END_METHOD
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Accumulator
VAR_INPUT
    AddValue : INT;
    Reset : BOOL;
END_VAR
VAR_OUTPUT
    Total : INT;
END_VAR
VAR
    running : INT := 0;
END_VAR
    IF Reset THEN
        running := 0;
    ELSE
        running := running + AddValue;
    END_IF;
    Total := running;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Doubler
VAR_INPUT
    Value : INT;
END_VAR
VAR_OUTPUT
    Result : INT;
END_VAR
    Result := Value * 2;
END_FUNCTION_BLOCK

TEST_CASE 'Method call returns value'
VAR
    fb : FB_Base;
END_VAR
    ASSERT_EQ(fb.GetValue(), 10);
END_TEST_CASE

TEST_CASE 'Inherited method works'
VAR
    child : FB_Child;
END_VAR
    ASSERT_EQ(child.GetValue(), 10);
    ASSERT_EQ(child.GetDouble(), 20);
END_TEST_CASE

TEST_CASE 'FB accumulates state across calls'
VAR
    acc : FB_Accumulator;
END_VAR
    acc(AddValue := 10, Reset := FALSE);
    ASSERT_EQ(acc.Total, 10);

    acc(AddValue := 5, Reset := FALSE);
    ASSERT_EQ(acc.Total, 15);

    acc(AddValue := 3, Reset := FALSE);
    ASSERT_EQ(acc.Total, 18);
END_TEST_CASE

TEST_CASE 'FB reset clears accumulated state'
VAR
    acc : FB_Accumulator;
END_VAR
    acc(AddValue := 100, Reset := FALSE);
    ASSERT_EQ(acc.Total, 100);

    acc(AddValue := 0, Reset := TRUE);
    ASSERT_EQ(acc.Total, 0);

    acc(AddValue := 7, Reset := FALSE);
    ASSERT_EQ(acc.Total, 7);
END_TEST_CASE

TEST_CASE 'Multiple FB instances are independent'
VAR
    a : FB_Accumulator;
    b : FB_Accumulator;
END_VAR
    a(AddValue := 10, Reset := FALSE);
    b(AddValue := 20, Reset := FALSE);

    ASSERT_EQ(a.Total, 10);
    ASSERT_EQ(b.Total, 20);

    a(AddValue := 5, Reset := FALSE);
    ASSERT_EQ(a.Total, 15);
    ASSERT_EQ(b.Total, 20);
END_TEST_CASE

TEST_CASE 'FB with simple transformation'
VAR
    d : FB_Doubler;
END_VAR
    d(Value := 21);
    ASSERT_EQ(d.Result, 42);

    d(Value := 0);
    ASSERT_EQ(d.Result, 0);

    d(Value := -5);
    ASSERT_EQ(d.Result, -10);
END_TEST_CASE
