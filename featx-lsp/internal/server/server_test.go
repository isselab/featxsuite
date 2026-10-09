package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isselab/featxsuite/featx-lsp/internal/jsonrpc"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

const modelSrc = `RB-1-BASE:
    xor navigation : node {static early} ?
        to_loading : node {dynamic late}
        to_delivery : node {dynamic late}
    boot : node {static late}
`

// session scripts a client: it frames the given messages, runs the server to
// completion over them, and returns everything the server wrote back.
type session struct {
	in  bytes.Buffer
	out bytes.Buffer
	srv *Server
	id  int
}

func newSession() *session {
	s := &session{}
	s.srv = New(jsonrpc.NewConn(&s.in, &s.out), io.Discard)
	return s
}

// request queues a request and returns the id it was given.
func (s *session) request(t *testing.T, method string, params any) json.RawMessage {
	t.Helper()

	s.id++
	id := json.RawMessage(strings.TrimSpace(string(mustJSON(t, s.id))))

	s.queue(t, &jsonrpc.Message{ID: id, Method: method, Params: mustJSON(t, params)})
	return id
}

func (s *session) notify(t *testing.T, method string, params any) {
	t.Helper()
	s.queue(t, &jsonrpc.Message{Method: method, Params: mustJSON(t, params)})
}

func (s *session) queue(t *testing.T, m *jsonrpc.Message) {
	t.Helper()

	if err := jsonrpc.NewConn(nil, &s.in).Write(m); err != nil {
		t.Fatalf("queueing %q: %v", m.Method, err)
	}
}

// run serves the queued messages and decodes the replies.
func (s *session) run(t *testing.T) []*jsonrpc.Message {
	t.Helper()

	err := s.srv.Run()
	if err != nil && !errors.Is(err, ErrExit) {
		t.Fatalf("Run: %v", err)
	}

	var out []*jsonrpc.Message
	conn := jsonrpc.NewConn(&s.out, io.Discard)

	for {
		m, err := conn.Read()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("decoding server output: %v", err)
		}
		out = append(out, m)
	}
}

// open queues the initialize handshake followed by a didOpen.
func (s *session) open(t *testing.T, uri, text string) {
	t.Helper()

	s.request(t, "initialize", lsp.InitializeParams{})
	s.notify(t, "initialized", struct{}{})
	s.notify(t, "textDocument/didOpen", lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{URI: uri, LanguageID: "featx", Version: 1, Text: text},
	})
}

func TestInitializeAdvertisesCapabilities(t *testing.T) {
	s := newSession()
	s.request(t, "initialize", lsp.InitializeParams{})

	msgs := s.run(t)
	if len(msgs) == 0 {
		t.Fatal("no reply to initialize")
	}

	var result lsp.InitializeResult
	if err := json.Unmarshal(msgs[0].Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	caps := result.Capabilities

	if caps.TextDocumentSync != lsp.SyncFull {
		t.Errorf("TextDocumentSync = %d, want %d", caps.TextDocumentSync, lsp.SyncFull)
	}
	if !caps.HoverProvider {
		t.Error("HoverProvider = false")
	}
	if !caps.DocumentSymbolProvider {
		t.Error("DocumentSymbolProvider = false")
	}
	if !caps.DocumentFormattingProvider {
		t.Error("DocumentFormattingProvider = false")
	}
	if !caps.DefinitionProvider {
		t.Error("DefinitionProvider = false")
	}
	if caps.CompletionProvider == nil {
		t.Fatal("CompletionProvider = nil")
	}
	if result.ServerInfo == nil || result.ServerInfo.Name != Name {
		t.Errorf("ServerInfo = %+v", result.ServerInfo)
	}
}

func TestDidOpenPublishesSyntaxDiagnostics(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, "M:\n    nav : node {static}\n")

	diags := diagnosticsFor(t, s.run(t), uri)

	if !hasMessage(diags, `feature "nav" has no binding time (early|late)`) {
		t.Errorf("missing diagnostic; got %v", messagesOf(diags))
	}
}

func TestDidChangeRepublishes(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, "M:\n    nav : node {static}\n")
	s.notify(t, "textDocument/didChange", lsp.DidChangeTextDocumentParams{
		TextDocument: lsp.VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []lsp.TextDocumentContentChangeEvent{
			{Text: "M:\n    nav: node {static early}\n"},
		},
	})

	msgs := s.run(t)

	// The last publish for this document should be clean.
	diags := diagnosticsFor(t, msgs, uri)
	for _, d := range diags {
		if d.Severity == lsp.SeverityError {
			t.Errorf("unexpected error after the fix: %s", d.Message)
		}
	}
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, "M:\n    nav : node {static}\n")
	s.notify(t, "textDocument/didClose", lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	if diags := diagnosticsFor(t, s.run(t), uri); len(diags) != 0 {
		t.Errorf("diagnostics were not cleared: %v", messagesOf(diags))
	}
}

