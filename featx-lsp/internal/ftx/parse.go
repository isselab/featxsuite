package ftx

import (
	"fmt"
	"sort"
	"strings"
)

const (
	headerModel  = "feature_model"
	headerConfig = "feature_config"
)

type token struct {
	text string
	col  int // UTF-16 offset of the token start within its line
}

// Parse reads a document and reports the syntax problems it finds in
// File.Diagnostics. Parsing is error-tolerant: a malformed entry still
// contributes whatever it declared, so that outlines and hovers keep working
// while a line is half-typed.
func Parse(src string) *File {
	f := &File{Index: make(map[string]*Feature)}

	type level struct {
		indent int
		node   *Feature
	}
	var stack []level
	rootIndent := -1

	headerDone := false

	// The feature a bracketed constraint line attaches to: the entry above it.
	var last *Feature

	for ln, raw := range strings.Split(src, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		toks := tokenize(line)
		if len(toks) == 0 {
			continue
		}

		indent := indentOf(line)

		var ft *Feature

		isConstraintLine := toks[0].text == "!" || toks[0].text == "["

		// A validator may sit on its own line beneath the parameter it
		// constrains, the way a constraint sits beneath its feature. A
		// parameter has no children, so an indented line under one is never
		// anything else, and a parameter carrying several validators reads
		// better one per line than strung along behind its value.
		isValidatorLine := startsValidator(toks, 0)

		if !headerDone {
			headerDone = true

			if isConstraintLine {
				// Neither dialect opens with a constraint, and a model must
				// open with its root, so this can only be a configuration.
				f.Kind = KindConfig
				f.parseConstraint(ln, indent, toks, nil)
				continue
			}

			root, consumed := f.parseDocumentStart(ln, toks)
			if consumed {
				continue
			}

			if root != nil {
				// A model opens with its root feature, which nests like any
				// other entry; everything else hangs beneath it.
				ft = root
			} else {
				// A configuration opens straight onto its first entry.
				ft = f.parseEntry(ln, indent, toks)
			}
		} else if isConstraintLine {
			// A constraint line belongs to the entry above it rather than
			// opening a new one, so it never takes part in the nesting.
			f.parseConstraint(ln, indent, toks, last)
			continue
		} else if isValidatorLine {
			f.parseValidatorLine(ln, indent, toks, last)
			continue
		} else {
			ft = f.parseEntry(ln, indent, toks)
		}

		if ft == nil {
			continue
		}

		ft.Indent = indent
		ft.Line = ln
		last = ft

		// Attach to the innermost open entry that is indented less than this one.
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			if rootIndent < 0 {
				rootIndent = indent
			} else if indent != rootIndent {
				f.diag(SeverityWarning, ft.NameRange,
					"inconsistent indentation: %q is indented %d columns, its siblings %d",
					ft.Name, indent, rootIndent)
			}
			f.Roots = append(f.Roots, ft)
		} else {
			parent := stack[len(stack)-1].node
			if len(parent.Children) > 0 && parent.Children[0].Indent != indent {
				f.diag(SeverityWarning, ft.NameRange,
					"inconsistent indentation: %q is indented %d columns, its siblings %d",
					ft.Name, indent, parent.Children[0].Indent)
			}
			ft.Parent = parent
			parent.Children = append(parent.Children, ft)
		}

		stack = append(stack, level{indent: indent, node: ft})

		f.Order = append(f.Order, ft)
		if prev, dup := f.Index[ft.Name]; dup {
			f.diag(SeverityError, ft.NameRange,
				"feature %q is already declared on line %d", ft.Name, prev.Line+1)
		} else {
			f.Index[ft.Name] = ft
		}
	}

	if !headerDone {
		// A warning, not an error: an empty file is unfinished rather than
		// wrong, and placeholders should not fail a batch check.
		f.diag(SeverityWarning, Range{},
			"empty document: expected a root feature (%q) or a %q header", "NAME:", headerConfig+":")
	}

	return f
}

