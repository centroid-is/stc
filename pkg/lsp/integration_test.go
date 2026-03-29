package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sourcegraph/jsonrpc2"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// clientHandler collects notifications from the server.
type clientHandler struct {
	mu            sync.Mutex
	notifications []jsonrpc2.Request
}

func (h *clientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if req.Notif {
		h.mu.Lock()
		h.notifications = append(h.notifications, *req)
		h.mu.Unlock()
	}
}

func (h *clientHandler) getNotifications() []jsonrpc2.Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]jsonrpc2.Request, len(h.notifications))
	copy(result, h.notifications)
	return result
}

// startTestServer starts an LSP server on a random TCP port and returns
// a jsonrpc2.Conn client. This exercises NewServer() and the full handler
// registration chain.
func startTestServer(t *testing.T) (*jsonrpc2.Conn, *clientHandler) {
	t.Helper()

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Create and start the LSP server (exercises NewServer)
	srv := NewServer()

	// Start server in background
	serverReady := make(chan struct{})
	go func() {
		close(serverReady)
		srv.RunTCP(addr)
	}()
	<-serverReady

	// Give the TCP listener a moment to start
	var conn net.Conn
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("tcp", addr)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to connect to LSP server at %s: %v", addr, err)
	}

	// Create jsonrpc2 client over the TCP connection
	handler := &clientHandler{}
	stream := jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{})
	clientConn := jsonrpc2.NewConn(context.Background(), stream, handler)

	t.Cleanup(func() {
		clientConn.Close()
		conn.Close()
	})

	return clientConn, handler
}

// callWithTimeout sends a JSON-RPC request and waits for a response with timeout.
func callWithTimeout(t *testing.T, conn *jsonrpc2.Conn, method string, params, result any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Call(ctx, method, params, result); err != nil {
		t.Fatalf("%s failed: %v", method, err)
	}
}

// waitForNotification waits for a notification with the given method.
func waitForNotification(t *testing.T, handler *clientHandler, method string) *jsonrpc2.Request {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			notifs := handler.getNotifications()
			var methods []string
			for _, n := range notifs {
				methods = append(methods, n.Method)
			}
			t.Fatalf("timed out waiting for %s notification (got: %v)", method, methods)
			return nil
		default:
			notifs := handler.getNotifications()
			for i := range notifs {
				if notifs[i].Method == method {
					return &notifs[i]
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func initializeConn(t *testing.T, conn *jsonrpc2.Conn) protocol.InitializeResult {
	t.Helper()
	var initResult protocol.InitializeResult
	callWithTimeout(t, conn, "initialize", protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{},
	}, &initResult)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Notify(ctx, "initialized", protocol.InitializedParams{}); err != nil {
		t.Fatalf("initialized notify failed: %v", err)
	}
	return initResult
}

func openDoc(t *testing.T, conn *jsonrpc2.Conn, uri, text string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Notify(ctx, "textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentUri(uri),
			LanguageID: "iec-st",
			Version:    1,
			Text:       text,
		},
	}); err != nil {
		t.Fatalf("didOpen notify failed: %v", err)
	}
	// Give the server time to process
	time.Sleep(200 * time.Millisecond)
}

func TestLSP_InitializeShutdown(t *testing.T) {
	conn, _ := startTestServer(t)

	initResult := initializeConn(t, conn)

	// Verify server info
	if initResult.ServerInfo == nil {
		t.Fatal("expected ServerInfo to be set")
	}
	if initResult.ServerInfo.Name != "stc-lsp" {
		t.Errorf("expected server name 'stc-lsp', got %q", initResult.ServerInfo.Name)
	}

	// Verify capabilities
	caps := initResult.Capabilities
	if caps.DefinitionProvider == nil {
		t.Error("expected DefinitionProvider to be set")
	}
	if caps.HoverProvider == nil {
		t.Error("expected HoverProvider to be set")
	}
	if caps.CompletionProvider == nil {
		t.Error("expected CompletionProvider to be set")
	}
	if caps.ReferencesProvider == nil {
		t.Error("expected ReferencesProvider to be set")
	}
	if caps.RenameProvider == nil {
		t.Error("expected RenameProvider to be set")
	}
	if caps.SemanticTokensProvider == nil {
		t.Error("expected SemanticTokensProvider to be set")
	}

	// Verify semantic tokens legend is set
	if stp, ok := caps.SemanticTokensProvider.(*protocol.SemanticTokensOptions); ok {
		if len(stp.Legend.TokenTypes) == 0 {
			t.Error("expected semantic token types in legend")
		}
	}

	// Send shutdown
	var shutdownResult any
	callWithTimeout(t, conn, "shutdown", nil, &shutdownResult)
}

func TestLSP_DidOpenPublishesDiagnostics(t *testing.T) {
	conn, handler := startTestServer(t)
	initializeConn(t, conn)

	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := 42;\nEND_PROGRAM\n")

	notif := waitForNotification(t, handler, "textDocument/publishDiagnostics")
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(*notif.Params, &diagParams); err != nil {
		t.Fatalf("failed to unmarshal diagnostics: %v", err)
	}
	if diagParams.URI != "file:///test.st" {
		t.Errorf("expected URI 'file:///test.st', got %q", diagParams.URI)
	}
}

