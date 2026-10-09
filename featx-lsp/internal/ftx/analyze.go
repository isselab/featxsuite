package ftx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Analyze reports the problems that can be found in one document on its own:
// markers and clauses used in the wrong kind of file, ill-typed values, and
// constraints that are unsatisfiable or redundant against their own model.
func Analyze(f *File) []Diagnostic {
	var out []Diagnostic

	add := func(sev Severity, r Range, format string, args ...any) {
		out = append(out, Diagnostic{Range: r, Severity: sev, Message: fmt.Sprintf(format, args...)})
	}

	for _, ft := range f.Order {
		out = append(out, checkShape(f.Kind, ft)...)

		switch f.Kind {
		case KindModel:
			if ft.Selected {
				add(SeverityError, ft.MarkerRange,
					"%q selects a feature, which belongs in a feature configuration, not a model", "*")
			}

			if ft.Grouping != GroupNone {
				switch len(ft.Children) {
				case 0:
					add(SeverityError, ft.GroupingRange,
						"%q groups the children of %q, which has none", ft.Grouping, ft.Name)
				case 1:
					add(SeverityWarning, ft.GroupingRange,
						"%q over the single child %q has no effect", ft.Grouping, ft.Children[0].Name)
				}
			}

		case KindConfig:
			if ft.Optional {
				add(SeverityWarning, ft.MarkerRange,
					"%q declares a feature optional, which belongs in a feature model; use %q to select it",
					"?", "*")
			}

			if ft.Grouping != GroupNone {
				add(SeverityWarning, ft.GroupingRange,
					"%q groups children in a feature model and has no meaning in a configuration", ft.Grouping)
			}

			for _, c := range ft.Constraints {
				add(SeverityWarning, c.Range,
					"%q is a constraint on the feature model and has no meaning in a configuration",
					c.Keyword())
			}
		}
	}

	if f.Kind == KindModel {
		out = append(out, checkConstraints(f)...)
		out = append(out, checkRequiresCycles(f)...)
	}

	return out
}

// checkShape validates one entry against its declared type: what may carry a
// value, what may carry bindings, and what may contain other features.
func checkShape(kind Kind, ft *Feature) []Diagnostic {
	var out []Diagnostic

	add := func(sev Severity, r Range, format string, args ...any) {
		out = append(out, Diagnostic{Range: r, Severity: sev, Message: fmt.Sprintf(format, args...)})
	}

	if ft.IsRoot || ft.Type == TypeUnset {
		// The root declares no type, and for anything else the parser has
		// already said why one is missing.
		return out
	}

	if ft.Type == TypeNode {
		if ft.HasDefault {
			add(SeverityError, ft.DefaultRange,
				"a node has no value of its own; %q belongs on a number, string or bool feature", "()")
		}
		return out
	}

	// A value feature is a parameter of the node enclosing it, so it cannot
	// hold features itself.
	if len(ft.Children) > 0 {
		add(SeverityError, ft.TypeRange,
			"%q is a %s and cannot contain features; only a node can", ft.Name, ft.Type)
	}

	// A configuration is a flat list, so only a model says anything by where
	// a value feature sits. The root is not a node, so a parameter directly
	// beneath it has nothing to be bound with.
	if kind == KindModel && ft.BindingNode() == nil {
		add(SeverityWarning, ft.TypeRange,
			"%q is a %s but no enclosing node is bound with it", ft.Name, ft.Type)
	}

	// A configuration that omits the value is taking the model's default, so
	// only an ill-typed one is worth reporting.
	if ft.HasDefault {
		if err := checkLiteral(ft.Type, ft.Default); err != nil {
			add(SeverityError, ft.DefaultRange, "%s", err)
		}
	}

	return out
}

// checkLiteral reports whether a parenthesised value suits its declared type.
func checkLiteral(t Type, lit string) error {
	switch t {
	case TypeNumber:
		if _, err := strconv.ParseFloat(lit, 64); err != nil {
			return fmt.Errorf("%q is not a number", lit)
		}

	case TypeBool:
		if lit != "true" && lit != "false" {
			return fmt.Errorf("%q is not a bool: expected true or false", lit)
		}

	case TypeString:
		// Any text is a string. A quoted literal is accepted as written.
	}

	return nil
}

