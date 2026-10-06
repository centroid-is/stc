package emit

import (
	"os"
	"strings"
	"testing"
)

const emptyArgSrc = "PROGRAM P\nVAR\n\tb : BOOL;\n\tt : TON;\nEND_VAR\nt(IN := b, PT := , Q => , ET => );\nEND_PROGRAM\n"

func TestEmitStructAT(t *testing.T) {
	data, err := os.ReadFile("../../tests/twincat_probes/structat.st")
	if err != nil {
		t.Fatal(err)
	}
	out := emitClean(t, string(data), DefaultOptions())
	if strings.Count(out, "I1 AT %I* : BOOL;") != 1 {
		t.Fatalf("expected struct member AT in output:\n%s", out)
	}
	if !strings.Contains(out, "I2 : BOOL;") {
		t.Fatalf("I2 should print without AT:\n%s", out)
	}
	if again := emitClean(t, out, DefaultOptions()); again != out {
		t.Fatalf("emit not idempotent.\nfirst:\n%s\nsecond:\n%s", out, again)
	}

	out = emitClean(t, "TYPE S :\nSTRUCT\n\ta AT %Q* : INT := 5;\nEND_STRUCT\nEND_TYPE\n", DefaultOptions())
	if !strings.Contains(out, "a AT %Q* : INT := 5;") {
		t.Fatalf("AT with init value lost:\n%s", out)
	}
}

func TestEmitEmptyArg(t *testing.T) {
	out := emitClean(t, emptyArgSrc, DefaultOptions())
	if !strings.Contains(out, "t(IN := b, PT :=, Q =>, ET =>);") {
		t.Fatalf("empty args not printed:\n%s", out)
	}
	if again := emitClean(t, out, DefaultOptions()); again != out {
		t.Fatalf("emit not idempotent.\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}
