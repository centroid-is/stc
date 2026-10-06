package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// helpFunc returns the --help output of binary ("stc" or "stc-mcp") for the
// subcommand path.
type helpFunc func(binary string, path []string) (string, error)

// checkDocLines checks every stc and stc-mcp command line in the fenced
// shell blocks of content against the CLI's --help output.
//
// Only blocks tagged bash, sh, shell, console or untagged are checked. A
// block whose info string has a second word (for example
// "bash pending-phase-27") documents a feature that has not landed and is
// skipped. Each line may start with a "$ " prompt.
func checkDocLines(file, content string, help helpFunc) []string {
	c := &docChecker{help: help, cache: map[string]helpInfo{}}
	var fails []string
	inBlock, check := false, false
	for i, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			if inBlock {
				inBlock = false
				continue
			}
			inBlock = true
			info := strings.Fields(strings.TrimPrefix(line, "```"))
			check = len(info) == 0 || (len(info) == 1 && shellLangs[strings.ToLower(info[0])])
			continue
		}
		if !inBlock || !check {
			continue
		}
		line = strings.TrimPrefix(line, "$ ")
		toks := shellFields(line)
		if len(toks) == 0 || (toks[0] != "stc" && toks[0] != "stc-mcp") {
			continue
		}
		if msg := c.checkCommand(toks); msg != "" {
			fails = append(fails, fmt.Sprintf("%s:%d: %s", file, i+1, msg))
		}
	}
	return fails
}

var shellLangs = map[string]bool{"bash": true, "sh": true, "shell": true, "console": true}

// helpInfo is the parsed --help of one command path.
type helpInfo struct {
	text  string
	subs  map[string]bool
	group bool // usage is only "<path> [command]": an argument must be a subcommand
	err   error
}

type docChecker struct {
	help  helpFunc
	cache map[string]helpInfo
}

func (c *docChecker) info(binary string, path []string) helpInfo {
	key := binary + " " + strings.Join(path, " ")
	if hi, ok := c.cache[key]; ok {
		return hi
	}
	text, err := c.help(binary, path)
	hi := helpInfo{text: text, subs: map[string]bool{}, err: err}
	section := ""
	usage := 0
	groupUsage := 0
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(l, " ") && strings.HasSuffix(t, ":") {
			section = t
			continue
		}
		switch section {
		case "Available Commands:":
			hi.subs[strings.Fields(t)[0]] = true
		case "Usage:":
			usage++
			if strings.HasSuffix(t, "[command]") {
				groupUsage++
			}
		}
	}
	hi.group = usage > 0 && usage == groupUsage
	c.cache[key] = hi
	return hi
}

var flagTok = regexp.MustCompile(`^\[?(--?[A-Za-z][A-Za-z0-9-]*)`)

// checkCommand resolves the subcommand path of toks and checks every flag.
// It stops at a pipe, redirect or command separator outside brackets.
func (c *docChecker) checkCommand(toks []string) string {
	binary := toks[0]
	var path []string
	walking := binary == "stc"
	depth := 0
	var flags []string
	for _, tok := range toks[1:] {
		if depth == 0 && (tok == "|" || tok == "&&" || tok == ";" || tok == "||" || strings.HasPrefix(tok, ">") || strings.HasPrefix(tok, "2>")) {
			break
		}
		depth += strings.Count(tok, "[") - strings.Count(tok, "]")
		if m := flagTok.FindStringSubmatch(tok); m != nil {
			walking = false
			flags = append(flags, m[1])
			continue
		}
		if !walking {
			continue
		}
		hi := c.info(binary, path)
		if hi.subs[tok] {
			path = append(path, tok)
			continue
		}
		walking = false
		if hi.group && !strings.HasPrefix(tok, "<") && !strings.HasPrefix(tok, "[") {
			return fmt.Sprintf("%s has no subcommand %s", cmdName(binary, path), tok)
		}
	}
	hi := c.info(binary, path)
	if hi.err != nil {
		return fmt.Sprintf("%s --help failed: %v", cmdName(binary, path), hi.err)
	}
	for _, f := range flags {
		if !hasFlag(binary, hi.text, f) {
			return fmt.Sprintf("%s has no flag %s", cmdName(binary, path), f)
		}
	}
	return ""
}

func cmdName(binary string, path []string) string {
	return strings.TrimSpace(binary + " " + strings.Join(path, " "))
}

// hasFlag reports whether help lists flag. cobra prints "--name" and "-x,";
// stc-mcp uses Go's flag package, which prints "-name" for both spellings.
func hasFlag(binary, help, flag string) bool {
	name := strings.TrimLeft(flag, "-")
	var pat string
	switch {
	case binary == "stc-mcp":
		pat = `(?m)^\s+-` + regexp.QuoteMeta(name) + `(\s|$)`
	case strings.HasPrefix(flag, "--"):
		pat = `--` + regexp.QuoteMeta(name) + `(\s|,|$)`
	default:
		pat = `(?m)^\s+-` + regexp.QuoteMeta(name) + `,`
	}
	return regexp.MustCompile(pat).MatchString(help)
}

