# godotyaml

***schema_version = 0***

`go.yaml` is the centralized file for the metadata and configuration of Go projects, filling the gaps of `go.mod`


This repo provides:
- The spec and standard of `go.yaml` file. This is the main focus of this README file.
- A small utility library for parsing `go.yaml` files to be used by other applications. Usage of this library is completely optional.

## Philosophy
- `go.yaml` is not a replacement for `go.mod` and is only used for metadata/configuration that is not natively supported by official go toolchain.
- `go.yaml` will/can contain both project-level metadata and settings and external tool/library/package configurations.
- No tool/library/package will be treated as first-class citizen with special privileges with the exception of official Go toolchain.
- `go.yaml` can be validated but an invalid `go.yaml` will not prevent your Go code from compiling and running.

## Anatomy of `go.yaml`

### Sections
A `go.yaml` file has broadly two sections:

1. Standardized structures (the fields and values the schema defines)
2. Arbitrary configuration for external tools/packages/libraries which can be placed inside `go.yaml` instead of a standalone file. These arbitrary configurations can only be placed under `external` key

### Root reservation
Please note: **All keys at the root level is designed by the schema as an standard and arbitrary data should not be placed at the root level. An external tool/package/library can place their config, without limiation, in `external` object.**

### Usage of `external`
The `external` object is only used for configs of the tools and each tool should only define their own configuration and should avoid defining category level configs.

For example, a linter (named `myLinter` for example), should not use `external.linter` for their config but they should use `external.myLinter`. Another linter can define another config under `external.secondLinter`.

If a category of external tools (such as linters, builders, releasers, etc.) use overlapping configurations, validated by community feedback, we will then promote those overlapping those config to the schema's root. So, using generic configs such as `external.linter`, `external.build`, etc. is highly discouraged.

## Spec of `go.yaml`
`go.yaml` can be validated but all fields are inherently optional.

```yaml
name: myproject # The name of the project (not the module name)
description: A useful tool # Optional human-readable description of your project
version: 1.26.3 # The version of your project
schema_version: 0 # The version of schema
repository: https//... # The URL of the repository of the project
issue_tracker: https://... # The URL of the issue tracker of the project
license: MIT # The license of the project

# The list of authors
# Each author has a required field of `name` and all other fields are optional. You can add no authors or add multiple authors
authors: 
  - name: Jane Doe
    email: jane@example.com
    organization: Vieolo
    url: example.com
  - name: John Smith
  
  
# The map of executable entry points of the project. A project can have multiple entry points to produce executables. The name of the executable (e.g., server, admin, etc.) is the arbitrary nickname you have for the entry point, the `entrypoint` is required and other fields are optional. A library with no executable, naturally, can skip this field entirely
executables:
  server:
    entrypoint: ./cmd/server
    description: HTTP API server.
  admin:
    entrypoint: ./cmd/admin
    description: Administrative CLI.
  other-exec:
    entrypoint: ./other/main


# The `external` is used for third-party external tools/packages/libraries to define their configs. Each tool has to use a key and place their config under that key (e.g., external.myLinter) and should not place their config on the root of the file or directly inside the `external` object to create an isolation for all tools of the project. The `external` objects have no schema or specs and each tool is free to define their own config structure.
external:

  builderTool:
    path: ./main.go
    mode: strict

  myLinter:
    rules:
      - rule1
      - rule2
    ignore-dep: true

  awesomeReleaser:
    target: public
```

## Go library of this repo
This repo, besides the spec of `go.yaml`, provides a light parser of `go.yaml` that other applications can use to avoid recreating a parser on their own everytime, even though you are free to use your own implementation.

The `godotyaml` library focuses on parsing `go.yaml` based on the schema version and maintain the data integrity and order of the file upon change.

A few points about this library:
- It is a library and not a CLI or executable
- It validates the schema to the point that whether it can parse the file or not. An invalid `go.yaml` file will not prevent the go compiler to compile or run your code.
- It does not validate the semantic correctness of any value, such as URLs, licenses, etc.
- It does not enforce any schema on the sub-keys of `external` and it remains an open space for external tools to define their config
- It does not refuse to parse a `go.yaml` file if an unsupported field exists in the root of the file


## Installation of library

```sh
go get github.com/vieolo/godotyaml
```

## Usage of library

### Read root metadata

```go
doc, err := godotyaml.Load("go.yaml")
if err != nil {
    log.Fatal(err)
}

fmt.Println(doc.Name(), doc.Version())

execs, _ := doc.Executables()
fmt.Println(execs["server"].Entrypoint)
```

### Read a tool's config

```go
// Decode external.my-linter into your own type.
type lintConfig struct {
    Linters map[string][]string `yaml:"linters"`
}

var cfg lintConfig
ok, err := doc.DecodeConfig("my-linter", &cfg)
if err != nil {
    log.Fatal(err)
}
if !ok {
    // section not present
}

// Or get the raw node and decode it yourself.
node, ok := doc.Config("my-linter")
```

### Write a tool's config

```go
// Updates only external.my-linter; every other section, all comments, and
// key ordering are left untouched.
err := doc.SetConfig("my-linter", map[string]any{
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
