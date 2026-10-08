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

// completionsFor picks candidates from where the cursor sits on the line: past
// the colon only bindings are legal, at the start of a line a grouping or a
// feature name is, and in between only a name.
func (s *Server) completionsFor(file *ftx.File, uri, prefix string) []lsp.CompletionItem {
	if colon := strings.LastIndex(prefix, ":"); colon >= 0 {
		return bindingCompletions(prefix[colon+1:])
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

// bindingCompletions offers the binding words still missing from the entry, so
// that a line already naming a mode only offers times.
func bindingCompletions(afterColon string) []lsp.CompletionItem {
	var haveMode, haveTime bool

	for _, word := range strings.Fields(afterColon) {
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
			Detail:        fmt.Sprintf("%s %s", declared.Mode, declared.Time),
			Documentation: fmt.Sprintf("Declared in feature model %q.", model.ModelName),
			InsertText:    fmt.Sprintf("%s: %s %s", declared.Name, declared.Mode, declared.Time),
		})
	}

	return items
}
