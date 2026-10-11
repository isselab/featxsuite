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

import (
	"fmt"
	"strings"
)

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
	TypeBool
	TypeInt
	TypeDouble
	TypeString
	TypeBoolArray
	TypeIntArray
	TypeDoubleArray
	TypeStringArray
)

// typeNames is the surface spelling of each type, which is also the spelling
// generate_parameter_library uses, so no translation is needed when emitting.
var typeNames = map[Type]string{
	TypeNode:        "node",
	TypeBool:        "bool",
	TypeInt:         "int",
	TypeDouble:      "double",
	TypeString:      "string",
	TypeBoolArray:   "bool_array",
	TypeIntArray:    "int_array",
	TypeDoubleArray: "double_array",
	TypeStringArray: "string_array",
}

func (t Type) String() string { return typeNames[t] }

// IsValue reports whether the type carries a value rather than behaviour.
func (t Type) IsValue() bool {
	return t != TypeUnset && t != TypeNode
}

// IsArray reports whether the type holds a list of values.
func (t Type) IsArray() bool {
	switch t {
	case TypeBoolArray, TypeIntArray, TypeDoubleArray, TypeStringArray:
		return true
	}
	return false
}

// IsNumeric reports whether the type can be compared against bounds.
func (t Type) IsNumeric() bool {
	switch t {
	case TypeInt, TypeDouble, TypeIntArray, TypeDoubleArray:
		return true
	}
	return false
}

// Element is the scalar type an array holds, or the type itself when it is
// already scalar.
func (t Type) Element() Type {
	switch t {
	case TypeBoolArray:
		return TypeBool
	case TypeIntArray:
		return TypeInt
	case TypeDoubleArray:
		return TypeDouble
	case TypeStringArray:
		return TypeString
	}
	return t
}

// ValidatorKind is the constraint a parameter places on its own value.
//
// Every validator is written by name, with its arguments in angle brackets, as
// generate_parameter_library spells it:
//
//	bounds<0.0, 3.0>      within a range, both ends admitted
//	gt<0.0>  gt_eq<0.0>   above a lower bound, excluded or admitted
//	lt<3.0>  lt_eq<3.0>   below an upper bound, excluded or admitted
//	one_of<slow, fast>    one of an enumeration
//	fixed_size<4>         an exact length
//	size_gt<2> size_lt<9> a length compared, strictly
//	not_empty<>           a length of at least one
//
// A name says which library function applies, so nothing has to be inferred
// from how many arguments were written or from a marker on one of them. A
// parameter may carry several, written one after another, which the library
// applies in turn.
type ValidatorKind int

const (
	ValidatorNone ValidatorKind = iota
	ValidatorBounds
	ValidatorGt
	ValidatorGtEq
	ValidatorLt
	ValidatorLtEq
	ValidatorOneOf
	ValidatorFixedSize
	ValidatorSizeGt
	ValidatorSizeLt
	ValidatorNotEmpty
)

// Domain is what a validator can be written on. The library decides these: a
// range compares the value, a size counts it, and some only make sense over
// the elements of an array.
type Domain int

const (
	// DomainNumeric compares the value itself, so it needs a numeric type.
	DomainNumeric Domain = iota
	// DomainAnyValue compares the value without ordering it.
	DomainAnyValue
	// DomainSized counts a length, which a string and an array both have.
	DomainSized
)

// Accepts reports whether a type may carry a validator of this domain.
func (d Domain) Accepts(t Type) bool {
	switch d {
	case DomainNumeric:
		return t.IsNumeric()
	case DomainSized:
		return t == TypeString || t.IsArray()
	default:
		return t.IsValue()
	}
}

func (d Domain) String() string {
	switch d {
	case DomainNumeric:
		return "a numeric type"
	case DomainSized:
		return "a string or an array"
	default:
		return "a parameter"
	}
}

// ValidatorSpec describes one validator. Everything that reads a validator —
// the parser, the checker, the formatter and the generator — reads it from
// here, so adding one is a row rather than a row and a handful of switches.
type ValidatorSpec struct {
	Kind ValidatorKind
	Name string // how it is written, and what the library calls it

	// MinArgs and MaxArgs bound the argument count. MaxArgs is -1 when any
	// number is allowed.
	MinArgs int
	MaxArgs int

	Domain Domain

	// Nested marks a validator the library takes as a single list argument,
	// which the generator writes one level deeper.
	Nested bool

	// Counts marks a validator whose arguments are lengths rather than values,
	// so they are whole numbers whatever the parameter's type.
	Counts bool
}