// TestCrossFileDiagnostics checks the whole path: a configuration opened in a
// directory that also holds a model is validated against that model, with the
// model read from disk.
func TestCrossFileDiagnostics(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "features.ftx"), []byte(modelSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(dir, "configs.ftx")
	configSrc := "" +
		"    navigation *{dynamic early}\n" +
		"    to_loading *{dynamic late}\n" +
		"    to_delivery *{dynamic late}\n" +
		"    nonsense {static late}\n"

	if err := os.WriteFile(configPath, []byte(configSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	uri := pathToURI(configPath)

	s := newSession()
	s.open(t, uri, configSrc)

	diags := diagnosticsFor(t, s.run(t), uri)

	for _, want := range []string{
		`feature "navigation" is configured "dynamic" but the model allows only "static"`,
		`feature "nonsense" is not declared in feature model "RB-1-BASE"`,
		`"navigation" is an xor group, so only one of its children may be selected; ` +
			`"to_loading", "to_delivery" are all selected`,
	} {
		if !hasMessage(diags, want) {
			t.Errorf("missing diagnostic %q; got %v", want, messagesOf(diags))
		}
	}
}

func TestHoverShowsModelDeclaration(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "features.ftx"), []byte(modelSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(dir, "configs.ftx")
	configSrc := "navigation *{static early}\n"
	uri := pathToURI(configPath)

	s := newSession()
	s.open(t, uri, configSrc)

	// Line 0, character 6 is inside "navigation".
	id := s.request(t, "textDocument/hover", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: 0, Character: 6},
	})

	var hover lsp.Hover
	decodeResult(t, s.run(t), id, &hover)

	for _, want := range []string{"navigation", "selected", "RB-1-BASE", "allowed binding mode"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Errorf("hover does not mention %q:\n%s", want, hover.Contents.Value)
		}
	}
}

func TestDefinitionJumpsToTheModel(t *testing.T) {
	dir := t.TempDir()

	modelPath := filepath.Join(dir, "features.ftx")
	if err := os.WriteFile(modelPath, []byte(modelSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(dir, "configs.ftx")
	uri := pathToURI(configPath)

	s := newSession()
	s.open(t, uri, "to_loading {dynamic late}\n")

	id := s.request(t, "textDocument/definition", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: 0, Character: 6},
	})

	var loc lsp.Location
	decodeResult(t, s.run(t), id, &loc)

	if loc.URI != pathToURI(modelPath) {
		t.Errorf("URI = %q, want %q", loc.URI, pathToURI(modelPath))
	}

	// to_loading is declared on the third line of the model.
	if loc.Range.Start.Line != 2 {
		t.Errorf("Start.Line = %d, want 2", loc.Range.Start.Line)
	}
}

func TestDocumentSymbolNests(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, modelSrc)

	id := s.request(t, "textDocument/documentSymbol", lsp.DocumentSymbolParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	var syms []lsp.DocumentSymbol
	decodeResult(t, s.run(t), id, &syms)

	// The outline mirrors the document: the root feature, then its children.
	if len(syms) != 1 {
		t.Fatalf("len(syms) = %d, want 1 (the root feature)", len(syms))
	}
	if syms[0].Name != "RB-1-BASE" {
		t.Errorf("syms[0].Name = %q, want the root feature", syms[0].Name)
	}
	if len(syms[0].Children) != 2 {
		t.Fatalf("root has %d children, want 2 (navigation, boot)", len(syms[0].Children))
	}

	nav := syms[0].Children[0]
	if nav.Name != "navigation" {
		t.Errorf("syms[0].Name = %q", nav.Name)
	}
	if len(nav.Children) != 2 {
		t.Errorf("navigation has %d children, want 2", len(nav.Children))
	}
	if !strings.Contains(nav.Detail, "xor") {
		t.Errorf("Detail = %q, want it to mention xor", nav.Detail)
	}

	// The symbol range must cover the subtree so that folding works.
	if nav.Range.End.Line < 3 {
		t.Errorf("navigation range ends at line %d, want it to span its children", nav.Range.End.Line)
	}
}