// shellFields splits a command line like a shell: whitespace separates
// words, and single or double quotes group them.
func shellFields(s string) []string {
	var out []string
	var b strings.Builder
	inWord := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inWord = true
		case r == ' ' || r == '\t':
			if inWord {
				out = append(out, b.String())
				b.Reset()
				inWord = false
			}
		default:
			b.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		out = append(out, b.String())
	}
	return out
}

// fakeHelp is a tiny CLI: stc with serve (flags --opcua, --io) and the
// group vendor with import, and stc-mcp with -project and -io.
func fakeHelp(binary string, path []string) (string, error) {
	key := binary + " " + strings.Join(path, " ")
	switch strings.TrimSpace(key) {
	case "stc":
		return "Usage:\n  stc [command]\n\nAvailable Commands:\n  serve       Run\n  vendor      Vendor tools\n\nFlags:\n  -f, --format string   Output format\n", nil
	case "stc serve":
		return "Usage:\n  stc serve <project> [flags]\n\nFlags:\n      --io strings   exports\n      --opcua string   addr\n  -D, --define strings  defs\n\nGlobal Flags:\n  -f, --format string   Output format\n", nil
	case "stc vendor":
		return "Usage:\n  stc vendor [command]\n\nAvailable Commands:\n  import      Import\n\nFlags:\n  -h, --help   help\n", nil
	case "stc vendor import":
		return "Usage:\n  stc vendor import <x.tsproj> [flags]\n\nFlags:\n      --out string   dir\n\nGlobal Flags:\n  -f, --format string   Output format\n", nil
	case "stc-mcp":
		return "Usage of stc-mcp:\n  -io value\n    \texports\n  -project string\n    \tproject\n", nil
	}
	return "", nil
}

func TestDocsCLICheckerUnit(t *testing.T) {
	doc := strings.Join([]string{
		"# Title", // 1
		"```bash", // 2
		"$ stc serve x.tsproj --opcua :4840 --foo",                // 3
		"stc vendor init beckhoff",                                // 4
		"stc vendor import x.tsproj --out d -f json",              // 5
		"stc-mcp --project p --io \"a b.xml\" --bogus",            // 6
		"stc serve \"a b\" [--io X | --define Y] | grep x --nope", // 7
		"```",                       // 8
		"```text",                   // 9
		"stc serve --notchecked",    // 10
		"```",                       // 11
		"```bash pending-phase-27",  // 12
		"stc serve --scenario x",    // 13
		"```",                       // 14
		"stc serve --outside-fence", // 15
		"```iec",                    // 16
		"stc serve --iec-block",     // 17
		"```",                       // 18
		"```",                       // 19
		"stc nosuch --x",            // 20
		"```",                       // 21
	}, "\n")
	got := checkDocLines("docs/X.md", doc, fakeHelp)
	want := []string{
		"docs/X.md:3: stc serve has no flag --foo",
		"docs/X.md:4: stc vendor has no subcommand init",
		"docs/X.md:6: stc-mcp has no flag --bogus",
		"docs/X.md:20: stc has no subcommand nosuch",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDocsCLIInventedFlagFails(t *testing.T) {
	doc := "```bash\nstc serve app.st --invented-flag\n```\n"
	got := checkDocLines("docs/OPCUA.md", doc, fakeHelp)
	if len(got) != 1 || got[0] != "docs/OPCUA.md:2: stc serve has no flag --invented-flag" {
		t.Fatalf("got %q", got)
	}
}

var (
	mcpBinOnce sync.Once
	mcpBinPath string
	mcpBinErr  error
)

// stcMCPBinary builds cmd/stc-mcp once, next to the stc binary.
func stcMCPBinary(t *testing.T) string {
	t.Helper()
	stc := stcBinary(t)
	mcpBinOnce.Do(func() {
		name := "stc-mcp"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		mcpBinPath = filepath.Join(filepath.Dir(stc), name)
		out, err := exec.Command("go", "build", "-o", mcpBinPath, "../cmd/stc-mcp").CombinedOutput()
		if err != nil {
			mcpBinErr = fmt.Errorf("go build stc-mcp: %v\n%s", err, out)
		}
	})
	if mcpBinErr != nil {
		t.Fatal(mcpBinErr)
	}
	return mcpBinPath
}

// TestDocsCLI fails when a doc names an stc or stc-mcp subcommand or flag
// that the CLI does not have.
func TestDocsCLI(t *testing.T) {
	bins := map[string]string{"stc": stcBinary(t), "stc-mcp": stcMCPBinary(t)}
	help := func(binary string, path []string) (string, error) {
		args := append(append([]string{}, path...), "--help")
		out, err := exec.Command(bins[binary], args...).CombinedOutput()
		if binary == "stc-mcp" { // Go's flag package exits 0 or 2 on -help
			err = nil
		}
		return string(out), err
	}
	files, err := filepath.Glob("../docs/*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no docs found: %v", err)
	}
	files = append(files, "../README.md", "../stdlib/vendor/beckhoff/ethercat_io.md")
	sort.Strings(files)
	c := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		rel := strings.TrimPrefix(filepath.ToSlash(f), "../")
		for _, msg := range checkDocLines(rel, string(data), help) {
			t.Error(msg)
			c++
		}
	}
	if c > 0 {
		t.Logf("%d doc command line(s) do not match the CLI", c)
	}
}
