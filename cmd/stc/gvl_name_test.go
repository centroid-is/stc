package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const gvlProbe = "../../tests/twincat_probes/gvl1.st"
const ectProbe = "../../tests/twincat_probes/ECT.st"

// runStcIn runs the stc binary with dir as working directory, so the
// incremental cache of `stc check` lands in dir.
func runStcIn(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(stcBinary, args...)
	cmd.Dir = dir
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if coverDir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	}
	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error running stc: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode
}

// identName returns obj[key].name when obj[key] is an Ident object.
func identName(obj map[string]any, key string) string {
	m, _ := obj[key].(map[string]any)
	s, _ := m["name"].(string)
	return s
}

// gvlDecls returns the GVLDecl nodes in `stc parse --format json` output.
func gvlDecls(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	return findNodes(doc["ast"], func(m map[string]any) bool { return m["kind"] == "GVLDecl" })
}

func requireGVLName(t *testing.T, stdout, want string) {
	t.Helper()
	gvls := gvlDecls(t, stdout)
	if len(gvls) != 1 {
		t.Fatalf("expected 1 GVLDecl, got %d", len(gvls))
	}
	if got := identName(gvls[0], "name"); got != want {
		t.Errorf("GVLDecl name = %q, want %q", got, want)
	}
}

func TestGVLNameFlag(t *testing.T) {
	t.Run("parse renames GVL", func(t *testing.T) {
		stdout, stderr, code := runStc(t, "parse", "--gvl-name", "ECT_RENAMED", "--format", "json", gvlProbe)
		if code != 0 {
			t.Fatalf("exit %d; stderr: %s", code, stderr)
		}
		requireGVLName(t, stdout, "ECT_RENAMED")
	})

	t.Run("parse without flag uses basename", func(t *testing.T) {
		stdout, stderr, code := runStc(t, "parse", "--format", "json", gvlProbe)
		if code != 0 {
			t.Fatalf("exit %d; stderr: %s", code, stderr)
		}
		requireGVLName(t, stdout, "gvl1")
	})

	t.Run("parse sanitises name", func(t *testing.T) {
		stdout, stderr, code := runStc(t, "parse", "--gvl-name", "bad-name", "--format", "json", gvlProbe)
		if code != 0 {
			t.Fatalf("exit %d; stderr: %s", code, stderr)
		}
		requireGVLName(t, stdout, "bad_name")
	})

	t.Run("check resolves renamed GVL", func(t *testing.T) {
		dir := t.TempDir()
		file := writeTestST(t, dir, "globals.st", `VAR_GLOBAL
    x : BOOL;
END_VAR

PROGRAM Main
VAR
    y : BOOL;
END_VAR
    y := G.x;
    G.x := y;
END_PROGRAM
`)
		stdout, stderr, code := runStcIn(t, dir, "check", "--gvl-name", "G", "--format", "json", file)
		if code != 0 {
			t.Fatalf("exit %d; stdout: %s; stderr: %s", code, stdout, stderr)
		}
		if strings.TrimSpace(stdout) != "[]" {
			t.Errorf("expected no diagnostics, got: %s", stdout)
		}
	})

	t.Run("check without flag does not resolve G", func(t *testing.T) {
		dir := t.TempDir()
		file := writeTestST(t, dir, "globals.st", `VAR_GLOBAL
    x : BOOL;
END_VAR

PROGRAM Main
VAR
    y : BOOL;
END_VAR
    y := G.x;
END_PROGRAM
`)
		_, _, code := runStcIn(t, dir, "check", "--format", "json", file)
		if code == 0 {
			t.Errorf("expected G to be unresolved without --gvl-name")
		}
	})

	t.Run("check is stable across incremental cache hits", func(t *testing.T) {
		dir := t.TempDir()
		file := writeTestST(t, dir, "globals.st", `VAR_GLOBAL
    x : BOOL;
END_VAR

PROGRAM Main
VAR
    y : BOOL;
END_VAR
    y := G.x;
    z := G.x;
END_PROGRAM
`)
		// Text mode reports how many files were re-parsed, which proves the
		// second run is an incremental cache hit.
		_, err1, code1 := runStcIn(t, dir, "check", "--gvl-name", "G", file)
		_, err2, code2 := runStcIn(t, dir, "check", "--gvl-name", "G", file)
		if !strings.Contains(err1, "(1/1 files re-parsed)") {
			t.Fatalf("expected a cold parse on the first run: %s", err1)
		}
		if !strings.Contains(err2, "(0/1 files re-parsed)") {
			t.Fatalf("expected a cache hit on the second run: %s", err2)
		}
		diagLines := func(s string) string {
			lines := strings.Split(strings.TrimSpace(s), "\n")
			return strings.Join(lines[:len(lines)-1], "\n")
		}
		if code1 != code2 || diagLines(err1) != diagLines(err2) {
			t.Errorf("cold and cached runs differ:\ncold (%d): %s\ncached (%d): %s", code1, err1, code2, err2)
		}
		// Only the undeclared z is reported; G.x resolves on both runs.
		if !strings.Contains(err2, `undeclared identifier "z"`) || !strings.Contains(err2, "1 error(s)") {
			t.Errorf("expected exactly the undeclared z on the cached run: %s", err2)
		}
	})

	for _, sub := range []string{"fmt", "emit"} {
		t.Run(sub+" accepts flag", func(t *testing.T) {
			stdout, stderr, code := runStc(t, sub, "--gvl-name", "X", gvlProbe)
			if code != 0 {
				t.Fatalf("exit %d; stderr: %s", code, stderr)
			}
			if !strings.Contains(stdout, "VAR_GLOBAL") {
				t.Errorf("expected VAR_GLOBAL in output, got: %s", stdout)
			}
		})
	}

	for _, sub := range []string{"parse", "check", "fmt", "emit"} {
		t.Run(sub+" rejects multiple files", func(t *testing.T) {
			stdout, stderr, code := runStc(t, sub, "--gvl-name", "X", gvlProbe, ectProbe)
			if code == 0 {
				t.Fatalf("expected non-zero exit; stdout: %s", stdout)
			}
			if !strings.Contains(stderr, "gvl-name") {
				t.Errorf("expected usage error mentioning gvl-name on stderr, got: %s", stderr)
			}
		})

		t.Run(sub+" rejects multiple files as JSON", func(t *testing.T) {
			stdout, _, code := runStc(t, sub, "--gvl-name", "X", "--format", "json", gvlProbe, ectProbe)
			if code == 0 {
				t.Fatalf("expected non-zero exit; stdout: %s", stdout)
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
				t.Fatalf("stdout is not a JSON object: %v\n%s", err, stdout)
			}
			msg, _ := obj["error"].(string)
			if !strings.Contains(msg, "gvl-name") {
				t.Errorf("error = %q, want it to mention gvl-name", msg)
			}
		})
	}
}