// parseDocumentStart reads the first line, which decides the dialect. Neither
// kind carries a header, so the root's colon is what tells them apart: a model
// opens with `NAME:`, which needs no type because nothing binds or configures
// the root, while a configuration opens straight onto its first entry.
//
// It returns the root feature for a model, and whether the line was consumed
// without producing one. A configuration's first line is not consumed: it is
// an entry and the caller parses it as one.
func (f *File) parseDocumentStart(ln int, toks []token) (*Feature, bool) {
	head := toks[0]

	// The old header forms are reported rather than read as feature names,
	// since otherwise a stale file parses as a model called "feature_config".
	switch head.text {
	case headerConfig:
		f.Kind = KindConfig
		f.diag(SeverityError, rangeOf(ln, head),
			"a configuration no longer takes a header: delete this line and start with its first entry")
		return nil, true

	case headerModel:
		f.Kind = KindModel
		f.diag(SeverityError, rangeOf(ln, head),
			"a model now opens with its root feature: write %q instead of %q",
			"NAME:", headerModel+": NAME")
		return nil, true
	}

	// `NAME:` and nothing else is a root feature; anything else is an entry,
	// and only a configuration opens with one of those.
	isRootLine := len(toks) >= 2 && toks[1].text == ":"

	if !isRootLine {
		f.Kind = KindConfig
		return nil, false
	}

	f.Kind = KindModel
	f.HeaderRange = rangeOf(ln, head)

	if !isName(head.text) {
		f.diag(SeverityError, rangeOf(ln, head),
			"expected a root feature name, found %q", head.text)
		return nil, true
	}

	root := &Feature{
		Name:      head.text,
		NameRange: rangeOf(ln, head),
		IsRoot:    true,
	}

	f.ModelName = root.Name
	f.ModelNameRange = root.NameRange

	if rest := toks[2:]; len(rest) > 0 {
		f.diag(SeverityError, rangeOf(ln, rest[0]),
			"the root feature %q takes no type or binding, found %q", root.Name, rest[0].text)
	}

	return root, false
}

// parseEntry reads one entry in whichever dialect the header selected. The two
// differ: a model declares a feature's type and what it allows, a
// configuration only names a feature, marks it and fixes its binding.
func (f *File) parseEntry(ln, indent int, toks []token) *Feature {
	if f.Kind == KindConfig {
		return f.parseConfigEntry(ln, indent, toks)
	}
	return f.parseModelEntry(ln, indent, toks)
}

// parseConfigEntry reads
//
//	NAME [marker] ["{" mode time "}"] ["(" literal ")"]
//
// as in `navigation *{static early}` or, for a typed parameter that is bound
// with its node and so has no binding of its own, `vel_max *`. A configuration
// restates neither the type nor the grouping, both of which belong to the
// model; whether an entry ought to carry a binding depends on the type the
// model gave it, so CrossCheck decides that rather than the parser.
func (f *File) parseConfigEntry(ln, indent int, toks []token) *Feature {
	ft := &Feature{Indent: indent, Line: ln}
	i := 0

	if g, ok := groupingFrom(toks[i].text); ok {
		// Kept in the tree so Analyze can report it against the right range.
		ft.Grouping = g
		ft.GroupingRange = rangeOf(ln, toks[i])
		i++
	}

	if i >= len(toks) || !isName(toks[i].text) {
		at := toks[len(toks)-1]
		if i < len(toks) {
			at = toks[i]
		}
		f.diag(SeverityError, rangeOf(ln, at), "expected a feature name, found %q", at.text)
		return nil
	}
	ft.Name = toks[i].text
	ft.NameRange = rangeOf(ln, toks[i])
	i++

	if i < len(toks) && (toks[i].text == "?" || toks[i].text == "*") {
		ft.HasMarker = true
		ft.Optional = toks[i].text == "?"
		ft.Selected = toks[i].text == "*"
		ft.MarkerRange = rangeOf(ln, toks[i])
		i++
	}

	if i < len(toks) && toks[i].text == ":" {
		f.diag(SeverityError, rangeOf(ln, toks[i]),
			"a configuration entry declares no type; write %q", ft.Name+" *{static early}")
		return ft
	}

	for i < len(toks) {
		switch toks[i].text {
		case "{":
			if ft.HasBindings {
				f.diag(SeverityError, rangeOf(ln, toks[i]),
					"feature %q already declares its binding", ft.Name)
			}
			i = f.parseBindings(ln, ft, toks, i)
		case "(":
			if ft.HasDefault {
				f.diag(SeverityError, rangeOf(ln, toks[i]),
					"feature %q already declares a value", ft.Name)
			}
			i = f.parseDefault(ln, ft, toks, i)
		case "<", "<<":
			f.diag(SeverityError, rangeOf(ln, toks[i]),
				"a validator belongs on the parameter in the feature model, not in a configuration")
			i = f.retiredValidator(ln, ft, toks, i)
		default:
			f.diag(SeverityError, rangeOf(ln, toks[i]),
				"unexpected %q after %q; a binding goes in %q and a value in %q",
				toks[i].text, ft.Name, "{}", "()")
			i++
		}
	}

	return ft
}

