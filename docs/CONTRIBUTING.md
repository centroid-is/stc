# Contributing

Developer guide for contributing to the stc project.

## Prerequisites

- **Go 1.25+** (see `go.mod` for exact version)
- **Git**
- Optional: `golangci-lint` for local linting

## Building and Testing

### Build

```bash
# Build all packages
go build ./...

# Build the stc binary with version info
make build
# Produces: ./stc

# Install to $GOPATH/bin
make install
```

### Test

```bash
# Run all Go tests
make test
# or
go test ./... -count=1

# Run Go tests with race detection and coverage
make test-full
# Produces: coverage.out

# Run ST test suites
go run ./cmd/stc test tests/
# or (if stc is on PATH)
stc test tests/

# Run a specific ST test suite
go run ./cmd/stc test tests/motor_control

# Run with verbose output
go test ./... -v -count=1
```

### Lint

```bash
# Go vet (always works)
go vet ./...

# golangci-lint (if installed)
make lint
```

### Clean

```bash
make clean
```

## Code Organization

### Packages

The project follows a clean separation between CLI commands and library packages:

```
cmd/stc/         CLI commands (one file per subcommand)
cmd/stc-mcp/     MCP server (tool handlers wrap pkg/ functions)
pkg/             Library packages (all compiler logic)
stdlib/          Shipped vendor stubs and behavioral mocks
editors/         VS Code extension
tests/           ST test suites and parse corpus
```

### Package Conventions

- Each `pkg/` package is a self-contained library with a clear public API.
- Internal types are unexported. The public API is documented with Go doc comments.
- Test files are `*_test.go` in the same package (white-box testing).
- Test fixtures live in `testdata/` subdirectories within each package.

### Key Files per Package

| Package | Entry Point | Purpose |
|---------|-------------|---------|
| `pkg/lexer` | `lexer.go` | `Tokenize(filename, source)` |
| `pkg/parser` | `parser.go` | `Parse(filename, source)` |
| `pkg/preprocess` | `preprocess.go` | `Preprocess(source, Options)` |
| `pkg/checker` | `resolve.go`, `checker.go` | Two-pass type checking |
| `pkg/analyzer` | `analyzer.go` | `Analyze(files, config, opts)` |
| `pkg/interp` | `interpreter.go` | `NewInterpreter()`, expression/statement evaluation |
| `pkg/interp` | `scan.go` | `ScanCycleEngine` with deterministic time |
| `pkg/interp` | `fb_instance.go` | `StandardFB` interface, `FBInstance` |
| `pkg/testing` | `runner.go` | `Run(dir)`, `RunWithOpts(dir, opts)` |
| `pkg/sim` | `sim.go` | `SimulationEngine`, `SimConfig` |
| `pkg/emit` | `emit.go` | `Emit(file, Options)` |
| `pkg/format` | `format.go` | `Format(file, FormatOptions)` |
| `pkg/lint` | `lint.go` | `LintFile(file, LintOptions)` |
| `pkg/lsp` | `server.go` | `Run()` starts LSP on stdio |
| `pkg/project` | `config.go` | `LoadConfig(path)`, `FindConfig(dir)` |
| `pkg/vendor` | `loader.go` | `LoadLibraries(cfg, dir)` |
| `pkg/vendor` | `mock.go` | `LoadMocks(cfg, dir)` |
| `pkg/vendor` | `extract.go` | `ExtractProject(plcprojPath)` |
| `pkg/incremental` | `incremental.go` | `NewIncrementalAnalyzer(cacheDir)` |

## Adding Features Checklist

### New ST Language Feature

- [ ] Add token type(s) to `pkg/lexer/` if needed
- [ ] Add AST node type(s) to `pkg/ast/` with JSON marshaling
- [ ] Add parser rule(s) to `pkg/parser/`
- [ ] Add type-checking logic to `pkg/checker/`
- [ ] Add interpreter evaluation to `pkg/interp/`
- [ ] Add emitter output to `pkg/emit/`
- [ ] Add formatter handling to `pkg/format/`
- [ ] Add unit tests at each level
- [ ] Add integration tests in `tests/` (ST test files or parse corpus entries)

