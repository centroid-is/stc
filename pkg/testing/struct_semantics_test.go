package testing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInline writes src as a single _test.st file into a temp dir and runs it.
// Every TEST_CASE in src is expected to pass; failures are reported verbatim.
func runInline(t *testing.T, src string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "inline_test.st")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	result, err := Run(dir)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Total == 0 {
		t.Fatal("no tests discovered")
	}
	if result.HasFailures() {
		var sb strings.Builder
		for _, suite := range result.Suites {
			for _, tc := range suite.Tests {
				if !tc.Passed {
					sb.WriteString("\n  FAIL: " + tc.Name)
					if tc.Error != "" {
						sb.WriteString(" -- " + tc.Error)
					}
					for _, a := range tc.Assertions {
						if !a.Passed {
							sb.WriteString("\n      assert: " + a.Message)
						}
					}
				}
			}
		}
		t.Errorf("%d/%d tests failed:%s", result.Failed+result.Errors, result.Total, sb.String())
	}
}

const structPrelude = `
TYPE ST_Item :
STRUCT
	position : REAL;
	xOccupied : BOOL;
	uID : UDINT;
END_STRUCT
END_TYPE
`

// A user-defined STRUCT used as an array element type must be zero-initialised
// as a struct, not fall through to an INT zero.
func TestArrayOfStruct_MemberAssignPersists(t *testing.T) {
	runInline(t, structPrelude+`
TEST_CASE 'member assignment into an array element persists'
VAR
	q : ARRAY [1..10] OF ST_Item;
END_VAR
	q[3].position := 250.0;
	q[3].xOccupied := TRUE;
	q[3].uID := UDINT#42;
	ASSERT_NEAR(q[3].position, 250.0, 0.001, 'position readback');
	ASSERT_TRUE(q[3].xOccupied, 'flag readback');
	ASSERT_EQ(q[3].uID, UDINT#42, 'id readback');
END_TEST_CASE
`)
}

// Each array element must own its own storage. A single shared zero value would
// make one write visible in every slot.
func TestArrayOfStruct_ElementsAreIndependent(t *testing.T) {
	runInline(t, structPrelude+`
TEST_CASE 'array elements do not alias each other'
VAR
	q : ARRAY [1..10] OF ST_Item;
END_VAR
	q[1].position := 100.0;
	q[2].position := 200.0;
	ASSERT_NEAR(q[1].position, 100.0, 0.001, 'slot 1 unaffected by slot 2');
	ASSERT_NEAR(q[2].position, 200.0, 0.001, 'slot 2 holds its own value');
	ASSERT_NEAR(q[3].position, 0.0, 0.001, 'untouched slot still zero');
	ASSERT_FALSE(q[4].xOccupied, 'untouched flag still false');
END_TEST_CASE
`)
}

// STRUCT is a value type in IEC 61131-3: assignment copies.
func TestStruct_AssignmentCopies(t *testing.T) {
	runInline(t, structPrelude+`
TEST_CASE 'struct assignment copies rather than aliases'
VAR
	a : ST_Item;
	b : ST_Item;
END_VAR
	a.position := 100.0;
	b := a;
	a.position := 999.0;
	ASSERT_NEAR(b.position, 100.0, 0.001, 'b must not follow later writes to a');
	ASSERT_NEAR(a.position, 999.0, 0.001, 'a keeps its own new value');
END_TEST_CASE
`)
}

// The shift-down pattern used by queue removal: batches[i] := batches[i+1].
// If element assignment aliases, the whole tail collapses onto one value.
func TestArrayOfStruct_ShiftDownPattern(t *testing.T) {
	runInline(t, structPrelude+`
TEST_CASE 'shifting elements down preserves distinct values'
VAR
	q : ARRAY [1..10] OF ST_Item;
	i : INT;
END_VAR
	q[1].position := 3000.0; q[1].uID := UDINT#1; q[1].xOccupied := TRUE;
	q[2].position := 2000.0; q[2].uID := UDINT#2; q[2].xOccupied := TRUE;
	q[3].position := 1000.0; q[3].uID := UDINT#3; q[3].xOccupied := TRUE;

	FOR i := INT#1 TO INT#9 BY INT#1 DO
		q[i] := q[i+1];
	END_FOR
	q[10].position := 0.0;
	q[10].xOccupied := FALSE;
	q[10].uID := UDINT#0;

	ASSERT_NEAR(q[1].position, 2000.0, 0.001, 'shifted down by one');
	ASSERT_NEAR(q[2].position, 1000.0, 0.001, 'shifted down by one');
	ASSERT_EQ(q[1].uID, UDINT#2, 'ids travel with the shift');
	ASSERT_EQ(q[2].uID, UDINT#3, 'ids travel with the shift');
	ASSERT_FALSE(q[3].xOccupied, 'vacated slot cleared');
	ASSERT_EQ(q[3].uID, UDINT#0, 'vacated id cleared');
END_TEST_CASE
`)
}