// parseModelEntry reads
//
//	[grouping] NAME [marker] ":" type ["{" mode time "}"] ["(" literal ")"]
//
// It returns nil only when the line has no usable feature name.
func (f *File) parseModelEntry(ln, indent int, toks []token) *Feature {
	ft := &Feature{Indent: indent, Line: ln}
	i := 0

	if g, ok := groupingFrom(toks[i].text); ok {
		ft.Grouping = g
		ft.GroupingRange = rangeOf(ln, toks[i])
		i++
	}

	if i >= len(toks) || !isName(toks[i].text) {
		at := toks[len(toks)-1]
		if i < len(toks) {
			at = toks[i]
		}
		f.diag(SeverityError, rangeOf(ln, at), "expected a feature name, found %q", at.text)
		return nil
	}
	ft.Name = toks[i].text
	ft.NameRange = rangeOf(ln, toks[i])
	i++

	if i < len(toks) && (toks[i].text == "?" || toks[i].text == "*") {
		f.diag(SeverityError, rangeOf(ln, toks[i]),
			"%q goes at the end of the line, after the type", toks[i].text)
		i++
	}

	if i >= len(toks) {
		f.diag(SeverityError, ft.NameRange,
			"expected %q and a type after feature %q", ":", ft.Name)
		return ft
	}

	if toks[i].text != ":" {
		f.diag(SeverityError, rangeOf(ln, toks[i]),
			"expected %q after feature %q, found %q", ":", ft.Name, toks[i].text)
		return ft
	}
	colon := toks[i]
	i++

	if i >= len(toks) {
		f.diag(SeverityError, rangeOf(ln, colon),
			"expected a type (%s) after %q", typeList(), ":")
		return ft
	}

	t, ok := typeFrom(toks[i].text)
	if !ok {
		f.diag(SeverityError, rangeOf(ln, toks[i]),
			"unknown type %q: expected one of %s", toks[i].text, typeList())
		return ft
	}
	ft.Type = t
	ft.TypeRange = rangeOf(ln, toks[i])
	i++

	// The braced binding pair, the parenthesised default and the trailing
	// optional marker may appear in any order after the type.
	for i < len(toks) {
		switch toks[i].text {
		case "{":
			if ft.HasBindings {
				f.diag(SeverityError, rangeOf(ln, toks[i]),
					"feature %q already declares its bindings", ft.Name)
			}
			i = f.parseBindings(ln, ft, toks, i)

		case "(":
			if ft.HasDefault {
				f.diag(SeverityError, rangeOf(ln, toks[i]),
					"feature %q already declares a value", ft.Name)
			}
			i = f.parseDefault(ln, ft, toks, i)

		case "<", "<<":
			i = f.retiredValidator(ln, ft, toks, i)

		case "?", "*":
			if ft.HasMarker {
				f.diag(SeverityError, rangeOf(ln, toks[i]),
					"feature %q already carries a marker", ft.Name)
			}
			ft.HasMarker = true
			ft.Optional = toks[i].text == "?"
			ft.Selected = toks[i].text == "*"
			ft.MarkerRange = rangeOf(ln, toks[i])
			i++

		default:
			// A validator is a name followed by its arguments, so it starts
			// here rather than at a bracket.
			if startsValidator(toks, i) {
				i = f.parseValidator(ln, ft, toks, i)
				continue
			}

			f.diag(SeverityError, rangeOf(ln, toks[i]),
				"unexpected %q after the type of %q; bindings go in %q, a value in %q, "+
					"a validator is written as %q, and %q marks it optional",
				toks[i].text, ft.Name, "{}", "()", "bounds<0.0, 3.0>", "?")
			i++
		}
	}

	if ft.Type == TypeNode && !ft.HasBindings {
		f.diag(SeverityError, ft.TypeRange,
			"node %q must declare its bindings, as in %q", ft.Name, "{static early}")
	}

	return ft
}

