// Demo EtherCAT GVL: every TcLinkTo shape bound to Demo Device 1/2.
// Tests name this GVL ECT (the parser derives demo_ect from the file).
{attribute 'qualified_only'}
VAR_GLOBAL
	{attribute 'TcLinkTo' := '.I1 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input;
				.I2 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 2^Input;
				.I3 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 3^Input;
				.I4 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 4^Input;
				.I5 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 5^Input;
				.I6 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 6^Input;
				.I7 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 7^Input;
				.I8 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 8^Input;'}
	A1_01 : ST_DemoEL1008;
	{attribute 'TcLinkTo' := '.O1 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 1^Output;
				.O2 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 2^Output;
				.O3 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 3^Output;
				.O4 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 4^Output;
				.O5 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 5^Output;
				.O6 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 6^Output;
				.O7 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 7^Output;
				.O8 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 8^Output;'}
	A1_02 : ST_DemoEL2008;
	{attribute 'TcLinkTo' := '.p_stat_Enabled                 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Status^Enabled;
				.p_stat_Tripped := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Status^Tripped;
				.p_Current := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.03 (EL9222-5500)^OCP Inputs Channel 1^Current;
				.p_cmd_Reset := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.03 (EL9222-5500)^OCP Outputs Channel 1^Control^Reset'}
	A1_03 : ST_DemoEL9222;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.04 (EL2912)^Module 3 (DEVICEIO)^FIELDVOLTAGE Field Voltage Status^Fieldvoltage Underrange'}
	A1_04_Underrange AT %I* : BOOL;
	{attribute 'TcLinkTo' := '.q_uCMD := TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)^Outputs^CMD;
				.q_iLFR := TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)^Outputs^LFR;
				.i_uETA := TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)^Inputs^ETA;
				.i_iRFR := TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)^Inputs^RFR;
				.amsaddr := TIID^Device 1 (EtherCAT)^DEMO.CN01.FD01 (ATV320 EtherCAT)^InfoData^AdsAddr;'}
	CN01 : FB_DemoDrive;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.V1 (CTEU-EtherCAT Modular)^Module 1 (VAEM-L1-S-8-PT [16DO])^Outputs^C1 Output'}
	V1_C1 AT %Q* : BYTE;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^WcState^WcState'}
	A1_01_WcState AT %I* : BOOL;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^InfoData^State'}
	A1_01_State AT %I* : WORD;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Inputs^DevState'}
	Dev1_DevState AT %I* : UINT;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Inputs^SlaveCount'}
	Dev1_SlaveCount AT %I* : UINT;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Inputs^Frm0State'}
	Dev1_Frm0State AT %I* : UINT;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Inputs^Frm0WcState'}
	Dev1_Frm0WcState AT %I* : UINT;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^InfoData^AmsNetId'}
	Dev1_AmsNetId AT %I* : T_AmsNetIdArr;
	{attribute 'TcLinkTo' := 'TIID^Device 2 (EtherCAT)^D2.A1.01 (EL1008)^Channel 1^Input'}
	D2_I1 AT %I* : BOOL;
END_VAR