func TestFormattingReturnsAnEdit(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, "M:\n  nav   :  node{static   early}\n")

	id := s.request(t, "textDocument/formatting", lsp.DocumentFormattingParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	var edits []lsp.TextEdit
	decodeResult(t, s.run(t), id, &edits)

	if len(edits) != 1 {
		t.Fatalf("len(edits) = %d, want 1", len(edits))
	}
	if want := "M:\n    nav : node {static early}\n"; edits[0].NewText != want {
		t.Errorf("NewText = %q, want %q", edits[0].NewText, want)
	}
}

func TestFormattingRefusesBrokenDocument(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, "M:\n    nav : node {static}\n")

	id := s.request(t, "textDocument/formatting", lsp.DocumentFormattingParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})

	var edits []lsp.TextEdit
	decodeResult(t, s.run(t), id, &edits)

	if len(edits) != 0 {
		t.Errorf("len(edits) = %d, want 0 for a document with syntax errors", len(edits))
	}
}

func TestCompletionOffersTypesAfterColon(t *testing.T) {
	uri := "file:///tmp/features.ftx"
	src := "M:\n    nav : \n"

	s := newSession()
	s.open(t, uri, src)

	id := s.request(t, "textDocument/completion", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: 1, Character: len("    nav : ")},
	})

	var list lsp.CompletionList
	decodeResult(t, s.run(t), id, &list)

	labels := labelsOf(list.Items)

	for _, want := range []string{"node", "number", "string", "bool"} {
		if !contains(labels, want) {
			t.Errorf("labels = %v, want %q", labels, want)
		}
	}
	if contains(labels, "static") {
		t.Errorf("labels = %v: a binding word is not legal before the braces", labels)
	}
}

func TestCompletionOffersBindingsInsideBraces(t *testing.T) {
	uri := "file:///tmp/features.ftx"
	src := "M:\n    nav : node {static \n"

	s := newSession()
	s.open(t, uri, src)

	// Inside the braces, where a binding time is still missing.
	id := s.request(t, "textDocument/completion", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: 1, Character: len("    nav : node {static ")},
	})

	var list lsp.CompletionList
	decodeResult(t, s.run(t), id, &list)

	labels := labelsOf(list.Items)

	if !contains(labels, "early") || !contains(labels, "late") {
		t.Errorf("labels = %v, want early and late", labels)
	}
	if contains(labels, "static") || contains(labels, "dynamic") {
		t.Errorf("labels = %v, should not re-offer a mode that is already set", labels)
	}
	if contains(labels, "node") {
		t.Errorf("labels = %v: a type is not legal inside the braces", labels)
	}
}

func TestCompletionOffersFeatureNamesInsideAConstraint(t *testing.T) {
	uri := "file:///tmp/features.ftx"
	src := modelSrc + "    [\n"

	s := newSession()
	s.open(t, uri, src)

	line := strings.Count(modelSrc, "\n")

	id := s.request(t, "textDocument/completion", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: line, Character: len("    [")},
	})

	var list lsp.CompletionList
	decodeResult(t, s.run(t), id, &list)

	labels := labelsOf(list.Items)

	if !contains(labels, "to_loading") || !contains(labels, "boot") {
		t.Errorf("labels = %v, want the model's own feature names", labels)
	}
	if contains(labels, "node") {
		t.Errorf("labels = %v: a type is not a constraint operand", labels)
	}
}

func TestCompletionOffersGroupingAtLineStart(t *testing.T) {
	uri := "file:///tmp/features.ftx"

	s := newSession()
	s.open(t, uri, "M:\n    \n")

	id := s.request(t, "textDocument/completion", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: 1, Character: 4},
	})

	var list lsp.CompletionList
	decodeResult(t, s.run(t), id, &list)

	labels := labelsOf(list.Items)
	if !contains(labels, "xor") || !contains(labels, "or") {
		t.Errorf("labels = %v, want xor and or", labels)
	}
}

func TestCompletionOffersUnconfiguredFeatures(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "features.ftx"), []byte(modelSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(dir, "configs.ftx")
	uri := pathToURI(configPath)

	s := newSession()
	s.open(t, uri, "navigation *{static early}\n\n")

	id := s.request(t, "textDocument/completion", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: 2, Character: 4},
	})

	var list lsp.CompletionList
	decodeResult(t, s.run(t), id, &list)

	labels := labelsOf(list.Items)

	if !contains(labels, "to_loading") || !contains(labels, "boot") {
		t.Errorf("labels = %v, want the unconfigured features", labels)
	}
	if contains(labels, "navigation") {
		t.Error("navigation is already configured and should not be offered")
	}
}

