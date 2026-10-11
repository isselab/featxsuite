package server

import (
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
	"github.com/isselab/featxsuite/featx-lsp/internal/jsonrpc"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

func (s *Server) documentSymbol(m *jsonrpc.Message) error {
	var p lsp.DocumentSymbolParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	src, ok := s.text(p.TextDocument.URI)
	if !ok {
		return s.conn.Reply(m.ID, []lsp.DocumentSymbol{})
	}

	file := ftx.Parse(src)
	docLines := lines(src)

	syms := make([]lsp.DocumentSymbol, 0, len(file.Roots))
	for _, root := range file.Roots {
		syms = append(syms, symbolFor(root, docLines))
	}

	return s.conn.Reply(m.ID, syms)
}

func symbolFor(feature *ftx.Feature, docLines []string) lsp.DocumentSymbol {
	kind := lsp.SymbolKindField
	if len(feature.Children) > 0 {
		kind = lsp.SymbolKindNamespace
	}

	children := make([]lsp.DocumentSymbol, 0, len(feature.Children))
	for _, c := range feature.Children {
		children = append(children, symbolFor(c, docLines))
	}

	sym := lsp.DocumentSymbol{
		Name:           feature.Name,
		Detail:         detailFor(feature),
		Kind:           kind,
		Range:          spanOf(feature, docLines),
		SelectionRange: toRange(feature.NameRange),
	}

	if len(children) > 0 {
		sym.Children = children
	}

	return sym
}

// detailFor is the grey annotation shown beside a symbol in the outline.
func detailFor(feature *ftx.Feature) string {
	var parts []string

	if feature.Grouping != ftx.GroupNone {
		parts = append(parts, feature.Grouping.String())
	}

	if feature.Type != ftx.TypeUnset {
		parts = append(parts, feature.Type.String())
	}

	if feature.HasBindings && feature.Mode != ftx.ModeUnset && feature.Time != ftx.TimeUnset {
		parts = append(parts, feature.Binding())
	}

	if feature.HasDefault {
		parts = append(parts, "= "+feature.DefaultText())
	}

	switch {
	case feature.Optional:
		parts = append(parts, "optional")
	case feature.Selected:
		parts = append(parts, "selected")
	}

	return strings.Join(parts, " · ")
}

// spanOf covers a feature and everything nested under it, so that folding a
// symbol folds its subtree.
func spanOf(feature *ftx.Feature, docLines []string) lsp.Range {
	last := lastLine(feature)

	end := 0
	if last >= 0 && last < len(docLines) {
		end = len(docLines[last])
	}

	return lsp.Range{
		Start: lsp.Position{Line: feature.Line, Character: 0},
		End:   lsp.Position{Line: last, Character: end},
	}
}

func lastLine(feature *ftx.Feature) int {
	last := feature.Line
	for _, c := range feature.Children {
		if l := lastLine(c); l > last {
			last = l
		}
	}
	return last
}

func (s *Server) formatting(m *jsonrpc.Message) error {
	var p lsp.DocumentFormattingParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	src, ok := s.text(p.TextDocument.URI)
	if !ok {
		return s.conn.Reply(m.ID, []lsp.TextEdit{})
	}

	formatted, ok := ftx.Format(ftx.Parse(src))
	if !ok || formatted == src {
		// Refuse to reformat a document with syntax errors rather than
		// rewriting it from an incomplete parse.
		return s.conn.Reply(m.ID, []lsp.TextEdit{})
	}

	docLines := lines(src)
	lastIdx := len(docLines) - 1

	return s.conn.Reply(m.ID, []lsp.TextEdit{{
		Range: lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: lastIdx, Character: len(docLines[lastIdx])},
		},
		NewText: formatted,
	}})
}
