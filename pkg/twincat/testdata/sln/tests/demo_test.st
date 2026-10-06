(* Runs against the Demo project with: stc test --project "Demo/Demo solution.tsproj" tests *)
TEST_CASE 'FB_Motor runs, starts and exposes Speed'
VAR
    m : FB_Motor;
    ok : BOOL;
END_VAR
    m.enable := TRUE;
    m();
    ASSERT_TRUE(m.running);
    ok := m.Start(force := FALSE);
    ASSERT_TRUE(ok);
    m.Speed := 42;
    ASSERT_EQ(m.Speed, 42);
END_TEST_CASE

TEST_CASE 'F_Add, ST_Point and GVL_Main'
VAR
    p : ST_Point;
END_VAR
    p.x := 2;
    p.y := 3;
    ASSERT_EQ(F_Add(a := p.x, b := p.y), 5);
    ASSERT_TRUE(GVL_Main.mode = E_Mode.Idle);
    GVL_Main.counter := 7;
    ASSERT_EQ(GVL_Main.counter, 7);
END_TEST_CASE
