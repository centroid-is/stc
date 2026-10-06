package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/centroid-is/stc/pkg/twincat"
)

const uaTimeout = 30 * time.Second

var (
	opcuaGolden   = filepath.Join("opcua_golden")
	st301Fixture  = filepath.Join(opcuaGolden, "st301_shape")
	stcBinOnce    sync.Once
	stcBinPath    string
	stcBinErr     error
	stcBinTempDir string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if stcBinTempDir != "" {
		_ = os.RemoveAll(stcBinTempDir)
	}
	os.Exit(code)
}

// stcBinary builds cmd/stc once per test run.
func stcBinary(t *testing.T) string {
	t.Helper()
	stcBinOnce.Do(func() {
		stcBinTempDir, stcBinErr = os.MkdirTemp("", "stc-tests-bin")
		if stcBinErr != nil {
			return
		}
		name := "stc"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		stcBinPath = filepath.Join(stcBinTempDir, name)
		out, err := exec.Command("go", "build", "-o", stcBinPath, "../cmd/stc").CombinedOutput()
		if err != nil {
			stcBinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if stcBinErr != nil {
		t.Fatal(stcBinErr)
	}
	return stcBinPath
}

func freeLoopback(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// execServe starts the stc binary's `serve` on a free port and returns
// its endpoint. The process is killed on cleanup.
func execServe(t *testing.T, project string) string {
	t.Helper()
	bin := stcBinary(t)
	var lastErr string
	for attempt := 0; attempt < 3; attempt++ {
		addr := freeLoopback(t)
		cmd := exec.Command(bin, "serve", project, "--opcua", addr, "--run-for", "5m",
			"--pki-dir", t.TempDir(), "--format", "json")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		lines := make(chan string, 1)
		go func() {
			sc := bufio.NewScanner(stdout)
			sc.Buffer(make([]byte, 1<<20), 64<<20)
			if sc.Scan() {
				lines <- sc.Text()
			}
			close(lines)
			_, _ = io.Copy(io.Discard, stdout)
		}()
		select {
		case line, ok := <-lines:
			if ok {
				var info struct {
					Endpoint string `json:"endpoint"`
				}
				if err := json.Unmarshal([]byte(line), &info); err != nil {
					t.Fatalf("serve start-up line %q: %v", line, err)
				}
				t.Cleanup(func() {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				})
				return info.Endpoint
			}
		case <-time.After(uaTimeout):
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		lastErr = stderr.String()
	}
	t.Fatalf("stc serve did not start: %s", lastErr)
	return ""
}

// dialAnonymous connects with SecurityPolicy None and an anonymous identity.
func dialAnonymous(t *testing.T, endpoint string) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), uaTimeout)
	defer cancel()
	c, err := client.Dial(ctx, endpoint, client.WithInsecureSkipVerify(),
		client.WithSecurityPolicyURI(ua.SecurityPolicyURINone, ua.MessageSecurityModeNone))
	if err != nil {
		t.Fatalf("dial %s: %v", endpoint, err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), uaTimeout)
		defer cancel()
		if err := c.Close(ctx); err != nil {
			_ = c.Abort(ctx)
		}
	})
	return c
}

// analyzeSTDir parses and analyses every .st file of dir.
func analyzeSTDir(t *testing.T, dir string) analyzer.AnalysisResult {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.SourceFile
	for _, e := range entries {
		if !strings.EqualFold(filepath.Ext(e.Name()), ".st") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, pipeline.Parse(e.Name(), string(b), map[string]bool{"STC_SIM": true}).File)
	}
	return analyzer.Analyze(files, nil, analyzer.AnalyzeOpts{})
}

// analyzeTwinCAT imports and analyses a .tsproj or .plcproj.
func analyzeTwinCAT(t *testing.T, path string) analyzer.AnalysisResult {
	t.Helper()
	defines := map[string]bool{"STC_SIM": true}
	m, _, err := twincat.Import(path, twincat.Options{Defines: defines})
	if err != nil {
		t.Fatalf("import %s: %v", filepath.Base(path), err)
	}
	return analyzer.AnalyzeProject(m, nil, defines)
}

// serveInProcess serves res over OPC UA on a free loopback port, like
// stc serve without the scan loop, and returns an anonymous client.
func serveInProcess(t *testing.T, res analyzer.AnalysisResult) *client.Client {
	t.Helper()
	rt, err := interp.NewRuntime(res.Files, interp.RuntimeOpts{LibraryFiles: res.LibraryFiles})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := symtree.Build(res)
	if err != nil {
		t.Fatal(err)
	}
	src := bind.NewRuntimeSource(rt)
	space, _ := opcua.Build(bind.Root(tree), src)
	for attempt := 0; ; attempt++ {
		cfg := opcua.DefaultConfig()
		cfg.Endpoint = freeLoopback(t)
		cfg.PKIDir = t.TempDir()
		srv, err := opcua.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := srv.Publish(space, src); err != nil {
			t.Fatal(err)
		}
		if err = srv.Start(); err == nil {
			t.Cleanup(func() { _ = srv.Stop() })
			return dialAnonymous(t, srv.Endpoint())
		}
		_ = srv.Stop()
		if attempt == 2 {
			t.Fatal(err)
		}
	}
}

