(* Integer wrap-around test suite (RUNT-05)
   Typed integer stores and typed arithmetic wrap to the IEC width of the
   type. Operands use typed literals so the suite does not depend on the
   checker adopting untyped literals.
*)

TEST_CASE 'INT wraps from 32767 to -32768'
VAR
    i : INT := INT#32767;
END_VAR
    i := i + INT#1;
    ASSERT_EQ(i, INT#-32768);
    i := i - INT#1;
    ASSERT_EQ(i, INT#32767);
END_TEST_CASE

TEST_CASE 'UINT wraps from 0 to 65535'
VAR
    u : UINT;
END_VAR
    u := u - UINT#1;
    ASSERT_EQ(u, UINT#65535);
    u := u + UINT#1;
    ASSERT_EQ(u, UINT#0);
END_TEST_CASE

TEST_CASE 'Byte-wide types wrap'
VAR
    b : BYTE := BYTE#255;
    s : SINT := SINT#127;
    us : USINT;
END_VAR
    b := b + BYTE#1;
    ASSERT_EQ(b, BYTE#0);
    s := s + SINT#1;
    ASSERT_EQ(s, SINT#-128);
    us := us - USINT#1;
    ASSERT_EQ(us, USINT#255);
END_TEST_CASE

TEST_CASE 'DINT and UDINT wrap'
VAR
    d : DINT := DINT#2147483647;
    ud : UDINT;
END_VAR
    d := d + DINT#1;
    ASSERT_EQ(d, DINT#-2147483648);
    ud := ud - UDINT#1;
    ASSERT_EQ(ud, UDINT#4294967295);
END_TEST_CASE
