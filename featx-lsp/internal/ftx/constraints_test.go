package ftx

import (
	"strings"
	"testing"
)

func TestParseRequiresConstraint(t *testing.T) {
	f := Parse(modelSrc)

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	ft, ok := f.Lookup("to_loading")
	if !ok {
		t.Fatal("to_loading not found")
	}

	if len(ft.Constraints) != 1 {
		t.Fatalf("len(Constraints) = %d, want 1", len(ft.Constraints))
	}

	c := ft.Constraints[0]

	if c.Negated {
		t.Error("Negated = true, want false")
	}
	if c.Op != OpAnd {
		t.Errorf("Op = %v, want %v", c.Op, OpAnd)
	}

	want := []string{"square", "attach", "lift"}
	got := names(c.Operands)

	if len(got) != len(want) {
		t.Fatalf("Operands = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Operands[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseExcludesConstraint(t *testing.T) {
	f := Parse("M:\n    a : node {static early}\n        ![b && c]\n" +
		"    b : node {static early}\n    c : node {static early}\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a, _ := f.Lookup("a")
	if len(a.Excludes()) != 1 {
		t.Fatalf("Excludes = %v, want one constraint", a.Excludes())
	}
	if len(a.Requires()) != 0 {
		t.Errorf("Requires = %v, want none", a.Requires())
	}
	if got := names(a.Excludes()[0].Operands); len(got) != 2 {
		t.Errorf("operands = %v, want 2", got)
	}
}

func TestParseDisjunctiveConstraint(t *testing.T) {
	f := Parse("M:\n    a : node {static early}\n        [b || c]\n" +
		"    b : node {static early}\n    c : node {static early}\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a, _ := f.Lookup("a")
	if got := a.Requires()[0].Op; got != OpOr {
		t.Errorf("Op = %v, want %v", got, OpOr)
	}
}

func TestParseSingleOperandConstraint(t *testing.T) {
	f := Parse("M:\n    a : node {static early}\n        [b]\n    b : node {static early}\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a, _ := f.Lookup("a")
	c := a.Requires()[0]

	if c.Op != OpNone {
		t.Errorf("Op = %v, want %v for a single operand", c.Op, OpNone)
	}
	if got := names(c.Operands); len(got) != 1 || got[0] != "b" {
		t.Errorf("Operands = %v, want [b]", got)
	}
}

func TestParseSeveralConstraintLines(t *testing.T) {
	f := Parse("M:\n    a : node {static early}\n        [b]\n    ![c]\n" +
		"    b : node {static early}\n    c : node {static early}\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a, _ := f.Lookup("a")
	if len(a.Constraints) != 2 {
		t.Fatalf("len(Constraints) = %d, want 2", len(a.Constraints))
	}
	if len(a.Requires()) != 1 || len(a.Excludes()) != 1 {
		t.Errorf("Requires = %v, Excludes = %v", a.Requires(), a.Excludes())
	}
}

func TestParseOperandRangesPointAtEachName(t *testing.T) {
	f := Parse("M:\n    a : node {static early}\n        [bb && ccc]\n" +
		"    bb : node {static early}\n    ccc : node {static early}\n")

	a, _ := f.Lookup("a")

	for _, ref := range a.Requires()[0].Operands {
		if got := ref.Range.EndChar - ref.Range.StartChar; got != len(ref.Name) {
			t.Errorf("%q spans %d columns, want %d", ref.Name, got, len(ref.Name))
		}
	}
}

func TestParseConstraintErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "no operands",
			src:  "M:\n    a : node {static early}\n        []\n",
			want: "a constraint needs at least one feature name",
		},
		{
			name: "trailing connective",
			src:  "M:\n    a : node {static early}\n        [b &&]\n    b : node {static early}\n",
			want: `a constraint cannot end with "&&"`,
		},
		{
			name: "leading connective",
			src:  "M:\n    a : node {static early}\n        [&& b]\n    b : node {static early}\n",
			want: `expected a feature name before "&&"`,
		},
		{
			name: "missing connective",
			src:  "M:\n    a : node {static early}\n        [b c]\n    b : node {static early}\n    c : node {static early}\n",
			want: `expected "&&" or "||" between feature names, found "c"`,
		},
		{
			name: "mixed connectives",
			src:  "M:\n    a : node {static early}\n        [b && c || d]\n    b : node {static early}\n    c : node {static early}\n    d : node {static early}\n",
			want: `a constraint cannot mix "&&" and "||"; split it into separate constraints`,
		},
		{
			name: "unclosed bracket",
			src:  "M:\n    a : node {static early}\n        [b\n",
			want: `unclosed "[" in a constraint`,
		},
		{
			name: "bang without a bracket",
			src:  "M:\n    a : node {static early}\n    !b\n",
			want: `expected "[" after "!" to write an exclusion`,
		},
		{
			name: "constraint before any feature",
			src:  "[b]\n",
			want: "a constraint must follow the feature it applies to",
		},
		{
			name: "type keyword as an operand",
			src:  "M:\n    a : node {static early}\n        [node]\n",
			want: `"node" is not a valid feature name in a constraint`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if f := Parse(tc.src); !hasMessage(f.Diagnostics, tc.want) {
				t.Errorf("missing diagnostic %q; got %v", tc.want, messages(f.Diagnostics))
			}
		})
	}
}

func TestParseConstraintIndentationMismatch(t *testing.T) {
	// Level with its feature rather than beneath it.
	f := Parse("M:\n    a : node {static early}\n    [b]\n    b : node {static early}\n")

	want := `a constraint is written beneath the feature it applies to: indent it past "a" at 4 columns`
	if !hasMessage(f.Diagnostics, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(f.Diagnostics))
	}
}

func TestAnalyzeShapeChecks(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "node with a value",
			src:  "M:\n    a : node {static early} (3)\n",
			want: `a node has no value of its own; "()" belongs on a parameter`,
		},
		{
			name: "value feature with children",
			src:  "M:\n    a : node {static early}\n        n : double (1)\n            m : double (2)\n",
			want: `"n" is a double and cannot contain features; only a node can`,
		},
		{
			name: "top-level value feature",
			src:  "M:\n    n : double (1)\n",
			want: `"n" is a double but no enclosing node is bound with it`,
		},
		{
			name: "ill-typed number",
			src:  "M:\n    a : node {static early}\n        n : double (fast)\n",
			want: `"fast" is not a double`,
		},
		{
			name: "ill-typed bool",
			src:  "M:\n    a : node {static early}\n        b : bool (yes)\n",
			want: `"yes" is not a bool: expected true or false`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := Parse(tc.src)
			if got := Analyze(f); !hasMessage(got, tc.want) {
				t.Errorf("missing diagnostic %q; got %v", tc.want, messages(got))
			}
		})
	}
}

