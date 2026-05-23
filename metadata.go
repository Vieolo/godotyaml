package godotyaml

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
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
