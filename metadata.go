package godotyaml

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Author describes a single entry in the root authors field. Only Name is
// expected to be present and the remaining fields are optional and tools may carry
// additional information the spec adds later by decoding the raw node directly.
type Author struct {
	Name         string `yaml:"name"`
	Email        string `yaml:"email,omitempty"`
	Organization string `yaml:"organization,omitempty"`
	URL          string `yaml:"url,omitempty"`
}

// Executable describes one executable entry point the project produces. It is
// project metadata only as it carries no output paths, OS/arch targets, build
// flags, or per-executable versions (every executable inherits the single
// project version). Build-specific concerns belong in a build tool's
// external.<toolname> section, not here.
type Executable struct {
	Entrypoint  string `yaml:"entrypoint"`            // directory holding package main, relative to project root
	Description string `yaml:"description,omitempty"` // optional human-readable purpose
}

// Executables maps an author-chosen executable name to its definition. Absence
// of the root key and an empty map are equivalent (both have length 0): the
// project produces no executables, i.e. it is a library.
type Executables map[string]Executable

// Metadata is the root project metadata of a go.yaml, as a plain struct, so a
// tool can hand the whole of it to New at once.
//
// Every field is optional, matching the spec. A field left at its zero value is
// either written with a default value or left out of the file entirely,
// depending on New's omitDefaults argument.
type Metadata struct {
	Name          string
	Description   string
	Version       string
	SchemaVersion int
	Repository    string
	IssueTracker  string
	Homepage      string
	Documentation string
	License       string
	Authors       []Author
	Executables   Executables
}

// The placeholder values New writes for a scalar field the caller left unset.
//
// The URLs are all under example.com, which exists to be a placeholder and can
// never belong to somebody else. A scaffold that shipped a real-looking host
// would, left unedited, point a project's metadata at a repository or a site
// that is not the author's.
//
// license has no default on purpose. Every other field here is obviously a
// placeholder once read, but a licence is a legal claim about the project, and
// an unedited "MIT" left behind by a scaffold is a statement its author never
// made. An empty licence is the honest default.
const (
	defaultName          = "myapp"
	defaultDescription   = "my new app"
	defaultVersion       = "0.1.0"
	defaultRepository    = "https://example.com/myorg/myapp"
	defaultIssueTracker  = "https://example.com/myorg/myapp/issues"
	defaultHomepage      = "https://example.com"
	defaultDocumentation = "https://docs.example.com"
	defaultLicense       = ""
)

// The commented-out examples New writes for the two structured root fields.
//
// They are real Author and Executable values rather than hand-written YAML, so
// they are rendered by the same encoder that writes the file and cannot drift
// out of step with the types if the spec grows a field.
var (
	exampleAuthors = []Author{
		{Name: "Jane Doe", Email: "jane@example.com", Organization: "Example Org", URL: "https://example.com"},
		{Name: "John Smith"},
	}
	exampleExecutables = Executables{
		"server": {Entrypoint: "./cmd/server", Description: "HTTP API server."},
		"admin":  {Entrypoint: "./cmd/admin"},
	}
	exampleExternal = map[string]any{
		"mytool": map[string]any{
			"enabled": true,
			"paths":   []string{"./cmd"},
		},
	}
)

// builds a new go.yaml document from root metadata, for tools that offer an
// `init` command.
//
// Keys are written in the order the spec lists them. omitDefaults decides what
// becomes of a field the caller left at its zero value:
//
//   - false produces a scaffold meant to be edited. Every scalar key is written,
//     with a placeholder value for the ones the caller did not set, and authors,
//     executables and external follow as commented-out examples. The point of
//     the examples is that the shape of those three is the part nobody
//     remembers, so having it in the file removes a trip to the spec.
//   - true leaves unset keys out of the file altogether, so a caller that knows
//     only the project's name gets a three-line file rather than a scaffold. No
//     examples are written either.
//
// schema_version is written either way. Zero is the current schema version as
// well as the zero value, and a file that states which schema it was written
// against is far easier to migrate later.
//
// The examples are commented out rather than written as empty collections so
// that they are inert: the generated file parses to exactly the metadata the
// caller passed, and a tool reading it back sees no authors and no executables
// rather than examples it might mistake for real entries.
//
// Nothing here is validated, in keeping with the rest of the library: an empty
// name, or a version that is not a version, is written out as given.
//
// No external section is created; call SetExternalConfig or MergeExternalConfig
// on the result to add one, then Save (or SaveNew) to write the file. A section
// added that way is written above the commented examples, which stay in the file
// as documentation for the next tool.
func New(m Metadata, omitDefaults bool) *Document {
	d := newEmpty()

	appendScalarField(d.Root, "name", m.Name, defaultName, omitDefaults)
	appendScalarField(d.Root, "description", m.Description, defaultDescription, omitDefaults)
	appendScalarField(d.Root, "version", m.Version, defaultVersion, omitDefaults)
	appendMappingKey(d.Root, "schema_version", &yaml.Node{
		Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(m.SchemaVersion),
	})
	appendScalarField(d.Root, "repository", m.Repository, defaultRepository, omitDefaults)
	appendScalarField(d.Root, "issue_tracker", m.IssueTracker, defaultIssueTracker, omitDefaults)
	appendScalarField(d.Root, "homepage", m.Homepage, defaultHomepage, omitDefaults)
	appendScalarField(d.Root, "documentation", m.Documentation, defaultDocumentation, omitDefaults)
	appendScalarField(d.Root, "license", m.License, defaultLicense, omitDefaults)

	// A field the caller actually supplied is written for real; otherwise the
	// scaffold carries its shape as a commented example instead.
	var examples []string
	switch {
	case len(m.Authors) > 0:
		appendEncoded(d.Root, "authors", m.Authors)
	case !omitDefaults:
		examples = append(examples, exampleYAML("authors", exampleAuthors))
	}
	switch {
	case len(m.Executables) > 0:
		appendEncoded(d.Root, "executables", m.Executables)
	case !omitDefaults:
		examples = append(examples, exampleYAML("executables", exampleExecutables))
	}
	if !omitDefaults {
		examples = append(examples, exampleYAML("external", exampleExternal))
	}
	if len(examples) > 0 {
		// A foot comment on the root mapping renders after every key, and
		// survives a load/save cycle unchanged.
		d.Root.FootComment = strings.Join(examples, "\n\n")
	}
	return d
}

