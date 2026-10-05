package testing

import (
	"os"
	"path/filepath"
	"testing"
)

// runGVLSuite writes src as name in a temp dir, runs it, and returns the result.
func runGVLSuite(t *testing.T, name, src string) *RunResult {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	return result
}

// requireAllPassed fails the test, listing failures, unless want cases ran and all passed.
func requireAllPassed(t *testing.T, result *RunResult, want int) {
	t.Helper()
	if result.Total != want || result.Passed != want {
		for _, s := range result.Suites {
			for _, tc := range s.Tests {
				if !tc.Passed {
					t.Errorf("%s failed: error=%q assertions=%+v", tc.Name, tc.Error, tc.Assertions)
				}
			}
		}
		t.Fatalf("want %d passed of %d, got %d passed of %d", want, want, result.Passed, result.Total)
	}
}

func TestGVLRunner(t *testing.T) {
	t.Run("GVL state is fresh in every TEST_CASE", func(t *testing.T) {
		result := runGVLSuite(t, "gvl_x_test.st", `
VAR_GLOBAL
    counter : INT;
END_VAR

TEST_CASE 'first'
gvl_x_test.counter := gvl_x_test.counter + 1;
ASSERT_EQ(gvl_x_test.counter, 1);
END_TEST_CASE

TEST_CASE 'second'
gvl_x_test.counter := gvl_x_test.counter + 1;
ASSERT_EQ(gvl_x_test.counter, 1);
END_TEST_CASE
`)
		requireAllPassed(t, result, 2)
	})

	t.Run("bare access to a non-qualified GVL variable", func(t *testing.T) {
		result := runGVLSuite(t, "bare_test.st", `
VAR_GLOBAL
    level : INT := 5;
END_VAR

TEST_CASE 'bare'
level := level * 2;
ASSERT_EQ(level, 10);
ASSERT_EQ(bare_test.level, 10);
END_TEST_CASE
`)
		requireAllPassed(t, result, 1)
	})

	t.Run("FB and FUNCTION bodies resolve the GVL", func(t *testing.T) {
		result := runGVLSuite(t, "g_test.st", `
VAR_GLOBAL
    x : INT := 1;
END_VAR

FUNCTION_BLOCK FB_Bump
VAR_INPUT n : INT; END_VAR
g_test.x := g_test.x + 10;
x := x + 100;
END_FUNCTION_BLOCK

FUNCTION GetX : INT
GetX := x + g_test.x;
END_FUNCTION

TEST_CASE 'fb uses GVL'
VAR b : FB_Bump; END_VAR
b(n := 1);
ASSERT_EQ(g_test.x, 111);
ASSERT_EQ(GetX(), 222);
END_TEST_CASE
`)
		requireAllPassed(t, result, 1)
	})

	t.Run("GVL with struct and stdlib FB members", func(t *testing.T) {
		result := runGVLSuite(t, "io_test.st", `
TYPE ST_Pos : STRUCT x : INT; y : INT; END_STRUCT END_TYPE

VAR_GLOBAL
    pos : ST_Pos;
    t : TON;
END_VAR

TEST_CASE 'members'
io_test.pos.x := 5;
ASSERT_EQ(io_test.pos.x, 5);
ASSERT_EQ(pos.y, 0);
io_test.t(IN := TRUE, PT := T#10MS);
ADVANCE_TIME(T#20MS);
io_test.t(IN := TRUE, PT := T#10MS);
ASSERT_TRUE(io_test.t.Q);
END_TEST_CASE
`)
		requireAllPassed(t, result, 1)
	})
}
