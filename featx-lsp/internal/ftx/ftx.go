// Package ftx parses and analyses the FeatX feature-modelling DSL.
//
// Two document kinds share one grammar. A feature model declares the tree of
// available features and the bindings each one allows:
//
//	feature_model: RB-1-BASE
//	    xor navigation ?: static early
//	        to_loading : dynamic late
//
// A feature configuration selects from that tree and fixes the binding each
// feature is actually given:
//
//	feature_config:
//	    navigation *: static early
//	    to_loading : dynamic late
//
// An entry is `[grouping] name [marker]: <mode> <time>`, where the grouping
// (or|xor) constrains the entry's children, the marker is `?` for an optional
// feature or `*` for a selected one, mode is static|dynamic and time is
// early|late. Nesting is expressed with indentation.
package ftx

import "fmt"

// tabWidth is the column width a tab is counted as when measuring indentation.
const tabWidth = 4

// IndentUnit is the number of columns one nesting level adds. Top-level
// entries sit one unit in from the header.
const IndentUnit = 4

// Kind distinguishes the two document types, which is decided by the header.
type Kind int

const (
	KindUnknown Kind = iota
	KindModel        // feature_model: <name>
	KindConfig       // feature_config:
)

func (k Kind) String() string {
	switch k {
	case KindModel:
		return "feature model"
	case KindConfig:
		return "feature configuration"
	default:
		return "unknown"
	}
}

// Grouping is the constraint a feature imposes on its children.
type Grouping int

const (
	GroupNone Grouping = iota
	GroupOr            // one or more children
	GroupXor           // at most one child (alternative)
)

func (g Grouping) String() string {
	switch g {
	case GroupOr:
		return "or"
	case GroupXor:
		return "xor"
	default:
		return ""
	}
}

// Mode is a binding mode: whether a feature can be rebound at runtime.
type Mode int

const (
	ModeUnset Mode = iota
	ModeStatic
	ModeDynamic
)

func (m Mode) String() string {
	switch m {
	case ModeStatic:
		return "static"
	case ModeDynamic:
		return "dynamic"
	default:
		return ""
	}
}

// Time is a binding time: whether a feature is bound at compile time or later.
type Time int

const (
	TimeUnset Time = iota
	TimeEarly
	TimeLate
)

func (t Time) String() string {
	switch t {
	case TimeEarly:
		return "early"
	case TimeLate:
		return "late"
	default:
		return ""
	}
}

// Range is a span on a single line, in the coordinates LSP uses: a 0-based
// line and 0-based character offsets counted in UTF-16 code units.
type Range struct {
	Line      int
	StartChar int
	EndChar   int
}

// Contains reports whether the range covers the given line and character.
func (r Range) Contains(line, char int) bool {
	return r.Line == line && char >= r.StartChar && char <= r.EndChar
}

// Severity mirrors the LSP DiagnosticSeverity values.
type Severity int

const (
	SeverityError       Severity = 1
	SeverityWarning     Severity = 2
	SeverityInformation Severity = 3
	SeverityHint        Severity = 4
)

// Diagnostic is one problem found in a document.
type Diagnostic struct {
	Range    Range
	Severity Severity
	Message  string
}

// Feature is one entry in a model or configuration.
type Feature struct {
	Name      string
	NameRange Range

	// Grouping constrains this feature's children, not the feature itself.
	Grouping      Grouping
	GroupingRange Range

	Optional    bool // `?`, only meaningful in a model
	Selected    bool // `*`, only meaningful in a configuration
	HasMarker   bool
	MarkerRange Range

	Mode      Mode
	ModeRange Range
	Time      Time
	TimeRange Range

	Indent   int
	Parent   *Feature
	Children []*Feature

	// Line is the line the entry was declared on, kept for cross-references
	// even when the entry failed to parse completely.
	Line int
}

// Depth is the feature's nesting level, 0 for a top-level entry.
func (f *Feature) Depth() int {
	d := 0
	for p := f.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// Binding renders the feature's binding pair as it appears in source.
func (f *Feature) Binding() string {
	return fmt.Sprintf("%s %s", f.Mode, f.Time)
}

// File is a parsed document.
type File struct {
	Kind Kind

	ModelName      string
	ModelNameRange Range
	HeaderRange    Range

	Roots []*Feature
	Order []*Feature          // every feature, in source order
	Index map[string]*Feature // by name; the first declaration wins

	// Diagnostics holds the syntax problems found while parsing. Semantic
	// problems come from Analyze and CrossCheck.
	Diagnostics []Diagnostic
}

// Lookup returns the feature with the given name, if the document declares it.
func (f *File) Lookup(name string) (*Feature, bool) {
	ft, ok := f.Index[name]
	return ft, ok
}

// FeatureAt returns the feature whose entry covers the given position.
func (f *File) FeatureAt(line, char int) (*Feature, bool) {
	for _, ft := range f.Order {
		if ft.Line != line {
			continue
		}
		if ft.NameRange.Contains(line, char) ||
			ft.ModeRange.Contains(line, char) ||
			ft.TimeRange.Contains(line, char) ||
			ft.GroupingRange.Contains(line, char) {
			return ft, true
		}
	}
	return nil, false
}

func (f *File) diag(sev Severity, r Range, format string, args ...any) {
	f.Diagnostics = append(f.Diagnostics, Diagnostic{
		Range:    r,
		Severity: sev,
		Message:  fmt.Sprintf(format, args...),
	})
}

// utf16Len reports the number of UTF-16 code units in s, the unit LSP counts
// character offsets in.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xFFFF {
			n++
		}
	}
	return n
}