// uaReader reads values and decodes structs and enums the way the tfc HMI
// does: struct fields by name from the served StructureDefinition, enums
// as name(value) from the served EnumDefinition.
type uaReader struct {
	c     *client.Client
	enums map[string]map[int64]string // DataType id -> names; nil entry = not an enum
	defs  map[string]*ua.StructureDefinition
}

func newUAReader(c *client.Client) *uaReader {
	return &uaReader{c: c, enums: map[string]map[int64]string{}, defs: map[string]*ua.StructureDefinition{}}
}

func (r *uaReader) attr(id ua.NodeID, attr uint32) (ua.DataValue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), uaTimeout)
	defer cancel()
	res, err := r.c.Read(ctx, &ua.ReadRequest{NodesToRead: []ua.ReadValueID{{NodeID: id, AttributeID: attr}}})
	if err != nil {
		return ua.DataValue{}, err
	}
	if len(res.Results) != 1 {
		return ua.DataValue{}, fmt.Errorf("read %v: %d results", id, len(res.Results))
	}
	return res.Results[0], nil
}

// enumNames returns the value names of an Enumeration DataType, or nil.
func (r *uaReader) enumNames(dt ua.NodeID) map[int64]string {
	key := fmt.Sprint(dt)
	if names, ok := r.enums[key]; ok {
		return names
	}
	var names map[int64]string
	if dv, err := r.attr(dt, ua.AttributeIDDataTypeDefinition); err == nil && dv.StatusCode.IsGood() {
		var def *ua.EnumDefinition
		switch d := dv.Value.(type) {
		case ua.EnumDefinition:
			def = &d
		case *ua.EnumDefinition:
			def = d
		}
		if def != nil {
			names = map[int64]string{}
			for _, f := range def.Fields {
				names[f.Value] = f.Name
			}
		}
	}
	r.enums[key] = names
	return names
}

// structDef returns the StructureDefinition of dt and registers a Go type
// built from it for its DefaultEncodingId, so the client decodes the
// ExtensionObject body instead of dropping it.
func (r *uaReader) structDef(dt ua.NodeID) (*ua.StructureDefinition, error) {
	key := fmt.Sprint(dt)
	if d, ok := r.defs[key]; ok {
		return d, nil
	}
	dv, err := r.attr(dt, ua.AttributeIDDataTypeDefinition)
	if err != nil {
		return nil, err
	}
	var def *ua.StructureDefinition
	switch d := dv.Value.(type) {
	case ua.StructureDefinition:
		def = &d
	case *ua.StructureDefinition:
		def = d
	default:
		return nil, fmt.Errorf("DataType %v has no StructureDefinition (%v)", dt, dv.StatusCode)
	}
	r.defs[key] = def
	typ, err := r.goType(def)
	if err != nil {
		return nil, err
	}
	nsDV, err := r.attr(ua.VariableIDServerNamespaceArray, ua.AttributeIDValue)
	if err != nil {
		return nil, err
	}
	nsURIs, _ := nsDV.Value.([]string)
	enc := ua.ToExpandedNodeID(def.DefaultEncodingID, nsURIs)
	if _, ok := ua.FindTypeForBinaryEncodingID(enc); !ok {
		ua.RegisterBinaryEncodingID(typ, enc)
	}
	return def, nil
}

// goType builds a struct type whose field i has the wire type of def.Fields[i].
func (r *uaReader) goType(def *ua.StructureDefinition) (reflect.Type, error) {
	var fields []reflect.StructField
	for i, f := range def.Fields {
		t, err := r.fieldType(f)
		if err != nil {
			return nil, err
		}
		if f.ValueRank >= 1 {
			t = reflect.SliceOf(t)
		}
		fields = append(fields, reflect.StructField{Name: fmt.Sprintf("F%d", i), Type: t})
	}
	return reflect.StructOf(fields), nil
}

