PROGRAM MAIN
VAR
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Term 2 (EL1008)^Channel 1^Input'}
	I1 AT %I* : BOOL;
	v : ARRAY[0..1] OF INT;
	n : INT;
	s : STRING;
END_VAR
n := F_X(sMsg := s, sLabel := 'tray/ok per min', values := v);
n := F_X(a := E_T.destination);
END_PROGRAM