// checkConstraints validates the bracketed lines against the model that
// declares them: every operand must exist, and a constraint must not be
// self-referential, contradictory, or already implied by the nesting.
func checkConstraints(f *File) []Diagnostic {
	var out []Diagnostic

	add := func(sev Severity, r Range, format string, args ...any) {
		out = append(out, Diagnostic{Range: r, Severity: sev, Message: fmt.Sprintf(format, args...)})
	}

	for _, ft := range f.Order {
		required := make(map[string]bool)

		for _, ref := range operandsOf(ft.Requires()) {
			required[ref.Name] = true
		}

		for _, c := range ft.Constraints {
			if c.Negated && c.Op == OpOr {
				add(SeverityWarning, c.Range,
					"%q in an exclusion has no effect: every operand is forbidden either way", "||")
			}

			seen := make(map[string]bool, len(c.Operands))

			for _, ref := range c.Operands {
				if seen[ref.Name] {
					add(SeverityWarning, ref.Range,
						"%q is listed twice in the same constraint on %q", ref.Name, ft.Name)
					continue
				}
				seen[ref.Name] = true

				if ref.Name == ft.Name {
					add(SeverityError, ref.Range, "feature %q %s itself", ft.Name, c.Kind())
					continue
				}

				target, ok := f.Lookup(ref.Name)
				if !ok {
					add(SeverityError, ref.Range,
						"feature %q is not declared in this feature model", ref.Name)
					continue
				}

				if c.Negated {
					if target.IsAncestorOf(ft) || ft.IsAncestorOf(target) {
						add(SeverityWarning, ref.Range,
							"%q and %q are nested within one another, so excluding makes the inner one unselectable",
							ft.Name, ref.Name)
					}
					continue
				}

				if target.IsAncestorOf(ft) {
					add(SeverityWarning, ref.Range,
						"%q encloses %q, so requiring it is redundant", ref.Name, ft.Name)
				}
			}
		}

		for _, ref := range operandsOf(ft.Excludes()) {
			if required[ref.Name] {
				add(SeverityError, ref.Range,
					"feature %q both requires and excludes %q", ft.Name, ref.Name)
			}
		}

		// Requiring a feature already implies requiring everything it is
		// nested within, so listing an enclosing feature alongside an enclosed
		// one is noise.
		requires := operandsOf(ft.Requires())

		for _, outer := range requires {
			enclosing, ok := f.Lookup(outer.Name)
			if !ok {
				continue
			}

			for _, inner := range requires {
				if inner.Name == outer.Name {
					continue
				}

				if enclosed, ok := f.Lookup(inner.Name); ok && enclosing.IsAncestorOf(enclosed) {
					add(SeverityWarning, outer.Range,
						"%q encloses %q, which %q also requires, so listing it is redundant",
						outer.Name, inner.Name, ft.Name)
					break
				}
			}
		}
	}

	return out
}

// checkRequiresCycles reports requirement loops, which no selection can
// satisfy without selecting the whole loop. Only conjunctive requirements can
// force a cycle; a disjunction leaves an escape.
func checkRequiresCycles(f *File) []Diagnostic {
	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)

	state := make(map[string]int, len(f.Order))
	var path []string
	var out []Diagnostic

	var visit func(name string)
	visit = func(name string) {
		ft, ok := f.Lookup(name)
		if !ok {
			return
		}

		state[name] = onStack
		path = append(path, name)

		for _, c := range ft.Requires() {
			if c.Op == OpOr {
				continue
			}

			for _, ref := range c.Operands {
				switch state[ref.Name] {
				case onStack:
					out = append(out, Diagnostic{
						Range:    ref.Range,
						Severity: SeverityError,
						Message: fmt.Sprintf("requirement cycle: %s",
							strings.Join(append(cycleFrom(path, ref.Name), ref.Name), " requires ")),
					})
				case unvisited:
					visit(ref.Name)
				}
			}
		}

		path = path[:len(path)-1]
		state[name] = done
	}

	for _, ft := range f.Order {
		if state[ft.Name] == unvisited {
			visit(ft.Name)
		}
	}

	return out
}

