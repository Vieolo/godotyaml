// Package godotyaml is the reference Go library for reading and writing
// go.yaml, a centralized config and metadata file for Go projects.
//
// It is the parser/helper layer other tools depend on so they do not each
// roll their own YAML handling for go.yaml. A go.yaml file has two zones: a
// closed set of root project metadata (name, version, schema_version,
// binaries, ...) exposed here as typed accessors, and an open external
// namespace where each tool stores arbitrary config under external.<toolname>.
// External sections are treated as opaque: the library reads them, hands them
// back to the caller to decode, and writes a single tool's section back without
// disturbing the rest of the file.
//
// The yaml.v3 node tree is kept as the internal source of truth so that
// comments, key ordering, and unknown root keys survive a load/save cycle, and
// so that updating one tool's section never re-serializes (and never corrupts)
// the rest of the document.
//
// # Non-goals
//
// godotyaml is deliberately small and unopinionated. It does NOT:
//
//   - validate the semantic correctness of any value (URLs, SPDX license
//     identifiers, version strings, and so on are returned verbatim);
//   - enforce any schema on the external namespace;
//   - refuse to parse files with unknown root keys or unknown schema_version
//     values — both are preserved and surfaced rather than rejected;
//   - expose helpers specific to any individual tool. No consuming tool is
//     privileged; every tool's config lives at external.<toolname> on equal
//     footing.
//
// It is not a CLI, not a validator, and not a schema enforcer.
package godotyaml