var validatorSpecs = []ValidatorSpec{
	{Kind: ValidatorBounds, Name: "bounds", MinArgs: 2, MaxArgs: 2, Domain: DomainNumeric},
	{Kind: ValidatorGt, Name: "gt", MinArgs: 1, MaxArgs: 1, Domain: DomainNumeric},
	{Kind: ValidatorGtEq, Name: "gt_eq", MinArgs: 1, MaxArgs: 1, Domain: DomainNumeric},
	{Kind: ValidatorLt, Name: "lt", MinArgs: 1, MaxArgs: 1, Domain: DomainNumeric},
	{Kind: ValidatorLtEq, Name: "lt_eq", MinArgs: 1, MaxArgs: 1, Domain: DomainNumeric},
	{Kind: ValidatorOneOf, Name: "one_of", MinArgs: 1, MaxArgs: -1, Domain: DomainAnyValue, Nested: true},
	{Kind: ValidatorFixedSize, Name: "fixed_size", MinArgs: 1, MaxArgs: 1, Domain: DomainSized, Counts: true},
	{Kind: ValidatorSizeGt, Name: "size_gt", MinArgs: 1, MaxArgs: 1, Domain: DomainSized, Counts: true},
	{Kind: ValidatorSizeLt, Name: "size_lt", MinArgs: 1, MaxArgs: 1, Domain: DomainSized, Counts: true},
	{Kind: ValidatorNotEmpty, Name: "not_empty", MinArgs: 0, MaxArgs: 0, Domain: DomainSized},
}

// Written is how the validator appears in a message: `bounds<>`, matching both
// the DSL and the generated schema.
func (s ValidatorSpec) Written() string { return s.Name + "<>" }

// arity describes the argument count a validator accepts.
func (s ValidatorSpec) arity() string {
	switch {
	case s.MaxArgs < 0 && s.MinArgs == 1:
		return "one argument or more"
	case s.MaxArgs < 0:
		return fmt.Sprintf("%d arguments or more", s.MinArgs)
	case s.MinArgs == s.MaxArgs && s.MinArgs == 1:
		return "one argument"
	case s.MinArgs == s.MaxArgs:
		return fmt.Sprintf("%d arguments", s.MinArgs)
	default:
		return fmt.Sprintf("%d to %d arguments", s.MinArgs, s.MaxArgs)
	}
}

// ValidatorSpecs returns every validator the DSL knows, in the order they are
// documented. Completion and hover read this.
func ValidatorSpecs() []ValidatorSpec { return validatorSpecs }

// LookupValidator finds a validator by the name it is written with.
func LookupValidator(name string) (ValidatorSpec, bool) {
	for _, spec := range validatorSpecs {
		if spec.Name == name {
			return spec, true
		}
	}
	return ValidatorSpec{}, false
}

// Spec returns the description of this kind.
func (v ValidatorKind) Spec() ValidatorSpec {
	for _, spec := range validatorSpecs {
		if spec.Kind == v {
			return spec
		}
	}
	return ValidatorSpec{}
}

// String is the name the validator is written with, which is also the library's
// own name for it.
func (v ValidatorKind) String() string { return v.Spec().Name }

// Library is how the generated schema keys this validator.
func (v ValidatorKind) Library() string { return v.Spec().Name + "<>" }

// IsRange reports whether this validator constrains a numeric interval.
func (v ValidatorKind) IsRange() bool {
	switch v {
	case ValidatorBounds, ValidatorGtEq, ValidatorGt, ValidatorLtEq, ValidatorLt:
		return true
	}
	return false
}

// HasLower reports whether the validator constrains the low end of a range.
func (v ValidatorKind) HasLower() bool {
	switch v {
	case ValidatorBounds, ValidatorGtEq, ValidatorGt:
		return true
	}
	return false
}

// HasUpper reports whether the validator constrains the high end of a range.
func (v ValidatorKind) HasUpper() bool {
	switch v {
	case ValidatorBounds, ValidatorLtEq, ValidatorLt:
		return true
	}
	return false
}

// LowerExcludes reports whether a value equal to the lower bound is rejected.
func (v ValidatorKind) LowerExcludes() bool { return v == ValidatorGt }

// UpperExcludes reports whether a value equal to the upper bound is rejected.
func (v ValidatorKind) UpperExcludes() bool { return v == ValidatorLt }

// Validator is a parameter's value constraint, with its arguments kept
// verbatim so that the generator and the formatter both reproduce them.
type Validator struct {
	Kind  ValidatorKind
	Args  []string
	Range Range

	// NameRange covers the name alone, for a diagnostic about the validator
	// rather than about one of its arguments.
	NameRange Range
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

	// DefaultValues are the literals in parentheses, one per element for an
	// array type. In a configuration they are the value chosen rather than a
	// fallback.
	HasDefault    bool
	DefaultValues []string
	DefaultRange  Range

	// Validators constrain the parameter's own value. A parameter may carry
	// several, which the library applies in turn.
	Validators []Validator

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

// DefaultText renders the parenthesised value as it appears in source.
func (f *Feature) DefaultText() string {
	return strings.Join(f.DefaultValues, ", ")
}

// ValidatorText renders the angle-bracketed validator as it appears in source,
// or "" when the feature has none.
func (f *Feature) ValidatorText() string {
	parts := make([]string, 0, len(f.Validators))

	for _, v := range f.Validators {
		if v.Kind == ValidatorNone {
			continue
		}
		parts = append(parts, v.Kind.String()+"<"+strings.Join(v.Args, ", ")+">")
	}

	return strings.Join(parts, " ")
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
