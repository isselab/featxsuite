package server

import (
	"os"

	"github.com/isselab/featxsuite/featx-lsp/internal/ftx"
	"github.com/isselab/featxsuite/featx-lsp/internal/lint"
	"github.com/isselab/featxsuite/featx-lsp/internal/lsp"
)

// publish analyses a document and sends its diagnostics to the client.
func (s *Server) publish(uri string) error {
	src, ok := s.text(uri)
	if !ok {
		return nil
	}

	path, err := uriToPath(uri)
	if err != nil {
		// Without a path the sibling model cannot be found, but everything
		// that does not need one is still worth reporting.
		s.log.Printf("checking %s: %v", uri, err)
	}

	diags := lint.Check(path, src, s.sourceOf)

	out := make([]lsp.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, lsp.Diagnostic{
			Range:    toRange(d.Range),
			Severity: int(d.Severity),
			Source:   Name,
			Message:  d.Message,
		})
	}

	return s.conn.Notify("textDocument/publishDiagnostics", lsp.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: out,
	})
}

// resolveModel finds the feature model that governs a configuration document
// and returns it with the URI it was loaded from.
func (s *Server) resolveModel(configURI string) (*ftx.File, string) {
	path, err := uriToPath(configURI)
	if err != nil {
		s.log.Printf("resolving model for %s: %v", configURI, err)
		return nil, ""
	}

	model, modelPath := lint.FindModel(path, s.sourceOf)
	if model == nil {
		return nil, ""
	}

	return model, pathToURI(modelPath)
}

// sourceOf returns the current text of a path, preferring an open buffer so
// that unsaved edits to a model are reflected in its configurations.
func (s *Server) sourceOf(path string) (string, bool) {
	if src, ok := s.text(pathToURI(path)); ok {
		return src, true
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	return string(b), true
}
