package server

import (
	"strings"
	"testing"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

// validatorNames picks the validator labels out of a completion response.
func validatorNames(items []lsp.CompletionItem) []string {
	var out []string
	for _, it := range items {
		if it.Detail == "validator" {
			out = append(out, it.Label)
		}
	}
	return out
}

const completionModel = `RB-1-BASE:
    demo : node {dynamic late}
        vel : double (1.5)
        mode : string (autonomous)
        inner : node {dynamic late}
`

// Only the validators that can apply to the parameter's type are offered, so
// the list answers "what can I write here" rather than "what exists".
func TestCompletionOffersValidatorsForTheType(t *testing.T) {
	file := ftx.Parse(completionModel)
	s := &Server{}

	cases := map[string]struct {
		prefix string
		line   int
		want   []string
	}{
		"a double takes ranges and an enumeration": {
			"        vel : double (1.5) ", 2,
			[]string{"bounds", "gt", "gt_eq", "lt", "lt_eq", "one_of"},
		},
		"a string takes lengths and an enumeration": {
			"        mode : string (autonomous) ", 3,
			[]string{"one_of", "fixed_size", "size_gt", "size_lt", "not_empty"},
		},
	}

	for name, c := range cases {
		got := validatorNames(s.completionsFor(file, "f.ftx", c.prefix, c.line))

		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}

// A node is bound rather than valued, so it carries no validator and none is
// suggested for one.
func TestCompletionOffersNoValidatorForANode(t *testing.T) {
	file := ftx.Parse(completionModel)
	s := &Server{}

	if got := validatorNames(s.completionsFor(file, "f.ftx", "        inner : node {dynamic late} ", 4)); len(got) != 0 {
		t.Errorf("offered %v for a node", got)
	}
}

// A validator may sit on its own line beneath its parameter, where there is no
// colon to go by, so the enclosing entry decides what is offered.
func TestCompletionOffersValidatorsOnAContinuationLine(t *testing.T) {
	file := ftx.Parse(completionModel)
	s := &Server{}

	if got := validatorNames(s.completionsFor(file, "f.ftx", "            ", 3)); len(got) == 0 {
		t.Error("offered nothing beneath a double")
	}

	got := validatorNames(s.completionsFor(file, "f.ftx", "            ", 4))
	if strings.Join(got, " ") != "one_of fixed_size size_gt size_lt not_empty" {
		t.Errorf("beneath a string: got %v", got)
	}
}

// A type comes before a validator, so until one is written the types are what
// is offered.
func TestCompletionOffersTypesBeforeValidators(t *testing.T) {
	file := ftx.Parse(completionModel)
	s := &Server{}

	items := s.completionsFor(file, "f.ftx", "        vel : ", 2)
	if len(validatorNames(items)) != 0 {
		t.Error("offered a validator before a type was written")
	}
	if len(items) == 0 {
		t.Error("offered no types")
	}
}

// Inside a validator's brackets sit its arguments, which are values of the
// parameter's type; suggesting validator names there would be wrong.
func TestCompletionIsQuietInsideValidatorBrackets(t *testing.T) {
	file := ftx.Parse(completionModel)
	s := &Server{}

	if items := s.completionsFor(file, "f.ftx", "        vel : double (1.5) bounds<", 2); len(items) != 0 {
		t.Errorf("offered %d item(s) inside a validator's arguments", len(items))
	}
}

// A validator that takes arguments is inserted open, so the cursor lands where
// they go; one that takes none is inserted closed.
func TestCompletionInsertsValidatorBrackets(t *testing.T) {
	file := ftx.Parse(completionModel)
	s := &Server{}

	for _, it := range s.completionsFor(file, "f.ftx", "        mode : string (autonomous) ", 3) {
		if it.Detail != "validator" {
			continue
		}
		want := it.Label + "<"
		if it.Label == "not_empty" {
			want = it.Label + "<>"
		}
		if it.InsertText != want {
			t.Errorf("%s inserts %q, want %q", it.Label, it.InsertText, want)
		}
	}
}
