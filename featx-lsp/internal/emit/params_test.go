package emit

import (
	"strings"
	"testing"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
)

const paramModel = `RB-1-BASE:
    nav : node {dynamic late}
        vel_max : double (1.5) bounds<0.0, 3.0>
        vel_min : double (1.0) gt_eq<0.0>
        tolerance : double (-0.01) lt_eq<0.0>
        mode_pick : double (7.0) one_of<0.0, 3.0, 7.0>
        offsets : double_array (0.0, 0.0, 0.0, 0.0) fixed_size<4>
        retries : int (3)
        verbose : bool (true)
        label : string (fast) one_of<slow, fast>
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

	if !strings.Contains(got["nav"], "read_only: false") {
		t.Errorf("a dynamic node's parameters must stay reconfigurable:\n%s", got["nav"])
	}
	if !strings.Contains(got["base"], "read_only: true") {
		t.Errorf("a static node's parameters must be read-only:\n%s", got["base"])
	}
}

// Both values are written out. Leaving the false case to the generator's
// default would make a specification silent about something the model states.
func TestParamsReadOnlyIsAlwaysStated(t *testing.T) {
	for node, spec := range render(t, paramModel) {
		params := strings.Count(spec, "\n    type: ")
		stated := strings.Count(spec, "\n    read_only: ")

		if params != stated {
			t.Errorf("%s: %d parameter(s) but %d read_only line(s):\n%s",
				node, params, stated, spec)
		}
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

	// The marker comes first; the node name is the first key.
	if !strings.HasPrefix(got["nav"], Preamble+"nav:\n") {
		t.Errorf("spec should open with the marker then the node name:\n%s", got["nav"])
	}
}

func TestParamsMarksWhatItGenerated(t *testing.T) {
	got := render(t, paramModel)

	if !IsGenerated([]byte(got["nav"])) {
		t.Errorf("spec is not recognisable as generated:\n%s", got["nav"])
	}
	// A schema a package wrote itself must not be mistaken for ours.
	if IsGenerated([]byte("nav:\n  vel_max:\n    type: double\n")) {
		t.Error("a hand-written schema was taken for generated")
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

// groupedModel writes a parameter at a choice point. The alternatives are the
// nodes that run, so they are the ones that need it.
const groupedModel = `RB-1-BASE:
    xor navigation : node {static early} ?
        vel_max : double (1.6) bounds<0.0, 3.0>
        to_loading : node {dynamic late}
        to_delivery : node {dynamic late}
    shelf : node {static early} ?
        max_weight : double (50.0)
        attach : node {dynamic late}
`

func TestParamsGroupingHandsItsParametersToEachAlternative(t *testing.T) {
	got := render(t, groupedModel)

	for _, node := range []string{"to_loading", "to_delivery"} {
		spec, ok := got[node]
		if !ok {
			t.Fatalf("no spec for alternative %q", node)
		}
		if !strings.Contains(spec, node+":") {
			t.Errorf("%s spec is not keyed on the alternative:\n%s", node, spec)
		}
		if !strings.Contains(spec, "vel_max:") {
			t.Errorf("%s did not receive the grouping's parameter:\n%s", node, spec)
		}
	}
}

// A grouping does no work itself, so declaring the parameter there would put
// it on a node the alternatives cannot read: ROS parameters belong to one node.
func TestParamsGroupingKeepsNothingForItself(t *testing.T) {
	if spec, ok := render(t, groupedModel)["navigation"]; ok {
		t.Errorf("the grouping declared parameters of its own:\n%s", spec)
	}
}

// Only a grouping hands parameters down. A plain node is a worker that owns
// what it declares, and its children own theirs.
func TestParamsPlainNodeKeepsItsOwnParameters(t *testing.T) {
	got := render(t, groupedModel)

	spec, ok := got["shelf"]
	if !ok {
		t.Fatal("a plain node did not keep its own parameter")
	}
	if !strings.Contains(spec, "max_weight:") {
		t.Errorf("shelf lost its parameter:\n%s", spec)
	}
	if _, ok := got["attach"]; ok {
		t.Error("a plain node's parameter was handed to its child")
	}
}

// read_only describes the node that received the parameter, not where it was
// written: an alternative bound dynamically stays reconfigurable even though
// the grouping above it is static.
func TestParamsReadOnlyFollowsTheReceivingNode(t *testing.T) {
	got := render(t, groupedModel)

	if !strings.Contains(got["to_loading"], "read_only: false") {
		t.Errorf("a dynamically bound alternative was not left reconfigurable:\n%s", got["to_loading"])
	}
	if !strings.Contains(got["shelf"], "read_only: true") {
		t.Errorf("a statically bound node was not marked read_only:\n%s", got["shelf"])
	}
}

func TestParamsEnclosingNamesWhereToLookForAPackage(t *testing.T) {
	specs, err := Params(ftx.Parse(groupedModel))
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range specs {
		switch s.Node {
		case "to_loading", "to_delivery":
			// An alternative is a component inside the grouping's package.
			if len(s.Enclosing) == 0 || s.Enclosing[0] != "navigation" {
				t.Errorf("%s: Enclosing = %v, want navigation first", s.Node, s.Enclosing)
			}
		case "shelf":
			// A top-level node sits under nothing.
			if len(s.Enclosing) != 0 {
				t.Errorf("shelf: Enclosing = %v, want empty", s.Enclosing)
			}
		}
	}
}

// A grouping with no alternatives has nowhere to hand its parameters, so it
// keeps them rather than dropping them without a word.
func TestParamsGroupingWithNoAlternativesKeepsItsParameters(t *testing.T) {
	got := render(t, `RB-1-BASE:
    xor orphan : node {static early} ?
        stray : double (1.0)