### New CLI Command

- [ ] Create `cmd/stc/<command>.go` with `newXxxCmd() *cobra.Command`
- [ ] Register in `cmd/stc/main.go`
- [ ] Support `--format json` output
- [ ] Add `cmd/stc/<command>_test.go` with integration tests
- [ ] Consider adding MCP tool handler in `cmd/stc-mcp/tools.go`
- [ ] Update `docs/CLI_REFERENCE.md`

### New Standard Library FB

- [ ] Create `pkg/interp/stdlib_<name>.go` implementing `StandardFB`
- [ ] Register in `StdlibFBFactory` via `init()`
- [ ] Add type signature to `pkg/types/builtin.go`
- [ ] Add `pkg/interp/stdlib_<name>_test.go`
- [ ] Add ST-level tests in `tests/stdlib_comprehensive/`

### New Vendor Stubs

- [ ] Create `.st` stub file(s) in `stdlib/vendor/<vendor>/`
- [ ] Add parse verification test in `stdlib/vendor/<vendor>/stubs_test.go`
- [ ] Document in `docs/VENDOR_LIBRARIES.md` if adding a new vendor

### New Lint Rule

- [ ] Add `check<RuleName>` function in `pkg/lint/`
- [ ] Call it from `LintFile()` in `pkg/lint/lint.go`
- [ ] Add tests in `pkg/lint/*_test.go`
- [ ] Document the rule code in `docs/CLI_REFERENCE.md`

## PR Workflow

1. **Fork and branch** from `main`.

2. **Write code** following existing patterns and conventions.

3. **Run tests locally**:
   ```bash
   go test ./... -count=1
   go vet ./...
   go run ./cmd/stc test tests/
   ```

4. **Check coverage** for critical packages:
   ```bash
   go test -coverprofile=coverage.out ./pkg/parser/ ./pkg/lexer/ ./pkg/checker/ ./pkg/interp/ ./pkg/types/ ./pkg/emit/
   go tool cover -func=coverage.out | tail -1
   ```

5. **Commit** with a descriptive message following conventional commits:
   - `feat: add WHILE loop support to interpreter`
   - `fix: correct type widening for SINT to INT`
   - `docs: update CLI reference for new sim flags`
   - `test: add edge cases for nested IF statements`
   - `refactor: simplify checker pass 2 scope handling`

6. **Open PR** against `main` on `centroid-is/stc`.

7. **CI checks** must pass:
   - Build and test on Linux, macOS, Windows
   - Go vet
   - Coverage thresholds
   - ST test suites

## Coverage Requirements

Coverage thresholds are enforced by CI via `.testcoverage.yml`:

| Scope | Threshold |
|-------|-----------|
| Overall project | 85% |
| `pkg/parser` | 95% |
| `pkg/lexer` | 95% |
| `pkg/checker` | 94% |
| `pkg/interp` | 95% |
| `pkg/types` | 95% |
| `pkg/emit` | 95% |

Excluded from coverage: `cmd/*/main.go`, `pkg/version/`.

## Style Guidelines

- Go code follows standard `gofmt` formatting.
- Package-level doc comments on every exported type and function.
- Error messages include context: `fmt.Errorf("loading config: %w", err)`.
- Test functions named `TestXxx_descriptive_name`.
- Table-driven tests preferred for repetitive cases.

## Dependencies

Direct dependencies are intentionally minimal:

| Dependency | Purpose |
|------------|---------|
| `github.com/BurntSushi/toml` | TOML config parsing |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/stretchr/testify` | Test assertions (Go tests only) |
| `github.com/tliron/glsp` | LSP protocol implementation |
| `github.com/modelcontextprotocol/go-sdk` | MCP server SDK |

Do not add dependencies without discussion. The project targets a single static binary with minimal external requirements.
