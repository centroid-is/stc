// Live session fixture for the stc-mcp sim tools: one conveyor with an HMI
// struct, a sensor input linked to Demo Device 1's EL1008 channel 1 and a
// running-cycle counter.
{attribute 'qualified_only'}
VAR_GLOBAL
	{attribute 'OPC.UA.DA' := '1'}
	conveyor : FB_Conveyor;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input'}
	{attribute 'OPC.UA.DA' := '1'}
	xSensor AT %I* : BOOL;
	{attribute 'OPC.UA.DA' := '1'}
	nRunCycles : DINT;
END_VAR
VAR_GLOBAL CONSTANT
	cMax : INT := 10;
END_VAR
