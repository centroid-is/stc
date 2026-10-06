(* TwinCAT dialect: enumerations.
   E_Mode is qualified_only, strict and to_string with an explicit UINT base
   type. E_State uses implicit numbering: a value without an initialiser is
   the previous value plus one. E_Plain has neither attribute, so its values
   are reachable bare and TO_STRING gives the number.
*)

{attribute 'qualified_only'}
{attribute 'strict'}
{attribute 'to_string'}
TYPE E_Mode : (a := 0, b := 1, c := 2) UINT;
END_TYPE

TYPE E_State : (tun := 0, rdy := 2, nst);
END_TYPE

TYPE E_Plain : (p0, p1, p2, p3, p4);
END_TYPE

FUNCTION F_Classify : DINT
VAR_INPUT
    m : E_Mode;
END_VAR
    CASE m OF
        E_Mode.a:
            F_Classify := 10;
        E_Mode.b, E_Mode.c:
            F_Classify := 20;
    ELSE
        F_Classify := -1;
    END_CASE
END_FUNCTION

FUNCTION F_Band : DINT
VAR_INPUT
    p : E_Plain;
END_VAR
    CASE p OF
        p0:
            F_Band := 0;
        p1..p3:
            F_Band := 1;
    ELSE
        F_Band := 2;
    END_CASE
END_FUNCTION

TEST_CASE 'CASE with qualified labels'
    ASSERT_EQ(F_Classify(E_Mode.a), 10);
    ASSERT_EQ(F_Classify(E_Mode.b), 20);
    ASSERT_EQ(F_Classify(E_Mode.c), 20);
END_TEST_CASE

TEST_CASE 'CASE with a range label'
    ASSERT_EQ(F_Band(p0), 0);
    ASSERT_EQ(F_Band(p1), 1);
    ASSERT_EQ(F_Band(p3), 1);
    ASSERT_EQ(F_Band(p4), 2);
END_TEST_CASE

TEST_CASE 'Implicit numbering continues from the previous value'
VAR
    s : E_State;
END_VAR
    s := E_State.nst;
    ASSERT_EQ(TO_DINT(s), 3);
    s := rdy;
    ASSERT_EQ(TO_DINT(s), 2);
    ASSERT_EQ(TO_DINT(E_State.tun), 0);
END_TEST_CASE

TEST_CASE 'TO_STRING gives the name for a to_string enum'
VAR
    m : E_Mode;
END_VAR
    m := E_Mode.b;
    ASSERT_EQ(TO_STRING(m), 'b');
END_TEST_CASE

TEST_CASE 'TO_STRING gives the number for a plain enum'
VAR
    p : E_Plain;
END_VAR
    p := p2;
    ASSERT_EQ(TO_STRING(p), '2');
END_TEST_CASE

TEST_CASE 'Inline enum in a VAR block'
VAR
    light : (red, amber, green) := amber;
END_VAR
    ASSERT_TRUE(light = amber);
    light := green;
    ASSERT_TRUE(light = green);
    ASSERT_FALSE(light = red);
END_TEST_CASE
