(* Parse-Only OOP Test
   This file exercises OOP features that parse correctly but are not
   yet fully supported in the interpreter. Validated with `stc parse`.

   Features tested at parse level:
   - FUNCTION_BLOCK with METHOD (multiple methods, with/without return types)
   - FUNCTION_BLOCK with PROPERTY (getter/setter)
   - FUNCTION_BLOCK with EXTENDS (inheritance)
   - INTERFACE declaration
   - FUNCTION_BLOCK IMPLEMENTS interface
   - Access modifiers (PUBLIC, PRIVATE, PROTECTED)
*)

(* ===== INTERFACE ===== *)

INTERFACE IMovable
METHOD Move : BOOL
VAR_INPUT
    dx : INT;
    dy : INT;
END_VAR
END_METHOD

METHOD GetPosition : INT
END_METHOD
END_INTERFACE

INTERFACE IResettable
METHOD Reset : BOOL
END_METHOD
END_INTERFACE

(* ===== BASE FUNCTION_BLOCK WITH METHODS ===== *)

FUNCTION_BLOCK FB_Shape
VAR
    _x : INT := 0;
    _y : INT := 0;
    _name : STRING := '';
END_VAR

METHOD PUBLIC SetPosition : BOOL
VAR_INPUT
    newX : INT;
    newY : INT;
END_VAR
    _x := newX;
    _y := newY;
    SetPosition := TRUE;
END_METHOD

METHOD PUBLIC GetX : INT
    GetX := _x;
END_METHOD

METHOD PUBLIC GetY : INT
    GetY := _y;
END_METHOD

METHOD PRIVATE InternalCalc : INT
    InternalCalc := _x * _y;
END_METHOD

(* Method without return type *)
METHOD PUBLIC MoveTo
VAR_INPUT
    targetX : INT;
    targetY : INT;
END_VAR
    _x := targetX;
    _y := targetY;
END_METHOD

END_FUNCTION_BLOCK

(* ===== INHERITANCE WITH EXTENDS ===== *)

FUNCTION_BLOCK FB_Rectangle EXTENDS FB_Shape
VAR
    _width : INT := 0;
    _height : INT := 0;
END_VAR

METHOD PUBLIC SetSize : BOOL
VAR_INPUT
    w : INT;
    h : INT;
END_VAR
    _width := w;
    _height := h;
    SetSize := TRUE;
END_METHOD

METHOD PUBLIC Area : INT
    Area := _width * _height;
END_METHOD

METHOD PUBLIC Perimeter : INT
    Perimeter := 2 * (_width + _height);
END_METHOD

END_FUNCTION_BLOCK

(* ===== IMPLEMENTS INTERFACE ===== *)

FUNCTION_BLOCK FB_MovableShape EXTENDS FB_Shape IMPLEMENTS IMovable, IResettable
VAR
    _moveCount : INT := 0;
END_VAR

METHOD PUBLIC Move : BOOL
VAR_INPUT
    dx : INT;
    dy : INT;
END_VAR
    _x := _x + dx;
    _y := _y + dy;
    _moveCount := _moveCount + 1;
    Move := TRUE;
END_METHOD

METHOD PUBLIC GetPosition : INT
    GetPosition := _x;
END_METHOD

METHOD PUBLIC Reset : BOOL
    _x := 0;
    _y := 0;
    _moveCount := 0;
    Reset := TRUE;
END_METHOD

END_FUNCTION_BLOCK

(* ===== PROPERTY ===== *)

FUNCTION_BLOCK FB_WithProperty
VAR
    _value : INT := 0;
    _readCount : INT := 0;
END_VAR

PROPERTY Value : INT
    GET
        _readCount := _readCount + 1;
        Value := _value;
    END_GET
    SET
        _value := Value;
    END_SET
END_PROPERTY

METHOD PUBLIC GetReadCount : INT
    GetReadCount := _readCount;
END_METHOD

END_FUNCTION_BLOCK

(* ===== PROTECTED ACCESS ===== *)

FUNCTION_BLOCK FB_BaseWithProtected
VAR
    publicVar : INT := 0;
END_VAR

METHOD PUBLIC PublicMethod : INT
    PublicMethod := publicVar;
END_METHOD

METHOD PROTECTED ProtectedHelper : INT
    ProtectedHelper := publicVar * 2;
END_METHOD

END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_DerivedWithProtected EXTENDS FB_BaseWithProtected

METHOD PUBLIC UseProtected : INT
    UseProtected := publicVar + 10;
END_METHOD

END_FUNCTION_BLOCK
