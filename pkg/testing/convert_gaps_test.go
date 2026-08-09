package testing

import "testing"

// Conversions used by real TwinCAT code (FB_Track uses TIME_TO_REAL; typed
// counters need INT_TO_UDINT / UDINT_TO_REAL; LREAL accumulators need the
// REAL<->LREAL pair) that were missing from the stdlib.
func TestConversionGaps(t *testing.T) {
	runInline(t, `
TEST_CASE 'LREAL_TO_REAL and REAL_TO_LREAL round-trip'
VAR
	l : LREAL;
	r : REAL;
END_VAR
	r := 2.5;
	l := REAL_TO_LREAL(r);
	l := l + REAL_TO_LREAL(r);
	ASSERT_NEAR(LREAL_TO_REAL(l), 5.0, 0.001, 'accumulated in LREAL');
END_TEST_CASE

TEST_CASE 'INT_TO_UDINT converts'
VAR
	i : INT;
	u : UDINT;
END_VAR
	i := INT#42;
	u := INT_TO_UDINT(i);
	ASSERT_EQ(u, UDINT#42, 'integer widened to UDINT');
END_TEST_CASE

TEST_CASE 'UDINT_TO_REAL converts'
VAR
	u : UDINT;
	r : REAL;
END_VAR
	u := UDINT#7;
	r := UDINT_TO_REAL(u);
	ASSERT_NEAR(r, 7.0, 0.001, 'UDINT as REAL');
END_TEST_CASE

TEST_CASE 'UDINT_TO_LREAL converts'
VAR
	u : UDINT;
END_VAR
	u := UDINT#3;
	ASSERT_NEAR(LREAL_TO_REAL(UDINT_TO_LREAL(u)), 3.0, 0.001, 'UDINT as LREAL');
END_TEST_CASE

TEST_CASE 'TIME_TO_REAL yields milliseconds'
VAR
	r : REAL;
END_VAR
	r := TIME_TO_REAL(T#1500ms);
	ASSERT_NEAR(r, 1500.0, 0.001, 'TIME in ms as REAL');
	r := TIME_TO_REAL(T#2s);
	ASSERT_NEAR(r, 2000.0, 0.001, '2 seconds = 2000 ms');
END_TEST_CASE
`)
}