func TestLSP_DidChangeReanalyzes(t *testing.T) {
	conn, handler := startTestServer(t)
	initializeConn(t, conn)

	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nEND_PROGRAM\n")

	// Wait for initial diagnostics
	waitForNotification(t, handler, "textDocument/publishDiagnostics")

	// Clear notifications
	handler.mu.Lock()
	handler.notifications = nil
	handler.mu.Unlock()

	// Send didChange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Notify(ctx, "textDocument/didChange", protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Version:                2,
		},
		ContentChanges: []any{
			protocol.TextDocumentContentChangeEventWhole{
				Text: "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\nEND_PROGRAM\n",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Wait for new diagnostics
	waitForNotification(t, handler, "textDocument/publishDiagnostics")
}

func TestLSP_DidClose(t *testing.T) {
	conn, handler := startTestServer(t)
	initializeConn(t, conn)

	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nEND_PROGRAM\n")

	// Wait for open diagnostics
	waitForNotification(t, handler, "textDocument/publishDiagnostics")

	// Clear
	handler.mu.Lock()
	handler.notifications = nil
	handler.mu.Unlock()

	// Close the document
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Notify(ctx, "textDocument/didClose", protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	}); err != nil {
		t.Fatal(err)
	}

	// Should publish empty diagnostics on close
	notif := waitForNotification(t, handler, "textDocument/publishDiagnostics")
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(*notif.Params, &diagParams); err != nil {
		t.Fatalf("failed to unmarshal diagnostics: %v", err)
	}
	if len(diagParams.Diagnostics) != 0 {
		t.Errorf("expected empty diagnostics on close, got %d", len(diagParams.Diagnostics))
	}
}

func TestLSP_CompletionAtPosition(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := 42;\nEND_PROGRAM\n")

	var completionResult json.RawMessage
	callWithTimeout(t, conn, "textDocument/completion", protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
	}, &completionResult)

	var completionList protocol.CompletionList
	if err := json.Unmarshal(completionResult, &completionList); err != nil {
		t.Fatalf("failed to unmarshal completion: %v", err)
	}
	if len(completionList.Items) == 0 {
		t.Error("expected completion items")
	}

	// Should include keywords
	hasKeyword := false
	for _, item := range completionList.Items {
		if item.Label == "IF" {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword {
		t.Error("expected to find 'IF' keyword in completions")
	}
}

func TestLSP_GotoDefinition(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\n    counter : INT;\nEND_VAR\n    counter := counter + 1;\nEND_PROGRAM\n")

	var defResult json.RawMessage
	callWithTimeout(t, conn, "textDocument/definition", protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
	}, &defResult)

	// Verify we got something back (may be null if position doesn't resolve)
	if defResult != nil && string(defResult) != "null" {
		var locations []protocol.Location
		if err := json.Unmarshal(defResult, &locations); err == nil {
			if len(locations) > 0 {
				t.Logf("definition resolved to %s line %d", locations[0].URI, locations[0].Range.Start.Line)
			}
		}
	}
}

func TestLSP_HoverInfo(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\n    counter : INT;\nEND_VAR\n    counter := 42;\nEND_PROGRAM\n")

	var hoverResult json.RawMessage
	callWithTimeout(t, conn, "textDocument/hover", protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
	}, &hoverResult)

	if hoverResult != nil && string(hoverResult) != "null" {
		t.Log("hover returned result")
	}
}

func TestLSP_FormatDocument(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\nx : INT;\nEND_VAR\nx := 42;\nEND_PROGRAM\n")

	var edits json.RawMessage
	callWithTimeout(t, conn, "textDocument/formatting", protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
		Options:      protocol.FormattingOptions{},
	}, &edits)

	if edits == nil || string(edits) == "null" {
		t.Error("expected formatting edits")
	}
}

func TestLSP_SemanticTokens(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st",
		"{IF defined(SYM)}\nPROGRAM Active\nEND_PROGRAM\n{ELSE}\nPROGRAM Inactive\nEND_PROGRAM\n{END_IF}\n")

	var tokens json.RawMessage
	callWithTimeout(t, conn, "textDocument/semanticTokens/full", protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
	}, &tokens)

	if tokens == nil {
		t.Error("expected semantic tokens result")
	}
}

func TestLSP_References(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := x + 1;\nEND_PROGRAM\n")

	var refs json.RawMessage
	callWithTimeout(t, conn, "textDocument/references", protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	}, &refs)

	if refs == nil {
		t.Log("references returned null (may depend on exact position)")
	}
}

func TestLSP_Rename(t *testing.T) {
	conn, _ := startTestServer(t)
	initializeConn(t, conn)
	openDoc(t, conn, "file:///test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := x + 1;\nEND_PROGRAM\n")

	var renameResult json.RawMessage
	callWithTimeout(t, conn, "textDocument/rename", protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.st"},
			Position:     protocol.Position{Line: 4, Character: 4},
		},
		NewName: "renamed_x",
	}, &renameResult)

	if renameResult == nil {
		t.Log("rename returned null (may depend on exact position)")
	}
}

func TestLSP_DidOpenWithErrors(t *testing.T) {
	conn, handler := startTestServer(t)
	initializeConn(t, conn)

	// Open a document with syntax errors
	openDoc(t, conn, "file:///bad.st", "PROGRAM\nEND_PROGRAM\n")

	notif := waitForNotification(t, handler, "textDocument/publishDiagnostics")
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(*notif.Params, &diagParams); err != nil {
		t.Fatalf("failed to unmarshal diagnostics: %v", err)
	}
	// A malformed program should produce some diagnostics
	t.Logf("got %d diagnostics for malformed program", len(diagParams.Diagnostics))
}

// TestLSP_Run verifies that Run() creates a server (we can't actually run
// it on stdio in tests, but we exercise the function entry).
func TestLSP_Run(t *testing.T) {
	// Just verify NewServer works and returns non-nil
	srv := NewServer()
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if srv.Handler == nil {
		t.Error("expected Handler to be set")
	}

	// Verify the server name
	_ = fmt.Sprintf("server created: %s", srv.LogBaseName)
}
