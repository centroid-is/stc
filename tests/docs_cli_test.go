package tests

import (
	"strings"
	"testing"
)

// helpFunc returns the --help output of binary ("stc" or "stc-mcp") for the
// subcommand path.
type helpFunc func(binary string, path []string) (string, error)

// checkDocLines checks every stc and stc-mcp command line in the fenced
// shell blocks of content against the CLI's --help output.
func checkDocLines(file, content string, help helpFunc) []string {
	return nil
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
		return "Usage:\n  stc vendor import <x.tsproj> [flags]\n\nFlags:\n      --out string   dir\n", nil
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
