package ftx

import (
	"fmt"
	"sort"
	"strings"
)

// Analyze reports the problems that can be found in one document on its own,
// which are mostly markers used in the wrong kind of file.
func Analyze(f *File) []Diagnostic {
	var out []Diagnostic

	add := func(sev Severity, r Range, format string, args ...any) {
		out = append(out, Diagnostic{Range: r, Severity: sev, Message: fmt.Sprintf(format, args...)})
	}

	for _, ft := range f.Order {
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

		if entry.Selected && declared.Parent != nil {
			if parentCfg, ok := cfg.Lookup(declared.Parent.Name); ok && !parentCfg.Selected {
				add(SeverityError, entry.MarkerRange,
					"feature %q is selected but its parent %q is not", entry.Name, declared.Parent.Name)
			}
		}
	}

	out = append(out, checkAlternatives(cfg, model)...)
	out = append(out, checkCoverage(cfg, model)...)

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
// featx needs an entry for every feature it is asked to bind.
func checkCoverage(cfg, model *File) []Diagnostic {
	var missing []string

	for _, declared := range model.Order {
		if declared.Name == "" {
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

func quoteNames(fs []*Feature) string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = fmt.Sprintf("%q", f.Name)
	}
	return strings.Join(names, ", ")
}