// Same requirement for arrays of struct declared in an FB's VAR section, which
// take a different initialisation path than test-case locals.
func TestArrayOfStruct_InsideFunctionBlock(t *testing.T) {
	runInline(t, structPrelude+`
FUNCTION_BLOCK FB_Holder
VAR_INPUT
	i_xGo : BOOL;
END_VAR
VAR_OUTPUT
	q_rSum : REAL;
END_VAR
VAR
	arr : ARRAY [1..3] OF ST_Item;
END_VAR
IF i_xGo THEN
	arr[1].position := 10.0;
	arr[2].position := 20.0;
	q_rSum := arr[1].position + arr[2].position + arr[3].position;
END_IF
END_FUNCTION_BLOCK

TEST_CASE 'FB-local array of struct is usable'
VAR
	h : FB_Holder;
END_VAR
	h(i_xGo := TRUE);
	ASSERT_NEAR(h.q_rSum, 30.0, 0.001, 'writes persisted and third slot stayed zero');
END_TEST_CASE
`)
}

// VAR_IN_OUT must pass the array by reference so the callee mutates the
// caller's storage -- this is what the batch queue primitives rely on.
func TestArrayOfStruct_VarInOutMutatesCaller(t *testing.T) {
	runInline(t, structPrelude+`
FUNCTION_BLOCK FB_Writer
VAR_INPUT
	i_iIndex : INT;
	i_rPosition : REAL;
END_VAR
VAR_IN_OUT
	items : ARRAY [1..10] OF ST_Item;
END_VAR
items[i_iIndex].position := i_rPosition;
items[i_iIndex].xOccupied := TRUE;
END_FUNCTION_BLOCK

TEST_CASE 'VAR_IN_OUT array of struct is mutated in place'
VAR
	w : FB_Writer;
	q : ARRAY [1..10] OF ST_Item;
END_VAR
	w(i_iIndex := INT#4, i_rPosition := 777.0, items := q);
	ASSERT_NEAR(q[4].position, 777.0, 0.001, 'callee wrote into the caller array');
	ASSERT_TRUE(q[4].xOccupied, 'callee flag visible to caller');
	ASSERT_FALSE(q[5].xOccupied, 'neighbouring slot untouched');
END_TEST_CASE
`)
}

// A struct whose member is an array, and an array of structs nested one level
// deeper -- both need recursive resolution of the element/member types.
func TestNestedAggregates(t *testing.T) {
	runInline(t, `
TYPE ST_Inner :
STRUCT
	value : REAL;
END_STRUCT
END_TYPE

TYPE ST_Outer :
STRUCT
	name : REAL;
	slots : ARRAY [1..4] OF ST_Inner;
END_STRUCT
END_TYPE

TEST_CASE 'struct member that is an array of struct'
VAR
	o : ST_Outer;
END_VAR
	o.slots[2].value := 12.5;
	ASSERT_NEAR(o.slots[2].value, 12.5, 0.001, 'nested member write persists');
	ASSERT_NEAR(o.slots[1].value, 0.0, 0.001, 'sibling slot independent');
END_TEST_CASE

TEST_CASE 'nested struct assignment copies'
VAR
	a : ST_Outer;
	b : ST_Outer;
END_VAR
	a.slots[1].value := 5.0;
	b := a;
	a.slots[1].value := 6.0;
	ASSERT_NEAR(b.slots[1].value, 5.0, 0.001, 'deep copy on assignment');
END_TEST_CASE
`)
}

// Assigning an array element through an indexed base that is itself a member
// access: s.arr[i] := val.
func TestAssignIndexThroughMemberBase(t *testing.T) {
	runInline(t, `
TYPE ST_Bag :
STRUCT
	slots : ARRAY [1..4] OF REAL;
END_STRUCT
END_TYPE

TEST_CASE 'assignment to an array inside a struct'
VAR
	b : ST_Bag;
END_VAR
	b.slots[2] := 42.0;
	ASSERT_NEAR(b.slots[2], 42.0, 0.001, 'indexed member assignment persists');
	ASSERT_NEAR(b.slots[1], 0.0, 0.001, 'sibling untouched');
END_TEST_CASE
`)
}
