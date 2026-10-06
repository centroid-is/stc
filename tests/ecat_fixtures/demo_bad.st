// Deliberately broken links: one diagnostic per ECAT code.
{attribute 'qualified_only'}
VAR_GLOBAL
	// ECAT001: mistyped channel segment
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Chanel 1^Input'}
	Typo AT %I* : BOOL;
	// ECAT002: ST_DemoEL1008 has no I9
	{attribute 'TcLinkTo' := '.I9 := TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 8^Input'}
	NoMember : ST_DemoEL1008;
	// ECAT003: WORD on a 1-bit input
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 2^Input'}
	TooWide AT %I* : WORD;
	// ECAT004: output variable on an input entry
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 3^Input'}
	WrongDir AT %Q* : BOOL;
	// ECAT005: two variables on Channel 4
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 4^Input'}
	DupA AT %I* : BOOL;
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 4^Input'}
	DupB AT %I* : BOOL;
	// ECAT006: malformed value
	{attribute 'TcLinkTo' := '.I1 TIID^Device 1 (EtherCAT)'}
	Malformed : ST_DemoEL1008;
END_VAR
