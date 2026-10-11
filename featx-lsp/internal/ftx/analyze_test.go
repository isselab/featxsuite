package ftx

import (
	"strings"
	"testing"
)

func TestAnalyzeRejectsSelectionInModel(t *testing.T) {
	f := Parse("M:\n    nav : node {static early} *\n")

	want := `"*" selects a feature, which belongs in a feature configuration, not a model`
	if got := Analyze(f); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestAnalyzeGroupingWithoutChildren(t *testing.T) {
	f := Parse("M:\n    xor nav : node {static early}\n")

	want := `"xor" groups the children of "nav", which has none`
	if got := Analyze(f); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestAnalyzeGroupingOverSingleChild(t *testing.T) {
	f := Parse("M:\n    xor nav : node {static early}\n        only : node {dynamic late}\n")

	want := `"xor" over the single child "only" has no effect`
	if got := Analyze(f); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestAnalyzeWarnsOnOptionalInConfig(t *testing.T) {
	f := Parse("nav ?{static early}\n")

	want := `"?" declares a feature optional, which belongs in a feature model; use "*" to select it`
	if got := Analyze(f); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestAnalyzeWarnsOnGroupingInConfig(t *testing.T) {
	f := Parse("xor elevator ?{static early}\n")

	want := `"xor" groups children in a feature model and has no meaning in a configuration`
	if got := Analyze(f); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckUnknownFeature(t *testing.T) {
	cfg := Parse("nonsense {static early}\n")

	want := `feature "nonsense" is not declared in feature model "RB-1-BASE"`
	if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckBindingMismatch(t *testing.T) {
	// The model allows navigation only `static early`.
	cfg := Parse("navigation {dynamic late}\n")

	for _, want := range []string{
		`feature "navigation" is configured "dynamic" but the model allows only "static"`,
		`feature "navigation" is configured "late" but the model allows only "early"`,
	} {
		if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
			t.Errorf("missing diagnostic %q; got %v", want, messages(got))
		}
	}
}

func TestCrossCheckSelectedChildOfUnselectedParent(t *testing.T) {
	cfg := Parse("navigation {static early}\n" +
		"    to_base *{dynamic late}\n")

	want := `feature "to_base" is selected but its parent "navigation" is not`
	if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckAllowsSelectedChildOfSelectedParent(t *testing.T) {
	cfg := Parse("navigation *{static early}\n" +
		"    vel_max *\n    to_base *{dynamic late}\n")

	for _, d := range CrossCheck(cfg, Parse(modelSrc)) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestCrossCheckXorAllowsOneSelection(t *testing.T) {
	cfg := Parse("elevator *{static early}\n" +
		"    lift *{dynamic late}\n    drop {dynamic late}\n")

	for _, d := range CrossCheck(cfg, Parse(modelSrc)) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestCrossCheckXorRejectsTwoSelections(t *testing.T) {
	cfg := Parse("elevator *{static early}\n" +
		"    lift *{dynamic late}\n    drop *{dynamic late}\n")

	want := `"elevator" is an xor group, so only one of its children may be selected; ` +
		`"lift", "drop" are all selected`

	if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckReportsCoverage(t *testing.T) {
	cfg := Parse("navigation *{static early}\n")

	var hint *Diagnostic
	for _, d := range CrossCheck(cfg, Parse(modelSrc)) {
		if d.Severity == SeverityHint {
			hint = &d
			break
		}
	}

	if hint == nil {
		t.Fatal("expected a coverage hint")
	}

	// Everything but navigation is missing: 15 of the model's 16 features.
	want := `feature model "RB-1-BASE" declares 15 feature(s) this configuration omits: ` +
		`attach, boot, circular, drop, elevator, init_checks, lift, localization, pose, shelf, ` +
		`square, to_base, to_delivery, to_loading, vel_max`

	if hint.Message != want {
		t.Errorf("hint = %q, want %q", hint.Message, want)
	}
}

func TestCrossCheckIgnoresMismatchedKinds(t *testing.T) {
	model := Parse(modelSrc)

	if got := CrossCheck(model, model); got != nil {
		t.Errorf("a model checked against itself should yield nothing, got %v", messages(got))
	}
	if got := CrossCheck(nil, model); got != nil {
		t.Errorf("a nil configuration should yield nothing, got %v", messages(got))
	}
}

// A strict bound must be checked as thoroughly as an inclusive one. These
// escaped both checks when gt and lt were added, because the analyser asked
// which kinds a validator was not rather than what it constrains.
func TestAnalyzeStrictRangesAreCheckedToo(t *testing.T) {
	cases := map[string]string{
		`p : string (fast) gt<0.0>`:  `gt<> needs a numeric type, and "p" is a string`,
		`p : double (5.0) gt<10.0>`:  "the value 5.0 is below the permitted minimum 10.0",
		`p : double (10.0) gt<10.0>`: "the value 10.0 equals the excluded minimum 10.0",
		`p : double (3.0) lt<3.0>`:   "the value 3.0 equals the excluded maximum 3.0",
	}

	for entry, want := range cases {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        " + entry + "\n")

		if got := Analyze(f); !hasMessage(got, want) {
			t.Errorf("%s: missing diagnostic %q; got %v", entry, want, messages(got))
		}
	}
}

// An inclusive bound admits its own value, so the boundary is legal there.
func TestAnalyzeInclusiveBoundaryIsAccepted(t *testing.T) {
	for _, entry := range []string{
		`p : double (10.0) gt_eq<10.0>`,
		`p : double (3.0) lt_eq<3.0>`,
		`p : double (3.0) bounds<0.0, 3.0>`,
	} {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        " + entry + "\n")

		for _, d := range Analyze(f) {
			if d.Severity == SeverityError {
				t.Errorf("%s: unexpected error %q", entry, d.Message)
			}
		}
	}
}

// bounds gives both ends, so anything else constraining the value states a
// limit the parameter already has.
func TestAnalyzeBoundsStandsAlone(t *testing.T) {
	conflicts := map[string]string{
		"bounds<0.0, 3.0> gt<1.0>":          "gt<>",
		"bounds<0.0, 3.0> gt_eq<1.0>":       "gt_eq<>",
		"bounds<0.0, 3.0> lt<2.0>":          "lt<>",
		"bounds<0.0, 3.0> lt_eq<2.0>":       "lt_eq<>",
		"bounds<0.0, 3.0> one_of<1.0, 1.5>": "one_of<>",
	}

	for src, named := range conflicts {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.5) " + src + "\n")

		var found bool
		for _, d := range Analyze(f) {
			if strings.Contains(d.Message, named) && strings.Contains(d.Message, "bounds<>") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: %s beside bounds<> was accepted; got %v", src, named, messages(Analyze(f)))
		}
	}
}

// The pair that replaces a one-sided bounds has to stay legal, or there is no
// way to write a range open at an end.
func TestAnalyzeComparisonsPairWithoutBounds(t *testing.T) {
	for _, src := range []string{
		"gt<0.0> lt<3.0>",
		"gt<0.0> lt_eq<3.0>",
		"gt_eq<0.0> lt<3.0>",
		"bounds<0.0, 3.0>",
	} {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.5) " + src + "\n")

		for _, d := range Analyze(f) {
			if d.Severity == SeverityError {
				t.Errorf("%s: unexpected error %q", src, d.Message)
			}
		}
	}
}
