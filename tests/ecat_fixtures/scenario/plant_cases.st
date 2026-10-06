// DEVX-01 acceptance tests. They need project mode, so the file is not
// named *_test.st (plain `stc test tests/` would discover it); copy it to a
// tests directory as *_test.st and run
//   stc test <dir> --project demo_types.st --project ECT.st --project ECT_Diag.st
//     --project MAIN.st --io "Demo Device [12].xml"
// Every TEST_CASE starts from a fresh project and EtherCAT network.
// Scans are 10 ms (the default MAIN task). Device_1_Diag[i] is bus position
// i - 1, so DEMO.A1.01 (EL1008) is Device_1_Diag[2].

TEST_CASE 'input by variable path'
// An unlinked variable reads back at once.
SET('ECT_Diag.rSetpoint', 5.0);
ASSERT_NEAR(GET('ECT_Diag.rSetpoint'), 5.0, 0.001);
// A linked input is written to its EtherCAT slot and copied in at the next scan.
SET('ECT.A1_01.I1', TRUE);
RUN_CYCLES(2);
ASSERT_TRUE(GET('ECT.A1_01.I1'), 'GET after SET');
ASSERT_TRUE(ECT.A1_01.I1, 'direct GVL read');
ASSERT_TRUE(ECT.A1_02.O1, 'interlock output follows the input');
END_TEST_CASE

TEST_CASE 'input by link path'
SIM_SET_LINK('TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input', TRUE);
RUN_CYCLES(2);
ASSERT_TRUE(ECT.A1_01.I1, 'linked variable sees the slot');
ASSERT_TRUE(GET('TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input'), 'GET on the link');
END_TEST_CASE

TEST_CASE 'EL9222 trip'
RUN_CYCLES(2);
ASSERT_TRUE(ECT.A1_03.p_stat_Enabled, 'channel enabled before the trip');
SIM_TRIP('DEMO.A1.03 (EL9222-5500)', 1);
RUN_CYCLES(2);
ASSERT_FALSE(ECT.A1_03.p_stat_Enabled, 'channel disabled after the trip');
END_TEST_CASE

TEST_CASE 'removed slave shows in ECT_Diag'
RUN_CYCLES(20);
ASSERT_TRUE(ECT_Diag.Device_1_Diag[2].p_stat_bOk, 'EL1008 OK before removal');
SIM_SLAVE_STATE('DEMO.A1.01 (EL1008)', 'not_present');
RUN_CYCLES(20);
ASSERT_FALSE(ECT_Diag.Device_1_Diag[2].p_stat_bOk, 'EL1008 not OK after removal');
ASSERT_TRUE(ECT_Diag.Device_1_Diag[2].p_stat_nLinkState <> 0, 'link state reports the removal');
END_TEST_CASE

TEST_CASE 'isolation: the removed slave is back'
RUN_CYCLES(20);
ASSERT_TRUE(ECT_Diag.Device_1_Diag[2].p_stat_bOk, 'fresh network per TEST_CASE');
ASSERT_EQ(ECT_Diag.Device_1_Diag[2].p_stat_nLinkState, 0);
END_TEST_CASE

TEST_CASE 'drive fault'
RUN_CYCLES(30);
ASSERT_FALSE(GET('MAIN.xDriveFault'), 'no fault before');
SIM_DRIVE_FAULT('DEMO.CN01.FD01 (ATV320 EtherCAT)', 16);
RUN_CYCLES(10);
ASSERT_TRUE(GET('MAIN.xDriveFault'), 'CiA402 fault bit in ETA');
END_TEST_CASE

TEST_CASE 'setpoint ramp'
SIM_RAMP('ECT_Diag.rSetpoint', 0.0, 10.0, T#50ms);
RUN_CYCLES(6);
ASSERT_NEAR(ECT_Diag.rSetpoint, 10.0, 0.001);
END_TEST_CASE

TEST_CASE 'ADVANCE_TIME runs whole scans'
ADVANCE_TIME(T#100ms);
ASSERT_EQ(ECT_Diag.nScanCount, UDINT#10);
END_TEST_CASE
