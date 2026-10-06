// Live OPC UA fixture (Phase 29-01): the p_cmd_* set-TRUE / FB-clears
// handshake, a sensor whose HMI status follows a %I* input, and a probe
// that detects writes applied in the middle of a scan.

TYPE E_ConvState :
(
	init := 0,
	idle := 1,
	rdy := 2,
	run := 3
);
END_TYPE

{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_Conveyor_HMI :
STRUCT
	p_cmd_Start : BOOL;
	p_cmd_Stop : BOOL;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_Running : BOOL;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_State : E_ConvState;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_Starts : DINT;
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_StartCycle : DINT;
END_STRUCT
END_TYPE

{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_Sensor_HMI :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_xRaw : BOOL;
END_STRUCT
END_TYPE

FUNCTION_BLOCK FB_Conveyor
VAR
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Conveyor_HMI;
END_VAR
IF HMI.p_stat_State = E_ConvState.init THEN
	HMI.p_stat_State := E_ConvState.rdy;
END_IF;
// HMI sets a command TRUE; the FB acts on it and clears it (FB-clears).
IF HMI.p_cmd_Start THEN
	HMI.p_stat_Running := TRUE;
	HMI.p_stat_State := E_ConvState.run;
	HMI.p_stat_Starts := HMI.p_stat_Starts + 1;
	HMI.p_stat_StartCycle := GVL_Live.nCycle;
	HMI.p_cmd_Start := FALSE;
END_IF;
IF HMI.p_cmd_Stop THEN
	HMI.p_stat_Running := FALSE;
	HMI.p_stat_State := E_ConvState.rdy;
	HMI.p_cmd_Stop := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Sensor
VAR
	{attribute 'OPC.UA.DA' := '1'}
	HMI : ST_Sensor_HMI;
	xIn AT %I* : BOOL;
END_VAR
HMI.p_stat_xRaw := xIn;
END_FUNCTION_BLOCK

// MAIN samples GVL_Live.xProbe at the start and at the end of its body
// with work in between. A write applied in the middle of a scan would make
// the two samples differ and count a mismatch.
PROGRAM MAIN
VAR
	nCycle : DINT;
	xFirst : BOOL;
	xLast : BOOL;
	i : INT;
	nWork : DINT;
END_VAR
xFirst := GVL_Live.xProbe;
nCycle := nCycle + 1;
GVL_Live.nCycle := nCycle;
FOR i := 1 TO 200 DO
	nWork := nWork + i;
END_FOR;
GVL_Live.conveyor();
GVL_Live.sensor();
xLast := GVL_Live.xProbe;
IF xFirst <> xLast THEN
	GVL_Live.nMismatch := GVL_Live.nMismatch + 1;
END_IF;
IF xFirst THEN
	GVL_Live.nProbeTrue := GVL_Live.nProbeTrue + 1;
END_IF;
END_PROGRAM