var builtinGo = map[ua.NodeID]reflect.Type{
	ua.DataTypeIDBoolean: reflect.TypeOf(false), ua.DataTypeIDSByte: reflect.TypeOf(int8(0)),
	ua.DataTypeIDByte: reflect.TypeOf(uint8(0)), ua.DataTypeIDInt16: reflect.TypeOf(int16(0)),
	ua.DataTypeIDUInt16: reflect.TypeOf(uint16(0)), ua.DataTypeIDInt32: reflect.TypeOf(int32(0)),
	ua.DataTypeIDUInt32: reflect.TypeOf(uint32(0)), ua.DataTypeIDInt64: reflect.TypeOf(int64(0)),
	ua.DataTypeIDUInt64: reflect.TypeOf(uint64(0)), ua.DataTypeIDFloat: reflect.TypeOf(float32(0)),
	ua.DataTypeIDDouble: reflect.TypeOf(float64(0)), ua.DataTypeIDString: reflect.TypeOf(""),
	ua.DataTypeIDDateTime: reflect.TypeOf(time.Time{}), ua.DataTypeIDByteString: reflect.TypeOf(ua.ByteString("")),
	ua.DataTypeIDEnumeration: reflect.TypeOf(int32(0)),
}

func (r *uaReader) fieldType(f ua.StructureField) (reflect.Type, error) {
	b, err := r.builtin(f.DataType)
	if err != nil {
		return nil, err
	}
	if b == ua.DataTypeIDStructure {
		def, err := r.structDef(f.DataType)
		if err != nil {
			return nil, err
		}
		return r.goType(def)
	}
	if t, ok := builtinGo[b]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("field %s: builtin %v not handled", f.Name, b)
}

// builtin resolves dt to its ns=0 builtin, Enumeration or Structure by
// walking inverse HasSubtype references.
func (r *uaReader) builtin(dt ua.NodeID) (ua.NodeID, error) {
	for i := 0; i < 16; i++ {
		if n, ok := dt.(ua.NodeIDNumeric); ok && n.NamespaceIndex == 0 &&
			(n.ID <= 25 || dt == ua.DataTypeIDEnumeration || dt == ua.DataTypeIDStructure) {
			return dt, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), uaTimeout)
		res, err := r.c.Browse(ctx, &ua.BrowseRequest{NodesToBrowse: []ua.BrowseDescription{{
			NodeID: dt, BrowseDirection: ua.BrowseDirectionInverse, ReferenceTypeID: ua.ReferenceTypeIDHasSubtype,
			IncludeSubtypes: true, ResultMask: uint32(ua.BrowseResultMaskAll),
		}}})
		cancel()
		if err != nil || len(res.Results) != 1 || len(res.Results[0].References) == 0 {
			return nil, fmt.Errorf("no supertype for %v: %v", dt, err)
		}
		dt = ua.ToNodeID(res.Results[0].References[0].NodeID, nil)
	}
	return nil, fmt.Errorf("supertype chain of %v too deep", dt)
}

// read reads id's Value and renders it as the HMI would: structs as
// {field: value} by served names, enums as name(value).
func (r *uaReader) read(id ua.NodeID) (any, ua.StatusCode, error) {
	dtv, err := r.attr(id, ua.AttributeIDDataType)
	if err != nil {
		return nil, 0, err
	}
	dt, _ := dtv.Value.(ua.NodeID)
	var def *ua.StructureDefinition
	if dt != nil {
		if b, err := r.builtin(dt); err == nil && b == ua.DataTypeIDStructure {
			if def, err = r.structDef(dt); err != nil {
				return nil, 0, err
			}
		}
	}
	dv, err := r.attr(id, ua.AttributeIDValue)
	if err != nil {
		return nil, 0, err
	}
	if !dv.StatusCode.IsGood() {
		return nil, dv.StatusCode, nil
	}
	if def != nil {
		v, err := r.render(reflect.ValueOf(dv.Value), def)
		return v, dv.StatusCode, err
	}
	if dt != nil {
		if names := r.enumNames(dt); names != nil {
			return enumText(names, reflect.ValueOf(dv.Value)), dv.StatusCode, nil
		}
	}
	return dv.Value, dv.StatusCode, nil
}

func (r *uaReader) render(v reflect.Value, def *ua.StructureDefinition) (map[string]any, error) {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct || v.NumField() != len(def.Fields) {
		return nil, fmt.Errorf("value %v does not match its StructureDefinition", v)
	}
	out := map[string]any{}
	for i, f := range def.Fields {
		fv := v.Field(i)
		if names := r.enumNames(f.DataType); names != nil && f.ValueRank < 1 {
			out[f.Name] = enumText(names, fv)
			continue
		}
		if b, err := r.builtin(f.DataType); err == nil && b == ua.DataTypeIDStructure && f.ValueRank < 1 {
			nd, err := r.structDef(f.DataType)
			if err != nil {
				return nil, err
			}
			if out[f.Name], err = r.render(fv, nd); err != nil {
				return nil, err
			}
			continue
		}
		out[f.Name] = fv.Interface()
	}
	return out, nil
}

func enumText(names map[int64]string, v reflect.Value) string {
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	var n int64
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = int64(v.Uint())
	default:
		return fmt.Sprint(v.Interface())
	}
	if name, ok := names[n]; ok {
		return fmt.Sprintf("%s(%d)", name, n)
	}
	return fmt.Sprint(n)
}
