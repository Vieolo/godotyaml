# Changelog

## v0.2.0 (2026-07-31)
- Added `MergeExternalConfig` to deep merge a config into `external.<name>`
- Added `SetExternalConfigKey` to replace one top-level key of a section outright, leaving the section's other keys intact.
- Added `RemoveExternalConfig` to remove an `external.<name>`
- Added `RemoveExternalConfigKey` to remove a single key from a section and reports whether it was present.
- Added `New` to build a new document from root `Metadata`, for tools offering an `init` command. Its `omitDefaults` argument chooses between an editable scaffold and a file holding only the fields that were set. The scaffold fills unset scalars with placeholders (`myapp`, `0.1.0`, `example.com` URLs), leaves `license` empty, and carries `authors`, `executables` and `external` as commented-out examples so their shape does not have to be looked up. `schema_version` is always written
- Added `SaveNew` to write a document only when the path is still free, so an `init` cannot modify an existing `go.yaml`
- Fixed the mapping collapse of the empty `external`
- `Save` is now atomic

#### Breaking changes
- Updated the underlying yaml dependency to `go.yaml.in/yaml/v3` which means that if you relied on raw `*yaml.Node`, you should change your imports


## v0.1.0 (2026-05-23)
- Initial release