// parseBindings reads `{ mode time }`, accepting the pair in either order.
func (f *File) parseBindings(ln int, ft *Feature, toks []token, i int) int {
	open := toks[i]
	i++

	var inner []token
	closed := false

	for ; i < len(toks); i++ {
		if toks[i].text == "}" {
			closed = true
			i++
			break
		}
		inner = append(inner, toks[i])
	}

	if !closed {
		f.diag(SeverityError, rangeOf(ln, open), "unclosed %q in feature %q", "{", ft.Name)
	}

	ft.HasBindings = true

	if len(inner) == 0 {
		f.diag(SeverityError, rangeOf(ln, open),
			"expected a binding mode (static|dynamic) and a binding time (early|late) in %q", "{}")
		return i
	}

	for _, t := range inner {
		switch {
		case isMode(t.text):
			m, _ := modeFrom(t.text)
			if ft.Mode != ModeUnset {
				f.diag(SeverityError, rangeOf(ln, t),
					"binding mode is already %q, found %q", ft.Mode, t.text)
				continue
			}
			ft.Mode = m
			ft.ModeRange = rangeOf(ln, t)

		case isTime(t.text):
			tm, _ := timeFrom(t.text)
			if ft.Time != TimeUnset {
				f.diag(SeverityError, rangeOf(ln, t),
					"binding time is already %q, found %q", ft.Time, t.text)
				continue
			}
			ft.Time = tm
			ft.TimeRange = rangeOf(ln, t)

		default:
			f.diag(SeverityError, rangeOf(ln, t),
				"unknown binding %q: expected static, dynamic, early or late", t.text)
		}
	}

	last := rangeOf(ln, inner[len(inner)-1])

	if ft.Mode == ModeUnset {
		f.diag(SeverityError, last, "feature %q has no binding mode (static|dynamic)", ft.Name)
	}
	if ft.Time == TimeUnset {
		f.diag(SeverityError, last, "feature %q has no binding time (early|late)", ft.Name)
	}

	return i
}

// parseDefault reads `( literal (, literal)* )`, keeping each element
// verbatim so that the type checker can judge it and the formatter and the
// generator can reproduce it. An array type takes several elements.
func (f *File) parseDefault(ln int, ft *Feature, toks []token, i int) int {
	open := toks[i]
	i++

	ft.HasDefault = true

	var (
		inner   []token
		closed  bool
		current []string
	)

	flush := func() {
		if len(current) > 0 {
			ft.DefaultValues = append(ft.DefaultValues, strings.Join(current, " "))
			current = nil
		}
	}

	for ; i < len(toks); i++ {
		if toks[i].text == ")" {
			closed = true
			i++
			break
		}

		if toks[i].text == "," {
			if len(current) == 0 {
				f.diag(SeverityError, rangeOf(ln, toks[i]), "expected a value before %q", ",")
			}
			flush()
			continue
		}

		inner = append(inner, toks[i])
		current = append(current, toks[i].text)
	}

	trailing := closed && len(inner) > 0 && len(current) == 0
	flush()

	if !closed {
		f.diag(SeverityError, rangeOf(ln, open), "unclosed %q in feature %q", "(", ft.Name)
	}

	if len(inner) == 0 {
		f.diag(SeverityError, rangeOf(ln, open), "expected a value in %q", "()")
		ft.DefaultRange = rangeOf(ln, open)
		return i
	}

	if trailing {
		f.diag(SeverityError, rangeOf(ln, open), "the value list ends with a trailing %q", ",")
	}

	ft.DefaultRange = Range{
		Line:      ln,
		StartChar: inner[0].col,
		EndChar:   inner[len(inner)-1].col + utf16Len(inner[len(inner)-1].text),
	}

	return i
}