// appendScalarField appends key: value to mapping m. An unset field is skipped
// when the caller asked for defaults to be omitted, and otherwise takes the
// placeholder value. The value is tagged as a string, so the emitter quotes
// anything that would otherwise read back as a number or a bool.
func appendScalarField(m *yaml.Node, key, value, def string, omitDefaults bool) {
	if value == "" {
		if omitDefaults {
			return
		}
		value = def
	}
	appendMappingKey(m, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

// exampleYAML renders key: value as YAML for use as a comment body. The emitter
// adds the "# " prefixes when the comment is written, so the text is left plain
// here. An encoding failure is impossible for the values New passes and yields
// an empty block, which Join then drops.
func exampleYAML(key string, value any) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string]any{key: value}); err != nil {
		_ = enc.Close()
		return ""
	}
	if err := enc.Close(); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}

// appendEncoded appends key: <encoded value> to mapping m.
//
// Encoding cannot fail for the types New passes here (Author and Executable are
// structs of strings), and New has no error to return, so an impossible failure
// skips the key rather than writing a half-formed one. TestNewFieldsAndOrdering
// covers both fields, so a future field of an unencodable type fails the tests
// rather than silently disappearing.
func appendEncoded(m *yaml.Node, key string, value any) {
	var n yaml.Node
	if err := n.Encode(value); err != nil {
		return
	}
	appendMappingKey(m, key, &n)
}

// Name returns the root name field.
func (d *Document) Name() string { return d.scalar("name") }

// Description returns the root description field, or "" if absent.
func (d *Document) Description() string { return d.scalar("description") }

// Version returns the root version field.
func (d *Document) Version() string { return d.scalar("version") }

// SchemaVersion returns the root schema_version as an integer.
//
// The schema version is a single incrementing integer (0, 1, 2, ...). There are
// no minor versions such as 1.1. It returns (0, nil) when the key is absent. If
// the value is present but not a valid integer it returns a non-nil error: the
// file still parses (Load/Parse never reject it) and the malformed value is
// surfaced here rather than silently coerced. Quoted scalars (e.g. "1") are
// accepted.
func (d *Document) SchemaVersion() (int, error) {
	v := mappingValue(d.Root, "schema_version")
	if v == nil || v.Kind != yaml.ScalarNode {
		return 0, nil
	}
	n, err := strconv.Atoi(v.Value)
	if err != nil {
		return 0, fmt.Errorf("godotyaml: schema_version %q is not an integer: %w", v.Value, err)
	}
	return n, nil
}

// Repository returns the root repository field, or "" if absent.
func (d *Document) Repository() string { return d.scalar("repository") }

// IssueTracker returns the root issue_tracker field, or "" if absent.
func (d *Document) IssueTracker() string { return d.scalar("issue_tracker") }

// Homepage returns the root homepage field (the project's website), or "" if
// absent.
func (d *Document) Homepage() string { return d.scalar("homepage") }

// Documentation returns the root documentation field (the project's docs URL),
// or "" if absent.
func (d *Document) Documentation() string { return d.scalar("documentation") }

// License returns the root license field, or "" if absent.
func (d *Document) License() string { return d.scalar("license") }

// Authors returns the root authors field as a list.
//
// The field may appear in the file as a bare string, a single mapping, or a
// sequence of strings and/or mappings; all forms are normalized to []Author. A
// bare string becomes an Author with only Name set. It returns nil when the
// field is absent and an error only if a mapping entry cannot be decoded.
func (d *Document) Authors() ([]Author, error) {
	v := mappingValue(d.Root, "authors")
	if v == nil {
		return nil, nil
	}

	switch v.Kind {
	case yaml.ScalarNode:
		return []Author{{Name: v.Value}}, nil
	case yaml.MappingNode:
		var a Author
		if err := v.Decode(&a); err != nil {
			return nil, err
		}
		return []Author{a}, nil
	case yaml.SequenceNode:
		authors := make([]Author, 0, len(v.Content))
		for _, item := range v.Content {
			if item.Kind == yaml.ScalarNode {
				authors = append(authors, Author{Name: item.Value})
				continue
			}
			var a Author
			if err := item.Decode(&a); err != nil {
				return nil, err
			}
			authors = append(authors, a)
		}
		return authors, nil
	default:
		return nil, nil
	}
}

// Executables returns the root executables field, or nil if absent. Absence and
// an empty map are equivalent. It returns an error only if the section does not
// match the Executables shape.
func (d *Document) Executables() (Executables, error) {
	v := mappingValue(d.Root, "executables")
	if v == nil {
		return nil, nil
	}
	var e Executables
	if err := v.Decode(&e); err != nil {
		return nil, err
	}
	return e, nil
}

// scalar returns the string value of a root scalar key, or "" if the key is
// absent or its value is not a scalar.
func (d *Document) scalar(key string) string {
	v := mappingValue(d.Root, key)
	if v == nil || v.Kind != yaml.ScalarNode {
		return ""
	}
	return v.Value
}
