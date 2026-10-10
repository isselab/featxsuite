package ament

import (
	"os"
	"path/filepath"
	"testing"
)

// prefix builds an install prefix: components registered under a group, and
// executables in lib/<pkg>/<pkg>.
func prefix(t *testing.T, components map[string]string, executables ...string) string {
	t.Helper()

	root := t.TempDir()

	for group, body := range components {
		dir := filepath.Join(root, "share", "ament_index", "resource_index", filepath.Dir(group))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(group)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, pkg := range executables {
		dir := filepath.Join(root, "lib", pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, pkg), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func TestClassNameMatchesTheResolversConvention(t *testing.T) {
	cases := map[string]string{
		"lift":        "Lift",
		"to_loading":  "ToLoading",
		"init_checks": "InitChecks",
		"init-checks": "InitChecks",
		"two words":   "TwoWords",
		// The rest of each word is lowercased, as featxcli does, so that the
		// two cannot disagree about whether an implementation exists.
		"RB-1-BASE": "Rb1Base",
		"":          "",
	}

	for feature, want := range cases {
		if got := ClassName(feature); got != want {
			t.Errorf("ClassName(%q) = %q, want %q", feature, got, want)
		}
	}
}

func TestReadFindsComponentsInBothGroups(t *testing.T) {
	root := prefix(t, map[string]string{
		"rclcpp_components/elevator":  "featx_plugin::Lift;lib/liblift.so\nfeatx_plugin::Drop;lib/libdrop.so\n",
		"rclpy_components/py_plugins": "featx_plugin::Boot;py_plugins.boot:Boot\n",
	})

	index := Read([]string{root})

	for class, wantPkg := range map[string]string{
		"Lift": "elevator",
		"Drop": "elevator",
		"Boot": "py_plugins",
	} {
		if got := index.Components[class]; got != wantPkg {
			t.Errorf("Components[%q] = %q, want %q", class, got, wantPkg)
		}
	}
}

func TestReadDropsTheNamespace(t *testing.T) {
	// A feature is resolved by class name, so a package may register under any
	// namespace and still be found.
	root := prefix(t, map[string]string{
		"rclcpp_components/vendor": "some_other_ns::Hoist;lib/libhoist.so\n",
	})

	if _, ok := Read([]string{root}).Components["Hoist"]; !ok {
		t.Error("a class in an unexpected namespace was not found")
	}
}

func TestReadSkipsCommentsAndMalformedLines(t *testing.T) {
	root := prefix(t, map[string]string{
		"rclpy_components/pkg": "# a comment\n\nfeatx_plugin::Good;pkg.mod:Good\nnotargethere\n",
	})

	index := Read([]string{root})

	if _, ok := index.Components["Good"]; !ok {
		t.Error("a well-formed entry was skipped")
	}
	if len(index.Components) != 1 {
		t.Errorf("Components = %v, want only Good", index.Components)
	}
}

func TestReadFindsExecutablesOnlyWhenNamedAfterThePackage(t *testing.T) {
	root := prefix(t, nil, "elevator", "navigation")

	// A package whose lib directory holds something else does not count: a
	// manual composition is installed as lib/<pkg>/<pkg>.
	other := filepath.Join(root, "lib", "localization")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "something_else"), []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}

	index := Read([]string{root})

	for _, want := range []string{"elevator", "navigation"} {
		if !index.Executables[want] {
			t.Errorf("executable %q not found", want)
		}
	}
	if index.Executables["localization"] {
		t.Error("a package with no eponymous executable was counted")
	}
}

func TestReadAcrossPrefixesPrefersTheFirst(t *testing.T) {
	// AMENT_PREFIX_PATH is ordered, and the loader takes the first match, so
	// this must agree with it.
	first := prefix(t, map[string]string{"rclcpp_components/mine": "featx_plugin::Lift;lib/a.so\n"})
	second := prefix(t, map[string]string{"rclcpp_components/theirs": "featx_plugin::Lift;lib/b.so\n"})

	if got := Read([]string{first, second}).Components["Lift"]; got != "mine" {
		t.Errorf("Components[Lift] = %q, want mine (the first prefix)", got)
	}
}

func TestEmptyDistinguishesNothingBuiltFromNothingFound(t *testing.T) {
	if !Read(nil).Empty() {
		t.Error("an index read from no prefixes should be empty")
	}
	if !Read([]string{t.TempDir()}).Empty() {
		t.Error("an index read from an empty prefix should be empty")
	}

	root := prefix(t, nil, "elevator")
	if Read([]string{root}).Empty() {
		t.Error("an index with an executable is not empty")
	}
}

func TestReadIgnoresAMissingPrefix(t *testing.T) {
	// A stale entry on AMENT_PREFIX_PATH must not stop the rest being read.
	root := prefix(t, map[string]string{"rclcpp_components/pkg": "featx_plugin::Lift;lib/a.so\n"})

	index := Read([]string{filepath.Join(root, "gone"), root})

	if _, ok := index.Components["Lift"]; !ok {
		t.Error("a missing prefix prevented the others being read")
	}
}

func TestPrefixesSplitsThePath(t *testing.T) {
	t.Setenv("AMENT_PREFIX_PATH", "/one"+string(os.PathListSeparator)+"/two")

	got := Prefixes()
	if len(got) != 2 || got[0] != "/one" || got[1] != "/two" {
		t.Errorf("Prefixes() = %v, want [/one /two]", got)
	}

	t.Setenv("AMENT_PREFIX_PATH", "")
	if got := Prefixes(); got != nil {
		t.Errorf("Prefixes() = %v, want nil when unset", got)
	}
}
