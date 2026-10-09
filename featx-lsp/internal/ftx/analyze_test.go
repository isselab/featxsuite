package ftx

import "testing"

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
