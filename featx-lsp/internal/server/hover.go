package server

import (
	"fmt"
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
	"github.com/isselab/featxsuite/featx-lsp/internal/jsonrpc"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

func (s *Server) hover(m *jsonrpc.Message) error {
	var p lsp.TextDocumentPositionParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	file, ok := s.parse(p.TextDocument.URI)
	if !ok {
		return s.conn.Reply(m.ID, nil)
	}

	feature, ok := file.FeatureAt(p.Position.Line, p.Position.Character)
	if !ok {
		return s.conn.Reply(m.ID, nil)
	}

	var model *ftx.File
	if file.Kind == ftx.KindConfig {
		model, _ = s.resolveModel(p.TextDocument.URI)
	}

	r := toRange(feature.NameRange)

	return s.conn.Reply(m.ID, lsp.Hover{
		Contents: lsp.MarkupContent{
			Kind:  lsp.Markdown,
			Value: describe(file, feature, model),
		},
		Range: &r,
	})
}

func (s *Server) definition(m *jsonrpc.Message) error {
	var p lsp.TextDocumentPositionParams
	if err := unmarshal(m.Params, &p); err != nil {
		return err
	}

	file, ok := s.parse(p.TextDocument.URI)
	if !ok || file.Kind != ftx.KindConfig {
		// Inside a model the declaration is the entry under the cursor, so
		// there is nowhere to jump to.
		return s.conn.Reply(m.ID, nil)
	}

	feature, ok := file.FeatureAt(p.Position.Line, p.Position.Character)
	if !ok {
		return s.conn.Reply(m.ID, nil)
	}

	model, modelURI := s.resolveModel(p.TextDocument.URI)
	if model == nil {
		return s.conn.Reply(m.ID, nil)
	}

	declared, ok := model.Lookup(feature.Name)
	if !ok {
		return s.conn.Reply(m.ID, nil)
	}

	return s.conn.Reply(m.ID, lsp.Location{
		URI:   modelURI,
		Range: toRange(declared.NameRange),
	})
}

// describe renders the hover card for a feature. When the feature comes from a
// configuration and its model could be resolved, the model's own declaration
// is shown alongside, which is where binding mismatches become obvious.
func describe(file *ftx.File, feature *ftx.Feature, model *ftx.File) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "**%s**", feature.Name)

	switch {
	case file.Kind == ftx.KindModel && file.ModelName != "":
		fmt.Fprintf(&sb, " · feature model `%s`", file.ModelName)
	case file.Kind == ftx.KindConfig:
		sb.WriteString(" · feature configuration")
	}

	sb.WriteString("\n\n")

	for _, line := range facts(file.Kind, feature) {
		fmt.Fprintf(&sb, "- %s\n", line)
	}

	if file.Kind != ftx.KindConfig || model == nil {
		return sb.String()
	}

	declared, ok := model.Lookup(feature.Name)
	if !ok {
		fmt.Fprintf(&sb, "\n⚠ not declared in feature model `%s`\n", model.ModelName)
		return sb.String()
	}

	fmt.Fprintf(&sb, "\nDeclared in feature model `%s`:\n\n", model.ModelName)

	for _, line := range facts(ftx.KindModel, declared) {
		fmt.Fprintf(&sb, "- %s\n", line)
	}

	return sb.String()
}

// facts lists the properties worth showing for a feature, which differ between
// the two document kinds: a model declares what is allowed, a configuration
// records what was chosen.
func facts(kind ftx.Kind, feature *ftx.Feature) []string {
	var out []string

	if feature.Type != ftx.TypeUnset {
		out = append(out, fmt.Sprintf("type: `%s`", feature.Type))
	}

	if kind == ftx.KindConfig {
		if feature.Selected {
			out = append(out, "selected (`*`)")
		} else {
			out = append(out, "not selected")
		}
	}

	if feature.HasDefault {
		label := "default"
		if kind == ftx.KindConfig {
			label = "value"
		}
		out = append(out, fmt.Sprintf("%s: `%s`", label, feature.Default))
	}

	// A value feature without bindings of its own is bound with its node.
	if feature.Type.IsValue() && !feature.HasBindings {
		if n := feature.BindingNode(); n != nil {
			out = append(out, fmt.Sprintf("bound with `%s` (`%s`)", n.Name, n.Binding()))
		}
	}

	if feature.Mode != ftx.ModeUnset {
		label := "binding mode"
		if kind == ftx.KindModel {
			label = "allowed binding mode"
		}
		out = append(out, fmt.Sprintf("%s: `%s`", label, feature.Mode))
	}

	if feature.Time != ftx.TimeUnset {
		label := "binding time"
		if kind == ftx.KindModel {
			label = "allowed binding time"
		}
		out = append(out, fmt.Sprintf("%s: `%s`", label, feature.Time))
	}

	if kind == ftx.KindModel {
		if feature.Optional {
			out = append(out, "optional (`?`)")
		} else {
			out = append(out, "mandatory")
		}
	}

	if feature.Parent != nil {
		out = append(out, fmt.Sprintf("parent: `%s`", feature.Parent.Name))
	}

	if len(feature.Children) > 0 {
		names := make([]string, len(feature.Children))
		for i, c := range feature.Children {
			names[i] = fmt.Sprintf("`%s`", c.Name)
		}

		if feature.Grouping != ftx.GroupNone {
			out = append(out, fmt.Sprintf("children (`%s`): %s",
				feature.Grouping, strings.Join(names, ", ")))
		} else {
			out = append(out, fmt.Sprintf("children: %s", strings.Join(names, ", ")))
		}
	}

	for _, c := range feature.Constraints {
		operands := make([]string, len(c.Operands))
		for i, ref := range c.Operands {
			operands[i] = fmt.Sprintf("`%s`", ref.Name)
		}

		joiner := ", "
		if c.Op != ftx.OpNone {
			joiner = fmt.Sprintf(" `%s` ", c.Op)
		}

		out = append(out, fmt.Sprintf("%s: %s", c.Kind(), strings.Join(operands, joiner)))
	}

	return out
}
