# ST Test Skill

Write and run ST unit tests with assertions and time simulation.

## When to Use

- After implementing a new function block or function
- When fixing a bug (write a regression test first)
- To verify timer-based logic with deterministic time simulation
- Before refactoring existing ST code

## Writing Tests

### Test File Naming

Test files use the `_test.st` suffix alongside the source file:

| Source File | Test File |
|-------------|-----------|
| `motor_control.st` | `motor_control_test.st` |
| `pid_controller.st` | `pid_controller_test.st` |

### Test Structure

```st
TEST_CASE 'descriptive test name'
    VAR
        (* Declare test variables and FB instances here *)
        uut : MyFunctionBlock;
        result : INT;
    END_VAR

    (* Setup: configure inputs *)
    uut.input1 := 10;
    uut.input2 := 20;

    (* Act: execute the logic *)
    uut();

    (* Assert: verify outputs *)
    ASSERT_EQ(uut.output, 30);
END_TEST_CASE
```

### Assertions

| Assertion | Purpose | Example |
|-----------|---------|---------|
| `ASSERT_TRUE(cond)` | Condition is TRUE | `ASSERT_TRUE(motor.isRunning)` |
| `ASSERT_FALSE(cond)` | Condition is FALSE | `ASSERT_FALSE(alarm.active)` |
| `ASSERT_EQ(actual, expected)` | Values are equal | `ASSERT_EQ(counter.value, 10)` |
| `ASSERT_NEAR(actual, expected, tolerance)` | Float comparison | `ASSERT_NEAR(temp, 25.0, 0.1)` |

### Timer and FB Testing

For testing timer-based logic, use `ADVANCE_TIME` to simulate time passing deterministically:

```st
TEST_CASE 'TON timer activates after preset time'
    VAR
        onDelay : TON;
    END_VAR

    (* Start the timer *)
    onDelay(IN := TRUE, PT := T#500ms);
    ASSERT_FALSE(onDelay.Q);

    (* Advance time past the preset *)
    ADVANCE_TIME(T#600ms);
    onDelay(IN := TRUE, PT := T#500ms);
    ASSERT_TRUE(onDelay.Q);
END_TEST_CASE
```

### I/O Mocking

Set input variables directly before calling FB logic -- no special mocking framework needed:

```st
TEST_CASE 'motor stops on emergency'
    VAR
        motor : MotorController;
    END_VAR

    (* Mock inputs *)
    motor.startCmd := TRUE;
    motor.emergencyStop := FALSE;
    motor();
    ASSERT_TRUE(motor.running);

    (* Trigger emergency *)
    motor.emergencyStop := TRUE;
    motor();
    ASSERT_FALSE(motor.running);
END_TEST_CASE
```

## Running Tests

### Basic Test Run

```bash
stc test <directory> --format json
```

Runs all `*_test.st` files in the specified directory.

### JUnit XML Output (for CI)

```bash
stc test <directory> --format junit
```

### Expected JSON Output

```json
{
  "total": 5,
  "passed": 4,
  "failed": 1,
  "errors": 0,
  "results": [
    {"name": "TON timer activates after preset time", "status": "passed", "duration_ms": 2},
    {"name": "motor stops on emergency", "status": "failed", "assertion": "ASSERT_FALSE", "file": "motor_test.st", "line": 15, "col": 5, "message": "expected FALSE but got TRUE"}
  ]
}
```

## Interpreting Results

- **passed**: Test assertions all succeeded
- **failed**: One or more assertions did not hold -- check the `message`, `file`, `line`, and `col` fields
- **error**: Test could not execute (parse error, missing dependency, etc.)

## Error Handling

- **Parse errors in test file**: Fix syntax in the `_test.st` file first, then re-run
- **Missing source file**: Ensure the POU under test is available (in the same directory or imported)
- **Timer tests flaky**: Always use `ADVANCE_TIME` for deterministic behavior -- never rely on wall-clock time
- **Assertion position**: Failed assertions report `file:line:col` for precise location
