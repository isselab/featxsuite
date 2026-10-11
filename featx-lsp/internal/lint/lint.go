// Package lint runs the full diagnostic pipeline over a document, including
// the cross-file rules that need the document's sibling feature model.
//
// It sits between the pure ftx analyser and the two front ends: the language
// server, which resolves text from open editor buffers, and the -lint command,
// which reads from disk. Both get identical verdicts because both come through
// here.
package lint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
)

// Ext is the extension featx DSL documents use.
const Ext = ".ftx"

// SourceFunc returns the current text of a path. A front end that holds
// unsaved buffers supplies its own so that edits are seen before they are
// written.
type SourceFunc func(path string) (string, bool)

// ReadFile is the SourceFunc that reads from disk.
func ReadFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// Check parses src and returns every diagnostic that applies to it. For a
// feature configuration that includes the cross-file rules, checked against
// the model FindModel locates beside it; a configuration with no model nearby
// is still checked for everything that does not need one.
func Check(path, src string, source SourceFunc) []ftx.Diagnostic {
	file := ftx.Parse(src)

	diags := make([]ftx.Diagnostic, 0, len(file.Diagnostics))
	diags = append(diags, file.Diagnostics...)
	diags = append(diags, ftx.Analyze(file)...)

	if file.Kind == ftx.KindConfig {
		if model, _ := FindModel(path, source); model != nil {
			diags = append(diags, ftx.CrossCheck(file, model)...)
		}
	}

	return diags
}

// FindModel locates the feature model governing the document at path, by
// scanning that document's own directory for a .ftx file whose header is
// feature_model. It returns the parsed model and the path it came from, or
// nil when the directory holds no model.
func FindModel(path string, source SourceFunc) (*ftx.File, string) {
	dir := filepath.Dir(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ""
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), Ext) {
			continue
		}
		names = append(names, e.Name())
	}

	// Deterministic order, so a directory holding two models always resolves
	// the same way rather than following the filesystem's whim.
	sort.Strings(names)

	for _, name := range names {
		candidate := filepath.Join(dir, name)
		if candidate == path {
			continue
		}

		src, ok := source(candidate)
		if !ok {
			continue
		}

		if file := ftx.Parse(src); file.Kind == ftx.KindModel {
			return file, candidate
		}
	}

	return nil, ""
}
