(* Schneider Electric EcoStruxure -- PLCopen Motion Control stubs *)
(* Schneider-specific parameters differ from Beckhoff. *)

(* Schneider AXIS_REF -- different internal structure from Beckhoff *)
TYPE SE_AXIS_REF :
STRUCT
    AxisId      : UDINT;
    AxisState   : INT;
END_STRUCT
END_TYPE

FUNCTION_BLOCK MC_Power
VAR_INPUT
    Axis            : SE_AXIS_REF;
    Enable          : BOOL;
    EnablePositive  : BOOL;
    EnableNegative  : BOOL;
    ControllerMode  : INT;
END_VAR
VAR_OUTPUT
    Status      : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis            : SE_AXIS_REF;
    Execute         : BOOL;
    Position        : LREAL;
    Velocity        : LREAL;
    Acceleration    : LREAL;
    Deceleration    : LREAL;
    Jerk            : LREAL;
END_VAR
VAR_OUTPUT
    Done            : BOOL;
    Busy            : BOOL;
    Active          : BOOL;
    CommandAborted  : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_Stop
VAR_INPUT
    Axis            : SE_AXIS_REF;
    Execute         : BOOL;
    Deceleration    : LREAL;
    Jerk            : LREAL;
END_VAR
VAR_OUTPUT
    Done            : BOOL;
    Busy            : BOOL;
    Active          : BOOL;
    CommandAborted  : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
END_FUNCTION_BLOCK
