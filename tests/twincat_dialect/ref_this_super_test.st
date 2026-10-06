(* TwinCAT dialect: REF= references, THIS^ and SUPER^.
   `r REF= x;` binds a REFERENCE TO variable to x; reads and writes of r then
   go through to x. Inside a FUNCTION_BLOCK, THIS^ is the current instance
   and an unqualified call to one of its METHODs dispatches on it. In a
   derived FB, SUPER^.M() calls the base implementation and SUPER^() runs
   the base body.
*)

TYPE ST_Pair :
STRUCT
    m : DINT;
    k : DINT;
END_STRUCT
END_TYPE

FUNCTION_BLOCK FB_Base
VAR_OUTPUT
    x : DINT;
    bodyRuns : DINT;
END_VAR
    bodyRuns := bodyRuns + 1;

METHOD Inc : DINT
VAR_INPUT
    step : DINT;
END_VAR
    THIS^.x := THIS^.x + step;
    Inc := THIS^.x;
END_METHOD

METHOD M : DINT
    M := 100;
END_METHOD

METHOD Twice : DINT
    Inc(step := 1);
    THIS^.Inc(step := 1);
    Twice := x;
END_METHOD
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
VAR_OUTPUT
    fromSuper : DINT;
END_VAR
    SUPER^();
    fromSuper := SUPER^.M();

METHOD M : DINT
    M := SUPER^.M() + 1;
END_METHOD
END_FUNCTION_BLOCK

TEST_CASE 'Reference to a variable reads and writes through'
VAR
    n : DINT;
    r : REFERENCE TO DINT;
END_VAR
    n := 4;
    r REF= n;
    ASSERT_EQ(r, 4);
    r := 9;
    ASSERT_EQ(n, 9);
    n := 11;
    ASSERT_EQ(r, 11);
END_TEST_CASE

TEST_CASE 'Reference to a struct member'
VAR
    s : ST_Pair;
    r : REFERENCE TO DINT;
END_VAR
    r REF= s.m;
    r := 5;
    ASSERT_EQ(s.m, 5);
    ASSERT_EQ(s.k, 0);
END_TEST_CASE

TEST_CASE 'Reference to an array element keeps its element'
VAR
    arr : ARRAY[0..3] OF DINT;
    i : DINT;
    r : REFERENCE TO DINT;
END_VAR
    i := 1;
    r REF= arr[i];
    i := 2;
    r := 7;
    ASSERT_EQ(arr[1], 7);
    ASSERT_EQ(arr[2], 0);
END_TEST_CASE

TEST_CASE 'THIS^ member access and method calls'
VAR
    fb : FB_Base;
    v : DINT;
END_VAR
    v := fb.Inc(step := 3);
    ASSERT_EQ(v, 3);
    ASSERT_EQ(fb.x, 3);
    v := fb.Twice();
    ASSERT_EQ(v, 5);
    ASSERT_EQ(fb.x, 5);
END_TEST_CASE

TEST_CASE 'SUPER^ method call and base body'
VAR
    d : FB_Derived;
END_VAR
    d();
    ASSERT_EQ(d.fromSuper, 100);
    ASSERT_EQ(d.bodyRuns, 1);
    ASSERT_EQ(d.M(), 101);
    d();
    ASSERT_EQ(d.bodyRuns, 2);
END_TEST_CASE
