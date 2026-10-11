// Package ament reads what a built workspace says it provides.
//
// This is the only part of featx-lsp that looks outside the .ftx files. It
// exists so that a feature declared in a model can be checked against the
// implementation that is supposed to exist for it, which is otherwise only
// discovered when `featx load` fails at runtime.
//
// What it reads is the install space, not the sources: a component announces
// itself through the ament resource index when its package is built and
// installed, and a feature bound early is an executable in a package's lib
// directory. Nothing here can say anything useful about a workspace that has
// not been built.
package ament

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The resource index groups components register under, one per client
// library. Which group a component is in says which language implements it,
// and that is not only informative: a feature bound at compile time can only
// be implemented in the language that has one.
const (
	GroupCPP    = "rclcpp_components"
	GroupPython = "rclpy_components"
)

// Groups are the groups Read looks in.
var Groups = []string{GroupCPP, GroupPython}

// Registration is where one class came from.
type Registration struct {
	Package string
	Group   string
}

// Index is what a set of install prefixes provides.
type Index struct {
	// Components maps a registered class name, without its namespace, to where
	// it was registered. The namespace is dropped because that is how a
	// feature is resolved: by class name alone.
	Components map[string]Registration

	// Executables holds the packages that install an executable named after
	// themselves, which is the shape a manual composition takes.
	Executables map[string]bool

	// Prefixes are where this was read from, for reporting.
	Prefixes []string
}

// Empty reports whether nothing was found at all, which means the workspace is
// unbuilt or unsourced rather than that every feature is unimplemented.
func (i Index) Empty() bool {
	return len(i.Components) == 0 && len(i.Executables) == 0
}

// Prefixes returns the install prefixes on AMENT_PREFIX_PATH, nearest first.
func Prefixes() []string {
	raw := os.Getenv("AMENT_PREFIX_PATH")
	if raw == "" {
		return nil
	}

	var out []string
	for _, p := range strings.Split(raw, string(os.PathListSeparator)) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Read collects what the given prefixes provide.
func Read(prefixes []string) Index {
	index := Index{
		Components:  map[string]Registration{},
		Executables: map[string]bool{},
		Prefixes:    prefixes,
	}

	for _, prefix := range prefixes {
		for _, group := range Groups {
			readGroup(index, group,
				filepath.Join(prefix, "share", "ament_index", "resource_index", group))
		}
		readExecutables(index, prefix)
	}

	return index
}

// readGroup records every class registered by every package in one group.
func readGroup(index Index, group, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			name, _, target := cut(line, ";")
			if target == "" {
				continue
			}

			// A feature is resolved by class name, so the namespace a package
			// chose to register under does not matter here either.
			class := name
			if i := strings.LastIndex(name, "::"); i >= 0 {
				class = name[i+2:]
			}

			if class = strings.TrimSpace(class); class != "" {
				// First prefix on the path wins, as it does for the loader.
				if _, seen := index.Components[class]; !seen {
					index.Components[class] = Registration{
						Package: entry.Name(),
						Group:   group,
					}
				}
			}
		}
	}
}

// readExecutables records packages installing an executable of their own name,
// which is where ros2 run looks and so where a manual composition lands.
func readExecutables(index Index, prefix string) {
	lib := filepath.Join(prefix, "lib")

	entries, err := os.ReadDir(lib)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pkg := entry.Name()
		if info, err := os.Stat(filepath.Join(lib, pkg, pkg)); err == nil && !info.IsDir() {
			index.Executables[pkg] = true
		}
	}
}

var separators = regexp.MustCompile(`[\s_-]+`)

// ClassName is the class a feature's implementation must be called.
//
// It matches what featxcli does when it resolves a feature, including
// lowercasing the rest of each word, so that the two cannot disagree about
// whether an implementation exists.
func ClassName(feature string) string {
	var sb strings.Builder

	for _, word := range separators.Split(feature, -1) {
		if word == "" {
			continue
		}
		sb.WriteString(strings.ToUpper(word[:1]))
		sb.WriteString(strings.ToLower(word[1:]))
	}

	return sb.String()
}

// cut is strings.Cut, kept local so the package builds on the Go version in
// go.mod without relying on a newer standard library.
func cut(s, sep string) (before, found, after string) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], sep, s[i+len(sep):]
	}
	return s, "", ""
}
