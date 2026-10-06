(* Initialiser test suite (RUNT-06)
   Array, repetition, struct-array, TYPE default and FB-instance
   initialisers apply when the variable is instantiated. Literals are typed
   where the checker's literal adoption would otherwise matter.
*)

TYPE ST_Item :
STRUCT
    id : INT := INT#5;
    name : STRING := 'x';
END_STRUCT
END_TYPE

TYPE T_Speed : INT := INT#50; END_TYPE

FUNCTION_BLOCK FB_Base
VAR
    base_x : INT := INT#3;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Drive EXTENDS FB_Base
VAR_INPUT
    speed : INT;
END_VAR
VAR
    tbl : ARRAY[1..2] OF INT := [INT#4, INT#5];
    item : ST_Item := (name := 'drive');
END_VAR
END_FUNCTION_BLOCK

VAR_GLOBAL CONSTANT
    N_ITEMS : INT := 3;
END_VAR

TEST_CASE 'Array initialiser'
VAR
    arr : ARRAY[1..3] OF INT := [INT#1, INT#2, INT#3];
END_VAR
    ASSERT_EQ(arr[1], INT#1);
    ASSERT_EQ(arr[2], INT#2);
    ASSERT_EQ(arr[3], INT#3);
END_TEST_CASE

TEST_CASE 'Repetition and tail defaults'
VAR
    rep : ARRAY[1..5] OF INT := [3(INT#0), INT#7];
END_VAR
    ASSERT_EQ(rep[3], INT#0);
    ASSERT_EQ(rep[4], INT#7);
    ASSERT_EQ(rep[5], INT#0);
END_TEST_CASE

TEST_CASE 'Struct array over a GVL constant bound'
VAR
    items : ARRAY[1..init_test.N_ITEMS] OF ST_Item := [(id := INT#1, name := 'a'), (id := INT#2)];
END_VAR
    ASSERT_EQ(items[1].name, 'a');
    ASSERT_EQ(items[2].id, INT#2);
    ASSERT_EQ(items[2].name, 'x');
    ASSERT_EQ(items[3].id, INT#5);
END_TEST_CASE

TEST_CASE 'TYPE defaults'
VAR
    s : ST_Item;
    v : T_Speed;
END_VAR
    ASSERT_EQ(s.id, INT#5);
    ASSERT_EQ(v, INT#50);
END_TEST_CASE

TEST_CASE 'FB instances get their own initialised members'
VAR
    d1 : FB_Drive;
    d2 : FB_Drive := (speed := INT#9);
END_VAR
    ASSERT_EQ(d1.base_x, INT#3);
    ASSERT_EQ(d2.tbl[2], INT#5);
    ASSERT_EQ(d2.speed, INT#9);
    d1.item.name := 'changed';
    ASSERT_EQ(d2.item.name, 'drive');
END_TEST_CASE
