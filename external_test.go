package godotyaml

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The document from the integration report that motivated the merge API: a
// sibling section, a comment above external, and a hand-written comment plus a
// hand-added key inside the section a tool regenerates.
const sectionDoc = `name: myproject
version: 1.2.3
# a comment before external
external:
  gomore:
    commands:
      build: "go build ./..."
  contour:
    # a comment someone wrote by hand inside contour's section
    bootstrap: [python]
    note: keep-me
`

func parseDoc(t *testing.T, src string) *Document {
	t.Helper()
	d, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func render(t *testing.T, d *Document) string {
	t.Helper()
	var sb strings.Builder
	if err := d.Write(&sb); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return sb.String()
}

func mustContain(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n---\n%s---", w, out)
		}
	}
}

func mustNotContain(t *testing.T, out string, unwanted ...string) {
	t.Helper()
	for _, w := range unwanted {
		if strings.Contains(out, w) {
			t.Errorf("output unexpectedly contains %q\n---\n%s---", w, out)
		}
	}
}

// TestMergePreservesDocumentAndSection is the guarantee the library rests on:
// updating one tool's section disturbs neither the rest of the document nor the
// parts of that section the caller did not supply.
func TestMergePreservesDocumentAndSection(t *testing.T) {
	d := parseDoc(t, sectionDoc)

	if err := d.MergeExternalConfig("contour", map[string]any{"bootstrap": []string{"go"}}); err != nil {
		t.Fatalf("MergeExternalConfig: %v", err)
	}
	out := render(t, d)

	// The rest of the document.
	mustContain(t, out,
		"name: myproject",
		"version: 1.2.3",
		"# a comment before external",
		"build: \"go build ./...\"",
	)
	// The untouched parts of the merged section.
	mustContain(t, out,
		"# a comment someone wrote by hand inside contour's section",
		"note: keep-me",
	)

	rt := parseDoc(t, out)

	var contour struct {
		Bootstrap []string `yaml:"bootstrap"`
		Note      string   `yaml:"note"`
	}
	ok, err := rt.DecodeExternalConfig("contour", &contour)
	if err != nil || !ok {
		t.Fatalf("DecodeExternalConfig(contour): ok=%v err=%v", ok, err)
	}
	// Sequences are replaced wholesale, never appended to.
	if len(contour.Bootstrap) != 1 || contour.Bootstrap[0] != "go" {
		t.Errorf("bootstrap = %v, want [go]", contour.Bootstrap)
	}
	if contour.Note != "keep-me" {
		t.Errorf("note = %q, want keep-me (a merge must not drop keys it was not given)", contour.Note)
	}

	var gomore struct {
		Commands map[string]string `yaml:"commands"`
	}
	ok, err = rt.DecodeExternalConfig("gomore", &gomore)
	if err != nil || !ok {
		t.Fatalf("DecodeExternalConfig(gomore): ok=%v err=%v", ok, err)
	}
	if gomore.Commands["build"] != "go build ./..." {
		t.Errorf("sibling section altered: %+v", gomore)
	}
	if names := strings.Join(rt.ExternalConfigNames(), ","); names != "gomore,contour" {
		t.Errorf("section ordering changed: %s", names)
	}
}

// TestMergeIsDeepForMappings pins the merge semantics: mappings merge at every
// depth, sequences and scalars are replaced.
func TestMergeIsDeepForMappings(t *testing.T) {
	const src = `external:
  tool:
    commands:
      build: b
      test: t
    flags:
      - -v
      - -count=1
    timeout: 5m
`
	d := parseDoc(t, src)

	err := d.MergeExternalConfig("tool", map[string]any{
		"commands": map[string]any{"test": "t2"},
		"flags":    []string{"-race"},
		"timeout":  "10m",
	})
	if err != nil {
		t.Fatalf("MergeExternalConfig: %v", err)
	}

	rt := parseDoc(t, render(t, d))
	var got struct {
		Commands map[string]string `yaml:"commands"`
		Flags    []string          `yaml:"flags"`
		Timeout  string            `yaml:"timeout"`
	}
	if ok, err := rt.DecodeExternalConfig("tool", &got); err != nil || !ok {
		t.Fatalf("DecodeExternalConfig: ok=%v err=%v", ok, err)
	}

	if got.Commands["build"] != "b" {
		t.Errorf("nested key not preserved by deep merge: %+v", got.Commands)
	}
	if got.Commands["test"] != "t2" {
		t.Errorf("nested key not updated: %+v", got.Commands)
	}
	if len(got.Flags) != 1 || got.Flags[0] != "-race" {
		t.Errorf("sequence should be replaced wholesale, got %v", got.Flags)
	}
	if got.Timeout != "10m" {
		t.Errorf("scalar should be replaced, got %q", got.Timeout)
	}
}

