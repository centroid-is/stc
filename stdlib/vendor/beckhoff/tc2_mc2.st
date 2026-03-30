(* Tc2_MC2 -- PLCopen Motion Control function blocks for Beckhoff TwinCAT 3 *)
(* Stub declarations for stc type-checking. No implementation bodies. *)

FUNCTION_BLOCK MC_Power
VAR_INPUT
    Axis            : AXIS_REF;
    Enable          : BOOL;
    Enable_Positive : BOOL;
    Enable_Negative : BOOL;
    Override_V      : LREAL := 100.0;
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
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Position        : LREAL;
    Velocity        : LREAL;
    Acceleration    : LREAL;
    Deceleration    : LREAL;
    Jerk            : LREAL;
    Direction       : MC_Direction;
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

FUNCTION_BLOCK MC_MoveRelative
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Distance        : LREAL;
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

FUNCTION_BLOCK MC_MoveVelocity
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Velocity        : LREAL;
    Acceleration    : LREAL;
    Deceleration    : LREAL;
    Jerk            : LREAL;
    Direction       : MC_Direction;
END_VAR
VAR_OUTPUT
    InVelocity      : BOOL;
    Busy            : BOOL;
    Active          : BOOL;
    CommandAborted  : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_Stop
VAR_INPUT
    Axis            : AXIS_REF;
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

FUNCTION_BLOCK MC_Home
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
    Position        : LREAL;
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

FUNCTION_BLOCK MC_Reset
VAR_INPUT
    Axis            : AXIS_REF;
    Execute         : BOOL;
END_VAR
VAR_OUTPUT
    Done            : BOOL;
    Busy            : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_ReadActualPosition
VAR_INPUT
    Axis            : AXIS_REF;
    Enable          : BOOL;
END_VAR
VAR_OUTPUT
    Valid           : BOOL;
    Busy            : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
    Position        : LREAL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_ReadActualVelocity
VAR_INPUT
    Axis            : AXIS_REF;
    Enable          : BOOL;
END_VAR
VAR_OUTPUT
    Valid           : BOOL;
    Busy            : BOOL;
    Error           : BOOL;
    ErrorID         : UDINT;
    ActualVelocity  : LREAL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_ReadStatus
VAR_INPUT
    Axis            : AXIS_REF;
    Enable          : BOOL;
END_VAR
VAR_OUTPUT
    Valid               : BOOL;
    Busy                : BOOL;
    Error               : BOOL;
    ErrorID             : UDINT;
    ErrorStop           : BOOL;
    Disabled            : BOOL;
    Stopping            : BOOL;
    StandStill          : BOOL;
    DiscreteMotion      : BOOL;
    ContinuousMotion    : BOOL;
    SynchronizedMotion  : BOOL;
    Homing              : BOOL;
END_VAR
END_FUNCTION_BLOCK