// parseValidatorLine reads a line holding only validators, which belong to the
// parameter above rather than opening an entry of their own.
func (f *File) parseValidatorLine(ln, indent int, toks []token, owner *Feature) {
	r := rangeOf(ln, toks[0])

	switch {
	case owner == nil:
		f.diag(SeverityError, r, "a validator needs a parameter above it to constrain")
		return

	case indent <= owner.Indent:
		f.diag(SeverityError, r,
			"a validator on its own line belongs to the parameter above it: "+
				"indent it past %q", owner.Name)
		return

	case !owner.Type.IsValue():
		f.diag(SeverityError, r,
			"a %s cannot carry a validator; only a parameter constrains its own value",
			owner.Type)
		return
	}

	for i := 0; i < len(toks); {
		if !startsValidator(toks, i) {
			f.diag(SeverityError, rangeOf(ln, toks[i]),
				"unexpected %q; a validator is written as %q", toks[i].text, "bounds<0.0, 3.0>")
			return
		}
		i = f.parseValidator(ln, owner, toks, i)
	}
}

// parseValidator reads one validator, written as a name followed by its
// arguments in angle brackets: `bounds<0.0, 3.0>`, or `not_empty<>` for one
// that takes none. The name says which validator it is, so nothing is inferred
// from the argument count.
//
// toks[i] is the name; toks[i+1] is "<".
func (f *File) parseValidator(ln int, ft *Feature, toks []token, i int) int {
	name := toks[i]
	nameRange := rangeOf(ln, name)
	i++ // the name

	open := toks[i]
	i++ // "<"

	var (
		args    []string
		current []string
		last    = open
		closed  bool
	)

	flush := func() {
		if len(current) > 0 {
			args = append(args, strings.Join(current, " "))
			current = nil
		}
	}

	for ; i < len(toks); i++ {
		if toks[i].text == ">" {
			closed = true
			last = toks[i]
			i++
			break
		}

		if toks[i].text == "," {
			if len(current) == 0 {
				f.diag(SeverityError, rangeOf(ln, toks[i]), "expected a value before %q", ",")
			}
			flush()
			continue
		}

		current = append(current, toks[i].text)
		last = toks[i]
	}

	flush()

	v := Validator{
		NameRange: nameRange,
		Args:      args,
		Range: Range{
			Line:      ln,
			StartChar: name.col,
			EndChar:   last.col + utf16Len(last.text),
		},
	}

	if !closed {
		f.diag(SeverityError, v.Range, "unclosed %q in %q", "<", name.text+"<")
	}

	spec, known := LookupValidator(name.text)
	if !known {
		f.diag(SeverityError, nameRange, "there is no validator called %q; %s",
			name.text, validatorSuggestion(name.text))
		return i
	}

	v.Kind = spec.Kind

	switch {
	case spec.MaxArgs == 0 && len(args) > 0:
		f.diag(SeverityError, v.Range, "%s takes no arguments", spec.Written())
		return i

	case len(args) < spec.MinArgs:
		f.diag(SeverityError, v.Range, "%s takes %s, and %d %s given",
			spec.Written(), spec.arity(), len(args), plural(len(args), "was", "were"))
		return i

	case spec.MaxArgs >= 0 && len(args) > spec.MaxArgs:
		f.diag(SeverityError, v.Range, "%s takes %s, and %d were given",
			spec.Written(), spec.arity(), len(args))
		return i
	}

	for _, existing := range ft.Validators {
		if existing.Kind == v.Kind {
			f.diag(SeverityError, nameRange,
				"feature %q already declares %s", ft.Name, spec.Written())
			return i
		}
	}

	ft.Validators = append(ft.Validators, v)
	return i
}

// startsValidator reports whether a validator begins at toks[i], which is the
// case for a name immediately followed by its argument list.
func startsValidator(toks []token, i int) bool {
	return i+1 < len(toks) && isName(toks[i].text) && toks[i+1].text == "<"
}