// CrossCheck validates a configuration against the model it configures. These
// are the rules the featx configurator enforces before it will bind anything,
// reported here while the file is still being edited.
func CrossCheck(cfg, model *File) []Diagnostic {
	if cfg == nil || model == nil || cfg.Kind != KindConfig || model.Kind != KindModel {
		return nil
	}

	var out []Diagnostic

	add := func(sev Severity, r Range, format string, args ...any) {
		out = append(out, Diagnostic{Range: r, Severity: sev, Message: fmt.Sprintf(format, args...)})
	}

	for _, entry := range cfg.Order {
		declared, ok := model.Lookup(entry.Name)
		if !ok {
			add(SeverityError, entry.NameRange,
				"feature %q is not declared in feature model %q", entry.Name, model.ModelName)
			continue
		}

		// Whether an entry should carry a binding follows from the type the
		// model gave it: a node is bound, a parameter is bound with its node.
		switch {
		case declared.Type == TypeNode && !entry.HasBindings:
			add(SeverityError, entry.NameRange,
				"node %q must be configured with a binding, as in %q",
				entry.Name, entry.Name+" *{"+declared.Binding()+"}")

		case declared.Type.IsValue() && entry.HasBindings:
			add(SeverityError, entry.ModeRange,
				"%q is a %s parameter of %q and is bound with it, so it takes no binding of its own",
				entry.Name, declared.Type, parentName(declared))
		}

		if declared.Type.IsValue() && entry.HasDefault {
			if err := checkLiteral(declared.Type, entry.Default); err != nil {
				add(SeverityError, entry.DefaultRange, "%s", err)
			}
		}

		if entry.HasBindings && declared.HasBindings {
			if entry.Mode != ModeUnset && declared.Mode != ModeUnset && entry.Mode != declared.Mode {
				add(SeverityError, entry.ModeRange,
					"feature %q is configured %q but the model allows only %q",
					entry.Name, entry.Mode, declared.Mode)
			}

			if entry.Time != TimeUnset && declared.Time != TimeUnset && entry.Time != declared.Time {
				add(SeverityError, entry.TimeRange,
					"feature %q is configured %q but the model allows only %q",
					entry.Name, entry.Time, declared.Time)
			}
		}

		if entry.Selected && declared.Parent != nil {
			if parentCfg, ok := cfg.Lookup(declared.Parent.Name); ok && !parentCfg.Selected {
				add(SeverityError, entry.MarkerRange,
					"feature %q is selected but its parent %q is not", entry.Name, declared.Parent.Name)
			}
		}
	}

	out = append(out, checkParameters(cfg, model)...)
	out = append(out, checkCrossTree(cfg, model)...)
	out = append(out, checkAlternatives(cfg, model)...)
	out = append(out, checkCoverage(cfg, model)...)

	return out
}

// checkParameters propagates selection to typed parameters: a parameter is
// part of the node it belongs to, so selecting the node selects it too.
func checkParameters(cfg, model *File) []Diagnostic {
	var out []Diagnostic

	for _, entry := range cfg.Order {
		if !entry.Selected {
			continue
		}

		declared, ok := model.Lookup(entry.Name)
		if !ok {
			continue // already reported as undeclared
		}

		for _, child := range declared.Children {
			if !child.Type.IsValue() {
				continue
			}

			param, configured := cfg.Lookup(child.Name)

			switch {
			case !configured:
				out = append(out, Diagnostic{
					Range:    entry.MarkerRange,
					Severity: SeverityError,
					Message: fmt.Sprintf(
						"%q is selected, so its %s parameter %q must be selected too, but this configuration omits it",
						entry.Name, child.Type, child.Name),
				})

			case !param.Selected:
				out = append(out, Diagnostic{
					Range:    param.NameRange,
					Severity: SeverityError,
					Message: fmt.Sprintf("%q must be selected because its parent %q is selected",
						child.Name, entry.Name),
				})
			}
		}
	}

	return out
}

