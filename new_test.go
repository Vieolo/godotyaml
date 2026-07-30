package godotyaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewFieldsAndOrdering pins the generated layout: spec key order, every
// field rendered, and the encoded authors/executables sections intact.
func TestNewFieldsAndOrdering(t *testing.T) {
	d := New(Metadata{
		Name:          "myproject",
		Description:   "A useful tool",
		Version:       "1.2.3",
		SchemaVersion: 0,
		Repository:    "https://github.com/vieolo/myproject",
		IssueTracker:  "https://github.com/vieolo/myproject/issues",
		Homepage:      "https://example.com",
		Documentation: "https://docs.example.com",
		License:       "MIT",
		Authors: []Author{
			{Name: "Jane Doe", Email: "jane@example.com", Organization: "Vieolo", URL: "example.com"},
			{Name: "John Smith"},
		},
		Executables: Executables{
			"server": {Entrypoint: "./cmd/server", Description: "HTTP API server."},
			"admin":  {Entrypoint: "./cmd/admin"},
		},
	}, true)

	out := render(t, d)
	mustContain(t, out,
		"name: myproject",
		"description: A useful tool",
		"version: 1.2.3",
		"schema_version: 0",
		"repository: https://github.com/vieolo/myproject",
		"issue_tracker: https://github.com/vieolo/myproject/issues",
		"homepage: https://example.com",
		"documentation: https://docs.example.com",
		"license: MIT",
		"organization: Vieolo",
		"entrypoint: ./cmd/server",
		"description: HTTP API server.",
	)

	rt := parseDoc(t, out)

	var order []string
	for i := 0; i+1 < len(rt.Root.Content); i += 2 {
		order = append(order, rt.Root.Content[i].Value)
	}
	want := []string{
		"name", "description", "version", "schema_version", "repository",
		"issue_tracker", "homepage", "documentation", "license", "authors",
		"executables",
	}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("key order:\n got %v\nwant %v", order, want)
	}

	// The generated document must read back through the library's own accessors.
	if rt.Name() != "myproject" || rt.Version() != "1.2.3" || rt.License() != "MIT" {
		t.Errorf("accessors disagree: name=%q version=%q license=%q", rt.Name(), rt.Version(), rt.License())
	}
	if sv, err := rt.SchemaVersion(); err != nil || sv != 0 {
		t.Errorf("SchemaVersion() = %d, err = %v", sv, err)
	}
	authors, err := rt.Authors()
	if err != nil || len(authors) != 2 || authors[0].Email != "jane@example.com" || authors[1].Name != "John Smith" {
		t.Errorf("Authors() = %+v, err = %v", authors, err)
	}
	execs, err := rt.Executables()
	if err != nil || len(execs) != 2 || execs["server"].Entrypoint != "./cmd/server" {
		t.Errorf("Executables() = %+v, err = %v", execs, err)
	}
	if execs["admin"].Description != "" {
		t.Errorf("optional description should stay empty, got %q", execs["admin"].Description)
	}
}

// With omitDefaults, a field left at its zero value is left out entirely, so a
// minimal init produces a minimal file. schema_version is the exception.
func TestNewOmitsUnsetFields(t *testing.T) {
	out := render(t, New(Metadata{Name: "lib", Version: "0.1.0"}, true))

	if want := "name: lib\nversion: 0.1.0\nschema_version: 0\n"; out != want {
		t.Errorf("minimal document:\n got %q\nwant %q", out, want)
	}

	// Even a wholly empty Metadata declares the schema it was written against.
	if got := render(t, New(Metadata{}, true)); got != "schema_version: 0\n" {
		t.Errorf("empty Metadata produced %q", got)
	}

	if got := render(t, New(Metadata{SchemaVersion: 3}, true)); got != "schema_version: 3\n" {
		t.Errorf("non-zero schema version produced %q", got)
	}
}

