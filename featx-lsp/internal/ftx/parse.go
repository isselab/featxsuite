package ftx

import "strings"

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
	var rootIndent = -1

	headerDone := false

	for ln, raw := range strings.Split(src, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		toks := tokenize(line)
		if len(toks) == 0 {
			continue
		}

		if !headerDone {
			headerDone = true
			if f.parseHeader(ln, toks) {
				continue
			}
			// Not a header. Fall through so the line is still read as an entry
			// and the rest of the document keeps parsing.
		}

		indent := indentOf(line)
		ft := f.parseEntry(ln, indent, toks)
		if ft == nil {
			continue
		}

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
			"empty document: expected a %q or %q header", headerModel+":", headerConfig+":")
	}

	return f
}

// parseHeader consumes the document header, reporting whether the line was one.
func (f *File) parseHeader(ln int, toks []token) bool {
	head := toks[0]
	f.HeaderRange = rangeOf(ln, head)

	switch head.text {
	case headerModel:
		f.Kind = KindModel
	case headerConfig:
		f.Kind = KindConfig
	default:
		f.diag(SeverityError, rangeOf(ln, head),
			"expected the document to start with %q or %q, found %q",
			headerModel+":", headerConfig+":", head.text)
		return false
	}

	if len(toks) < 2 || toks[1].text != ":" {
		f.diag(SeverityError, rangeOf(ln, head), "expected %q after %q", ":", head.text)
		return true
	}

	rest := toks[2:]

	if f.Kind == KindConfig {
		if len(rest) > 0 {
			f.diag(SeverityError, rangeOf(ln, rest[0]),
				"%q takes no name, found %q", headerConfig+":", rest[0].text)
		}
		return true
	}

	switch {
	case len(rest) == 0:
		f.diag(SeverityError, rangeOf(ln, toks[1]), "expected a model name after %q", headerModel+":")
	case len(rest) > 1:
		f.diag(SeverityError, rangeOf(ln, rest[1]),
			"a model name cannot contain spaces, found %q", rest[1].text)
	default:
		f.ModelName = rest[0].text
		f.ModelNameRange = rangeOf(ln, rest[0])
	}

	return true
}

// parseEntry reads one feature entry. It returns nil only when the line has no
// usable feature name.
func (f *File) parseEntry(ln, indent int, toks []token) *Feature {
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
		ft.HasMarker = true
		ft.Optional = toks[i].text == "?"
		ft.Selected = toks[i].text == "*"
		ft.MarkerRange = rangeOf(ln, toks[i])
		i++
	}

	if i >= len(toks) {
		f.diag(SeverityError, ft.NameRange,
			"expected %q and a binding after feature %q", ":", ft.Name)
		return ft
	}

	if toks[i].text != ":" {
		f.diag(SeverityError, rangeOf(ln, toks[i]),
			"expected %q after feature %q, found %q", ":", ft.Name, toks[i].text)
		return ft
	}
	colon := toks[i]
	i++

	f.parseBinding(ln, ft, colon, toks[i:])
	return ft
}

// parseBinding reads the `<mode> <time>` pair that follows the colon. The two
// are accepted in either order; exactly one of each must be present.
func (f *File) parseBinding(ln int, ft *Feature, colon token, rest []token) {
	if len(rest) == 0 {
		f.diag(SeverityError, rangeOf(ln, colon),
			"expected a binding mode (static|dynamic) and a binding time (early|late) after %q", ":")
		return
	}

	for _, t := range rest {
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

	last := rangeOf(ln, rest[len(rest)-1])

	if ft.Mode == ModeUnset {
		f.diag(SeverityError, last, "feature %q has no binding mode (static|dynamic)", ft.Name)
	}
	if ft.Time == TimeUnset {
		f.diag(SeverityError, last, "feature %q has no binding time (early|late)", ft.Name)
	}
}

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

	for _, r := range line {
		width := 1
		if r > 0xFFFF {
			width = 2
		}

		switch r {
		case ' ', '\t':
			flush()
		case ':', '?', '*':
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

// isName reports whether s can be a feature or model name. Anything that is
// not punctuation or a keyword qualifies, so that a misspelled binding is
// reported as a bad binding rather than a bad name.
func isName(s string) bool {
	if s == "" || s == ":" || s == "?" || s == "*" {
		return false
	}
	return !isMode(s) && !isTime(s)
}

func isMode(s string) bool { _, ok := modeFrom(s); return ok }
func isTime(s string) bool { _, ok := timeFrom(s); return ok }

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