// checkCrossTree enforces the model's bracketed constraints against what the
// configuration actually selects.
func checkCrossTree(cfg, model *File) []Diagnostic {
	var out []Diagnostic

	selected := func(name string) bool {
		entry, ok := cfg.Lookup(name)
		return ok && entry.Selected
	}

	for _, entry := range cfg.Order {
		if !entry.Selected {
			continue
		}

		declared, ok := model.Lookup(entry.Name)
		if !ok {
			continue // already reported as undeclared
		}

		for _, c := range declared.Requires() {
			// A disjunction is satisfied by any one operand; a conjunction
			// needs all of them.
			if c.Op == OpOr {
				if !anySelected(c.Operands, selected) {
					out = append(out, Diagnostic{
						Range:    entry.MarkerRange,
						Severity: SeverityError,
						Message: fmt.Sprintf("feature %q is selected and requires one of %s, none of which is selected",
							entry.Name, quoteRefs(c.Operands)),
					})
				}
				continue
			}

			for _, ref := range c.Operands {
				if selected(ref.Name) {
					continue
				}

				message := fmt.Sprintf("feature %q is selected, so %q must be selected too",
					entry.Name, ref.Name)
				if _, configured := cfg.Lookup(ref.Name); !configured {
					message = fmt.Sprintf("feature %q is selected and requires %q, which this configuration omits",
						entry.Name, ref.Name)
				}

				out = append(out, Diagnostic{
					Range:    entry.MarkerRange,
					Severity: SeverityError,
					Message:  message,
				})
			}
		}

		// Every operand of an exclusion is forbidden, whichever connective
		// joins them.
		for _, ref := range operandsOf(declared.Excludes()) {
			if !selected(ref.Name) {
				continue
			}

			out = append(out, Diagnostic{
				Range:    entry.MarkerRange,
				Severity: SeverityError,
				Message: fmt.Sprintf("features %q and %q exclude each other and cannot both be selected",
					entry.Name, ref.Name),
			})
		}
	}

	return out
}

// checkAlternatives enforces xor groupings: the children of an xor feature are
// alternatives, so no two of them may be selected at once.
func checkAlternatives(cfg, model *File) []Diagnostic {
	var out []Diagnostic

	for _, declared := range model.Order {
		if declared.Grouping != GroupXor {
			continue
		}

		var selected []*Feature
		for _, child := range declared.Children {
			// A typed parameter is not an alternative: its selection follows
			// from its node's, so counting it would make every xor group
			// unsatisfiable the moment the group itself is selected.
			if child.Type.IsValue() {
				continue
			}

			if entry, ok := cfg.Lookup(child.Name); ok && entry.Selected {
				selected = append(selected, entry)
			}
		}

		if len(selected) < 2 {
			continue
		}

		// Report every entry past the first, so each offending line is marked.
		for _, entry := range selected[1:] {
			out = append(out, Diagnostic{
				Range:    entry.MarkerRange,
				Severity: SeverityError,
				Message: fmt.Sprintf(
					"%q is an xor group, so only one of its children may be selected; %s are all selected",
					declared.Name, quoteNames(selected)),
			})
		}
	}

	return out
}

// checkCoverage notes model features the configuration says nothing about.
// featx needs an entry for every feature it is asked to bind or select.
func checkCoverage(cfg, model *File) []Diagnostic {
	var missing []string

	for _, declared := range model.Order {
		if declared.Name == "" || declared.IsRoot {
			continue
		}
		if _, ok := cfg.Lookup(declared.Name); !ok {
			missing = append(missing, declared.Name)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	sort.Strings(missing)

	return []Diagnostic{{
		Range:    cfg.HeaderRange,
		Severity: SeverityHint,
		Message: fmt.Sprintf("feature model %q declares %d feature(s) this configuration omits: %s",
			model.ModelName, len(missing), strings.Join(missing, ", ")),
	}}
}

// parentName names the feature a parameter belongs to, for diagnostics.
func parentName(ft *Feature) string {
	if ft.Parent == nil {
		return "the model"
	}
	return ft.Parent.Name
}

// operandsOf flattens the operands of several constraints.
func operandsOf(cs []Constraint) []Ref {
	var out []Ref
	for _, c := range cs {
		out = append(out, c.Operands...)
	}
	return out
}

func anySelected(refs []Ref, selected func(string) bool) bool {
	for _, r := range refs {
		if selected(r.Name) {
			return true
		}
	}
	return false
}

func quoteRefs(refs []Ref) string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = fmt.Sprintf("%q", r.Name)
	}
	return strings.Join(out, ", ")
}

func quoteNames(fs []*Feature) string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = fmt.Sprintf("%q", f.Name)
	}
	return strings.Join(names, ", ")
}

// cycleFrom returns the tail of path beginning at name.
func cycleFrom(path []string, name string) []string {
	for i, p := range path {
		if p == name {
			return path[i:]
		}
	}
	return path
}
