# godotyaml

`godotyaml` is the reference Go library for reading and writing `go.yaml`, the
centralized config and metadata file for Go projects. It is the parser/helper
layer that other tools depend on so they don't each have to roll their own YAML
handling for `go.yaml`.

## What it is, and what it is not

`godotyaml` is small and focused. It parses `go.yaml` into Go structures, exposes
the root project metadata as typed accessors, and lets a tool read and write its
own section of the `external` namespace without disturbing the rest of the file.

It is deliberately **not** a CLI, **not** a validator, and **not** a schema
enforcer. Specifically, it does **not**:

- validate the semantic correctness of any value (URLs, SPDX license IDs,
  version strings, etc. are returned verbatim);
- enforce any schema on the `external` namespace;
- refuse to parse files with unknown root keys or unknown `schema_version`
  values — both are preserved and surfaced rather than rejected;
- expose helpers specific to any individual tool.

## The three-repo relationship

`godotyaml` is one of three pieces:

- **`go.yaml`** — the standard (a spec, not code).
- **`godotyaml`** — this repo, the reference library.
- **`gomore`** — a separate CLI that consumes `godotyaml` like any other tool.

No tool has special authority over `go.yaml`. `gomore` is not privileged: its
config lives at `external.gomore`, exactly like any other tool's config (e.g.
`external.golangci-lint`). This repo contains no knowledge of any specific
tool's schema.

## The `go.yaml` structure

A `go.yaml` file has two zones:

- **Root** — a closed set of project metadata: `name`, `description`, `version`,
  `schema_version`, `repo`, `issue_tracker`, `license`, `author`, and
  `binaries`.
- **`external`** — an open namespace where each tool stores arbitrary config
  under its own sub-key, e.g. `external.gomore`. The library treats each
  `external.<toolname>` section as an opaque blob: it can read it, hand it back
  to the caller to decode, and write a tool's section back, but it never parses
  or validates the section's internal structure.

See [`testdata/go.yaml`](testdata/go.yaml) for a complete example.

## Design notes

- The parsed `yaml.v3` node tree is the internal source of truth; typed
  accessors are layered on top. This is what preserves comments, key ordering,
  and unknown root keys across a load/save cycle.
- Writing one tool's section mutates the node tree in place rather than
  re-serializing from a typed struct, so an update to one section cannot corrupt
  another section or the root metadata.
- `Config` returns the raw `*yaml.Node` so callers decode into their own types.
  This is the only representation that is both type-safe (on the caller's side)
  and round-trip faithful; `DecodeConfig` is a convenience layer over it.
- Emitted indentation is normalized to two spaces. `yaml.v3` does not record the
  source file's original indent width, so that single stylistic detail is the
  one thing not preserved; structure and ordering are.
- `author` may appear as a string, a single object, or a list; it is normalized
  to `[]Author`.

## Installation

```sh
go get github.com/vieolo/godotyaml
```

## Usage

### Read root metadata

```go
doc, err := godotyaml.Load("go.yaml")
if err != nil {
    log.Fatal(err)
}

fmt.Println(doc.Name(), doc.Version(), doc.SchemaVersion())

authors, err := doc.Authors()
if err != nil {
    log.Fatal(err)
}
for _, a := range authors {
    fmt.Println(a.Name, a.Email)
}

bins, _ := doc.Binaries() // binary name -> OS -> output path
fmt.Println(bins["exampled"]["linux"])
```

### Read a tool's config

```go
// Decode external.golangci-lint into your own type.
type lintConfig struct {
    Linters map[string][]string `yaml:"linters"`
}

var cfg lintConfig
ok, err := doc.DecodeConfig("golangci-lint", &cfg)
if err != nil {
    log.Fatal(err)
}
if !ok {
    // section not present
}

// Or get the raw node and decode it yourself.
node, ok := doc.Config("golangci-lint")
```

### Write a tool's config

```go
// Updates only external.golangci-lint; every other section, all comments, and
// key ordering are left untouched.
err := doc.SetConfig("golangci-lint", map[string]any{
    "linters": map[string]any{
        "enable": []string{"govet", "staticcheck"},
    },
})
if err != nil {
    log.Fatal(err)
}

if err := doc.Save("go.yaml"); err != nil {
    log.Fatal(err)
}
```

## License

[MIT](LICENSE)
