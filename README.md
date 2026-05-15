# Arch-Go Plugin for Architectural Decision Enforcement

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)

An [ad-guidance-tool](https://github.com/adr/ad-guidance-tool) enforcement plugin that compiles `code` rules from the ADE DSL into Go architecture tests using the [arch-go](https://github.com/arch-go/arch-go) framework. The generated `_test.go` files are run with the standard `go test` runner.

## Installation

Install from a GitHub release:

```sh
adg enforce plugin install arch-go --repo github.com/phi42/adplugin-arch-go
```

Or build from source and register locally:

```sh
go build -o arch-go
adg enforce plugin install arch-go --path ./arch-go
```

## Usage

```sh
adg enforce compile -i path/to/adr.rule -p arch-go -o ./internal
```

The plugin writes one `ADR_<id>_archgo_test.go` file per rule file into the output directory, formatted with `gofmt`. Run `go test ./...` in the target project to execute the generated tests.

The plugin auto-detects the target project's Go module path by walking up from the output directory to find the nearest `go.mod` file.

## Supported rules

Only `code` blocks are processed. `file` blocks and `custom` blocks are skipped with a warning.

| ADE DSL assertion     | arch-go equivalent                              |
| --------------------- | ----------------------------------------------- |
| `must not depend on`  | `DependenciesRule.ShouldNotDependsOn.Internal`  |
| `must only depend on` | `DependenciesRule.ShouldOnlyDependsOn.Internal` |
| `must be acyclic`     | `CyclesRule.ShouldNotContainCycles`             |

`exclude` clauses on dependency rules are not forwarded to arch-go because the `Dependencies` type has no per-rule exclusion field. Use multiple targeted rules instead.

## Unsupported rules

The following rule kinds have no arch-go equivalent and are skipped with a comment in the generated file:

- `must implement interface` / `must not implement interface`
- `must extend` / `must not extend`
- `must be annotated with` / `must not be annotated with`
- `must only be accessed by`
- `must be in` / `must not be in`
- `must match` / `must not match`
- `must be public` / `must be internal` / `must be private`
- `must be abstract` / `must be sealed` / `must be static`

When a rule file contains only unsupported rules the plugin still generates a valid test file with a `t.Logf` statement that documents the skip.

## License

Licensed under the [Apache License, Version 2.0](./LICENSE).