// TestMergeNeverRemoves documents the cost of merge semantics: a key the caller
// stops supplying stays in the file. Removal is explicit.
func TestMergeNeverRemoves(t *testing.T) {
	d := parseDoc(t, sectionDoc)
	if err := d.MergeExternalConfig("contour", map[string]any{"bootstrap": []string{"go"}}); err != nil {
		t.Fatalf("MergeExternalConfig: %v", err)
	}
	mustContain(t, render(t, d), "note: keep-me")

	if !d.RemoveExternalConfigKey("contour", "note") {
		t.Error("RemoveExternalConfigKey returned false for a present key")
	}
	out := render(t, d)
	mustNotContain(t, out, "note: keep-me")
	mustContain(t, out, "bootstrap:", "# a comment before external", "build: \"go build ./...\"")
}

// TestMergeCommentHandling pins which comment wins when a caller supplies its
// own nodes: a comment it carries replaces the one in the file, an absent one
// leaves the file's comment alone.
func TestMergeCommentHandling(t *testing.T) {
	t.Run("existing comments survive a plain merge", func(t *testing.T) {
		d := parseDoc(t, "external:\n  tool:\n    # explains the key\n    a: 1 # trailing\n")
		if err := d.MergeExternalConfig("tool", map[string]any{"a": 2}); err != nil {
			t.Fatalf("MergeExternalConfig: %v", err)
		}
		out := render(t, d)
		mustContain(t, out, "# explains the key", "# trailing", "a: 2")
	})

	t.Run("caller comments replace the file's", func(t *testing.T) {
		d := parseDoc(t, sectionDoc)
		sec := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		sec.Content = append(sec.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "bootstrap", HeadComment: "regenerated by contour"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "go"},
		)
		if err := d.MergeExternalConfig("contour", sec); err != nil {
			t.Fatalf("MergeExternalConfig: %v", err)
		}
		out := render(t, d)
		mustContain(t, out, "# regenerated by contour", "note: keep-me")
		mustNotContain(t, out, "# a comment someone wrote by hand")
	})
}

func TestMergeCreatesMissingSectionAndExternal(t *testing.T) {
	d := parseDoc(t, "name: x\n")
	if err := d.MergeExternalConfig("tool", map[string]any{"a": 1}); err != nil {
		t.Fatalf("MergeExternalConfig: %v", err)
	}
	out := render(t, d)
	mustContain(t, out, "external:", "tool:", "a: 1")
	mustNotContain(t, out, "{")
}

// TestSetExternalConfigReplacesSection pins the documented destructive
// behaviour of SetExternalConfig, so the merge API cannot be mistaken for it.
func TestSetExternalConfigReplacesSection(t *testing.T) {
	d := parseDoc(t, "external:\n  # about contour\n  contour:\n    # inner\n    a: 1\n    note: keep-me\n")
	if err := d.SetExternalConfig("contour", map[string]any{"a": 2}); err != nil {
		t.Fatalf("SetExternalConfig: %v", err)
	}
	out := render(t, d)

	// The section's interior is gone...
	mustNotContain(t, out, "# inner", "note: keep-me")
	// ...but the comment on the section key itself is not, since that node is reused.
	mustContain(t, out, "# about contour", "a: 2")
}

func TestSetExternalConfigKey(t *testing.T) {
	t.Run("replaces one key and leaves the others", func(t *testing.T) {
		d := parseDoc(t, "external:\n  tool:\n    # about commands\n    commands:\n      build: b\n      test: t\n    note: keep-me\n")
		if err := d.SetExternalConfigKey("tool", "commands", map[string]any{"test": "t2"}); err != nil {
			t.Fatalf("SetExternalConfigKey: %v", err)
		}
		out := render(t, d)

		mustContain(t, out, "# about commands", "test: t2", "note: keep-me")
		mustNotContain(t, out, "build: b") // the key's value is replaced, not merged
	})

	t.Run("creates section and external when absent", func(t *testing.T) {
		d := parseDoc(t, "name: x\n")
		if err := d.SetExternalConfigKey("tool", "a", 1); err != nil {
			t.Fatalf("SetExternalConfigKey: %v", err)
		}
		mustContain(t, render(t, d), "external:", "tool:", "a: 1")
	})

	t.Run("replaces a non-mapping section with a mapping", func(t *testing.T) {
		d := parseDoc(t, "external:\n  tool: scalar\n")
		if err := d.SetExternalConfigKey("tool", "a", 1); err != nil {
			t.Fatalf("SetExternalConfigKey: %v", err)
		}
		out := render(t, d)
		mustContain(t, out, "a: 1")
		mustNotContain(t, out, "scalar")
	})
}