// plural picks between two words by count.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// validatorSuggestion names the closest validator, or lists them when nothing
// is close, so a misspelling says what was meant rather than only that it was
// wrong.
func validatorSuggestion(given string) string {
	best, bestScore := "", 0

	for _, spec := range validatorSpecs {
		if score := commonPrefix(given, spec.Name); score > bestScore {
			best, bestScore = spec.Name, score
		}
	}

	if bestScore >= 2 {
		return fmt.Sprintf("did you mean %q?", best)
	}

	names := make([]string, 0, len(validatorSpecs))
	for _, spec := range validatorSpecs {
		names = append(names, spec.Name)
	}
	return "the validators are " + strings.Join(names, ", ")
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// retiredValidator reports a validator written the way the DSL used to spell
// it. The old forms inferred which validator was meant from the shape of the
// brackets; naming it removed the guessing, and these say what to write now.
func (f *File) retiredValidator(ln int, ft *Feature, toks []token, i int) int {
	open := toks[i]
	r := rangeOf(ln, open)

	switch open.text {
	case "<<":
		f.diag(SeverityError, r,
			"%q is no longer how an enumeration is written; use %q", "<<a, b>>", "one_of<a, b>")
	default:
		f.diag(SeverityError, r,
			"a validator is written by name before its arguments, as %q, %q or %q",
			"bounds<0.0, 3.0>", "gt<0.0>", "not_empty<>")
	}

	// Skip to the end of the bracketed group so the rest of the line still
	// parses and a second diagnostic is not raised for its contents.
	closer := ">"
	if open.text == "<<" {
		closer = ">>"
	}
	for i++; i < len(toks); i++ {
		if toks[i].text == closer {
			return i + 1
		}
	}
	_ = ft
	return i
}

// typeList renders the accepted type names for a diagnostic.
func typeList() string {
	out := make([]string, 0, len(typeNames))
	for _, name := range typeNames {
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// parseConstraint reads a bracketed line and attaches it to the entry above.
func (f *File) parseConstraint(ln, indent int, toks []token, owner *Feature) {
	c := Constraint{Line: ln}
	i := 0

	if toks[i].text == "!" {
		c.Negated = true
		i++

		if i >= len(toks) || toks[i].text != "[" {
			f.diag(SeverityError, rangeOf(ln, toks[0]),
				"expected %q after %q to write an exclusion", "[", "!")
			return
		}
	}

	open := toks[i]
	i++

	var inner []token
	closed := false

	for ; i < len(toks); i++ {
		if toks[i].text == "]" {
			closed = true
			i++
			break
		}
		inner = append(inner, toks[i])
	}

	end := rangeOf(ln, open).EndChar
	if closed {
		end = toks[i-1].col + 1
	}

	c.Range = Range{Line: ln, StartChar: rangeOf(ln, toks[0]).StartChar, EndChar: end}

	if !closed {
		f.diag(SeverityError, c.Range, "unclosed %q in a constraint", "[")
	}

	if i < len(toks) {
		f.diag(SeverityError, rangeOf(ln, toks[i]),
			"unexpected %q after the constraint", toks[i].text)
	}

	if owner == nil {
		f.diag(SeverityError, c.Range,
			"a constraint must follow the feature it applies to")
		return
	}

	if indent <= owner.Indent {
		f.diag(SeverityWarning, c.Range,
			"a constraint is written beneath the feature it applies to: indent it past %q at %d columns",
			owner.Name, owner.Indent)
	}

	f.parseOperands(ln, &c, inner)

	if len(c.Operands) == 0 {
		return
	}

	owner.Constraints = append(owner.Constraints, c)
}

// parseOperands reads `NAME (("&&"|"||") NAME)*`. Mixing the two connectives
// is rejected rather than given a precedence the notation does not define.
func (f *File) parseOperands(ln int, c *Constraint, toks []token) {
	wantName := true

	for _, t := range toks {
		if op, ok := opFrom(t.text); ok {
			if wantName {
				f.diag(SeverityError, rangeOf(ln, t), "expected a feature name before %q", t.text)
				continue
			}

			if c.Op != OpNone && c.Op != op {
				f.diag(SeverityError, rangeOf(ln, t),
					"a constraint cannot mix %q and %q; split it into separate constraints",
					c.Op, op)
			} else {
				c.Op = op
			}

			wantName = true
			continue
		}

		if !wantName {
			f.diag(SeverityError, rangeOf(ln, t),
				"expected %q or %q between feature names, found %q", "&&", "||", t.text)
		}

		if !isName(t.text) {
			f.diag(SeverityError, rangeOf(ln, t),
				"%q is not a valid feature name in a constraint", t.text)
			wantName = false
			continue
		}

		c.Operands = append(c.Operands, Ref{Name: t.text, Range: rangeOf(ln, t)})
		wantName = false
	}

	switch {
	case len(c.Operands) == 0:
		f.diag(SeverityError, c.Range, "a constraint needs at least one feature name")
	case wantName:
		f.diag(SeverityError, c.Range, "a constraint cannot end with %q", c.Op)
	}
}

// tokenize splits a line, treating the notation's punctuation as separate
// tokens so that `?:` and `{static` need no spaces around them.
func tokenize(line string) []token {
	var (
		toks     []token
		sb       strings.Builder
		col      int
		startCol int
	)

	flush := func() {
		if sb.Len() > 0 {
			toks = append(toks, token{text: sb.String(), col: startCol})
			sb.Reset()
		}
	}

	runes := []rune(line)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		width := 1
		if r > 0xFFFF {
			width = 2
		}

		// The doubled punctuation has to be recognised before the single
		// characters, or `&&` and `<<` become two unknown tokens each.
		if (r == '&' || r == '|' || r == '<' || r == '>') && i+1 < len(runes) && runes[i+1] == r {
			flush()
			toks = append(toks, token{text: string([]rune{r, r}), col: col})
			i++
			col += 2
			continue
		}

		switch r {
		case ' ', '\t':
			flush()
		case ':', '?', '*', '{', '}', '(', ')', '[', ']', '!', '&', '|', '<', '>', ',':
			flush()
			toks = append(toks, token{text: string(r), col: col})
		default:
			if sb.Len() == 0 {
				startCol = col
			}
			sb.WriteRune(r)
		}

		col += width
	}

	flush()
	return toks
}

func indentOf(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += tabWidth
		default:
			return n
		}
	}
	return n
}