func TestUnsupportedRequestGetsMethodNotFound(t *testing.T) {
	s := newSession()
	id := s.request(t, "textDocument/rename", struct{}{})

	msgs := s.run(t)

	for _, m := range msgs {
		if string(m.ID) != string(id) {
			continue
		}
		if m.Error == nil {
			t.Fatal("expected an error reply")
		}
		if m.Error.Code != jsonrpc.CodeMethodNotFound {
			t.Errorf("Code = %d, want %d", m.Error.Code, jsonrpc.CodeMethodNotFound)
		}
		return
	}

	t.Fatal("no reply with the request id")
}

func TestUnknownNotificationIsIgnored(t *testing.T) {
	s := newSession()
	s.notify(t, "$/setTrace", map[string]string{"value": "verbose"})

	if msgs := s.run(t); len(msgs) != 0 {
		t.Errorf("a notification must not be answered, got %d messages", len(msgs))
	}
}

func TestShutdownThenExit(t *testing.T) {
	s := newSession()
	s.request(t, "shutdown", struct{}{})
	s.notify(t, "exit", struct{}{})

	err := s.srv.Run()
	if !errors.Is(err, ErrExit) {
		t.Errorf("Run = %v, want ErrExit", err)
	}
	if !s.srv.ShutdownRequested() {
		t.Error("ShutdownRequested = false, want true")
	}
}

func TestExitWithoutShutdown(t *testing.T) {
	s := newSession()
	s.notify(t, "exit", struct{}{})

	if err := s.srv.Run(); !errors.Is(err, ErrExit) {
		t.Errorf("Run = %v, want ErrExit", err)
	}
	if s.srv.ShutdownRequested() {
		t.Error("ShutdownRequested = true, want false")
	}
}

func TestByteOffset(t *testing.T) {
	for _, tc := range []struct {
		line string
		char int
		want int
	}{
		{"abc", 0, 0},
		{"abc", 2, 2},
		{"abc", 9, 3},
		{"    nav", 4, 4},
		// A surrogate pair is two UTF-16 units but four bytes.
		{"\U0001F680ab", 2, 4},
	} {
		if got := byteOffset(tc.line, tc.char); got != tc.want {
			t.Errorf("byteOffset(%q, %d) = %d, want %d", tc.line, tc.char, got, tc.want)
		}
	}
}

func TestURIRoundTrip(t *testing.T) {
	path := "/home/u/ros2 ws/src/features.ftx"

	uri := pathToURI(path)

	got, err := uriToPath(uri)
	if err != nil {
		t.Fatalf("uriToPath: %v", err)
	}
	if got != path {
		t.Errorf("round trip = %q, want %q", got, path)
	}
}

func TestURIRejectsNonFileScheme(t *testing.T) {
	if _, err := uriToPath("https://example.com/x.ftx"); err == nil {
		t.Error("expected an error for a non-file uri")
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// decodeResult finds the reply carrying id and unmarshals its result.
func decodeResult(t *testing.T, msgs []*jsonrpc.Message, id json.RawMessage, v any) {
	t.Helper()

	for _, m := range msgs {
		if string(m.ID) != string(id) {
			continue
		}
		if m.Error != nil {
			t.Fatalf("server replied with an error: %v", m.Error)
		}
		if err := json.Unmarshal(m.Result, v); err != nil {
			t.Fatalf("unmarshal result %s: %v", m.Result, err)
		}
		return
	}

	t.Fatalf("no reply with id %s", id)
}

// diagnosticsFor returns the diagnostics from the last publish for a uri.
func diagnosticsFor(t *testing.T, msgs []*jsonrpc.Message, uri string) []lsp.Diagnostic {
	t.Helper()

	var out []lsp.Diagnostic
	found := false

	for _, m := range msgs {
		if m.Method != "textDocument/publishDiagnostics" {
			continue
		}

		var p lsp.PublishDiagnosticsParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			t.Fatalf("unmarshal publishDiagnostics: %v", err)
		}

		if p.URI == uri {
			out = p.Diagnostics
			found = true
		}
	}

	if !found {
		t.Fatalf("no publishDiagnostics for %q", uri)
	}

	return out
}

func messagesOf(ds []lsp.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Message
	}
	return out
}

func hasMessage(ds []lsp.Diagnostic, want string) bool {
	for _, d := range ds {
		if d.Message == want {
			return true
		}
	}
	return false
}

func labelsOf(items []lsp.CompletionItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
