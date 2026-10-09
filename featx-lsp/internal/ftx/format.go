package ftx

import "strings"

// Format renders the document in canonical form, indented IndentUnit columns
// per nesting level.
//
// A model opens with its root feature and writes each entry as
//
//	[grouping ]name : type [{mode time}] [(value)] [?]
//
// with the feature's constraints on their own lines one level further in. A
// configuration opens with its header and writes
//
//	name [marker][{mode time}] [(value)]
//
// It reports false and formats nothing if the document has syntax errors,
// since the parse tree would not faithfully represent what the author wrote.
func Format(f *File) (string, bool) {
	if f.Kind == KindUnknown {
		return "", false
	}

	for _, d := range f.Diagnostics {
		if d.Severity == SeverityError {
			return "", false
		}
	}

	var sb strings.Builder

	// A model's entries sit one level in from its root line; a configuration
	// has no line above its entries, so they start at column 0.
	depth := 0

	if f.Kind == KindModel {
		if f.ModelName == "" {
			return "", false
		}
		sb.WriteString(f.ModelName + ":\n")
		depth = 1
	}

	for _, ft := range f.TopLevel() {
		if !writeEntry(&sb, f.Kind, ft, depth) {
			return "", false
		}
	}

	return sb.String(), true
}

func writeEntry(sb *strings.Builder, kind Kind, ft *Feature, depth int) bool {
	if ft.Name == "" {
		return false
	}
	if kind == KindModel && ft.Type == TypeUnset {
		return false
	}
	if ft.HasBindings && (ft.Mode == ModeUnset || ft.Time == TimeUnset) {
		return false
	}

	indent := strings.Repeat(" ", depth*IndentUnit)
	sb.WriteString(indent)

	if ft.Grouping != GroupNone {
		sb.WriteString(ft.Grouping.String())
		sb.WriteByte(' ')
	}

	sb.WriteString(ft.Name)

	if kind == KindModel {
		writeModelTail(sb, ft)
	} else {
		writeConfigTail(sb, ft)
	}

	sb.WriteByte('\n')

	// A constraint is written one level in from the feature it applies to.
	for _, c := range ft.Constraints {
		sb.WriteString(strings.Repeat(" ", (depth+1)*IndentUnit))
		if c.Negated {
			sb.WriteByte('!')
		}
		sb.WriteByte('[')
		sb.WriteString(strings.Join(names(c.Operands), " "+c.Op.String()+" "))
		sb.WriteString("]\n")
	}

	for _, child := range ft.Children {
		if !writeEntry(sb, kind, child, depth+1) {
			return false
		}
	}

	return true
}

// writeModelTail writes `: type [{mode time}] [(value)] [?]`.
func writeModelTail(sb *strings.Builder, ft *Feature) {
	sb.WriteString(" : ")
	sb.WriteString(ft.Type.String())

	if ft.HasBindings {
		sb.WriteString(" {" + ft.Mode.String() + " " + ft.Time.String() + "}")
	}
	if ft.HasDefault {
		sb.WriteString(" (" + ft.DefaultText() + ")")
	}
	if v := ft.ValidatorText(); v != "" {
		sb.WriteString(" " + v)
	}
	if ft.Optional {
		sb.WriteString(" ?")
	}
}

// writeConfigTail writes `[marker][{mode time}] [(value)]`, where the marker
// sits against the braces: `nav *{static early}` but `boot {static late}`.
func writeConfigTail(sb *strings.Builder, ft *Feature) {
	if !ft.HasMarker && !ft.HasBindings && !ft.HasDefault {
		return
	}

	sb.WriteByte(' ')

	switch {
	case ft.Optional:
		sb.WriteByte('?')
	case ft.Selected:
		sb.WriteByte('*')
	}

	if ft.HasBindings {
		sb.WriteString("{" + ft.Mode.String() + " " + ft.Time.String() + "}")
	}
	if ft.HasDefault {
		if ft.HasBindings {
			sb.WriteByte(' ')
		}
		sb.WriteString("(" + ft.DefaultText() + ")")
	}
}