func rangeOf(line int, t token) Range {
	return Range{Line: line, StartChar: t.col, EndChar: t.col + utf16Len(t.text)}
}

// isName reports whether s can be a feature or model name. Punctuation, the
// connectives and the type and binding keywords are excluded, so that a
// misspelling is reported as what it was meant to be rather than as a name.
func isName(s string) bool {
	switch s {
	case "", ":", "?", "*", "{", "}", "(", ")", "[", "]", "!",
		"&", "|", "&&", "||", "<", ">", "<<", ">>", ",", "#":
		return false
	}
	return !isMode(s) && !isTime(s) && !isTypeName(s)
}

func isMode(s string) bool     { _, ok := modeFrom(s); return ok }
func isTime(s string) bool     { _, ok := timeFrom(s); return ok }
func isTypeName(s string) bool { _, ok := typeFrom(s); return ok }

func groupingFrom(s string) (Grouping, bool) {
	switch s {
	case "or":
		return GroupOr, true
	case "xor":
		return GroupXor, true
	}
	return GroupNone, false
}

func modeFrom(s string) (Mode, bool) {
	switch s {
	case "static":
		return ModeStatic, true
	case "dynamic":
		return ModeDynamic, true
	}
	return ModeUnset, false
}

func timeFrom(s string) (Time, bool) {
	switch s {
	case "early":
		return TimeEarly, true
	case "late":
		return TimeLate, true
	}
	return TimeUnset, false
}

func typeFrom(s string) (Type, bool) {
	for t, name := range typeNames {
		if name == s {
			return t, true
		}
	}
	return TypeUnset, false
}

func opFrom(s string) (Op, bool) {
	switch s {
	case "&&":
		return OpAnd, true
	case "||":
		return OpOr, true
	}
	return OpNone, false
}
