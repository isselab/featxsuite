// Package server wires the ftx analyser to the Language Server Protocol.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
	"github.com/isselab/featxsuite/featx-lsp/internal/jsonrpc"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

// Name and Version identify the server to the client.
const (
	Name    = "featx-lsp"
	Version = "0.1.0"
)

// ErrExit is returned by Run when the client sends the exit notification.
var ErrExit = errors.New("server: exit")

// Server handles one client connection. Messages are dispatched one at a time
// on the goroutine that calls Run, so the document store needs no locking.
type Server struct {
	conn *jsonrpc.Conn
	log  *log.Logger

	docs map[string]string // uri -> full text

	initialized  bool
	shutdownSeen bool
}

// New returns a server reading and writing framed messages on conn. Logs go to
// logw, which must not be the same stream as the protocol output.
func New(conn *jsonrpc.Conn, logw io.Writer) *Server {
	return &Server{
		conn: conn,
		log:  log.New(logw, Name+": ", log.LstdFlags),
		docs: make(map[string]string),
	}
}

// Run serves until the client closes the stream or sends exit. A clean
// shutdown returns ErrExit; a closed stream returns nil.
func (s *Server) Run() error {
	for {
		msg, err := s.conn.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		if err := s.dispatch(msg); err != nil {
			if errors.Is(err, ErrExit) {
				return ErrExit
			}
			// A failed handler must not take the session down; the client is
			// free to carry on sending requests.
			s.log.Printf("handling %q: %v", msg.Method, err)
		}
	}
}

// ShutdownRequested reports whether the client asked to shut down before
// sending exit, which decides the process exit status.
func (s *Server) ShutdownRequested() bool { return s.shutdownSeen }

func (s *Server) dispatch(m *jsonrpc.Message) error {
	switch m.Method {
	case "initialize":
		return s.initialize(m)

	case "initialized":
		s.initialized = true
		return nil

	case "shutdown":
		s.shutdownSeen = true
		return s.conn.Reply(m.ID, nil)

	case "exit":
		return ErrExit

	case "textDocument/didOpen":
		return s.didOpen(m)

	case "textDocument/didChange":
		return s.didChange(m)

	case "textDocument/didClose":
		return s.didClose(m)

	case "textDocument/didSave":
		// The text is already current from didChange; re-publishing keeps
		// cross-file diagnostics fresh when a sibling model is saved.
		return s.didSave(m)

	case "textDocument/hover":
		return s.hover(m)

	case "textDocument/completion":
		return s.completion(m)

	case "textDocument/documentSymbol":
		return s.documentSymbol(m)

	case "textDocument/formatting":
		return s.formatting(m)

	case "textDocument/definition":
		return s.definition(m)

	default:
		if m.IsNotification() {
			return nil // unknown notifications, including $/ ones, are ignored
		}
		return s.conn.ReplyError(m.ID, jsonrpc.CodeMethodNotFound,
			"unsupported method %q", m.Method)
	}
}

func (s *Server) initialize(m *jsonrpc.Message) error {
	result := lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync:           lsp.SyncFull,
			HoverProvider:              true,
			CompletionProvider:         &lsp.CompletionOptions{TriggerCharacters: []string{":"}},
			DocumentSymbolProvider:     true,
			DocumentFormattingProvider: true,
			DefinitionProvider:         true,
		},
		ServerInfo: &lsp.ServerInfo{Name: Name, Version: Version},
	}

	return s.conn.Reply(m.ID, result)
}

func (s *Server) didOpen(m *jsonrpc.Message) error {
	var p lsp.DidOpenTextDocumentParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	s.docs[p.TextDocument.URI] = p.TextDocument.Text
	return s.publish(p.TextDocument.URI)
}

func (s *Server) didChange(m *jsonrpc.Message) error {
	var p lsp.DidChangeTextDocumentParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	// Full sync: the last change carries the whole document.
	if n := len(p.ContentChanges); n > 0 {
		s.docs[p.TextDocument.URI] = p.ContentChanges[n-1].Text
	}

	return s.publish(p.TextDocument.URI)
}

func (s *Server) didClose(m *jsonrpc.Message) error {
	var p lsp.DidCloseTextDocumentParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	delete(s.docs, p.TextDocument.URI)

	// Clear the squiggles for a file we no longer track.
	return s.conn.Notify("textDocument/publishDiagnostics", lsp.PublishDiagnosticsParams{
		URI:         p.TextDocument.URI,
		Diagnostics: []lsp.Diagnostic{},
	})
}

func (s *Server) didSave(m *jsonrpc.Message) error {
	var p lsp.DidSaveTextDocumentParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	if p.Text != "" {
		s.docs[p.TextDocument.URI] = p.Text
	}

	// A saved model changes the verdict on every open configuration, so
	// refresh them all rather than just this document.
	for uri := range s.docs {
		if err := s.publish(uri); err != nil {
			return err
		}
	}

	return nil
}

// text returns the tracked contents of a document.
func (s *Server) text(uri string) (string, bool) {
	src, ok := s.docs[uri]
	return src, ok
}

// parse returns the parsed form of a tracked document.
func (s *Server) parse(uri string) (*ftx.File, bool) {
	src, ok := s.text(uri)
	if !ok {
		return nil, false
	}
	return ftx.Parse(src), true
}

func unmarshal(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return fmt.Errorf("missing params")
	}
	return json.Unmarshal(raw, v)
}

// uriToPath converts a file:// URI to a local path.
func uriToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("bad uri %q: %w", uri, err)
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("uri %q is not a file uri", uri)
	}
	return u.Path, nil
}

// pathToURI converts a local path to a file:// URI.
func pathToURI(path string) string {
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}

// lines splits a document into its lines for position arithmetic.
func lines(src string) []string {
	out := strings.Split(src, "\n")
	for i := range out {
		out[i] = strings.TrimRight(out[i], "\r")
	}
	return out
}

// lineAt returns the text of one line, or "" if the index is out of range.
func lineAt(src string, n int) string {
	ls := lines(src)
	if n < 0 || n >= len(ls) {
		return ""
	}
	return ls[n]
}

// byteOffset converts an LSP character offset, counted in UTF-16 code units,
// to a byte index into line.
func byteOffset(line string, char int) int {
	units := 0
	for i, r := range line {
		if units >= char {
			return i
		}
		units++
		if r > 0xFFFF {
			units++
		}
	}
	return len(line)
}

// toRange converts an ftx range to its LSP equivalent.
func toRange(r ftx.Range) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{Line: r.Line, Character: r.StartChar},
		End:   lsp.Position{Line: r.Line, Character: r.EndChar},
	}
}
