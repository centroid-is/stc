PROGRAM MAIN
VAR
	nCycle : DINT;
	xInit : BOOL;
	tmrCycle : TON;
END_VAR
xInit := TRUE;
nCycle := nCycle + 1;
tmrCycle(IN := xInit, PT := T#1S);
GVL_BatchLines.Drives_Line1[1]();
GVL_BatchLines.Drives_Line1[2]();
GVL_BatchLines.Conveyor();
sensors.EPW01_WA01_IS11();
sensors.EPW01_WA01_IS12();
END_PROGRAM
