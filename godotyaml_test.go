package godotyaml

import (
	"bytes"
	"strings"
	"testing"
)

// TestRoundTripPreservation loads a file with comments and unusual (non
// alphabetical) key ordering, mutates a single external section, and asserts
// that comments, the sibling section, unknown root keys, and ordering all
// survive.
func TestRoundTripPreservation(t *testing.T) {
	doc, err := Load("testdata/go.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := doc.Name(), "example-project"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if got, want := doc.Version(), "1.4.2"; got != want {
		t.Errorf("Version() = %q, want %q", got, want)
	}
	if got, want := doc.SchemaVersion(), "1"; got != want {
		t.Errorf("SchemaVersion() = %q, want %q", got, want)
	}

	// Mutate exactly one external section.
	newCfg := map[string]any{
		"linters": map[string]any{
			"enable": []string{"govet", "staticcheck", "errcheck"},
		},
		"run": map[string]any{"timeout": "10m"},
	}
	if err := doc.SetConfig("golangci-lint", newCfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := buf.String()

	// Comments must survive the round-trip.
	for _, want := range []string{
		"# go.yaml — project metadata and tool configuration.",
		"# The semantic version of the project.",
		"# gomore keeps an ordered list of tasks.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing comment %q\n---\n%s", want, out)
		}
	}

	// The unknown root key must be preserved, not dropped.
	if !strings.Contains(out, "future_field: experimental") {
		t.Errorf("output dropped unknown root key 'future_field'\n---\n%s", out)
	}

	// Reparse the written output and verify structure.
	rt, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}

	// Root key ordering must be preserved exactly (it is not alphabetical).
	var order []string
	for i := 0; i+1 < len(rt.root.Content); i += 2 {
		order = append(order, rt.root.Content[i].Value)
	}
	want := []string{
		"name", "version", "schema_version", "description", "repo",
		"issue_tracker", "license", "author", "future_field", "binaries",
		"external",
	}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("root key ordering changed:\n got %v\nwant %v", order, want)
	}

	// The sibling section (gomore) must be byte-for-byte intact after we only
	// touched golangci-lint.
	var gomore struct {
		Tasks    []string `yaml:"tasks"`
		Parallel bool     `yaml:"parallel"`
	}
	ok, err := rt.DecodeConfig("gomore", &gomore)
	if err != nil || !ok {
		t.Fatalf("DecodeConfig(gomore): ok=%v err=%v", ok, err)
	}
	if len(gomore.Tasks) != 2 || gomore.Tasks[0] != "build" || gomore.Tasks[1] != "test" || !gomore.Parallel {
		t.Errorf("sibling section gomore was altered: %+v", gomore)
	}

	// The mutated section must reflect the update.
	var lint struct {
		Linters map[string][]string `yaml:"linters"`
		Run     map[string]string   `yaml:"run"`
	}
	ok, err = rt.DecodeConfig("golangci-lint", &lint)
	if err != nil || !ok {
		t.Fatalf("DecodeConfig(golangci-lint): ok=%v err=%v", ok, err)
	}
	if len(lint.Linters["enable"]) != 3 || lint.Run["timeout"] != "10m" {
		t.Errorf("golangci-lint section not updated as expected: %+v", lint)
	}
}

func TestAuthorsAndBinaries(t *testing.T) {
	doc, err := Load("testdata/go.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	authors, err := doc.Authors()
	if err != nil {
		t.Fatalf("Authors: %v", err)
	}
	if len(authors) != 2 {
		t.Fatalf("Authors() len = %d, want 2", len(authors))
	}
	if authors[0].Name != "Jane Doe" || authors[0].Email != "jane@example.com" || authors[0].Organization != "Vieolo" {
		t.Errorf("authors[0] = %+v", authors[0])
	}
	if authors[1].Name != "John Smith" {
		t.Errorf("authors[1] = %+v", authors[1])
	}

	bins, err := doc.Binaries()
	if err != nil {
		t.Fatalf("Binaries: %v", err)
	}
	if bins["exampled"]["linux"] != "./dist/linux/exampled" {
		t.Errorf("binaries[exampled][linux] = %q", bins["exampled"]["linux"])
	}
	if bins["examplectl"]["windows"] != "./dist/windows/examplectl.exe" {
		t.Errorf("binaries[examplectl][windows] = %q", bins["examplectl"]["windows"])
	}
}

func TestConfigNamesAndMissing(t *testing.T) {
	doc, err := Load("testdata/go.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	names := doc.ConfigNames()
	if strings.Join(names, ",") != "gomore,golangci-lint" {
		t.Errorf("ConfigNames() = %v", names)
	}

	if _, ok := doc.Config("does-not-exist"); ok {
		t.Errorf("Config returned ok for a missing section")
	}
	ok, err := doc.DecodeConfig("does-not-exist", &struct{}{})
	if err != nil || ok {
		t.Errorf("DecodeConfig(missing): ok=%v err=%v", ok, err)
	}
}

func TestSetConfigOnEmptyDocument(t *testing.T) {
	doc, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse(empty): %v", err)
	}
	if err := doc.SetConfig("newtool", map[string]any{"enabled": true}); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(buf.String(), "external:") || !strings.Contains(buf.String(), "newtool:") {
		t.Errorf("empty-document write missing external section:\n%s", buf.String())
	}
}
