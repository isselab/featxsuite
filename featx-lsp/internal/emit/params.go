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

	// Enclosing names the nodes above this one, nearest first. A node is not
	// always a package: a grouping is built as one, and the alternatives it
	// groups are components inside it. When nothing is named after the node
	// itself, these say where to look next.
	Enclosing []string

	YAML []byte
}

// FileName is what the specification is called wherever it is written. The
// name is derived here so that printing it and writing it cannot disagree.
func (p ParamFile) FileName() string {
	return p.Node + "_params.yaml"
}

// Params renders the typed parameters of a feature model as
// generate_parameter_library specifications, one per node that declares any.
//
// The DSL's type names were chosen to match the library's, so no translation
// is needed. Two things are derived rather than written: `description` is
// always blank, and `read_only` follows the binding mode of the node the
// parameter ends up on, since a statically bound node cannot be rebound and so
// neither can its parameters.
//
// A grouping is a choice point rather than a worker: its children are the
// alternatives, and it is one of them that runs. A parameter written at a
// grouping therefore describes whichever alternative was selected, so it is
// declared on each of them instead of on the grouping. Without this a value
// like a maximum velocity would be declared on the node that does not move,
// where the nodes that do move cannot see it: ROS parameters belong to one
// node, and a grouping's children are nodes of their own.
func Params(f *ftx.File) ([]ParamFile, error) {
	if f.Kind != ftx.KindModel {
		return nil, fmt.Errorf("emit: expected a feature model, got a %s", f.Kind)
	}

	var out []ParamFile

	// inherited are the parameters an enclosing grouping handed down, and
	// above names the nodes this one sits under, nearest first.
	var walk func(ft *ftx.Feature, inherited []*ftx.Feature, above []string)
	walk = func(ft *ftx.Feature, inherited []*ftx.Feature, above []string) {
		own := parametersOf(ft)
		children := nodesOf(ft)

		under := above
		if ft.Type == ftx.TypeNode {
			under = concatNames([]string{ft.Name}, above)
		}

		// A grouping with alternatives declares nothing itself and passes
		// everything down. One without any has nowhere to pass to, so it
		// keeps its parameters rather than dropping them silently.
		if ft.Grouping != ftx.GroupNone && len(children) > 0 {
			passed := concat(inherited, own)
			for _, child := range children {
				walk(child, passed, under)
			}
			return
		}

		if declared := concat(inherited, own); ft.Type == ftx.TypeNode && len(declared) > 0 {
			out = append(out, ParamFile{
				Node:      ft.Name,
				Enclosing: above,
				YAML:      renderNode(ft, declared),
			})
		}

		// A plain node is not a choice point: it owns its parameters, and its
		// children own theirs.
		for _, child := range children {
			walk(child, nil, under)
		}
	}

	for _, ft := range f.TopLevel() {
		walk(ft, nil, nil)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Node < out[j].Node })
	return out, nil
}

// concat joins two parameter lists without aliasing either.
func concat(a, b []*ftx.Feature) []*ftx.Feature {
	if len(a) == 0 {
		return b
	}

	out := make([]*ftx.Feature, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}

// concatNames joins two name lists without aliasing either.
func concatNames(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}

// nodesOf returns a feature's immediate children that are nodes, in source
// order.
func nodesOf(ft *ftx.Feature) []*ftx.Feature {
	var out []*ftx.Feature
	for _, child := range ft.Children {
		if child.Type == ftx.TypeNode {
			out = append(out, child)
		}
	}
	return out
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
		// not reconfigurable either. This follows the node the specification
		// is being written for, not the parameter's place in the model: a
		// parameter handed down from a grouping belongs to the alternative
		// that received it, and is as reconfigurable as that alternative is.
		if node.Mode == ftx.ModeStatic {
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
