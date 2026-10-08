package ftx

import "testing"

// modelSrc mirrors featxcli/featxcli/model/features.ftx.
const modelSrc = `feature_model: RB-1-BASE
    xor navigation ?: static early
        to_loading : dynamic late
        to_delivery : dynamic late
        to_base : dynamic late
    xor elevator ?: static early
        lift : dynamic late
        drop : dynamic late
    shelf ?: static early
        attach ?: dynamic late
    init_checks : static late
    boot : static late
`

// configSrc mirrors featxcli/featxcli/model/configs.ftx.
const configSrc = `feature_config:
    navigation *: static early
    to_loading : dynamic late
    to_delivery : dynamic late
    to_base : dynamic late
    elevator *: static early
    lift : dynamic late
    drop : dynamic late
    shelf *: static early
    attach : dynamic late
    init_checks : static late
    boot : static late
`

func TestParseModelHeader(t *testing.T) {
	f := Parse(modelSrc)

	if f.Kind != KindModel {
		t.Fatalf("Kind = %v, want %v", f.Kind, KindModel)
	}
	if f.ModelName != "RB-1-BASE" {
		t.Errorf("ModelName = %q, want %q", f.ModelName, "RB-1-BASE")
	}
	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestParseConfigHeader(t *testing.T) {
	f := Parse(configSrc)

	if f.Kind != KindConfig {
		t.Fatalf("Kind = %v, want %v", f.Kind, KindConfig)
	}
	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestParseEntryFields(t *testing.T) {
	f := Parse(modelSrc)

	nav, ok := f.Lookup("navigation")
	if !ok {
		t.Fatal("navigation not found")
	}

	if nav.Grouping != GroupXor {
		t.Errorf("Grouping = %v, want %v", nav.Grouping, GroupXor)
	}
	if !nav.Optional {
		t.Error("Optional = false, want true")
	}
	if nav.Selected {
		t.Error("Selected = true, want false")
	}
	if nav.Mode != ModeStatic {
		t.Errorf("Mode = %v, want %v", nav.Mode, ModeStatic)
	}
	if nav.Time != TimeEarly {
		t.Errorf("Time = %v, want %v", nav.Time, TimeEarly)
	}
	if got := len(nav.Children); got != 3 {
		t.Fatalf("len(Children) = %d, want 3", got)
	}

	want := []string{"to_loading", "to_delivery", "to_base"}
	for i, name := range want {
		if got := nav.Children[i].Name; got != name {
			t.Errorf("Children[%d].Name = %q, want %q", i, got, name)
		}
		if nav.Children[i].Parent != nav {
			t.Errorf("Children[%d].Parent is not navigation", i)
		}
	}
}

func TestParseNestingDepth(t *testing.T) {
	f := Parse(modelSrc)

	for _, tc := range []struct {
		name  string
		depth int
	}{
		{"navigation", 0},
		{"to_loading", 1},
		{"shelf", 0},
		{"attach", 1},
		{"boot", 0},
	} {
		ft, ok := f.Lookup(tc.name)
		if !ok {
			t.Errorf("%s not found", tc.name)
			continue
		}
		if got := ft.Depth(); got != tc.depth {
			t.Errorf("%s depth = %d, want %d", tc.name, got, tc.depth)
		}
	}

	if got := len(f.Roots); got != 5 {
		t.Errorf("len(Roots) = %d, want 5", got)
	}
}

func TestParseSelectionMarker(t *testing.T) {
	f := Parse(configSrc)

	nav, ok := f.Lookup("navigation")
	if !ok {
		t.Fatal("navigation not found")
	}
	if !nav.Selected {
		t.Error("navigation Selected = false, want true")
	}

	lift, ok := f.Lookup("lift")
	if !ok {
		t.Fatal("lift not found")
	}
	if lift.Selected {
		t.Error("lift Selected = true, want false")
	}
	if lift.HasMarker {
		t.Error("lift HasMarker = true, want false")
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "missing header",
			src:  "navigation: static early\n",
			want: `expected the document to start with "feature_model:" or "feature_config:", found "navigation"`,
		},
		{
			name: "model without a name",
			src:  "feature_model:\n",
			want: `expected a model name after "feature_model:"`,
		},
		{
			name: "config with a name",
			src:  "feature_config: RB-1-BASE\n",
			want: `"feature_config:" takes no name, found "RB-1-BASE"`,
		},
		{
			name: "unknown binding word",
			src:  "feature_model: M\n    nav: static sometimes\n",
			want: `unknown binding "sometimes": expected static, dynamic, early or late`,
		},
		{
			name: "missing binding time",
			src:  "feature_model: M\n    nav: static\n",
			want: `feature "nav" has no binding time (early|late)`,
		},
		{
			name: "missing binding mode",
			src:  "feature_model: M\n    nav: early\n",
			want: `feature "nav" has no binding mode (static|dynamic)`,
		},
		{
			name: "no binding at all",
			src:  "feature_model: M\n    nav:\n",
			want: `expected a binding mode (static|dynamic) and a binding time (early|late) after ":"`,
		},
		{
			name: "missing colon",
			src:  "feature_model: M\n    nav static early\n",
			want: `expected ":" after feature "nav", found "static"`,
		},
		{
			name: "duplicate feature",
			src:  "feature_model: M\n    nav: static early\n    nav: static early\n",
			want: `feature "nav" is already declared on line 2`,
		},
		{
			name: "two modes",
			src:  "feature_model: M\n    nav: static dynamic\n",
			want: `binding mode is already "static", found "dynamic"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := Parse(tc.src)
			if !hasMessage(f.Diagnostics, tc.want) {
				t.Errorf("missing diagnostic %q; got %v", tc.want, messages(f.Diagnostics))
			}
		})
	}
}

func TestParseRecoversAfterBadLine(t *testing.T) {
	src := "feature_model: M\n    broken\n    good: static early\n"

	f := Parse(src)

	if _, ok := f.Lookup("good"); !ok {
		t.Error("a malformed line stopped later entries from parsing")
	}
}

func TestParseInconsistentIndentation(t *testing.T) {
	src := "feature_model: M\n    a: static early\n      b: static early\n        c: static early\n"

	f := Parse(src)

	// b is a child of a; c is indented further still, so it is a child of b
	// rather than a sibling, and nothing is inconsistent.
	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}

	b, ok := f.Lookup("b")
	if !ok {
		t.Fatal("b not found")
	}
	if b.Parent == nil || b.Parent.Name != "a" {
		t.Errorf("b.Parent = %v, want a", b.Parent)
	}
}

func TestParseSiblingIndentationMismatch(t *testing.T) {
	// c is indented too shallow to be b's child but too deep to be a's
	// sibling, so it lands beside b on a different column.
	src := "feature_model: M\n    a: static early\n        b: static early\n      c: static early\n"

	f := Parse(src)

	if !hasMessage(f.Diagnostics, `inconsistent indentation: "c" is indented 6 columns, its siblings 8`) {
		t.Errorf("missing indentation warning; got %v", messages(f.Diagnostics))
	}

	c, ok := f.Lookup("c")
	if !ok {
		t.Fatal("c not found")
	}
	if c.Parent == nil || c.Parent.Name != "a" {
		t.Errorf("c.Parent = %v, want a", c.Parent)
	}
}

func TestTokenizeAdjacentMarker(t *testing.T) {
	// `?:` has no space between the marker and the colon.
	toks := tokenize("    xor navigation ?: static early")

	want := []string{"xor", "navigation", "?", ":", "static", "early"}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens %v, want %d", len(toks), texts(toks), len(want))
	}

	for i, w := range want {
		if toks[i].text != w {
			t.Errorf("token %d = %q, want %q", i, toks[i].text, w)
		}
	}

	if toks[0].col != 4 {
		t.Errorf("xor col = %d, want 4", toks[0].col)
	}
	if toks[1].col != 8 {
		t.Errorf("navigation col = %d, want 8", toks[1].col)
	}
}

func TestColumnsAreUTF16Units(t *testing.T) {
	// U+1F680 is a surrogate pair, so it counts as two UTF-16 code units.
	f := Parse("feature_model: M\n    \U0001F680ab: static early\n")

	ft, ok := f.Lookup("\U0001F680ab")
	if !ok {
		t.Fatalf("feature not found; got %v", f.Index)
	}

	if got := ft.NameRange.EndChar - ft.NameRange.StartChar; got != 4 {
		t.Errorf("name width = %d UTF-16 units, want 4", got)
	}
}

func TestIndentOfCountsTabs(t *testing.T) {
	if got := indentOf("\tfoo"); got != tabWidth {
		t.Errorf("indentOf(tab) = %d, want %d", got, tabWidth)
	}
	if got := indentOf("    foo"); got != 4 {
		t.Errorf("indentOf(4 spaces) = %d, want 4", got)
	}
}

func TestParseEmptyDocument(t *testing.T) {
	f := Parse("")

	if !hasMessage(f.Diagnostics, `empty document: expected a "feature_model:" or "feature_config:" header`) {
		t.Errorf("missing diagnostic; got %v", messages(f.Diagnostics))
	}
}

func texts(toks []token) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = t.text
	}
	return out
}

func messages(ds []Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Message
	}
	return out
}

func errorsOf(ds []Diagnostic) []string {
	var out []string
	for _, d := range ds {
		if d.Severity == SeverityError {
			out = append(out, d.Message)
		}
	}
	return out
}

func hasMessage(ds []Diagnostic, want string) bool {
	for _, d := range ds {
		if d.Message == want {
			return true
		}
	}
	return false
}
