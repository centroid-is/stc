(* Array Test Suite
   Tests array declaration, indexing, and multi-dimensional arrays.
*)

TEST_CASE 'Array indexing'
VAR
    arr : ARRAY[1..5] OF INT;
END_VAR
    arr[1] := 10;
    arr[5] := 50;
    ASSERT_EQ(arr[1], 10);
    ASSERT_EQ(arr[5], 50);
END_TEST_CASE

TEST_CASE 'Multi-dimensional array'
VAR
    mat : ARRAY[1..3, 1..3] OF INT;
END_VAR
    mat[1,1] := 1;
    mat[2,2] := 4;
    mat[3,3] := 9;
    ASSERT_EQ(mat[2,2], 4);
END_TEST_CASE
