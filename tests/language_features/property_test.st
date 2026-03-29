(* Property Test Suite
   Tests PROPERTY GET/SET dispatch on function blocks.
*)

FUNCTION_BLOCK FB_WithProperty
VAR
    _value : INT := 0;
END_VAR

PROPERTY Value : INT
    GET
        Value := _value;
    END_GET
    SET
        _value := Value * 2;
    END_SET
END_PROPERTY

END_FUNCTION_BLOCK

TEST_CASE 'Property getter returns internal value'
VAR
    fb : FB_WithProperty;
END_VAR
    fb._value := 42;
    ASSERT_EQ(fb.Value, 42);
END_TEST_CASE

TEST_CASE 'Property setter transforms value'
VAR
    fb : FB_WithProperty;
END_VAR
    fb.Value := 10;
    ASSERT_EQ(fb._value, 20);
END_TEST_CASE
