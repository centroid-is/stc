(* TwinCAT dialect: global variable lists (GVLs).
   The top-level VAR_GLOBAL block below is a GVL named after this file,
   gvl_test. It has no qualified_only attribute, so its variables are
   reachable both as gvl_test.x and as bare x. Every TEST_CASE starts from
   a freshly initialised GVL.
*)

TYPE ST_Pos :
STRUCT
    x : INT;
    y : INT;
END_STRUCT
END_TYPE

VAR_GLOBAL
    counter : INT := 0;
    limit   : INT := 10;
    pos     : ST_Pos;
END_VAR

FUNCTION_BLOCK FB_Counter
VAR_INPUT
    step : INT;
END_VAR
    gvl_test.counter := gvl_test.counter + step;
END_FUNCTION_BLOCK

TEST_CASE 'Qualified read and write'
    ASSERT_EQ(gvl_test.limit, 10);
    gvl_test.counter := gvl_test.counter + 3;
    ASSERT_EQ(gvl_test.counter, 3);
END_TEST_CASE

TEST_CASE 'Unqualified read and write'
    counter := counter + limit;
    ASSERT_EQ(counter, 10);
    ASSERT_EQ(gvl_test.counter, 10);
END_TEST_CASE

TEST_CASE 'Nested struct member write'
    gvl_test.pos.x := 5;
    pos.y := 7;
    ASSERT_EQ(gvl_test.pos.x, 5);
    ASSERT_EQ(gvl_test.pos.y, 7);
END_TEST_CASE

TEST_CASE 'Each test case gets a fresh GVL'
    ASSERT_EQ(counter, 0);
    ASSERT_EQ(gvl_test.pos.x, 0);
END_TEST_CASE

TEST_CASE 'FB body writes the GVL'
VAR
    fbCount : FB_Counter;
END_VAR
    fbCount(step := 2);
    fbCount(step := 2);
    ASSERT_EQ(gvl_test.counter, 4);
END_TEST_CASE

TEST_CASE 'Local variable shadows a GVL variable'
VAR
    limit : INT := 99;
END_VAR
    ASSERT_EQ(limit, 99);
    ASSERT_EQ(gvl_test.limit, 10);
END_TEST_CASE
