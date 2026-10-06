package checker

import (
	"os"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const namedFuncs = `FUNCTION F_X : INT
VAR_INPUT
	a : INT;
	b : INT := 5;
END_VAR
F_X := a + b;
END_FUNCTION

FUNCTION F_Out : INT
VAR_INPUT
	a : INT;
END_VAR
VAR_IN_OUT
	io : INT;
END_VAR
VAR_OUTPUT
	q : BOOL;
	cnt : DINT;
END_VAR
q := a > 0;
cnt := a;
F_Out := a;
END_FUNCTION
`

// runNamed checks body inside a PROGRAM that declares the variables the
// named-argument cases use, and returns the errors.
func runNamed(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	prog := `PROGRAM P
VAR
	n : INT;
	flag : BOOL;
	s : STRING;
	k : INT;
	big : DINT;
	lr : LREAL;
	small : SINT;
END_VAR
` + body + "\nEND_PROGRAM\n"
	ds, _ := runGVL(t, []gvlFile{{"funcs.st", namedFuncs}, {"main.st", prog}})
	return errorsOf(ds)
}

func TestNamedArgs(t *testing.T) {
	t.Run("named and positional calls type as the return type", func(t *testing.T) {
		assert.Empty(t, runNamed(t, "n := F_X(a := 1, b := 2);\nn := F_X(1, 2);\nn := F_X(B := k, A := k);"))
		errs := runNamed(t, "s := F_X(a := 1, b := 2);")
		got := requireCodes(t, errs, CodeTypeMismatch, 1)
		assert.Contains(t, got[0].Message, "cannot assign INT to STRING")
	})

	t.Run("omitted inputs and any positional/named mix (ruling A1)", func(t *testing.T) {
		assert.Empty(t, runNamed(t, "n := F_X(a := 1);\nn := F_X(1, b := 2);\nn := F_X(a := 1, 2);\nn := F_X(b := 2);"))
	})

	t.Run("unknown parameter name", func(t *testing.T) {
		errs := runNamed(t, "n := F_X(c := 1);")
		got := requireCodes(t, errs, CodeNoMember, 1)
		assert.Contains(t, got[0].Message, `F_X has no input parameter "c"`)
	})

	t.Run("unknown parameter value is still checked", func(t *testing.T) {
		errs := runNamed(t, "n := F_X(c := undeclared);")
		assert.Len(t, diagsWithCode(errs, CodeNoMember), 1)
		assert.Len(t, diagsWithCode(errs, CodeUndeclared), 1)
	})

	t.Run("duplicate bindings", func(t *testing.T) {
		errs := runNamed(t, "n := F_X(a := 1, a := 2);\nn := F_X(1, a := 2);\nn := F_X(b := 1, 2);")
		got := requireCodes(t, errs, CodeNoMember, 3)
		assert.Contains(t, got[0].Message, `parameter "a" of F_X is bound more than once`)
		assert.Contains(t, got[2].Message, `parameter "b" of F_X is bound more than once`)
	})

	t.Run("argument type mismatch", func(t *testing.T) {
		errs := runNamed(t, "n := F_X(a := 'x');\nn := F_X(a := s, b := 1);\nn := F_X(1, s);")
		got := requireCodes(t, errs, CodeWrongArgType, 3)
		assert.Contains(t, got[0].Message, `input parameter "a"`)
	})

	t.Run("literal and widening arguments", func(t *testing.T) {
		assert.Empty(t, runNamed(t, "n := F_X(a := 1, b := small);\nn := F_X(-1, 300);"))
		errs := runNamed(t, "n := F_X(a := big);\nn := F_X(a := 1.5);")
		requireCodes(t, errs, CodeWrongArgType, 2)
	})

	t.Run("argument count", func(t *testing.T) {
		errs := runNamed(t, "n := F_X(1, 2, 3);")
		got := requireCodes(t, errs, CodeWrongArgCount, 1)
		assert.Contains(t, got[0].Message, "F_X expects 2 argument(s), got 3")
		errs = runNamed(t, "n := F_X(1);")
		requireCodes(t, errs, CodeWrongArgCount, 1)
		errs = runNamed(t, "n := F_X(a := 1, 2, 3);")
		got = requireCodes(t, errs, CodeWrongArgCount, 1)
		assert.Contains(t, got[0].Message, "too many arguments")
	})

	t.Run("output bindings", func(t *testing.T) {
		assert.Empty(t, runNamed(t, "n := F_Out(a := 1, io := k, q => flag);\nn := F_Out(a := 1, io := k, cnt => big, q => flag);\nn := F_Out(1, k, cnt => lr);\nn := F_Out(a := 1, io := k, q => );"))
	})

	t.Run("output binding errors", func(t *testing.T) {
		errs := runNamed(t, "n := F_Out(a := 1, io := k, q => 5);")
		got := requireCodes(t, errs, CodeWrongArgType, 1)
		assert.Contains(t, got[0].Message, `output "q" must be bound to a variable`)

		errs = runNamed(t, "n := F_Out(a := 1, io := k, q => s);")
		got = requireCodes(t, errs, CodeWrongArgType, 1)
		assert.Contains(t, got[0].Message, `cannot bind output "q" (BOOL) to STRING`)

		errs = runNamed(t, "n := F_Out(a := 1, io := k, cnt => small);")
		requireCodes(t, errs, CodeWrongArgType, 1)

		errs = runNamed(t, "n := F_Out(a => k, io := k);")
		got = requireCodes(t, errs, CodeNoMember, 1)
		assert.Contains(t, got[0].Message, `F_Out has no output parameter "a"`)

		errs = runNamed(t, "n := F_Out(a := 1, io := k, q => flag, q => flag);")
		requireCodes(t, errs, CodeNoMember, 1)

		errs = runNamed(t, "n := F_Out(a := 1, io := k, q => nope);")
		requireCodes(t, errs, CodeUndeclared, 1)
	})

	t.Run("output binding targets", func(t *testing.T) {
		src := `TYPE ST : STRUCT
	b : BOOL;
	w : WORD;
END_STRUCT
END_TYPE

PROGRAM P
VAR
	st : ST;
	arr : ARRAY[0..1] OF BOOL;
	p : POINTER TO BOOL;
	k : INT;
	n : INT;
END_VAR
n := F_Out(a := 1, io := k, q => st.b);
n := F_Out(a := 1, io := k, q => arr[1]);
n := F_Out(a := 1, io := k, q => p^);
n := F_Out(a := 1, io := k, q => st.w.3);
n := F_Out(a := 1, io := k, q => (st.b));
END_PROGRAM
`
		ds, _ := runGVL(t, []gvlFile{{"funcs.st", namedFuncs}, {"main.st", src}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("call statement binds the same way", func(t *testing.T) {
		assert.Empty(t, runNamed(t, "F_X(b := 2, a := 1);\nF_Out(a := 1, io := k, q => flag);"))
		errs := runNamed(t, "F_X(b := 2, c := 1);\nF_X(a := 1, a := 1);\nF_X(a := s);\nF_Out(a := 1, io := k, q => 5);")
		assert.Len(t, diagsWithCode(errs, CodeNoMember), 2)
		assert.Len(t, diagsWithCode(errs, CodeWrongArgType), 2)
		assert.Len(t, errs, 4)
	})

	t.Run("statement with positional-first mixed args", func(t *testing.T) {
		assert.Empty(t, runNamed(t, "F_X(1, b := 2);"))
	})

	t.Run("unqualified method call with named args", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_Counter
VAR
	count : INT;
	n : INT;
	done : BOOL;
END_VAR
n := Inc(step := 2);
n := Inc(2, done => done);
n := Inc(stp := 2);
Inc(step := 1);

METHOD Inc : INT
VAR_INPUT
	step : INT := 1;
END_VAR
VAR_OUTPUT
	ok : BOOL;
END_VAR
count := count + step;
Inc := count;
END_METHOD
END_FUNCTION_BLOCK
`
		ds, table := runGVL(t, []gvlFile{{"main.st", src}})
		errs := errorsOf(ds)
		nm := diagsWithCode(errs, CodeNoMember)
		require.Len(t, nm, 2, "%v", errs)
		assert.Contains(t, nm[0].Message, `Inc has no output parameter "done"`)
		assert.Contains(t, nm[1].Message, `Inc has no input parameter "stp"`)
		assert.Len(t, errs, 2)
		m := table.LookupPOU("FB_Counter").LookupLocal("Inc").Type.(*types.FunctionType)
		assert.Equal(t, []string{"ok"}, paramNames(m.Outputs))
	})

	t.Run("inherited method call checks", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_Base
METHOD Add : INT
VAR_INPUT
	x : INT;
END_VAR
Add := x;
END_METHOD
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_D EXTENDS FB_Base
VAR
	n : INT;
END_VAR
n := Add(x := 1);
n := Add(y := 1);
END_FUNCTION_BLOCK
`
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		errs := errorsOf(ds)
		requireCodes(t, errs, CodeNoMember, 1)
	})

	t.Run("method named like a builtin binds to the method", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB
VAR
	n : INT;
END_VAR
n := LIMIT(lo := 1);

METHOD LIMIT : INT
VAR_INPUT
	lo : INT;
END_VAR
LIMIT := lo;
END_METHOD
END_FUNCTION_BLOCK
`
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("member callees stay unchecked (Pitfall 11)", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB
METHOD M : INT
VAR_INPUT
	a : INT;
END_VAR
M := a;
END_METHOD
END_FUNCTION_BLOCK

PROGRAM P
VAR
	inst : FB;
	n : INT;
END_VAR
n := inst.M(zz := 1);
END_PROGRAM
`
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("link probe has no named-argument errors", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/link.st")
		require.NoError(t, err)
		ds := runChecker(string(data))
		assert.Empty(t, diagsWithCode(ds, CodeNoMember))
		assert.Empty(t, diagsWithCode(ds, CodeWrongArgType))
		assert.Empty(t, diagsWithCode(ds, CodeWrongArgCount))
	})

	t.Run("FB call binds a named VAR_IN_OUT argument", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_Parameter
VAR_IN_OUT
	parameter : INT;
END_VAR
parameter := parameter + 1;
END_FUNCTION_BLOCK

PROGRAM P
VAR
	fb : FB_Parameter;
	k : INT;
	s : STRING;
END_VAR
fb(parameter := k);
fb(parameter := s);
END_PROGRAM
`
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		errs := errorsOf(ds)
		got := requireCodes(t, errs, CodeWrongArgType, 1)
		assert.Contains(t, got[0].Message, `"parameter"`)
	})
}
