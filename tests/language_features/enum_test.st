(* Enum Test Suite
   Tests enum declarations and usage with typed enum references.
*)

TYPE Color : (Red := 0, Green := 1, Blue := 2); END_TYPE

TEST_CASE 'Enum assignment and comparison'
VAR
    c : Color;
END_VAR
    c := Color#Green;
    ASSERT_EQ(c, Color#Green);
    ASSERT_TRUE(c <> Color#Red);
END_TEST_CASE

TEST_CASE 'Enum integer fallback'
VAR
    c : Color;
END_VAR
    c := 1;  (* Green *)
    ASSERT_EQ(c, 1);
    ASSERT_TRUE(c <> 0);  (* Not Red *)
END_TEST_CASE

TEST_CASE 'Enum CASE dispatch'
VAR
    c : INT := 2;  (* Blue *)
    name : STRING;
END_VAR
    CASE c OF
        0: name := 'Red';
        1: name := 'Green';
        2: name := 'Blue';
    ELSE
        name := 'Unknown';
    END_CASE;
    ASSERT_EQ(name, 'Blue');
END_TEST_CASE
