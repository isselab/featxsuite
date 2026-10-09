package emit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
)

const modelSrc = `RB-1-BASE:
    xor navigation : node {static early} ?
        to_loading : node {dynamic late}
            [square && attach]
        to_base : node {dynamic late}
    footprint : node {static early} ?
        square : node {dynamic late}
        radius : number (0.3)
    attach : node {dynamic late}
        ![to_base]
`

const configSrc = `navigation *{static early}
to_loading {dynamic late}
square *{dynamic late}
`

// decode parses emitted JSON into an untyped tree for inspection.
func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("emitted invalid JSON: %v\n%s", err, b)
	}
	return out
}

// find locates a feature by name anywhere in the emitted tree.
func find(t *testing.T, root map[string]any, name string) map[string]any {
	t.Helper()

	var walk func(n map[string]any) map[string]any
	walk = func(n map[string]any) map[string]any {
		if n["name"] == name {
			return n
		}
		subs, _ := n["sub"].([]any)
		for _, s := range subs {
			if child, ok := s.(map[string]any); ok {
				if hit := walk(child); hit != nil {
					return hit
				}
			}
		}
		return nil
	}

	hit := walk(root)
	if hit == nil {
		t.Fatalf("%q not found in emitted model", name)
	}
	return hit
}

func TestModelRootShape(t *testing.T) {
	b, err := Model(ftx.Parse(modelSrc))
	if err != nil {
		t.Fatalf("Model: %v", err)
	}

	root := decode(t, b)

	if root["name"] != "root" {
		t.Errorf("root name = %v, want root", root["name"])
	}

	// The synthetic root is not a feature: it must carry no binding or
	// constraint fields, matching the hand-written features.json.
	for _, key := range []string{"includes", "excludes", "isOptional", "bindingTimeAllowed", "grouping"} {
		if _, present := root[key]; present {
			t.Errorf("root should not carry %q", key)
		}
	}

	if subs, _ := root["sub"].([]any); len(subs) != 3 {
		t.Errorf("root has %d children, want 3", len(subs))
	}
}

func TestModelCapitalisesBindingVocabulary(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))
	root := decode(t, b)

	nav := find(t, root, "navigation")
	if nav["bindingTimeAllowed"] != "Early" {
		t.Errorf("bindingTimeAllowed = %v, want Early", nav["bindingTimeAllowed"])
	}
	if nav["bindingModeAllowed"] != "Static" {
		t.Errorf("bindingModeAllowed = %v, want Static", nav["bindingModeAllowed"])
	}

	leaf := find(t, root, "to_base")
	if leaf["bindingTimeAllowed"] != "Late" || leaf["bindingModeAllowed"] != "Dynamic" {
		t.Errorf("leaf bindings = %v/%v, want Late/Dynamic",
			leaf["bindingTimeAllowed"], leaf["bindingModeAllowed"])
	}
}

func TestModelConstraintsBecomeIncludesAndExcludes(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))
	root := decode(t, b)

	loading := find(t, root, "to_loading")

	inc, ok := loading["includes"].([]any)
	if !ok {
		t.Fatalf("includes = %#v, want an array", loading["includes"])
	}
	if len(inc) != 2 || inc[0] != "square" || inc[1] != "attach" {
		t.Errorf("includes = %v, want [square attach]", inc)
	}

	attach := find(t, root, "attach")
	exc, ok := attach["excludes"].([]any)
	if !ok {
		t.Fatalf("excludes = %#v, want an array", attach["excludes"])
	}
	if len(exc) != 1 || exc[0] != "to_base" {
		t.Errorf("excludes = %v, want [to_base]", exc)
	}
}

// TestModelEmptyConstraintsAreEmptyStrings pins the existing JSON's wart:
// Configurator.find_feature_includes branches on a string, so an unset clause
// must not become an empty array.
func TestModelEmptyConstraintsAreEmptyStrings(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))
	root := decode(t, b)

	base := find(t, root, "to_base")

	if base["includes"] != "" {
		t.Errorf("includes = %#v, want an empty string", base["includes"])
	}
	if base["excludes"] != "" {
		t.Errorf("excludes = %#v, want an empty string", base["excludes"])
	}
}

func TestModelCarriesOptionalityAndGrouping(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))
	root := decode(t, b)

	nav := find(t, root, "navigation")
	if nav["isOptional"] != true {
		t.Errorf("navigation isOptional = %v, want true", nav["isOptional"])
	}
	if nav["grouping"] != "xor" {
		t.Errorf("navigation grouping = %v, want xor", nav["grouping"])
	}

	leaf := find(t, root, "to_base")
	if leaf["isOptional"] != false {
		t.Errorf("to_base isOptional = %v, want false", leaf["isOptional"])
	}
	if _, present := leaf["grouping"]; present {
		t.Error("an ungrouped feature should not carry a grouping field")
	}
}

