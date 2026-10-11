package ftx

import "testing"

// modelSrc mirrors featxcli/featxcli/model/features.ftx.
const modelSrc = `RB-1-BASE:
    xor navigation : node {static early} ?
        vel_max : double (1.5)
        to_loading : node {dynamic late}
            [square && attach && lift]
        to_delivery : node {dynamic late}
            [circular && drop]
        to_base : node {dynamic late}
    xor elevator : node {static early} ?
        lift : node {dynamic late}
        drop : node {dynamic late}
    shelf : node {static early} ?
        attach : node {dynamic late} ?
    init_checks : node {static late}
        [boot && pose]
    localization : node {static late}
        pose : node {static late}
    boot : node {static late}
    square : node {dynamic late}
    circular : node {dynamic late}
`

// configSrc mirrors featxcli/featxcli/model/configs.ftx.
const configSrc = `navigation *{static early}
vel_max *
to_loading {dynamic late}
to_delivery {dynamic late}
to_base {dynamic late}
elevator *{static early}
lift {dynamic late}
drop {dynamic late}
shelf *{static early}
attach {dynamic late}
init_checks {static late}
localization {static late}
pose {static late}
boot {static late}
square {dynamic late}
circular {dynamic late}
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
	if nav.Type != TypeNode {
		t.Errorf("Type = %v, want node", nav.Type)
	}
	if !nav.HasBindings {
		t.Error("HasBindings = false, want true")
	}
	if nav.Mode != ModeStatic {
		t.Errorf("Mode = %v, want %v", nav.Mode, ModeStatic)
	}
	if nav.Time != TimeEarly {
		t.Errorf("Time = %v, want %v", nav.Time, TimeEarly)
	}
	if nav.HasDefault {
		t.Error("a node must not carry a default")
	}

	want := []string{"vel_max", "to_loading", "to_delivery", "to_base"}
	if got := len(nav.Children); got != len(want) {
		t.Fatalf("len(Children) = %d, want %d", got, len(want))
	}
	for i, name := range want {
		if got := nav.Children[i].Name; got != name {
			t.Errorf("Children[%d].Name = %q, want %q", i, got, name)
		}
	}
}

func TestParseTypedParameter(t *testing.T) {
	f := Parse(modelSrc)

	vel, ok := f.Lookup("vel_max")
	if !ok {
		t.Fatal("vel_max not found")
	}

	if vel.Type != TypeDouble {
		t.Errorf("Type = %v, want number", vel.Type)
	}
	if !vel.HasDefault {
		t.Fatal("HasDefault = false, want true")
	}
	if vel.DefaultText() != "1.5" {
		t.Errorf("Default = %q, want %q", vel.DefaultText(), "1.5")
	}
	if vel.HasBindings {
		t.Error("HasBindings = true: vel_max declares none")
	}

	// With no bindings of its own, the enclosing node's apply.
	if n := vel.BindingNode(); n == nil || n.Name != "navigation" {
		t.Errorf("BindingNode = %v, want navigation", n)
	}
}

func TestParseAllTypes(t *testing.T) {
	src := "M:\n" +
		"    root_node : node {static early}\n" +
		"        n : double (1)\n" +
		"        s : string (hello)\n" +
		"        b : bool (true)\n"

	f := Parse(src)
	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	for _, tc := range []struct {
		name string
		typ  Type
		def  string
	}{
		{"n", TypeDouble, "1"},
		{"s", TypeString, "hello"},
		{"b", TypeBool, "true"},
	} {
		ft, ok := f.Lookup(tc.name)
		if !ok {
			t.Errorf("%s not found", tc.name)
			continue
		}
		if ft.Type != tc.typ {
			t.Errorf("%s Type = %v, want %v", tc.name, ft.Type, tc.typ)
		}
		if ft.DefaultText() != tc.def {
			t.Errorf("%s Default = %q, want %q", tc.name, ft.DefaultText(), tc.def)
		}
	}
}

func TestParseBindingsInEitherOrder(t *testing.T) {
	f := Parse("M:\n    a : node {early static}\n")

	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a, _ := f.Lookup("a")
	if a.Mode != ModeStatic || a.Time != TimeEarly {
		t.Errorf("binding = %q, want %q", a.Binding(), "static early")
	}
}

func TestParseNestingDepth(t *testing.T) {
	f := Parse(modelSrc)

	for _, tc := range []struct {
		name  string
		depth int
	}{
		{"navigation", 0},
		{"vel_max", 1},
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
}

// TestParseConstraintLineDoesNotNest is the property that lets a constraint
// sit at its feature's own indentation: it must not become a parent for the
// entry that follows it.
func TestParseConstraintLineDoesNotNest(t *testing.T) {
	f := Parse(modelSrc)

	for _, name := range []string{"to_delivery", "localization"} {
		ft, ok := f.Lookup(name)
		if !ok {
			t.Fatalf("%s not found", name)
		}

		switch name {
		case "to_delivery":
			if ft.Parent == nil || ft.Parent.Name != "navigation" {
				t.Errorf("%s.Parent = %v, want navigation", name, ft.Parent)
			}
		case "localization":
			if ft.Parent == nil || !ft.Parent.IsRoot {
				t.Errorf("%s.Parent is not the root feature", name)
			}
		}
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
	if lift.Selected || lift.HasMarker {
		t.Error("lift should carry no marker")
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "root feature with a type",
			src:  "navigation: node {static early}\n",
			want: `the root feature "navigation" takes no type or binding, found "node"`,
		},
		{
			name: "old header form",
			src:  "feature_model: M\n",
			want: `a model now opens with its root feature: write "NAME:" instead of "feature_model: NAME"`,
		},
		{
			name: "old configuration header",
			src:  "feature_config:\n    navigation *{static early}\n",
			want: `a configuration no longer takes a header: delete this line and start with its first entry`,
		},
		{
			name: "missing type",
			src:  "M:\n    nav:\n",
			want: `expected a type (bool, bool_array, double, double_array, int, int_array, node, string, string_array) after ":"`,
		},
		{
			name: "unknown type",
			src:  "M:\n    nav : widget {static early}\n",
			want: `unknown type "widget": expected one of bool, bool_array, double, double_array, int, int_array, node, string, string_array`,
		},
		{
			name: "node without bindings",
			src:  "M:\n    nav : node\n",
			want: `node "nav" must declare its bindings, as in "{static early}"`,
		},
		{
			name: "unknown binding word",
			src:  "M:\n    nav : node {static sometimes}\n",
			want: `unknown binding "sometimes": expected static, dynamic, early or late`,
		},
		{
			name: "missing binding time",
			src:  "M:\n    nav : node {static}\n",
			want: `feature "nav" has no binding time (early|late)`,
		},
		{
			name: "empty bindings",
			src:  "M:\n    nav : node {}\n",
			want: `expected a binding mode (static|dynamic) and a binding time (early|late) in "{}"`,
		},
		{
			name: "unclosed bindings",
			src:  "M:\n    nav : node {static early\n",
			want: `unclosed "{" in feature "nav"`,
		},
		{
			name: "missing colon",
			src:  "M:\n    nav node {static early}\n",
			want: `expected ":" after feature "nav", found "node"`,
		},
		{
			name: "duplicate feature",
			src:  "M:\n    nav : node {static early}\n    nav : node {static early}\n",
			want: `feature "nav" is already declared on line 2`,
		},
		{
			name: "two modes",
			src:  "M:\n    nav : node {static dynamic}\n",
			want: `binding mode is already "static", found "dynamic"`,
		},
		{
			name: "empty value",
			src:  "M:\n    a : node {static early}\n        n : double ()\n",
			want: `expected a value in "()"`,
		},
		{
			name: "unclosed value",
			src:  "M:\n    a : node {static early}\n        n : double (1\n",
			want: `unclosed "(" in feature "n"`,
		},
		{
			name: "junk after the type",
			src:  "M:\n    nav : node static early\n",
			want: `unexpected "static" after the type of "nav"; bindings go in "{}", a value in "()", a validator is written as "bounds<0.0, 3.0>", and "?" marks it optional`,
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
	src := "M:\n    broken\n    good : node {static early}\n"

	f := Parse(src)

	if _, ok := f.Lookup("good"); !ok {
		t.Error("a malformed line stopped later entries from parsing")
	}
}

func TestParseSiblingIndentationMismatch(t *testing.T) {
	src := "M:\n    a : node {static early}\n" +
		"        b : node {static early}\n      c : node {static early}\n"

	f := Parse(src)

	if !hasMessage(f.Diagnostics, `inconsistent indentation: "c" is indented 6 columns, its siblings 8`) {
		t.Errorf("missing indentation warning; got %v", messages(f.Diagnostics))
	}
}

func TestTokenizeAdjacentPunctuation(t *testing.T) {
	toks := tokenize("    xor navigation : node {static early} ?")

	want := []string{"xor", "navigation", ":", "node", "{", "static", "early", "}", "?"}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens %v, want %d", len(toks), texts(toks), len(want))
	}
	for i, w := range want {
		if toks[i].text != w {
			t.Errorf("token %d = %q, want %q", i, toks[i].text, w)
		}
	}
}

func TestTokenizeConnectives(t *testing.T) {
	toks := tokenize("[square && attach || lift]")

	want := []string{"[", "square", "&&", "attach", "||", "lift", "]"}
	if len(toks) != len(want) {
		t.Fatalf("got %v, want %v", texts(toks), want)
	}
	for i, w := range want {
		if toks[i].text != w {
			t.Errorf("token %d = %q, want %q", i, toks[i].text, w)
		}
	}
}

func TestTokenizeNegatedConstraint(t *testing.T) {
	toks := tokenize("![circular]")

	want := []string{"!", "[", "circular", "]"}
	if len(toks) != len(want) {
		t.Fatalf("got %v, want %v", texts(toks), want)
	}
	for i, w := range want {
		if toks[i].text != w {
			t.Errorf("token %d = %q, want %q", i, toks[i].text, w)
		}
	}
}

func TestColumnsAreUTF16Units(t *testing.T) {
	f := Parse("M:\n    \U0001F680ab : node {static early}\n")

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
	// The specification's example mixes four spaces with a tab on one line.
	if got := indentOf("    \tfoo"); got != 8 {
		t.Errorf("indentOf(4 spaces + tab) = %d, want 8", got)
	}
}

func TestParseMixedIndentationFromTheSpec(t *testing.T) {
	// `vel_max` is written with four spaces then a tab, which must land it
	// beside `to_loading` rather than beneath it.
	src := "M:\n    xor navigation : node {static early} ?\n" +
		"    \tvel_max : double (1.5)\n        to_loading : node {dynamic late}\n"

	f := Parse(src)

	vel, ok := f.Lookup("vel_max")
	if !ok {
		t.Fatal("vel_max not found")
	}
	loading, ok := f.Lookup("to_loading")
	if !ok {
		t.Fatal("to_loading not found")
	}

	if vel.Parent == nil || vel.Parent.Name != "navigation" {
		t.Errorf("vel_max.Parent = %v, want navigation", vel.Parent)
	}
	if loading.Parent == nil || loading.Parent.Name != "navigation" {
		t.Errorf("to_loading.Parent = %v, want navigation", loading.Parent)
	}
	if errs := errorsOf(f.Diagnostics); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestParseEmptyDocument(t *testing.T) {
	f := Parse("")

	if !hasMessage(f.Diagnostics, `empty document: expected a root feature ("NAME:") or a "feature_config:" header`) {
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

func TestValidatorNamedForms(t *testing.T) {
	cases := []struct {
		src  string
		kind ValidatorKind
		args []string
	}{
		{"bounds<0.0, 3.0>", ValidatorBounds, []string{"0.0", "3.0"}},
		{"gt<0.0>", ValidatorGt, []string{"0.0"}},
		{"gt_eq<0.0>", ValidatorGtEq, []string{"0.0"}},
		{"lt<3.0>", ValidatorLt, []string{"3.0"}},
		{"lt_eq<3.0>", ValidatorLtEq, []string{"3.0"}},
		{"one_of<1.0, 2.0>", ValidatorOneOf, []string{"1.0", "2.0"}},
	}

	for _, c := range cases {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.0) " + c.src + "\n")
		vs := f.TopLevel()[0].Children[0].Validators

		if len(vs) != 1 {
			t.Fatalf("%s: %d validator(s), want 1", c.src, len(vs))
		}
		if vs[0].Kind != c.kind {
			t.Errorf("%s: kind = %v, want %v", c.src, vs[0].Kind, c.kind)
		}
		if len(vs[0].Args) != len(c.args) {
			t.Fatalf("%s: args = %v, want %v", c.src, vs[0].Args, c.args)
		}
		for i, want := range c.args {
			if vs[0].Args[i] != want {
				t.Errorf("%s: args[%d] = %q, want %q", c.src, i, vs[0].Args[i], want)
			}
		}
	}
}

// A validator taking no arguments has a form, which the bracket-only syntax
// could not express at all.
func TestValidatorWithNoArguments(t *testing.T) {
	f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : string (a) not_empty<>\n")
	vs := f.TopLevel()[0].Children[0].Validators

	if len(vs) != 1 || vs[0].Kind != ValidatorNotEmpty {
		t.Fatalf("validators = %v, want one not_empty", vs)
	}
	if len(vs[0].Args) != 0 {
		t.Errorf("args = %v, want none", vs[0].Args)
	}
}

// Several validators may be written one after another, which the library
// applies in turn.
func TestValidatorSeveralOnOneParameter(t *testing.T) {
	f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n" +
		"        p : string (autonomous) not_empty<> size_gt<2> size_lt<20>\n")
	vs := f.TopLevel()[0].Children[0].Validators

	want := []ValidatorKind{ValidatorNotEmpty, ValidatorSizeGt, ValidatorSizeLt}
	if len(vs) != len(want) {
		t.Fatalf("%d validator(s), want %d: %v", len(vs), len(want), vs)
	}
	for i, k := range want {
		if vs[i].Kind != k {
			t.Errorf("validators[%d] = %v, want %v", i, vs[i].Kind, k)
		}
	}
}

// The bracket-only forms inferred which validator was meant from the shape of
// the arguments. They are gone, and say what to write instead.
func TestValidatorRetiredFormsAreReported(t *testing.T) {
	cases := map[string]string{
		"double (1.0) <0.0, 3.0>": `a validator is written by name before its arguments, as "bounds<0.0, 3.0>", "gt<0.0>" or "not_empty<>"`,
		"double (1.0) <0.0, #>":   `a validator is written by name before its arguments, as "bounds<0.0, 3.0>", "gt<0.0>" or "not_empty<>"`,
		"double (1.0) <!0.0, #>":  `a validator is written by name before its arguments, as "bounds<0.0, 3.0>", "gt<0.0>" or "not_empty<>"`,
		"string (a) <<a, b>>":     `"<<a, b>>" is no longer how an enumeration is written; use "one_of<a, b>"`,
	}

	for entry, want := range cases {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : " + entry + "\n")

		if !hasMessage(f.Diagnostics, want) {
			t.Errorf("%s: missing %q; got %v", entry, want, messages(f.Diagnostics))
		}
	}
}

func TestValidatorUnknownNameSuggests(t *testing.T) {
	f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.0) size_greater<2>\n")

	want := `there is no validator called "size_greater"; did you mean "size_gt"?`
	if !hasMessage(f.Diagnostics, want) {
		t.Errorf("missing %q; got %v", want, messages(f.Diagnostics))
	}
}

func TestValidatorArityIsChecked(t *testing.T) {
	cases := map[string]string{
		"bounds<0.0>":  "bounds<> takes 2 arguments, and 1 was given",
		"not_empty<3>": "not_empty<> takes no arguments",
		"gt<0.0, 1.0>": "gt<> takes one argument, and 2 were given",
	}

	for src, want := range cases {
		f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.0) " + src + "\n")

		if !hasMessage(f.Diagnostics, want) {
			t.Errorf("%s: missing %q; got %v", src, want, messages(f.Diagnostics))
		}
	}
}

func TestValidatorRepeatedKindIsReported(t *testing.T) {
	f := Parse("RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.0) gt<0.0> gt<1.0>\n")

	want := `feature "p" already declares gt<>`
	if !hasMessage(f.Diagnostics, want) {
		t.Errorf("missing %q; got %v", want, messages(f.Diagnostics))
	}
}

// A validator may sit on its own line beneath the parameter it constrains, as
// a constraint sits beneath its feature.
func TestValidatorOnItsOwnLine(t *testing.T) {
	f := Parse(`RB-1-BASE:
    demo : node {dynamic late}
        p : string ("autonomous")
            one_of<"autonomous", "manual">
        q : double (1.5)
            gt<0.0> lt<3.0>
`)

	for _, d := range f.Diagnostics {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error: %s", d.Message)
		}
	}

	children := f.TopLevel()[0].Children
	if n := len(children[0].Validators); n != 1 {
		t.Errorf("p has %d validator(s), want 1", n)
	}
	if n := len(children[1].Validators); n != 2 {
		t.Errorf("q has %d validator(s), want 2", n)
	}
}

func TestValidatorLineNeedsAParameterAbove(t *testing.T) {
	cases := map[string]string{
		// Not indented past the parameter it would belong to.
		"RB-1-BASE:\n    demo : node {dynamic late}\n        p : double (1.0)\n        gt<0.0>\n": `a validator on its own line belongs to the parameter above it: indent it past "p"`,
		// A node does not carry validators.
		"RB-1-BASE:\n    demo : node {dynamic late}\n        inner : node {dynamic late}\n            gt<0.0>\n": "a node cannot carry a validator; only a parameter constrains its own value",
	}

	for src, want := range cases {
		f := Parse(src)
		if !hasMessage(f.Diagnostics, want) {
			t.Errorf("missing %q; got %v", want, messages(f.Diagnostics))
		}
	}
}