func TestAnalyzeAcceptsWellTypedValues(t *testing.T) {
	src := "M:\n    a : node {static early}\n" +
		"        n : double (-2.5e3)\n        s : string (a b c)\n        b : bool (false)\n"

	for _, d := range Analyze(Parse(src)) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestAnalyzeConstraintChecks(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "undeclared operand",
			src:  "M:\n    a : node {static early}\n        [ghost]\n",
			want: `feature "ghost" is not declared in this feature model`,
		},
		{
			name: "requires itself",
			src:  "M:\n    a : node {static early}\n        [a]\n",
			want: `feature "a" requires itself`,
		},
		{
			name: "excludes itself",
			src:  "M:\n    a : node {static early}\n        ![a]\n",
			want: `feature "a" excludes itself`,
		},
		{
			name: "requires and excludes the same feature",
			src:  "M:\n    a : node {static early}\n        [b]\n    ![b]\n    b : node {static early}\n",
			want: `feature "a" both requires and excludes "b"`,
		},
		{
			name: "duplicate operand",
			src:  "M:\n    a : node {static early}\n        [b && b]\n    b : node {static early}\n",
			want: `"b" is listed twice in the same constraint on "a"`,
		},
		{
			name: "requires an enclosing feature",
			src:  "M:\n    a : node {static early}\n        b : node {static early}\n            [a]\n",
			want: `"a" encloses "b", so requiring it is redundant`,
		},
		{
			name: "requires both a feature and its ancestor",
			src: "M:\n    a : node {static early}\n        [b && c]\n" +
				"    b : node {static early}\n        c : node {static early}\n",
			want: `"b" encloses "c", which "a" also requires, so listing it is redundant`,
		},
		{
			name: "excludes a descendant",
			src:  "M:\n    a : node {static early}\n        ![b]\n        b : node {static early}\n",
			want: `"a" and "b" are nested within one another, so excluding makes the inner one unselectable`,
		},
		{
			name: "or in an exclusion",
			src: "M:\n    a : node {static early}\n        ![b || c]\n" +
				"    b : node {static early}\n    c : node {static early}\n",
			want: `"||" in an exclusion has no effect: every operand is forbidden either way`,
		},
		{
			name: "two-feature cycle",
			src:  "M:\n    a : node {static early}\n        [b]\n    b : node {static early}\n        [a]\n",
			want: `requirement cycle: a requires b requires a`,
		},
		{
			name: "three-feature cycle",
			src: "M:\n    a : node {static early}\n        [b]\n" +
				"    b : node {static early}\n    [c]\n    c : node {static early}\n        [a]\n",
			want: `requirement cycle: a requires b requires c requires a`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := Parse(tc.src)
			if got := Analyze(f); !hasMessage(got, tc.want) {
				t.Errorf("missing diagnostic %q; got %v", tc.want, messages(got))
			}
		})
	}
}

