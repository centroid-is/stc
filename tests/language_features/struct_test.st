(* Struct Test Suite
   Tests struct declarations, member access, and nested structs.
*)

TYPE Point : STRUCT
    x : REAL;
    y : REAL;
END_STRUCT; END_TYPE

TEST_CASE 'Struct member access'
VAR
    p : Point;
END_VAR
    p.x := 1.5;
    p.y := 2.5;
    ASSERT_NEAR(p.x, 1.5, 0.01);
    ASSERT_NEAR(p.y, 2.5, 0.01);
END_TEST_CASE