// The scaffold an `init` command generates: placeholders for the scalar fields,
// and the shapes nobody remembers carried as commented-out examples.
const wantScaffold = `name: myapp
description: my new app
version: 0.1.0
schema_version: 0
repository: https://example.com/myorg/myapp
issue_tracker: https://example.com/myorg/myapp/issues
homepage: https://example.com
documentation: https://docs.example.com
license: ""

# authors:
#   - name: Jane Doe
#     email: jane@example.com
#     organization: Example Org
#     url: https://example.com
#   - name: John Smith

# executables:
#   admin:
#     entrypoint: ./cmd/admin
#   server:
#     entrypoint: ./cmd/server
#     description: HTTP API server.

# external:
#   mytool:
#     enabled: true
#     paths:
#       - ./cmd
`

func TestNewWritesDefaults(t *testing.T) {
	if got := render(t, New(Metadata{}, false)); got != wantScaffold {
		t.Errorf("scaffold:\n got:\n%s\nwant:\n%s", got, wantScaffold)
	}

	t.Run("license is left empty on purpose", func(t *testing.T) {
		// A licence is a legal claim; a scaffold must not make one for the
		// author. Every other scalar gets a placeholder.
		if !strings.Contains(wantScaffold, `license: ""`) {
			t.Error("scaffold should not default the licence to a real identifier")
		}
	})

	t.Run("set fields keep their values, the rest default", func(t *testing.T) {
		out := render(t, New(Metadata{Name: "myproject", License: "MIT"}, false))
		mustContain(t, out,
			"name: myproject",
			"license: MIT",
			"description: my new app",
			"version: 0.1.0",
		)
		mustNotContain(t, out, "name: myapp")
	})

	// The examples are comments, so the document a tool reads back holds exactly
	// the metadata that was passed and nothing the examples suggest.
	t.Run("the examples are inert", func(t *testing.T) {
		rt := parseDoc(t, wantScaffold)

		if rt.Name() != "myapp" || rt.Version() != "0.1.0" || rt.License() != "" {
			t.Errorf("accessors: name=%q version=%q license=%q", rt.Name(), rt.Version(), rt.License())
		}
		if sv, err := rt.SchemaVersion(); err != nil || sv != 0 {
			t.Errorf("SchemaVersion() = %d, err = %v", sv, err)
		}
		authors, err := rt.Authors()
		if err != nil || len(authors) != 0 {
			t.Errorf("commented authors must not decode as real ones: %+v (err %v)", authors, err)
		}
		execs, err := rt.Executables()
		if err != nil || len(execs) != 0 {
			t.Errorf("commented executables must not decode as real ones: %+v (err %v)", execs, err)
		}
		if names := rt.ExternalConfigNames(); len(names) != 0 {
			t.Errorf("commented external must not decode as a real section: %v", names)
		}
	})

	// A user edits the scaffold and a tool then writes to it; the examples must
	// not shift, duplicate, or vanish.
	t.Run("survives a load/save cycle unchanged", func(t *testing.T) {
		once := render(t, New(Metadata{}, false))
		twice := render(t, parseDoc(t, once))
		if once != twice {
			t.Errorf("scaffold is not stable across a round trip:\n got:\n%s\nwant:\n%s", twice, once)
		}
		if thrice := render(t, parseDoc(t, twice)); thrice != twice {
			t.Errorf("scaffold drifts on the third pass:\n%s", thrice)
		}
	})

	t.Run("a supplied field replaces its example", func(t *testing.T) {
		out := render(t, New(Metadata{
			Authors:     []Author{{Name: "Vieolo"}},
			Executables: Executables{"cli": {Entrypoint: "./cmd/cli"}},
		}, false))

		mustContain(t, out, "authors:\n  - name: Vieolo", "executables:\n  cli:")
		mustNotContain(t, out, "# authors:", "# executables:", "Jane Doe")
		mustContain(t, out, "# external:") // still absent, so still exemplified
	})

	t.Run("a scaffold still accepts a real external section", func(t *testing.T) {
		d := New(Metadata{Name: "x"}, false)
		if err := d.SetExternalConfig("mytool", map[string]any{"a": 1}); err != nil {
			t.Fatalf("SetExternalConfig: %v", err)
		}
		out := render(t, d)
		mustContain(t, out, "external:\n  mytool:\n    a: 1")
		mustNotContain(t, out, "external: {")

		// The result must still parse, with the real section visible and the
		// commented one ignored.
		rt := parseDoc(t, out)
		if names := strings.Join(rt.ExternalConfigNames(), ","); names != "mytool" {
			t.Errorf("ExternalConfigNames() = %q, want mytool", names)
		}
	})
}

