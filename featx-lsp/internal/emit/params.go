package emit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
)

// ParamFile is one generate_parameter_library specification, covering the
// parameters of a single node.
type ParamFile struct {
	Node string // the node whose parameters these are
	YAML []byte
}

// FileName is what the specification is called wherever it is written. The
// name is derived here so that printing it and writing it cannot disagree.
func (p ParamFile) FileName() string {
	return p.Node + "_params.yaml"
}

// Params renders the typed parameters of a feature model as
// generate_parameter_library specifications, one per node that has any.
//
// The DSL's type names were chosen to match the library's, so no translation
// is needed. Two things are derived rather than written: `description` is
// always blank, and `read_only` follows the node's binding mode, since a
// statically bound node cannot be rebound and so neither can its parameters.
func Params(f *ftx.File) ([]ParamFile, error) {
	if f.Kind != ftx.KindModel {
		return nil, fmt.Errorf("emit: expected a feature model, got a %s", f.Kind)
	}

	var out []ParamFile

	var walk func(ft *ftx.Feature)
	walk = func(ft *ftx.Feature) {
		if ft.Type == ftx.TypeNode {
			if params := parametersOf(ft); len(params) > 0 {
				out = append(out, ParamFile{Node: ft.Name, YAML: renderNode(ft, params)})
			}
		}

		for _, child := range ft.Children {
			walk(child)
		}
	}

	for _, ft := range f.TopLevel() {
		walk(ft)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Node < out[j].Node })
	return out, nil
}

// parametersOf returns a node's immediate typed parameters, in source order.
func parametersOf(node *ftx.Feature) []*ftx.Feature {
	var out []*ftx.Feature
	for _, child := range node.Children {
		if child.Type.IsValue() {
			out = append(out, child)
		}
	}
	return out
}

func renderNode(node *ftx.Feature, params []*ftx.Feature) []byte {
	var sb strings.Builder

	sb.WriteString(node.Name + ":\n")

	for _, p := range params {
		sb.WriteString("  " + p.Name + ":\n")
		sb.WriteString("    type: " + p.Type.String() + "\n")

		if p.HasDefault {
			sb.WriteString("    default_value: " + literal(p.Type, p.DefaultValues) + "\n")
		}

		// The DSL has no slot for prose, so the description is left for a
		// human to fill in rather than invented here.
		sb.WriteString("    description: \"\"\n")

		// A statically bound node is fixed once bound, so its parameters are
		// not reconfigurable either.
		if n := p.BindingNode(); n != nil && n.Mode == ftx.ModeStatic {
			sb.WriteString("    read_only: true\n")
		}

		if v := validation(p); v != "" {
			sb.WriteString("    validation:\n" + v)
		}
	}

	return []byte(sb.String())
}

// validation renders the parameter's validator as the library spells it.
func validation(p *ftx.Feature) string {
	v := p.Validator
	if v.Kind == ftx.ValidatorNone {
		return ""
	}

	element := p.Type.Element()

	switch v.Kind {
	case ftx.ValidatorOneOf:
		// one_of takes a list of permitted values, nested one level because
		// the library passes it as a single argument.
		return fmt.Sprintf("      %s: [%s]\n", v.Kind, list(element, v.Args))

	case ftx.ValidatorFixedSize:
		return fmt.Sprintf("      %s: [%s]\n", v.Kind, v.Args[0])

	default:
		return fmt.Sprintf("      %s: %s\n", v.Kind, list(element, v.Args))
	}
}

// literal renders a value, as a flow sequence for an array type.
func literal(t ftx.Type, values []string) string {
	if t.IsArray() {
		return list(t.Element(), values)
	}
	if len(values) == 0 {
		return `""`
	}
	return scalar(t, values[0])
}

// list renders values as a YAML flow sequence.
func list(element ftx.Type, values []string) string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = scalar(element, v)
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// scalar quotes a value only where YAML needs it, so that numbers and bools
// stay numbers and bools.
func scalar(t ftx.Type, value string) string {
	if t == ftx.TypeString {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
