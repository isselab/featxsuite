package emit

import (
	"strings"
	"testing"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
)

const paramModel = `RB-1-BASE:
    nav : node {dynamic late}
        vel_max : double (1.5) <0.0, 3.0>
        vel_min : double (1.0) <0.0, #>
        tolerance : double (-0.01) <#, 0.0>
        mode_pick : double (7.0) <<0.0, 3.0, 7.0>>
        offsets : double_array (0.0, 0.0, 0.0, 0.0) <4>
        retries : int (3)
        verbose : bool (true)
        label : string (fast) <<slow, fast>>
    base : node {static early}
        timeout : double (0.5)
    plain : node {static early}
`

func render(t *testing.T, src string) map[string]string {
	t.Helper()

	specs, err := Params(ftx.Parse(src))
	if err != nil {
		t.Fatalf("Params: %v", err)
	}

	out := make(map[string]string, len(specs))
	for _, s := range specs {
		out[s.Node] = string(s.YAML)
	}
	return out
}

// TestParamsOnePerNodeWithParameters: a node with no parameters produces no
// file, since generate_parameter_library would have nothing to generate.
func TestParamsOnePerNodeWithParameters(t *testing.T) {
	got := render(t, paramModel)

	if len(got) != 2 {
		t.Fatalf("rendered %d file(s) %v, want nav and base", len(got), keys(got))
	}
	for _, node := range []string{"nav", "base"} {
		if _, ok := got[node]; !ok {
			t.Errorf("no spec for %q", node)
		}
	}
	if _, ok := got["plain"]; ok {
		t.Error("a node with no parameters should produce no spec")
	}
}

func TestParamsValidatorForms(t *testing.T) {
	nav := render(t, paramModel)["nav"]

	for _, want := range []string{
		"bounds<>: [0.0, 3.0]",
		"gt_eq<>: [0.0]",
		"lt_eq<>: [0.0]",
		"one_of<>: [[0.0, 3.0, 7.0]]",
		"fixed_size<>: [4]",
		`one_of<>: [["slow", "fast"]]`,
	} {
		if !strings.Contains(nav, want) {
			t.Errorf("missing %q in:\n%s", want, nav)
		}
	}
}

func TestParamsTypesAndValues(t *testing.T) {
	nav := render(t, paramModel)["nav"]

	for _, want := range []string{
		"type: double",
		"type: double_array",
		"type: int",
		"type: bool",
		"type: string",
		"default_value: 1.5",
		"default_value: [0.0, 0.0, 0.0, 0.0]",
		"default_value: 3",
		"default_value: true",
		`default_value: "fast"`,
	} {
		if !strings.Contains(nav, want) {
			t.Errorf("missing %q in:\n%s", want, nav)
		}
	}

	// Only strings are quoted, so numbers and bools stay scalars.
	if strings.Contains(nav, `default_value: "1.5"`) {
		t.Error("a double was quoted")
	}
	if strings.Contains(nav, `default_value: "true"`) {
		t.Error("a bool was quoted")
	}
}

// TestParamsReadOnlyFollowsBindingMode is the substantive mapping: a static
// node cannot be rebound, so neither can its parameters.
func TestParamsReadOnlyFollowsBindingMode(t *testing.T) {
	got := render(t, paramModel)

	if strings.Contains(got["nav"], "read_only") {
		t.Errorf("a dynamic node's parameters must stay reconfigurable:\n%s", got["nav"])
	}
	if !strings.Contains(got["base"], "read_only: true") {
		t.Errorf("a static node's parameters must be read-only:\n%s", got["base"])
	}
}

func TestParamsDescriptionIsBlank(t *testing.T) {
	nav := render(t, paramModel)["nav"]

	if n := strings.Count(nav, `description: ""`); n != 8 {
		t.Errorf("found %d blank descriptions, want one per parameter (8)", n)
	}
}

func TestParamsNodeNameIsTheTopLevelKey(t *testing.T) {
	got := render(t, paramModel)

	if !strings.HasPrefix(got["nav"], "nav:\n") {
		t.Errorf("spec should open with the node name:\n%s", got["nav"])
	}
}

func TestParamsRejectsAConfiguration(t *testing.T) {
	if _, err := Params(ftx.Parse("nav *{static early}\n")); err == nil {
		t.Error("Params accepted a configuration")
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
