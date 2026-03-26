TYPE MyEnum : (Red, Green := 1, Blue := 2); END_TYPE
TYPE MyStruct :
STRUCT
    x : INT;
    y : REAL;
    name : STRING(50);
END_STRUCT
END_TYPE
TYPE MyArray : ARRAY[1..10] OF INT; END_TYPE
TYPE MySubrange : INT(0..100); END_TYPE
TYPE MyAlias : DINT; END_TYPE
