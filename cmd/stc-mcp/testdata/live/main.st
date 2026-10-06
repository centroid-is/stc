PROGRAM MAIN
GVL_Live.conveyor(xSensor := GVL_Live.xSensor);
IF GVL_Live.conveyor.xRunning THEN
	GVL_Live.nRunCycles := GVL_Live.nRunCycles + 1;
END_IF;
END_PROGRAM