func TestModelPreservesNesting(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))
	root := decode(t, b)

	nav := find(t, root, "navigation")
	subs, _ := nav["sub"].([]any)

	if len(subs) != 2 {
		t.Fatalf("navigation has %d children, want 2", len(subs))
	}

	first, _ := subs[0].(map[string]any)
	if first["name"] != "to_loading" {
		t.Errorf("first child = %v, want to_loading", first["name"])
	}
}

func TestConfigShape(t *testing.T) {
	b, err := Config(ftx.Parse(configSrc))
	if err != nil {
		t.Fatalf("Config: %v", err)
	}

	var doc struct {
		Configs []struct {
			Name        string `json:"name"`
			IsSelected  bool   `json:"isSelected"`
			BindingTime string `json:"bindingTime"`
			BindingMode string `json:"bindingMode"`
		} `json:"configs"`
	}

	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("emitted invalid JSON: %v\n%s", err, b)
	}

	if len(doc.Configs) != 3 {
		t.Fatalf("len(configs) = %d, want 3", len(doc.Configs))
	}

	first := doc.Configs[0]
	if first.Name != "navigation" || !first.IsSelected {
		t.Errorf("configs[0] = %+v, want navigation selected", first)
	}
	if first.BindingTime != "Early" || first.BindingMode != "Static" {
		t.Errorf("configs[0] bindings = %s/%s, want Early/Static", first.BindingTime, first.BindingMode)
	}

	if doc.Configs[1].IsSelected {
		t.Errorf("to_loading should not be selected")
	}
}

func TestModelCarriesTypeAndDefault(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))
	root := decode(t, b)

	radius := find(t, root, "radius")

	if radius["type"] != "number" {
		t.Errorf("type = %v, want number", radius["type"])
	}
	if radius["default"] != "0.3" {
		t.Errorf("default = %v, want 0.3", radius["default"])
	}

	// A value feature declares no bindings, so none may be invented for it.
	for _, key := range []string{"bindingTimeAllowed", "bindingModeAllowed"} {
		if _, present := radius[key]; present {
			t.Errorf("a value feature should not carry %q", key)
		}
	}

	nav := find(t, root, "navigation")
	if nav["type"] != "node" {
		t.Errorf("navigation type = %v, want node", nav["type"])
	}
	if _, present := nav["default"]; present {
		t.Error("a node should not carry a default")
	}
}

// TestModelRecordsDisjunction matters because the flat includes array cannot
// express "one of", and the existing reader loads every element.
func TestModelRecordsDisjunction(t *testing.T) {
	src := "M:\n    a : node {static early}\n        [b || c]\n" +
		"    b : node {static early}\n    c : node {static early}\n"

	b, err := Model(ftx.Parse(src))
	if err != nil {
		t.Fatalf("Model: %v", err)
	}

	a := find(t, decode(t, b), "a")

	if a["requiresOperator"] != "or" {
		t.Errorf("requiresOperator = %v, want or", a["requiresOperator"])
	}

	// A conjunction leaves the field out, so the array reads as it always has.
	b2, _ := Model(ftx.Parse(modelSrc))
	loading := find(t, decode(t, b2), "to_loading")

	if _, present := loading["requiresOperator"]; present {
		t.Error("a conjunction should not record an operator")
	}
}

func TestConfigCarriesNoTypeOrValue(t *testing.T) {
	src := "a *{static early}\n"

	b, err := Config(ftx.Parse(src))
	if err != nil {
		t.Fatalf("Config: %v", err)
	}

	var doc struct {
		Configs []map[string]any `json:"configs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("emitted invalid JSON: %v\n%s", err, b)
	}

	// A configuration entry declares neither, so neither may be invented.
	for _, key := range []string{"type", "value"} {
		if _, present := doc.Configs[0][key]; present {
			t.Errorf("a configuration entry should not carry %q", key)
		}
	}
	if doc.Configs[0]["bindingMode"] != "Static" {
		t.Errorf("bindingMode = %v, want Static", doc.Configs[0]["bindingMode"])
	}
}

func TestEmitIsIndentedWithFourSpaces(t *testing.T) {
	b, _ := Model(ftx.Parse(modelSrc))

	if !strings.Contains(string(b), "\n    \"sub\"") {
		t.Errorf("expected four-space indentation:\n%s", b)
	}
}

func TestEmitRejectsTheWrongKind(t *testing.T) {
	if _, err := Model(ftx.Parse(configSrc)); err == nil {
		t.Error("Model accepted a configuration")
	}
	if _, err := Config(ftx.Parse(modelSrc)); err == nil {
		t.Error("Config accepted a model")
	}
}
