# EtherCAT Terminal I/O Stub Examples for Beckhoff TwinCAT 3

This document provides example GVL (Global Variable List) stubs showing how
to declare AT-addressed variables for common EtherCAT I/O terminals. These
examples can be copied and adapted for your project's specific terminal layout.

## How EtherCAT I/O Mapping Works

EtherCAT terminals are mapped to the PLC I/O image through Process Data Objects
(PDOs). The mapping is configured in the TwinCAT System Manager, not in ST code.
ST code accesses I/O through AT-addressed variables that reference byte offsets
in the process image.

stc does not model specific terminal hardware. It maps AT addresses to a mock
I/O table for testing. The examples below show the AT address patterns for
common terminals.

## EL1008: 8-Channel Digital Input

8 digital inputs packed into 1 byte. Each channel is a single bit.

```iec
VAR_GLOBAL
    (* EL1008 at I/O offset byte 0 *)
    bDI_Ch1 AT %IX0.0 : BOOL;   (* Channel 1 *)
    bDI_Ch2 AT %IX0.1 : BOOL;   (* Channel 2 *)
    bDI_Ch3 AT %IX0.2 : BOOL;   (* Channel 3 *)
    bDI_Ch4 AT %IX0.3 : BOOL;   (* Channel 4 *)
    bDI_Ch5 AT %IX0.4 : BOOL;   (* Channel 5 *)
    bDI_Ch6 AT %IX0.5 : BOOL;   (* Channel 6 *)
    bDI_Ch7 AT %IX0.6 : BOOL;   (* Channel 7 *)
    bDI_Ch8 AT %IX0.7 : BOOL;   (* Channel 8 *)

    (* Or as a single byte for bit-level manipulation *)
    nDI_Byte AT %IB0 : BYTE;
END_VAR
```

## EL2008: 8-Channel Digital Output

8 digital outputs packed into 1 byte. Same bit layout as EL1008.

```iec
VAR_GLOBAL
    (* EL2008 at output offset byte 0 *)
    bDO_Ch1 AT %QX0.0 : BOOL;   (* Channel 1 *)
    bDO_Ch2 AT %QX0.1 : BOOL;   (* Channel 2 *)
    bDO_Ch3 AT %QX0.2 : BOOL;   (* Channel 3 *)
    bDO_Ch4 AT %QX0.3 : BOOL;   (* Channel 4 *)
    bDO_Ch5 AT %QX0.4 : BOOL;   (* Channel 5 *)
    bDO_Ch6 AT %QX0.5 : BOOL;   (* Channel 6 *)
    bDO_Ch7 AT %QX0.6 : BOOL;   (* Channel 7 *)
    bDO_Ch8 AT %QX0.7 : BOOL;   (* Channel 8 *)
END_VAR
```

## EL3064: 4-Channel Analog Input (0-10V, 12-bit)

4 analog inputs, each mapped to a 16-bit word (INT). With compact PDO mapping,
the 4 channels occupy 8 bytes (4 x 16-bit words).

```iec
VAR_GLOBAL
    (* EL3064 at input offset byte 0 *)
    nAI_Ch1 AT %IW0 : INT;      (* Channel 1: 0..32767 = 0..10V *)
    nAI_Ch2 AT %IW2 : INT;      (* Channel 2 *)
    nAI_Ch3 AT %IW4 : INT;      (* Channel 3 *)
    nAI_Ch4 AT %IW6 : INT;      (* Channel 4 *)
END_VAR
```

**Scaling note:** The raw INT value maps linearly to the physical voltage range.
For 12-bit resolution: 0 = 0V, 32767 = 10V. Scaling to engineering units should
be done in the PLC program.

## EL4034: 4-Channel Analog Output (16-bit)

4 analog outputs, each mapped to a 16-bit word (INT).

```iec
VAR_GLOBAL
    (* EL4034 at output offset byte 0 *)
    nAO_Ch1 AT %QW0 : INT;      (* Channel 1 *)
    nAO_Ch2 AT %QW2 : INT;      (* Channel 2 *)
    nAO_Ch3 AT %QW4 : INT;      (* Channel 3 *)
    nAO_Ch4 AT %QW6 : INT;      (* Channel 4 *)
END_VAR
```

## Usage in stc Testing

When writing tests with stc, use SET_IO and GET_IO to inject and read I/O values:

```iec
TEST_CASE 'Digital input triggers output'
VAR
    myProg : MyProgram;
END_VAR

(* Simulate EL1008 channel 1 going high *)
SET_IO('I', 0, 0, TRUE);

(* Run one scan cycle *)
myProg();

(* Verify EL2008 channel 1 went high *)
ASSERT_TRUE(GET_IO('Q', 0, 0), 'Output should follow input');

END_TEST_CASE
```

## Tips

- AT addresses in stubs should match your TwinCAT System Manager configuration.
- Use `%I*` or `%Q*` (wildcard) during development; assign specific addresses
  when the hardware layout is finalized.
- stc's overlap detection warns if two variables reference the same byte range
  (e.g., `%IW0` and `%IX0.3` overlap).
