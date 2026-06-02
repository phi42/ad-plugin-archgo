# Implementation Overview

This document explains the code structure, design, and execution flow of the archgo plugin. It covers the purpose of each file and the concepts behind the major components.

## Architecture

The plugin is a standalone Go binary. The `ade` tool invokes it by serializing a `Spec` protobuf message (which represents one parsed `.rule` file) and writing those bytes to the plugin's stdin. The plugin reads stdin, processes the rules, and communicates results through stdout (JSON info response) or stderr (progress, warnings, and errors).

The plugin supports a single mode:

| Mode    | Description                                                                        |
| ------- | ---------------------------------------------------------------------------------- |
| compile | Translates `code` rules into a Go test file using archgo and writes it to disk.    |

There is no verify mode because archgo is itself driven by `go test`, which the developer runs as part of their normal test workflow. Re-invoking `go test` from the plugin would duplicate that workflow and complicate test discovery.

## Package layout

```
ad-plugin-archgo/
├── main.go              entry point
├── cmd/
│   └── root.go          plugin protocol, mode dispatch, file I/O
└── archgo/
    ├── types.go         internal data types for the template pipeline
    ├── builder.go       rule translation (Spec → template data) and helpers
    ├── render.go        Go template execution and gofmt
    └── test.tmpl        embedded Go test file template
```

## Files

### `main.go`

The binary entry point. It delegates immediately to the `cmd` package and contains no logic of its own.

### `cmd`

#### `root.go`

Implements the plugin protocol and top-level flow:

- When invoked with `--info`, it prints a JSON descriptor listing the supported modes and config prefix, then exits. The `ade` host calls this before each invocation to verify that the plugin supports the requested mode.
- When invoked interactively (stdin is a terminal), it prints a help message and exits.
- Otherwise, it reads the serialized `Spec` protobuf from stdin and dispatches to compile mode.

In compile mode it resolves the output directory, detects the target project's Go module path, calls the builder, passes the result to the renderer, and writes the formatted output file.

### `archgo`

#### `types.go`

Defines the three data types that carry information through the translation pipeline:

- The top-level type holds everything needed to render one complete Go test file: the module path, the ADR id and title, the list of dependency rules, the list of cycle rules, and the list of rules that were skipped because archgo cannot express them.
- The dependency rule type holds the package pattern under inspection together with either an allow-list or a forbid-list of target packages.
- The cycle rule type holds only the package pattern under inspection.

#### `builder.go`

The core translation layer. It iterates over the rules in the `Spec`, dispatches by rule kind, and assembles the template data structure that the renderer consumes.

Each supported rule kind maps to a specific archgo construct:

- `RULE_DEPEND_ONLY` becomes a `DependenciesRule` with `ShouldOnlyDependsOn.Internal` populated from the targets.
- `RULE_NOT_DEPEND` becomes a `DependenciesRule` with `ShouldNotDependsOn.Internal` populated from the targets.
- `RULE_ACYCLIC` becomes a `CyclesRule` with `ShouldNotContainCycles` set to true.

All other rule kinds are appended to a skipped list, which the template renders as a comment block above the test function.

The file also contains:

- `GenFileName`, which produces the canonical output filename from an ADR id by replacing non-identifier characters with underscores.
- `DetectModulePath`, which walks up from the output directory looking for a `go.mod` file and returns the module path declared in it. The auto-detection allows users to omit module configuration entirely.
- `resolveTarget`, which translates a `TargetRef` into a Go package pattern. Scoped subjects fall through to their scope. Inline values have any `regex:` prefix stripped because archgo uses glob-style patterns. Named selectors are looked up in the spec's selector map.

#### `render.go`

Executes the embedded Go template with the template data, runs the result through `go/format` so the output is canonical Go source, and returns it as bytes. The template file is embedded into the binary at build time using `//go:embed`, so the plugin has no runtime file dependencies. If formatting fails the unformatted source is returned alongside the error so the caller can inspect the problem.

#### `test.tmpl`

A Go `text/template` that produces a complete, compilable Go source file containing one `Test` function per ADR. The function:

1. Builds a `config.Config` value from the dependency rules and cycle rules in the template data.
2. Calls `config.Load(modulePath)` to load module information for the target project.
3. Calls `archgo.CheckArchitecture` and fails the test if the result is not passing.

If the rule file contains no archgo-compatible rules the template emits a `t.Logf` call documenting the skip, so the file remains a valid, runnable test.

Any skipped rules are listed as comments above the test function.

## Execution flow

```
    ┌────────────────────┐
    │  ade compile       │
    │    -i adr.rule     │
    │    -p archgo       │
    └─────────┬──────────┘
              │
              │   
  (ade serializes .rule file
   writes it to plugin's stdin)
              │   
              │
              ▼
┌─────────────────────────────┐
│         [cmd/root.go]       │
│                             │
│  Read Spec from stdin       │
│  Detect module path         │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│     [archgo/builder.go]     │
│                             │
│  Translate code rules       │
│  Assemble template data     │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│     [archgo/render.go]      │
│                             │
│  Execute Go template        │
│  Run gofmt                  │
│  => Go source bytes         │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│         [cmd/root.go]       │
│                             │
│  Write test file            │
│  (ADR_<id>_archgo_test.go)  │
└─────────────────────────────┘
```
