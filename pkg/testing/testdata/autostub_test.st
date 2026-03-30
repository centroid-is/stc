TEST_CASE 'Auto-stub returns zero values'
VAR
    mover : MC_MoveAbsolute;
END_VAR
    mover(Execute := TRUE, Position := 100.0);
    ASSERT_FALSE(mover.Done, 'Auto-stub Done should be FALSE (zero)');
    ASSERT_FALSE(mover.Error, 'Auto-stub Error should be FALSE (zero)');
END_TEST_CASE
