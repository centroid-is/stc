TEST_CASE 'Mock FB returns expected value'
VAR
    mover : MC_MoveAbsolute;
END_VAR
    mover(Execute := TRUE, Position := 100.0);
    ASSERT_TRUE(mover.Done, 'Mock should set Done to TRUE');
END_TEST_CASE