func TestRemoveExternalConfig(t *testing.T) {
	t.Run("removes only the named section", func(t *testing.T) {
		d := parseDoc(t, sectionDoc)
		if !d.RemoveExternalConfig("contour") {
			t.Fatal("RemoveExternalConfig returned false for a present section")
		}
		out := render(t, d)
		mustNotContain(t, out, "contour", "note: keep-me", "# a comment someone wrote by hand")
		mustContain(t, out, "# a comment before external", "gomore:", "build: \"go build ./...\"", "name: myproject")
	})

	t.Run("reports absence", func(t *testing.T) {
		d := parseDoc(t, sectionDoc)
		if d.RemoveExternalConfig("nope") {
			t.Error("RemoveExternalConfig returned true for a missing section")
		}
		if d := parseDoc(t, "name: x\n"); d.RemoveExternalConfig("nope") {
			t.Error("RemoveExternalConfig returned true with no external mapping")
		}
	})

	t.Run("keeps an emptied external and the comment above it", func(t *testing.T) {
		d := parseDoc(t, "name: x\n# about external\nexternal:\n  only:\n    a: 1\n")
		if !d.RemoveExternalConfig("only") {
			t.Fatal("RemoveExternalConfig returned false")
		}
		out := render(t, d)
		mustContain(t, out, "# about external", "external: {}")

		rt := parseDoc(t, out)
		if len(rt.ExternalConfigNames()) != 0 {
			t.Errorf("emptied external should have no sections, got %v", rt.ExternalConfigNames())
		}
	})
}

func TestRemoveExternalConfigKey(t *testing.T) {
	d := parseDoc(t, "external:\n  tool:\n    # about a\n    a: 1\n    b: 2\n")

	if d.RemoveExternalConfigKey("nope", "a") {
		t.Error("returned true for a missing section")
	}
	if d.RemoveExternalConfigKey("tool", "nope") {
		t.Error("returned true for a missing key")
	}
	if !d.RemoveExternalConfigKey("tool", "a") {
		t.Fatal("returned false for a present key")
	}

	out := render(t, d)
	mustNotContain(t, out, "a: 1", "# about a")
	mustContain(t, out, "b: 2")
}

// TestEmptyMappingDoesNotGoFlow is a regression test. An empty mapping is
// emitted as "{}" and parses back in flow style, and yaml.v3 forces flow style
// onto every descendant of a flow node. Writing into such a document used to
// collapse the whole subtree onto one inline line.
func TestEmptyMappingDoesNotGoFlow(t *testing.T) {
	cases := []struct {
		name string
		src  string
		set  func(d *Document) error
	}{
		{
			name: "empty external",
			src:  "name: x\nexternal: {}\n",
			set: func(d *Document) error {
				return d.SetExternalConfig("tool", map[string]any{"a": map[string]any{"b": 1}})
			},
		},
		{
			name: "empty root",
			src:  "{}\n",
			set:  func(d *Document) error { return d.SetExternalConfig("tool", map[string]any{"a": 1}) },
		},
		{
			name: "empty section merged into",
			src:  "external:\n  tool: {}\n",
			set: func(d *Document) error {
				return d.MergeExternalConfig("tool", map[string]any{"a": map[string]any{"b": 1}})
			},
		},
		{
			name: "empty section written by key",
			src:  "external:\n  tool: {}\n",
			set:  func(d *Document) error { return d.SetExternalConfigKey("tool", "a", map[string]any{"b": 1}) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := parseDoc(t, tc.src)
			if err := tc.set(d); err != nil {
				t.Fatalf("set: %v", err)
			}
			out := render(t, d)
			if strings.ContainsAny(out, "{}") {
				t.Errorf("output should be block style, got:\n%s", out)
			}
			mustContain(t, out, "external:", "tool:")
		})
	}
}

// TestRemoveThenWriteStaysBlock covers the round trip the emptied-external rule
// creates: remove the last section, save, reload, write again.
func TestRemoveThenWriteStaysBlock(t *testing.T) {
	d := parseDoc(t, "name: x\nexternal:\n  only:\n    a: 1\n")
	if !d.RemoveExternalConfig("only") {
		t.Fatal("RemoveExternalConfig returned false")
	}

	rt := parseDoc(t, render(t, d))
	if err := rt.SetExternalConfig("newtool", map[string]any{"a": map[string]any{"b": 1}}); err != nil {
		t.Fatalf("SetExternalConfig: %v", err)
	}
	out := render(t, rt)
	if strings.ContainsAny(out, "{}") {
		t.Errorf("output should be block style, got:\n%s", out)
	}
	mustContain(t, out, "newtool:", "b: 1")
}
