package lsp

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

// NewServer creates a configured LSP server with document sync,
// diagnostics publishing, and formatting support.
func NewServer() *server.Server {
	store := NewDocumentStore()
	handler := protocol.Handler{}

	handler.Initialize = func(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
		capabilities := handler.CreateServerCapabilities()

		// Override text document sync to Full mode
		syncKind := protocol.TextDocumentSyncKindFull
		if opts, ok := capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions); ok {
			opts.Change = &syncKind
		}

		// Navigation and refactoring capabilities
		capabilities.DefinitionProvider = true
		capabilities.HoverProvider = true
		capabilities.CompletionProvider = &protocol.CompletionOptions{}
		capabilities.ReferencesProvider = true
		capabilities.RenameProvider = true

		return protocol.InitializeResult{
			Capabilities: capabilities,
			ServerInfo: &protocol.InitializeResultServerInfo{
				Name: "stc-lsp",
			},
		}, nil
	}

	handler.Initialized = func(ctx *glsp.Context, params *protocol.InitializedParams) error {
		return nil
	}

	handler.Shutdown = func(ctx *glsp.Context) error {
		return nil
	}

	handler.TextDocumentDidOpen = func(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
		doc := store.Open(
			params.TextDocument.URI,
			params.TextDocument.Text,
			int32(params.TextDocument.Version),
		)
		publishDiagnostics(ctx, params.TextDocument.URI, doc)
		return nil
	}

	handler.TextDocumentDidChange = func(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
		// We use Full sync, so ContentChanges[0] has the full document text
		var content string
		if len(params.ContentChanges) > 0 {
			switch change := params.ContentChanges[0].(type) {
			case protocol.TextDocumentContentChangeEventWhole:
				content = change.Text
			case protocol.TextDocumentContentChangeEvent:
				content = change.Text
			}
		}

		doc := store.Update(
			params.TextDocument.URI,
			content,
			int32(params.TextDocument.Version),
		)
		publishDiagnostics(ctx, params.TextDocument.URI, doc)
		return nil
	}

	handler.TextDocumentDidClose = func(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
		store.Close(params.TextDocument.URI)
		// Publish empty diagnostics to clear
		ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         params.TextDocument.URI,
			Diagnostics: []protocol.Diagnostic{},
		})
		return nil
	}

	handler.TextDocumentFormatting = handleFormatting(store)
	handler.TextDocumentDefinition = handleDefinition(store)
	handler.TextDocumentHover = handleHover(store)
	handler.TextDocumentCompletion = handleCompletion(store)
	handler.TextDocumentReferences = handleReferences(store)
	handler.TextDocumentRename = handleRename(store)
	handler.TextDocumentSemanticTokensFull = handleSemanticTokensFull(store)

	srv := server.NewServer(&handler, "stc-lsp", false)

	// Override the auto-generated SemanticTokensProvider to include our legend.
	// GLSP's CreateServerCapabilities sets Full=true when the handler is registered
	// but does not populate the legend, so we set it explicitly in the Initialize handler.
	origInit := handler.Initialize
	handler.Initialize = func(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
		result, err := origInit(ctx, params)
		if err != nil {
			return result, err
		}
		if initResult, ok := result.(protocol.InitializeResult); ok {
			initResult.Capabilities.SemanticTokensProvider = &protocol.SemanticTokensOptions{
				Legend: semanticTokensLegend,
				Full:   true,
			}
			return initResult, nil
		}
		return result, nil
	}

	return srv
}

// Run creates and starts the LSP server on stdio.
func Run() error {
	srv := NewServer()
	return srv.RunStdio()
}
