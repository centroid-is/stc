PROGRAM MAIN
VAR
	x : BOOL;
	n : INT;
	trig : R_TRIG;
	t : TON;
END_VAR
A050_ModbusCall();
A100_input();

ACTION A050_ModbusCall
{warning disable C0139}
n := n + 1;
END_ACTION

ACTION A100_input
trig(CLK := x);
t(IN := x, PT := T#1S);
END_ACTION
END_PROGRAM

FUNCTION_BLOCK FB_Drive
VAR_INPUT
	i_uETA AT %I* : UINT;
END_VAR
VAR_OUTPUT
	q_uCMD AT %Q* : UINT;
END_VAR
coe();

METHOD M_Reset : BOOL
q_uCMD := 0;
M_Reset := TRUE;
END_METHOD

ACTION coe
q_uCMD := i_uETA;
END_ACTION

ACTION sdo
q_uCMD := 16#000F;
END_ACTION
END_FUNCTION_BLOCK
