// Package godotyaml is the reference Go library for reading and writing
// go.yaml, a centralized config and metadata file for Go projects.
//
// It is the parser/helper layer other tools depend on so they do not each
// roll their own YAML handling for go.yaml. A go.yaml file has two zones: a
// closed set of root project metadata (name, version, schema_version,
// executables, ...) exposed here as typed accessors, and an open external
// namespace where each tool stores arbitrary config under external.<toolname>.
// New builds a document from that root metadata, so a tool offering an `init`
// command does not have to assemble the file itself.
// External sections are treated as opaque: the library reads them, hands them
// back to the caller to decode, and writes a single tool's section back without
// disturbing the rest of the file.
//
// Writing a section comes in two flavours, and which one a tool wants is a real
// decision rather than a detail. SetExternalConfig replaces external.<toolname>
// outright, so keys and hand-written comments inside it that the caller did not
// supply are discarded. MergeExternalConfig merges into it instead, leaving
// everything the caller did not mention (including comments) in place, at the
// cost of never removing anything. RemoveExternalConfig and
// RemoveExternalConfigKey are how things are removed deliberately.
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