`)

	spec, ok := got["orphan"]
	if !ok {
		t.Fatal("a grouping with no alternatives lost its parameters")
	}
	if !strings.Contains(spec, "stray:") {
		t.Errorf("parameter went missing:\n%s", spec)
	}
}

// exclusiveModel exercises every combination of open and closed ends.
const exclusiveModel = `RB-1-BASE:
    demo : node {dynamic late}
        at_least : double (1.0) gt_eq<0.0>
        more_than : double (1.0) gt<0.0>
        at_most : double (1.0) lt_eq<3.0>
        less_than : double (1.0) lt<3.0>
        closed : double (1.5) bounds<0.0, 3.0>
        open : double (1.5) gt<0.0> lt<3.0>
        half_lo : double (1.5) gt<0.0> lt_eq<3.0>
        half_hi : double (1.5) gt_eq<0.0> lt<3.0>
`

func TestParamsExclusiveBoundsUseTheStrictValidators(t *testing.T) {
	spec := render(t, exclusiveModel)["demo"]

	for _, want := range []string{
		"gt_eq<>: [0.0]", // at_least
		"gt<>: [0.0]",    // more_than
		"lt_eq<>: [3.0]", // at_most
		"lt<>: [3.0]",    // less_than
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("spec lacks %q:\n%s", want, spec)
		}
	}
}

// bounds<> includes both of its ends, so a range with an excluded end has no
// single-validator form. The library applies a list in turn, which is the same
// constraint expressed as two.
func TestParamsAnExcludedEndSplitsTheRange(t *testing.T) {
	spec := render(t, exclusiveModel)["demo"]

	cases := map[string][]string{
		"open":    {"gt<>: [0.0]", "lt<>: [3.0]"},
		"half_lo": {"gt<>: [0.0]", "lt_eq<>: [3.0]"},
		"half_hi": {"gt_eq<>: [0.0]", "lt<>: [3.0]"},
	}

	for name, wants := range cases {
		section := paramSection(spec, name)
		for _, want := range wants {
			if !strings.Contains(section, want) {
				t.Errorf("%s lacks %q:\n%s", name, want, section)
			}
		}
		if strings.Contains(section, "bounds<>") {
			t.Errorf("%s used bounds<>, which includes its ends:\n%s", name, section)
		}
	}
}

// A closed range stays one validator, so nothing already written changes.
func TestParamsClosedRangeStillUsesBounds(t *testing.T) {
	section := paramSection(render(t, exclusiveModel)["demo"], "closed")

	if !strings.Contains(section, "bounds<>: [0.0, 3.0]") {
		t.Errorf("a closed range no longer emits bounds<>:\n%s", section)
	}
	for _, unwanted := range []string{"gt", "lt"} {
		if strings.Contains(section, unwanted+"<>") {
			t.Errorf("a closed range emitted %s<>:\n%s", unwanted, section)
		}
	}
}

// A string default may be written with or without quotes and means the same
// thing; quoting it twice would put the quotes in the generated default.
func TestParamsStringDefaultIsQuotedOnce(t *testing.T) {
	got := render(t, `RB-1-BASE:
    demo : node {dynamic late}
        bare : string (autonomous)
        quoted : string ("autonomous")
`)["demo"]

	if strings.Contains(got, `\"`) {
		t.Errorf("a quoted default was escaped rather than read as quoted:\n%s", got)
	}
	if strings.Count(got, `default_value: "autonomous"`) != 2 {
		t.Errorf("the two spellings disagree:\n%s", got)
	}
}

// paramSection returns just one parameter's block from a spec.
func paramSection(spec, name string) string {
	start := strings.Index(spec, "\n  "+name+":\n")
	if start < 0 {
		return ""
	}
	rest := spec[start+1:]

	// The next parameter starts at the next two-space-indented key.
	for i := 1; i < len(rest)-3; i++ {
		if rest[i] == '\n' && rest[i+1] == ' ' && rest[i+2] == ' ' && rest[i+3] != ' ' {
			return rest[:i]
		}
	}
	return rest
}
