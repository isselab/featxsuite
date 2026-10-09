// Package emit renders parsed .ftx documents as the JSON the featx toolchain
// consumes, so that the DSL can be the single source of truth and the JSON a
// build artifact.
//
// The JSON shape is dictated by its existing readers — featxcli's Configurator
// and featxbinder's CMakeLists — so the field names and the capitalised
// binding vocabulary are reproduced exactly rather than modernised.
package emit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
)

// Model renders a feature model as features.json.
func Model(f *ftx.File) ([]byte, error) {
	if f.Kind != ftx.KindModel {
		return nil, fmt.Errorf("emit: expected a feature model, got a %s", f.Kind)
	}

	// The outer object keeps the name "root" because Configurator.traverseModel
	// tests for that exact string when deciding what the parent feature is.
	// The model's own root name travels alongside it instead.
	root := model{Name: "root", ModelName: f.ModelName, Sub: []node{}}
	for _, ft := range f.TopLevel() {
		root.Sub = append(root.Sub, feature(ft))
	}

	return marshal(root)
}

// Config renders a feature configuration as configs.json.
func Config(f *ftx.File) ([]byte, error) {
	if f.Kind != ftx.KindConfig {
		return nil, fmt.Errorf("emit: expected a feature configuration, got a %s", f.Kind)
	}

	doc := configs{Configs: []entry{}}
	for _, ft := range f.Order {
		doc.Configs = append(doc.Configs, entry{
			Name:        ft.Name,
			Type:        ft.Type.String(),
			IsSelected:  ft.Selected,
			Value:       ft.DefaultText(),
			BindingTime: title(ft.Time.String()),
			BindingMode: title(ft.Mode.String()),
		})
	}

	return marshal(doc)
}

// model is the outermost object of features.json.
type model struct {
	Name      string `json:"name"`
	ModelName string `json:"modelName,omitempty"`
	Sub       []node `json:"sub"`
}

// node mirrors one object in features.json. Field order follows the
// hand-written file so that generated and existing output diff cleanly.
type node struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	IsOptional *bool  `json:"isOptional,omitempty"`

	// Default carries the parenthesised value of a typed parameter.
	Default string `json:"default,omitempty"`

	// Grouping has no counterpart in the hand-written JSON, which cannot
	// represent alternative groups at all. It is emitted so the JSON stays a
	// faithful projection of the model; no existing reader looks at it.
	Grouping string `json:"grouping,omitempty"`

	// Includes and Excludes are empty strings rather than empty arrays when
	// unset, which is what the hand-written file does and what
	// Configurator.find_feature_includes branches on.
	Includes any `json:"includes"`
	Excludes any `json:"excludes"`

	// RequiresOperator records the connective when a requirement is a
	// disjunction. The flat Includes array cannot express "one of", and the
	// existing reader loads every element, so a consumer that honours "or"
	// has to look here.
	RequiresOperator string `json:"requiresOperator,omitempty"`

	BindingTimeAllowed string `json:"bindingTimeAllowed,omitempty"`
	BindingModeAllowed string `json:"bindingModeAllowed,omitempty"`

	Sub []node `json:"sub,omitempty"`
}

type configs struct {
	Configs []entry `json:"configs"`
}

type entry struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	IsSelected  bool   `json:"isSelected"`
	Value       string `json:"value,omitempty"`
	BindingTime string `json:"bindingTime,omitempty"`
	BindingMode string `json:"bindingMode,omitempty"`
}

func feature(ft *ftx.Feature) node {
	optional := ft.Optional

	n := node{
		Name:               ft.Name,
		Type:               ft.Type.String(),
		IsOptional:         &optional,
		Default:            ft.DefaultText(),
		Grouping:           ft.Grouping.String(),
		Includes:           operandList(ft.Requires()),
		Excludes:           operandList(ft.Excludes()),
		RequiresOperator:   disjunctive(ft.Requires()),
		BindingTimeAllowed: title(ft.Time.String()),
		BindingModeAllowed: title(ft.Mode.String()),
	}

	for _, child := range ft.Children {
		n.Sub = append(n.Sub, feature(child))
	}

	return n
}

// operandList flattens the operands of a feature's constraints the way the
// existing readers expect: an array when populated, an empty string when not.
func operandList(cs []ftx.Constraint) any {
	var out []string
	for _, c := range cs {
		for _, ref := range c.Operands {
			out = append(out, ref.Name)
		}
	}

	if len(out) == 0 {
		return ""
	}
	return out
}

// disjunctive reports "or" when any requirement is a disjunction, so that the
// flattened array is not mistaken for a conjunction.
func disjunctive(cs []ftx.Constraint) string {
	for _, c := range cs {
		if c.Op == ftx.OpOr {
			return "or"
		}
	}
	return ""
}

// title capitalises a binding word, since the JSON vocabulary is "Early" and
// "Static" where the DSL writes "early" and "static".
func title(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("emit: %w", err)
	}

	return buf.Bytes(), nil
}
