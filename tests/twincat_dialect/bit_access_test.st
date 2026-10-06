(* TwinCAT dialect: bit access on integer and bit-string values.
   `w.n` reads or writes bit n of an ANY_BIT/ANY_INT value. The GVL below is
   named after this file and is not qualified_only, so ECT and Modbus are
   reachable bare. ECT mirrors an EtherCAT image: a struct member holding a
   WORD of packed digital inputs.
*)

TYPE ST_Terminal :
STRUCT
    q_wDigitalInputs : WORD;
END_STRUCT
END_TYPE

TYPE ST_Ect :
STRUCT
    X : ST_Terminal;
END_STRUCT
END_TYPE

TYPE ST_Modbus :
STRUCT
    arr : ARRAY[0..3] OF WORD;
END_STRUCT
END_TYPE

VAR_GLOBAL CONSTANT
    BIT_READY : DINT := 2;
END_VAR

VAR_GLOBAL
    ECT    : ST_Ect;
    Modbus : ST_Modbus;
END_VAR

TEST_CASE 'Bit read on a nested struct member'
    ECT.X.q_wDigitalInputs := 16#0005;
    ASSERT_TRUE(ECT.X.q_wDigitalInputs.0);
    ASSERT_FALSE(ECT.X.q_wDigitalInputs.1);
    ASSERT_TRUE(ECT.X.q_wDigitalInputs.2);
    ASSERT_FALSE(ECT.X.q_wDigitalInputs.15);
END_TEST_CASE

TEST_CASE 'Bit write on an array element sets exactly that bit'
    Modbus.arr[0].3 := TRUE;
    ASSERT_EQ(Modbus.arr[0], 16#0008);
    ASSERT_TRUE(Modbus.arr[0].3);
    ASSERT_FALSE(Modbus.arr[0].2);
    ASSERT_EQ(Modbus.arr[1], 0);
    Modbus.arr[0].3 := FALSE;
    ASSERT_EQ(Modbus.arr[0], 0);
END_TEST_CASE

TEST_CASE 'BYTE, DWORD and LWORD bit writes'
VAR
    b  : BYTE;
    dw : DWORD;
    lw : LWORD;
END_VAR
    b.7 := TRUE;
    ASSERT_EQ(b, 16#80);
    dw.31 := TRUE;
    dw.0 := TRUE;
    ASSERT_EQ(dw, 16#80000001);
    lw.63 := TRUE;
    ASSERT_TRUE(lw.63);
    ASSERT_FALSE(lw.62);
    lw.63 := FALSE;
    ASSERT_EQ(lw, 0);
END_TEST_CASE

TEST_CASE 'Bit index given by a constant'
VAR
    w : WORD;
END_VAR
    w.BIT_READY := TRUE;
    ASSERT_EQ(w, 16#0004);
    ASSERT_TRUE(w.BIT_READY);
END_TEST_CASE
