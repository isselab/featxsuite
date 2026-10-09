package server

import (
	"fmt"
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
	"github.com/isselab/featxsuite/featx-lsp/internal/jsonrpc"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

var (
	modeItems = []lsp.CompletionItem{
		{Label: "static", Kind: lsp.CompletionItemKindEnum, Detail: "binding mode",
			Documentation: "Bound once; cannot be rebound while the configuration runs."},
		{Label: "dynamic", Kind: lsp.CompletionItemKindEnum, Detail: "binding mode",
			Documentation: "Can be loaded and unloaded while the configuration runs."},
	}

	timeItems = []lsp.CompletionItem{
		{Label: "early", Kind: lsp.CompletionItemKindEnum, Detail: "binding time",
			Documentation: "Bound at compile time, before the configuration starts."},
		{Label: "late", Kind: lsp.CompletionItemKindEnum, Detail: "binding time",
			Documentation: "Bound after the configuration has started."},
	}

	groupingItems = []lsp.CompletionItem{
		{Label: "xor", Kind: lsp.CompletionItemKindKeyword, Detail: "grouping",
			Documentation: "The children of this feature are alternatives; at most one may be selected."},
		{Label: "or", Kind: lsp.CompletionItemKindKeyword, Detail: "grouping",
			Documentation: "One or more of this feature's children may be selected."},
	}

	typeItems = []lsp.CompletionItem{
		{Label: "node", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A bindable unit of behaviour. Its bindings follow in braces.",
			InsertText:    "node {"},
		{Label: "number", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A numeric parameter of the enclosing node."},
		{Label: "string", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A textual parameter of the enclosing node."},
		{Label: "bool", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A true/false parameter of the enclosing node."},
	}
)

func (s *Server) completion(m *jsonrpc.Message) error {
	var p lsp.TextDocumentPositionParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	src, ok := s.text(p.TextDocument.URI)
	if !ok {
		return s.conn.Reply(m.ID, lsp.CompletionList{Items: []lsp.CompletionItem{}})
	}

	file := ftx.Parse(src)
	line := lineAt(src, p.Position.Line)
	prefix := line[:byteOffset(line, p.Position.Character)]

	items := s.completionsFor(file, p.TextDocument.URI, prefix)
	if items == nil {
		items = []lsp.CompletionItem{}
	}

	return s.conn.Reply(m.ID, lsp.CompletionList{Items: items})
}

// completionsFor picks candidates from where the cursor sits on the line:
// inside unclosed braces only binding words are legal, past the colon a type
// is, inside a bracketed constraint a feature name is, and at the start of a
// line a grouping or a feature name.
func (s *Server) completionsFor(file *ftx.File, uri, prefix string) []lsp.CompletionItem {
	// Braces bind tightest: a cursor inside them wants a mode or a time.
	if open := strings.LastIndex(prefix, "{"); open > strings.LastIndex(prefix, "}") {
		return bindingCompletions(prefix[open+1:])
	}

	// A bracketed constraint takes the names of other features.
	if open := strings.LastIndex(prefix, "["); open > strings.LastIndex(prefix, "]") {
		return s.declaredFeatures(file, uri)
	}

	if strings.LastIndex(prefix, ":") >= 0 {
		return typeItems
	}

	var items []lsp.CompletionItem

	if strings.TrimSpace(prefix) == "" && file.Kind == ftx.KindModel {
		items = append(items, groupingItems...)
	}

	if file.Kind == ftx.KindConfig {
		items = append(items, s.unconfiguredFeatures(file, uri)...)
	}

	return items
}

// declaredFeatures offers the names a constraint can refer to, which in a
// model are the model's own features.
func (s *Server) declaredFeatures(file *ftx.File, uri string) []lsp.CompletionItem {
	source := file
	if file.Kind == ftx.KindConfig {
		if model, _ := s.resolveModel(uri); model != nil {
			source = model
		}
	}

	var items []lsp.CompletionItem

	for _, ft := range source.Order {
		if ft.Name == "" {
			continue
		}

		items = append(items, lsp.CompletionItem{
			Label:  ft.Name,
			Kind:   lsp.CompletionItemKindProperty,
			Detail: ft.Type.String(),
		})
	}

	return items
}

// bindingCompletions offers the binding words still missing from the braces,
// so that one already naming a mode only offers times.
func bindingCompletions(inBraces string) []lsp.CompletionItem {
	var haveMode, haveTime bool

	for _, word := range strings.Fields(inBraces) {
		switch word {
		case "static", "dynamic":
			haveMode = true
		case "early", "late":
			haveTime = true
		}
	}

	var items []lsp.CompletionItem

	if !haveMode {
		items = append(items, modeItems...)
	}
	if !haveTime {
		items = append(items, timeItems...)
	}

	return items
}

// entryDetail renders a feature as a configuration entry would declare it.
func entryDetail(ft *ftx.Feature) string {
	out := ft.Type.String()

	if ft.HasBindings {
		out += " {" + ft.Binding() + "}"
	}
	if ft.HasDefault {
		out += " (" + ft.Default + ")"
	}

	return out
}

// unconfiguredFeatures offers model features the configuration has no entry
// for yet, which is the list an author is working through.
func (s *Server) unconfiguredFeatures(cfg *ftx.File, uri string) []lsp.CompletionItem {
	model, _ := s.resolveModel(uri)
	if model == nil {
		return nil
	}

	var items []lsp.CompletionItem

	for _, declared := range model.Order {
		if declared.Name == "" {
			continue
		}
		if _, configured := cfg.Lookup(declared.Name); configured {
			continue
		}

		items = append(items, lsp.CompletionItem{
			Label:         declared.Name,
			Kind:          lsp.CompletionItemKindProperty,
			Detail:        entryDetail(declared),
			Documentation: fmt.Sprintf("Declared in feature model %q.", model.ModelName),
			InsertText:    declared.Name + " : " + entryDetail(declared),
		})
	}

	return items
}