// TestAnalyzeDisjunctionIsNotACycle checks that an escape route stops a loop
// from being reported: a || requirement can be satisfied another way.
func TestAnalyzeDisjunctionIsNotACycle(t *testing.T) {
	src := "M:\n    a : node {static early}\n        [b || c]\n" +
		"    b : node {static early}\n    [a]\n    c : node {static early}\n"

	for _, d := range Analyze(Parse(src)) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestAnalyzeWarnsOnConstraintsInConfig(t *testing.T) {
	f := Parse("a *{static early}\n[b]\n")

	want := `"[...]" is a constraint on the feature model and has no meaning in a configuration`
	if got := Analyze(f); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestAnalyzeAcceptsTheRealModel(t *testing.T) {
	for _, d := range Analyze(Parse(modelSrc)) {
		if d.Severity == SeverityError {
			t.Errorf("the project's own feature model should analyse clean, got %q", d.Message)
		}
	}
}

// TestCrossCheckParameterTakesNoBinding: a parameter is bound with the node it
// belongs to, so a configuration gives it a selection but not a binding.
func TestCrossCheckParameterTakesNoBinding(t *testing.T) {
	cfg := Parse("vel_max *{dynamic late}\n")

	want := `"vel_max" is a double parameter of "navigation" and is bound with it, ` +
		`so it takes no binding of its own`
	if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

// TestCrossCheckNodeNeedsABinding is the mirror: the type decides which form
// an entry takes, and the parser cannot tell because a configuration declares
// no types.
func TestCrossCheckNodeNeedsABinding(t *testing.T) {
	cfg := Parse("navigation *\n")

	want := `node "navigation" must be configured with a binding, as in "navigation *{static early}"`
	if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

// TestCrossCheckSelectionReachesParameters: a parameter is part of its node, so
// selecting the node selects the parameter too.
func TestCrossCheckSelectionReachesParameters(t *testing.T) {
	omitted := Parse("navigation *{static early}\n")

	want := `"navigation" is selected, so its double parameter "vel_max" must be selected too, ` +
		`but this configuration omits it`
	if got := CrossCheck(omitted, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}

	unselected := Parse("navigation *{static early}\nvel_max\n")

	want = `"vel_max" must be selected because its parent "navigation" is selected`
	if got := CrossCheck(unselected, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}

	// Selected, so nothing to report.
	selected := Parse("navigation *{static early}\nvel_max *\n")

	for _, d := range CrossCheck(selected, Parse(modelSrc)) {
		if d.Severity == SeverityError && strings.Contains(d.Message, "vel_max") {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// TestCrossCheckParameterFreeWhenNodeUnselected: the rule only bites when the
// enclosing node is actually selected.
func TestCrossCheckParameterFreeWhenNodeUnselected(t *testing.T) {
	cfg := Parse("navigation {static early}\nvel_max\n")

	for _, d := range CrossCheck(cfg, Parse(modelSrc)) {
		if d.Severity == SeverityError && strings.Contains(d.Message, "vel_max") {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestCrossCheckConfiguredValueMustBeWellTyped(t *testing.T) {
	cfg := Parse("vel_max *(quick)\n")

	want := `"quick" is not a double`
	if got := CrossCheck(cfg, Parse(modelSrc)); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckUnsatisfiedRequirement(t *testing.T) {
	model := Parse(modelSrc)
	cfg := Parse("" +
		"    navigation *{static early}\n" +
		"    vel_max *\n" +
		"    to_loading *{dynamic late}\n" +
		"    square {dynamic late}\n" +
		"    attach {dynamic late}\n" +
		"    lift {dynamic late}\n")

	want := `feature "to_loading" is selected, so "square" must be selected too`
	if got := CrossCheck(cfg, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckDisjunctiveRequirement(t *testing.T) {
	model := Parse("M:\n    a : node {static early}\n        [b || c]\n" +
		"    b : node {static early}\n    c : node {static early}\n")

	unsatisfied := Parse("a *{static early}\n" +
		"    b {static early}\n    c {static early}\n")

	want := `feature "a" is selected and requires one of "b", "c", none of which is selected`
	if got := CrossCheck(unsatisfied, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}

	// One operand is enough to satisfy a disjunction.
	satisfied := Parse("a *{static early}\n" +
		"    b *{static early}\n    c {static early}\n")

	for _, d := range CrossCheck(satisfied, model) {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestCrossCheckExclusionViolated(t *testing.T) {
	model := Parse("M:\n    a : node {static early}\n        ![b]\n    b : node {static early}\n")
	cfg := Parse("a *{static early}\nb *{static early}\n")

	want := `features "a" and "b" exclude each other and cannot both be selected`
	if got := CrossCheck(cfg, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestCrossCheckAcceptsTheRealPair(t *testing.T) {
	for _, d := range CrossCheck(Parse(configSrc), Parse(modelSrc)) {
		if d.Severity == SeverityError {
			t.Errorf("the project's own model and configuration should agree, got %q", d.Message)
		}
	}
}

func TestFormatPreservesConstraints(t *testing.T) {
	src := "M:\n    a : node {static early}\n        [b&&c]\n    ![d]\n" +
		"    b : node {static early}\n    c : node {static early}\n    d : node {static early}\n"

	got, ok := Format(Parse(src))
	if !ok {
		t.Fatalf("Format refused: %v", messages(Parse(src).Diagnostics))
	}

	want := "M:\n    a : node {static early}\n        [b && c]\n        ![d]\n" +
		"    b : node {static early}\n    c : node {static early}\n    d : node {static early}\n"

	if got != want {
		t.Errorf("Format() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatPreservesTypesAndValues(t *testing.T) {
	src := "M:\n    a : node{static early}\n        n : double(1.5)\n"

	got, ok := Format(Parse(src))
	if !ok {
		t.Fatalf("Format refused: %v", messages(Parse(src).Diagnostics))
	}

	want := "M:\n    a : node {static early}\n        n : double (1.5)\n"
	if got != want {
		t.Errorf("Format() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatModelIsStable(t *testing.T) {
	got, ok := Format(Parse(modelSrc))
	if !ok {
		t.Fatal("Format refused the model")
	}
	if got != modelSrc {
		t.Errorf("formatting changed the model:\ngot:\n%s\nwant:\n%s", got, modelSrc)
	}
}

func TestFormatConfigIsStable(t *testing.T) {
	got, ok := Format(Parse(configSrc))
	if !ok {
		t.Fatal("Format refused the configuration")
	}
	if got != configSrc {
		t.Errorf("formatting changed the configuration:\ngot:\n%s\nwant:\n%s", got, configSrc)
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	once, ok := Format(Parse(modelSrc))
	if !ok {
		t.Fatal("Format refused the model")
	}

	twice, ok := Format(Parse(once))
	if !ok {
		t.Fatal("Format refused its own output")
	}

	if once != twice {
		t.Errorf("Format is not idempotent:\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
}

func TestFormatRefusesBrokenDocuments(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"no header", "a : node {static early}\n"},
		{"node without bindings", "M:\n    a : node\n"},
		{"unknown type", "M:\n    a : widget\n"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := Format(Parse(tc.src)); ok {
				t.Error("Format accepted a document with syntax errors")
			}
		})
	}
}

// TestAnalyzeRejectsGroupedParameter: a grouping constrains which children may
// be selected, and a parameter has none, ever.
func TestAnalyzeRejectsGroupedParameter(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "xor on a number",
			src:  "M:\n    nav : node {static early}\n        xor vel_max : double (1.5)\n",
			want: `a double parameter cannot carry "xor": only a node groups children`,
		},
		{
			name: "or on a string",
			src:  "M:\n    nav : node {static early}\n        or mode : string (fast)\n",
			want: `a string parameter cannot carry "or": only a node groups children`,
		},
		{
			name: "xor on a bool",
			src:  "M:\n    nav : node {static early}\n        xor flag : bool (true)\n",
			want: `a bool parameter cannot carry "xor": only a node groups children`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Analyze(Parse(tc.src))

			if !hasMessage(got, tc.want) {
				t.Errorf("missing diagnostic %q; got %v", tc.want, messages(got))
			}

			// The type rule replaces the childless-grouping message rather
			// than adding to it.
			for _, d := range got {
				if strings.Contains(d.Message, "which has none") {
					t.Errorf("a parameter should not be told it has no children: %s", d.Message)
				}
			}
		})
	}
}

// TestCrossCheckOrAllowsSeveralSelections pins the contrast with xor: an or
// group places no upper bound on how many children may be selected.
func TestCrossCheckOrAllowsSeveralSelections(t *testing.T) {
	model := Parse("M:\n    or sensors : node {static early}\n" +
		"        lidar : node {dynamic late}\n        sonar : node {dynamic late}\n" +
		"        radar : node {dynamic late}\n")

	cfg := Parse("sensors *{static early}\nlidar *{dynamic late}\n" +
		"sonar *{dynamic late}\nradar {dynamic late}\n")

	for _, d := range CrossCheck(cfg, model) {
		if d.Severity == SeverityError {
			t.Errorf("an or group may have several children selected, got %q", d.Message)
		}
	}
}

// TestCrossCheckXorAllowsOnlyOne is the mirror of the above.
func TestCrossCheckXorAllowsOnlyOne(t *testing.T) {
	model := Parse("M:\n    xor footprint : node {static early}\n" +
		"        circular : node {dynamic late}\n        square : node {dynamic late}\n")

	one := Parse("footprint *{static early}\ncircular *{dynamic late}\nsquare {dynamic late}\n")

	for _, d := range CrossCheck(one, model) {
		if d.Severity == SeverityError {
			t.Errorf("one selected child is allowed, got %q", d.Message)
		}
	}

	two := Parse("footprint *{static early}\ncircular *{dynamic late}\nsquare *{dynamic late}\n")

	want := `"footprint" is an xor group, so only one of its children may be selected; ` +
		`"circular", "square" are all selected`

	if got := CrossCheck(two, model); !hasMessage(got, want) {
		t.Errorf("missing diagnostic %q; got %v", want, messages(got))
	}
}

func TestParseAllValidatorForms(t *testing.T) {
	src := "M:\n    n : node {dynamic late}\n" +
		"        a : double (1.5) <0.0, 3.0>\n" +
		"        b : double (1.0) <0.0, #>\n" +
		"        c : double (-0.01) <#, 0.0>\n" +
		"        d : double (7.0) <<0.0, 3.0, 7.0>>\n" +
		"        e : double_array (0.0, 0.0, 0.0, 0.0) <4>\n"

	f := Parse(src)
	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, d := range Analyze(f) {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}

	for _, tc := range []struct {
		name string
		kind ValidatorKind
		args []string
	}{
		{"a", ValidatorBounds, []string{"0.0", "3.0"}},
		{"b", ValidatorGtEq, []string{"0.0"}},
		{"c", ValidatorLtEq, []string{"0.0"}},
		{"d", ValidatorOneOf, []string{"0.0", "3.0", "7.0"}},
		{"e", ValidatorFixedSize, []string{"4"}},
	} {
		ft, ok := f.Lookup(tc.name)
		if !ok {
			t.Errorf("%s not found", tc.name)
			continue
		}
		if ft.Validator.Kind != tc.kind {
			t.Errorf("%s kind = %v, want %v", tc.name, ft.Validator.Kind, tc.kind)
		}
		if got := strings.Join(ft.Validator.Args, ","); got != strings.Join(tc.args, ",") {
			t.Errorf("%s args = %v, want %v", tc.name, ft.Validator.Args, tc.args)
		}
	}
}

func TestParseArrayValue(t *testing.T) {
	f := Parse("M:\n    n : node {static early}\n        a : double_array (1.0, 2.5, 3.0)\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a, _ := f.Lookup("a")
	if got := a.DefaultValues; len(got) != 3 || got[0] != "1.0" || got[2] != "3.0" {
		t.Errorf("DefaultValues = %v, want [1.0 2.5 3.0]", got)
	}
	if got := a.DefaultText(); got != "1.0, 2.5, 3.0" {
		t.Errorf("DefaultText = %q", got)
	}
}

// TestParseStringValueKeepsSpaces: elements are comma-separated, so the tokens
// inside one element rejoin and a string needs no quoting.
func TestParseStringValueKeepsSpaces(t *testing.T) {
	f := Parse("M:\n    n : node {static early}\n        s : string (a b c)\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	s, _ := f.Lookup("s")
	if got := s.DefaultValues; len(got) != 1 || got[0] != "a b c" {
		t.Errorf("DefaultValues = %v, want [\"a b c\"]", got)
	}
}

func TestAnalyzeTypeAndValidatorErrors(t *testing.T) {
	const head = "M:\n    n : node {static early}\n        "

	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "int given a double",
			src:  head + "a : int (1.5)\n",
			want: `"1.5" is not an int`,
		},
		{
			name: "double given a word",
			src:  head + "a : double (fast)\n",
			want: `"fast" is not a double`,
		},
		{
			name: "scalar given several values",
			src:  head + "a : double (1.0, 2.0)\n",
			want: `"a" is a double and takes one value, not 2`,
		},
		{
			name: "array element ill-typed",
			src:  head + "a : int_array (1, two, 3)\n",
			want: `"two" is not an int`,
		},
		{
			name: "fixed_size on a scalar",
			src:  head + "a : double (1.0) <4>\n",
			want: `"<n>" fixes an array's length, and "a" is a double`,
		},
		{
			name: "fixed_size disagrees with the value",
			src:  head + "a : double_array (1.0, 2.0) <3>\n",
			want: `the value has 2 element(s) but "fixed_size<>" requires 3`,
		},
		{
			name: "range on a string",
			src:  head + "a : string (x) <0.0, 1.0>\n",
			want: `a range needs a numeric type, and "a" is a string`,
		},
		{
			name: "empty range",
			src:  head + "a : double (1.0) <3.0, 0.0>\n",
			want: "the range is empty: 3.0 is greater than 0.0",
		},
		{
			name: "default below the minimum",
			src:  head + "a : double (-1.0) <0.0, 3.0>\n",
			want: "the value -1.0 is below the permitted minimum 0.0",
		},
		{
			name: "default above the maximum",
			src:  head + "a : double (9.0) <0.0, 3.0>\n",
			want: "the value 9.0 is above the permitted maximum 3.0",
		},
		{
			name: "default outside the enumeration",
			src:  head + "a : double (5.0) <<0.0, 3.0>>\n",
			want: "the value 5.0 is not one of 0.0, 3.0",
		},
		{
			name: "validator on a node",
			src:  "M:\n    n : node {static early} <0, 1>\n",
			want: "a node cannot carry a validator; only a parameter constrains its own value",
		},
		{
			name: "both ends open",
			src:  head + "a : double (1.0) <#, #>\n",
			want: `both ends are "#", so this constrains nothing`,
		},
		{
			name: "too many range arguments",
			src:  head + "a : double (1.0) <0.0, 1.0, 2.0>\n",
			want: `a range takes two arguments; use "<<a, b, c>>" for an enumeration`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := Parse(tc.src)
			got := append(f.Diagnostics, Analyze(f)...)

			if !hasMessage(got, tc.want) {
				t.Errorf("missing diagnostic %q; got %v", tc.want, messages(got))
			}
		})
	}
}

func TestFormatPreservesValidators(t *testing.T) {
	src := "M:\n    n : node {static early}\n" +
		"        a : double (1.5) <0.0,3.0>\n" +
		"        b : double_array (1.0,2.0) <2>\n" +
		"        c : string (x) <<x, y>>\n"

	got, ok := Format(Parse(src))
	if !ok {
		t.Fatalf("Format refused: %v", messages(Parse(src).Diagnostics))
	}

	want := "M:\n    n : node {static early}\n" +
		"        a : double (1.5) <0.0, 3.0>\n" +
		"        b : double_array (1.0, 2.0) <2>\n" +
		"        c : string (x) <<x, y>>\n"

	if got != want {
		t.Errorf("Format() =\n%q\nwant\n%q", got, want)
	}
}
