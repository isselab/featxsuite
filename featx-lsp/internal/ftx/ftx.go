// Package ftx parses and analyses the FeatX feature-modelling DSL.
//
// A feature model opens with its root feature and declares the tree of
// available features, the type of each and the bindings it allows:
//
//	RB-1-BASE:
//	    xor navigation : node {static early} ?
//	        vel_max : number (1.6)
//	        to_loading : node {dynamic late}
//	            [square && attach && lift]
//
// A model entry is `[grouping] name : type [{mode time}] [(value)] [?]`, where
// the grouping (or|xor) constrains the entry's children, `?` marks the feature
// optional, and a bracketed line one level further in carries the cross-tree
// constraints: `[a && b]` requires, `![a]` excludes.
//
// A feature configuration selects from that tree and fixes the binding each
// node is actually given:
//
//	feature_config:
//	    navigation *{static early}
//	    vel_max *
//
// A configuration entry is `name [marker] [{mode time}] [(value)]`: it
// restates neither the type nor the grouping, `*` marks the feature selected,
// and a typed parameter takes no binding because it is bound with its node.
//
// Nesting is expressed with indentation in both dialects.
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

// Type is a feature's declared type. A node feature is a bindable unit of
// behaviour; the others are typed parameters of the node that encloses them.
type Type int

const (
	TypeUnset Type = iota
	TypeNode
	TypeNumber
	TypeString
	TypeBool
)

func (t Type) String() string {
	switch t {
	case TypeNode:
		return "node"
	case TypeNumber:
		return "number"
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	default:
		return ""
	}
}

// IsValue reports whether the type carries a value rather than behaviour.
func (t Type) IsValue() bool {
	return t == TypeNumber || t == TypeString || t == TypeBool
}

// Op is the connective joining the operands of a constraint.
type Op int

const (
	OpNone Op = iota // a single operand
	OpAnd            // &&
	OpOr             // ||
)

func (o Op) String() string {
	switch o {
	case OpAnd:
		return "&&"
	case OpOr:
		return "||"
	default:
		return ""
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

// Ref is a reference from one entry to another feature by name, carrying its
// source span so that a bad reference can be reported where it was written
// rather than on the whole line.
type Ref struct {
	Name  string
	Range Range
}

// names projects refs onto their names, in source order.
func names(refs []Ref) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Name
	}
	return out
}

// Constraint is a bracketed line attached to the feature above it: `[a && b]`
// requires every operand, `[a || b]` requires one of them, and the `!` prefix
// turns either into an exclusion.
type Constraint struct {
	Negated  bool
	Op       Op
	Operands []Ref

	// Range covers the whole bracketed expression, Line the line it sits on.
	Range Range
	Line  int
}

// Keyword names the constraint for diagnostics.
func (c Constraint) Keyword() string {
	if c.Negated {
		return "![...]"
	}
	return "[...]"
}

// Kind names what the constraint does, for prose.
func (c Constraint) Kind() string {
	if c.Negated {
		return "excludes"
	}
	return "requires"
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

	// IsRoot marks the topmost feature of a model, which is declared as
	// `NAME:` and needs no type: nothing binds or configures it.
	IsRoot bool

	Type      Type
	TypeRange Range

	// Bindings are written in braces and are mandatory on a node, which has
	// nothing else to say when it is bound. A value feature without them is
	// bound with the node that encloses it.
	HasBindings bool
	Mode        Mode
	ModeRange   Range
	Time        Time
	TimeRange   Range

	// Default is the literal in parentheses, verbatim. In a configuration it
	// is the value chosen rather than a fallback.
	HasDefault   bool
	Default      string
	DefaultRange Range

	// Constraints are the bracketed lines that followed this entry, in source
	// order, each either a requirement or an exclusion.
	Constraints []Constraint

	Indent   int
	Parent   *Feature
	Children []*Feature

	// Line is the line the entry was declared on, kept for cross-references
	// even when the entry failed to parse completely.
	Line int
}

// Depth is the feature's nesting level, 0 for a top-level entry. The root is
// not counted: it is the document, not a level of nesting.
func (f *Feature) Depth() int {
	d := 0
	for p := f.Parent; p != nil && !p.IsRoot; p = p.Parent {
		d++
	}
	return d
}

// IsAncestorOf reports whether f encloses other through the nesting.
func (f *Feature) IsAncestorOf(other *Feature) bool {
	for p := other.Parent; p != nil; p = p.Parent {
		if p == f {
			return true
		}
	}
	return false
}

// Binding renders the feature's binding pair as it appears in source.
func (f *Feature) Binding() string {
	return fmt.Sprintf("%s %s", f.Mode, f.Time)
}

// BindingNode walks outwards to the node whose bindings govern this feature,
// which is the feature itself when it declares its own.
func (f *Feature) BindingNode() *Feature {
	for n := f; n != nil; n = n.Parent {
		if n.HasBindings {
			return n
		}
	}
	return nil
}

// Requires returns the constraints this feature must satisfy.
func (f *Feature) Requires() []Constraint {
	return f.constraints(false)
}

// Excludes returns the constraints this feature must not satisfy.
func (f *Feature) Excludes() []Constraint {
	return f.constraints(true)
}

func (f *Feature) constraints(negated bool) []Constraint {
	var out []Constraint
	for _, c := range f.Constraints {
		if c.Negated == negated {
			out = append(out, c)
		}
	}
	return out
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

// TopLevel returns the entries beneath the root feature, or the roots
// themselves when the document declares none (a configuration, or a model
// whose first line failed to parse).
func (f *File) TopLevel() []*Feature {
	if len(f.Roots) == 1 && f.Roots[0].IsRoot {
		return f.Roots[0].Children
	}
	return f.Roots
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
