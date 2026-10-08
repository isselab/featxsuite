package ftx

import "strings"

// Format renders the document in canonical form: the header on the first
// line, then one entry per line indented IndentUnit columns per nesting level,
// written as `[grouping ]name <marker>: mode time` with single spaces between
// the parts. This is the style features.ftx and configs.ftx already use, so
// formatting either of them is a no-op.
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

	switch f.Kind {
	case KindModel:
		if f.ModelName == "" {
			return "", false
		}
		sb.WriteString(headerModel + ": " + f.ModelName + "\n")
	case KindConfig:
		sb.WriteString(headerConfig + ":\n")
	}

	for _, root := range f.Roots {
		if !writeEntry(&sb, root, 1) {
			return "", false
		}
	}

	return sb.String(), true
}

func writeEntry(sb *strings.Builder, ft *Feature, depth int) bool {
	if ft.Mode == ModeUnset || ft.Time == TimeUnset || ft.Name == "" {
		return false
	}

	sb.WriteString(strings.Repeat(" ", depth*IndentUnit))

	if ft.Grouping != GroupNone {
		sb.WriteString(ft.Grouping.String())
		sb.WriteByte(' ')
	}

	sb.WriteString(ft.Name)

	// The marker occupies the slot between the name and the colon, which stays
	// a single space wide when there is no marker: `nav ?:` but `boot :`.
	sb.WriteByte(' ')

	switch {
	case ft.Optional:
		sb.WriteByte('?')
	case ft.Selected:
		sb.WriteByte('*')
	}

	sb.WriteString(": ")
	sb.WriteString(ft.Mode.String())
	sb.WriteByte(' ')
	sb.WriteString(ft.Time.String())
	sb.WriteByte('\n')

	for _, child := range ft.Children {
		if !writeEntry(sb, child, depth+1) {
			return false
		}
	}

	return true
}
