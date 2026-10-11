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
		{Label: "bool", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A true/false parameter of the enclosing node."},
		{Label: "int", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A whole-number parameter of the enclosing node."},
		{Label: "double", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A floating-point parameter of the enclosing node."},
		{Label: "string", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A textual parameter of the enclosing node."},
		{Label: "bool_array", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A list of true/false values."},
		{Label: "int_array", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A list of whole numbers."},
		{Label: "double_array", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A list of floating-point values."},
		{Label: "string_array", Kind: lsp.CompletionItemKindKeyword, Detail: "type",
			Documentation: "A list of strings."},
	}
)

// validatorItems are the validators that may be written on a given type,
// taken from the one description of each rather than listed again here, so a
// validator added to the language is offered without this file changing.
func validatorItems(t ftx.Type) []lsp.CompletionItem {
	var items []lsp.CompletionItem

	for _, spec := range ftx.ValidatorSpecs() {
		if !spec.Domain.Accepts(t) {
			continue
		}

		// One that takes no arguments is complete as written. One that takes
		// some is left open, so the cursor sits where its arguments go rather
		// than past a bracket that has to be stepped back over. Snippets would
		// place it exactly, but a client that does not support them would
		// insert the placeholder as text.
		insert := spec.Name + "<"
		if spec.MaxArgs == 0 {
			insert = spec.Name + "<>"
		}

		items = append(items, lsp.CompletionItem{
			Label:         spec.Name,
			Kind:          lsp.CompletionItemKindFunction,
			Detail:        "validator",
			Documentation: validatorDoc(spec),
			InsertText:    insert,
		})
	}

	return items
}

func validatorDoc(spec ftx.ValidatorSpec) string {
	return fmt.Sprintf("`%s` — generates `%s`, and needs %s",
		spec.Name, spec.Written(), spec.Domain)
}

// typeAlreadyWritten reports the type named in the text after a colon, which
// says whether the cursor is still choosing a type or has moved past one.
func typeAlreadyWritten(after string) (ftx.Type, bool) {
	for _, word := range strings.Fields(after) {
		if t, ok := ftx.TypeNamed(word); ok {
			return t, true
		}
	}
	return ftx.TypeUnset, false
}

// entryAbove finds the entry a continuation line belongs to: the nearest one
// declared above it and indented less.
func entryAbove(file *ftx.File, line, indent int) *ftx.Feature {
	var best *ftx.Feature

	var walk func(ft *ftx.Feature)
	walk = func(ft *ftx.Feature) {
		if ft.Line < line && ft.Indent < indent {
			if best == nil || ft.Line > best.Line {
				best = ft
			}
		}
		for _, child := range ft.Children {
			walk(child)
		}
	}

	for _, ft := range file.TopLevel() {
		walk(ft)
	}

	return best
}

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

	items := s.completionsFor(file, p.TextDocument.URI, prefix, p.Position.Line)
	if items == nil {
		items = []lsp.CompletionItem{}
	}

	return s.conn.Reply(m.ID, lsp.CompletionList{Items: items})
}

// completionsFor picks candidates from where the cursor sits on the line:
// inside unclosed braces only binding words are legal, past the colon a type
// is, inside a bracketed constraint a feature name is, and at the start of a
// line a grouping or a feature name.
func (s *Server) completionsFor(file *ftx.File, uri, prefix string, line int) []lsp.CompletionItem {
	// Braces bind tightest: a cursor inside them wants a mode or a time.
	if open := strings.LastIndex(prefix, "{"); open > strings.LastIndex(prefix, "}") {
		return bindingCompletions(prefix[open+1:])
	}

	// A bracketed constraint takes the names of other features.
	if open := strings.LastIndex(prefix, "["); open > strings.LastIndex(prefix, "]") {
		return s.declaredFeatures(file, uri)
	}

	// Inside a validator's own brackets sit its arguments, which are values of
	// the parameter's type; nothing here knows what they should be.
	if open := strings.LastIndex(prefix, "<"); open > strings.LastIndex(prefix, ">") {
		return nil
	}

	if colon := strings.LastIndex(prefix, ":"); colon >= 0 {
		// A type comes first, and once one is written what follows it may be a
		// validator. Which validators depends on the type, so only those that
		// can apply are offered.
		if t, written := typeAlreadyWritten(prefix[colon+1:]); written {
			return validatorItems(t)
		}
		return typeItems
	}

	// A validator may also sit on its own line beneath the parameter it
	// constrains, where there is no colon to go by.
	if owner := entryAbove(file, line, indentOf(prefix)); owner != nil && owner.Type.IsValue() {
		return validatorItems(owner.Type)
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
		out += " (" + ft.DefaultText() + ")"
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

// indentOf counts the leading spaces of a line, which is how a continuation
// line says which entry it belongs to.
func indentOf(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}