// With omitDefaults the caller wants a minimal file, so no examples either.
func TestNewOmitsExamplesWhenOmittingDefaults(t *testing.T) {
	out := render(t, New(Metadata{Name: "lib"}, true))
	if strings.Contains(out, "#") {
		t.Errorf("minimal document should carry no commented examples:\n%s", out)
	}
}

// Values that would read back as a different type must survive the round trip.
func TestNewQuotesAmbiguousScalars(t *testing.T) {
	d := New(Metadata{Name: "true", Version: "1.0"}, true)
	rt := parseDoc(t, render(t, d))

	if rt.Name() != "true" {
		t.Errorf("Name() = %q, want %q", rt.Name(), "true")
	}
	if rt.Version() != "1.0" {
		t.Errorf("Version() = %q, want %q", rt.Version(), "1.0")
	}
}

// The whole point: New composes with the external API and the result is a normal
// document.
func TestNewWithExternalSection(t *testing.T) {
	d := New(Metadata{Name: "myproject", Version: "0.1.0"}, true)
	if err := d.SetExternalConfig("mytool", map[string]any{"enabled": true}); err != nil {
		t.Fatalf("SetExternalConfig: %v", err)
	}
	if err := d.MergeExternalConfig("othertool", map[string]any{"a": 1}); err != nil {
		t.Fatalf("MergeExternalConfig: %v", err)
	}

	out := render(t, d)
	if strings.ContainsAny(out, "{}") {
		t.Errorf("output should be block style, got:\n%s", out)
	}

	rt := parseDoc(t, out)
	if names := strings.Join(rt.ExternalConfigNames(), ","); names != "mytool,othertool" {
		t.Errorf("ExternalConfigNames() = %s", names)
	}
}

func TestSaveNew(t *testing.T) {
	t.Run("creates the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "go.yaml")
		if err := New(Metadata{Name: "x", Version: "0.1.0"}, true).SaveNew(path); err != nil {
			t.Fatalf("SaveNew: %v", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(content), "name: x\nversion: 0.1.0\nschema_version: 0\n"; got != want {
			t.Errorf("content = %q, want %q", got, want)
		}
	})

	t.Run("refuses to overwrite and leaves the file untouched", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "go.yaml")
		const original = "name: existing\nexternal:\n  othertool:\n    keep: me\n"
		if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		err := New(Metadata{Name: "new"}, true).SaveNew(path)
		if err == nil {
			t.Fatal("SaveNew overwrote an existing go.yaml")
		}
		if !os.IsExist(err) {
			t.Errorf("error should satisfy os.IsExist, got %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != original {
			t.Errorf("existing file was modified:\n%s", content)
		}
	})

	t.Run("refuses to overwrite an empty file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "go.yaml")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := New(Metadata{Name: "new"}, true).SaveNew(path); !os.IsExist(err) {
			t.Errorf("err = %v, want an os.IsExist error", err)
		}
	})

	t.Run("leaves no stub behind when the write fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "go.yaml") // parent does not exist
		if err := New(Metadata{Name: "x"}, true).SaveNew(path); err == nil {
			t.Fatal("SaveNew into a missing directory should fail")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("failed SaveNew left %v behind", entries)
		}
	})

	t.Run("created file matches os.WriteFile's mode", func(t *testing.T) {
		dir := t.TempDir()
		ref := filepath.Join(dir, "ref.yaml")
		if err := os.WriteFile(ref, []byte("name: x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		refInfo, err := os.Stat(ref)
		if err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(dir, "go.yaml")
		if err := New(Metadata{Name: "x"}, true).SaveNew(path); err != nil {
			t.Fatalf("SaveNew: %v", err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := fi.Mode().Perm(), refInfo.Mode().Perm(); got != want {
			t.Errorf("mode = %v, want %v", got, want)
		}
		if len(mustReadDirNames(t, dir)) != 2 {
			t.Errorf("unexpected leftovers: %v", mustReadDirNames(t, dir))
		}
	})
}

func mustReadDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
