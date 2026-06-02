# Arch-Go Plugin for Architectural Decision Enforcement

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)

An [ad-guidance-tool](https://github.com/adr/ad-guidance-tool) plugin that compiles `code` rules into Go architecture tests using the [arch-go](https://github.com/arch-go/arch-go) framework. The generated `_test.go` files are formatted with `gofmt` and run with the standard `go test` runner.

## Installation

Install from a GitHub release:

```sh
ade plugin install archgo --repo github.com/phi42/ad-plugin-archgo
```

Or build from source and register locally:

```sh
go build -o archgo
ade plugin install archgo --path ./archgo
```

## Prerequisites

The target Go project must reference the following modules in its `go.mod`:

- `github.com/arch-go/arch-go` (>= v1.5)

## Usage

### Compile

```sh
ade compile -i path/to/adr.rule -p archgo
```

The plugin writes one `ADR_<id>_archgo_test.go` file per rule file into the output directory. Run `go test ./...` in the target project to execute the generated tests.

The plugin auto-detects the target project's Go module path by walking up from the output directory to find the nearest `go.mod` file, so the generated `config.Load` call references the correct module.

### Configuration

Plugin-specific options are stored under the `plugin_configs.archgo` namespace and forwarded to the plugin at runtime. Set them with `ade config set` from the project root:

```sh
ade config set plugin_configs.archgo.output-dir ./internal/archtests
```

Pass `--global` to write the value to the user-level config instead of the project-level `.ade.yaml`.

| Config key                           | Required for | Description                                                                 |
| ------------------------------------ | ------------ | --------------------------------------------------------------------------- |
| `plugin_configs.archgo.output-dir`   | compile      | Directory in which to write the generated `_test.go` file. Defaults to `.`. |

## Supported rules

Only `code` blocks are processed. `file` and `custom` blocks are skipped with a warning.

| ADL assertion         | archgo condition                                |
| --------------------- | ----------------------------------------------- |
| `must not depend on`  | `DependenciesRule.ShouldNotDependsOn.Internal`  |
| `must only depend on` | `DependenciesRule.ShouldOnlyDependsOn.Internal` |
| `must be acyclic`     | `CyclesRule.ShouldNotContainCycles`             |

`exclude` clauses on dependency rules are not forwarded to archgo because the `Dependencies` type has no per-rule exclusion field. Use multiple targeted rules instead.

## Unsupported rules

The following rule kinds have no archgo equivalent and are skipped with a comment block in the generated file:

- `must implement interface` / `must not implement interface`: archgo's `NamingRule` only constrains naming of implementors, it cannot assert that types must implement an interface.
- `must extend` / `must not extend`: Go has no classical inheritance.
- `must be annotated with` / `must not be annotated with`: Go has no runtime annotations.
- `must only be accessed by`: archgo only checks outgoing dependencies.
- `must be in` / `must not be in`: archgo has no package location checks.
- `must match` / `must not match`: archgo has no regex naming rules.
- `must be public/internal/private`: archgo has no visibility checks.
- `must be abstract/sealed/static`: no Go equivalent.

When a rule file contains only unsupported rules the plugin still generates a valid test file with a `t.Logf` statement that documents the skip.

## Known limitations

`config.Load` resolves the module path against the current working directory at test runtime, not the location of the generated file. Run `go test` from the module root, or set the working directory accordingly, otherwise archgo will not find the source packages.

The `Package` field in archgo uses glob-style patterns (`...` for recursive). Selectors written with the `regex:` prefix in the DSL are accepted but have their prefix stripped, since archgo does not support regex matching.

## Documentation

See [docs/implementation.md](docs/implementation.md) for a high-level explanation of the code structure and implementation design.

## License

Licensed under the [Apache License, Version 2.0](./LICENSE).
