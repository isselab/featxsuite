package ftx

import "testing"

func TestAnalyzeRejectsSelectionInModel(t *testing.T) {
	f := Parse("feature_model: M\n    nav *: static early\n")

	want := `"*" selects a feature, which belongs in a feature configuration, not a model`
	if !hasMessage(Analyze(f), want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(Analyze(f)))
	}
}

func TestAnalyzeGroupingWithoutChildren(t *testing.T) {
	f := Parse("feature_model: M\n    xor nav: static early\n")

	want := `"xor" groups the children of "nav", which has none`
	if !hasMessage(Analyze(f), want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(Analyze(f)))
	}
}

func TestAnalyzeGroupingOverSingleChild(t *testing.T) {
	f := Parse("feature_model: M\n    xor nav: static early\n        only: dynamic late\n")

	want := `"xor" over the single child "only" has no effect`
	if !hasMessage(Analyze(f), want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(Analyze(f)))
	}
}

func TestAnalyzeWarnsOnOptionalInConfig(t *testing.T) {
	f := Parse("feature_config:\n    nav ?: static early\n")

	want := `"?" declares a feature optional, which belongs in a feature model; use "*" to select it`
	if !hasMessage(Analyze(f), want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(Analyze(f)))
	}
}

func TestAnalyzeWarnsOnGroupingInConfig(t *testing.T) {
	// configs.ftx carries `xor elevator ?: static early`, which is the case
	// this rule is here to catch.
	f := Parse("feature_config:\n    xor elevator ?: static early\n")

	want := `"xor" groups children in a feature model and has no meaning in a configuration`
	if !hasMessage(Analyze(f), want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(Analyze(f)))
	}
}

func TestAnalyzeAcceptsTheRealModel(t *testing.T) {
	f := Parse(modelSrc)

	if errs := errorsOf(Analyze(f)); len(errs) != 0 {
		t.Errorf("the project's own feature model should analyse clean, got %v", errs)
	}
}

func TestCrossCheckUnknownFeature(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("feature_config:\n    nonsense: static early\n")

	want := `feature "nonsense" is not declared in feature model "RB-1-BASE"`
	if got := CrossCheck(cfg, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckBindingMismatch(t *testing.T) {
	model := Parse(modelSrc)

	// The model allows navigation only `static early`.
	cfg := Parse("feature_config:\n    navigation: dynamic late\n")

	for _, want := range []string{
		`feature "navigation" is configured "dynamic" but the model allows only "static"`,
		`feature "navigation" is configured "late" but the model allows only "early"`,
	} {
		if got := CrossCheck(cfg, model); !hasMessage(got, want) {
			t.Errorf("missing diagnostic %q; got %v", want, messages(got))
		}
	}
}

func TestCrossCheckSelectedChildOfUnselectedParent(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("feature_config:\n    navigation: static early\n    to_loading *: dynamic late\n")

	want := `feature "to_loading" is selected but its parent "navigation" is not`
	if got := CrossCheck(cfg, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckAllowsSelectedChildOfSelectedParent(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("feature_config:\n    navigation *: static early\n    to_loading *: dynamic late\n")

	for _, d := range CrossCheck(cfg, model) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestCrossCheckXorAllowsOneSelection(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("feature_config:\n" +
		"    navigation *: static early\n" +
		"    to_loading *: dynamic late\n" +
		"    to_delivery : dynamic late\n")

	for _, d := range CrossCheck(cfg, model) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestCrossCheckXorRejectsTwoSelections(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("feature_config:\n" +
		"    navigation *: static early\n" +
		"    to_loading *: dynamic late\n" +
		"    to_delivery *: dynamic late\n")

	want := `"navigation" is an xor group, so only one of its children may be selected; ` +
		`"to_loading", "to_delivery" are all selected`

	if got := CrossCheck(cfg, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckReportsCoverage(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("feature_config:\n    navigation *: static early\n")

	var hint *Diagnostic
	for _, d := range CrossCheck(cfg, model) {
		if d.Severity == SeverityHint {
			hint = &d
			break
		}
	}

	if hint == nil {
		t.Fatal("expected a coverage hint")
	}

	// Everything but navigation is missing: 10 of the model's 11 features.
	want := `feature model "RB-1-BASE" declares 10 feature(s) this configuration omits: ` +
		`attach, boot, drop, elevator, init_checks, lift, shelf, to_base, to_delivery, to_loading`

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

func TestCrossCheckAcceptsTheRealPair(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse(configSrc)

	for _, d := range CrossCheck(cfg, model) {
		if d.Severity == SeverityError {
			t.Errorf("the project's own model and configuration should agree, got %q", d.Message)
		}
	}
}
