(* Interface and Implements Test Suite
   Tests that INTERFACE/IMPLEMENTS dispatches methods correctly.
*)

INTERFACE IMovable
    METHOD Move : INT
    VAR_INPUT
        distance : INT;
    END_VAR
    END_METHOD
END_INTERFACE

FUNCTION_BLOCK FB_Car IMPLEMENTS IMovable
VAR
    position : INT := 0;
END_VAR

METHOD Move : INT
VAR_INPUT
    distance : INT;
END_VAR
    position := position + distance;
    Move := position;
END_METHOD

END_FUNCTION_BLOCK

TEST_CASE 'FB implementing interface can call method'
VAR
    car : FB_Car;
    pos : INT;
END_VAR
    pos := car.Move(10);
    ASSERT_EQ(pos, 10);
    pos := car.Move(5);
    ASSERT_EQ(pos, 15);
END_TEST_CASE
