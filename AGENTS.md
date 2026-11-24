# AGENTS.md

Read this first, then open only the document for your task.

| Task | Read |
| --- | --- |
| Understand the project or architecture | [README.md](README.md) |
| Set up, build, test, or open a PR | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Write or change Go code (interfaces, config structs, mocks) | [STANDARDS.md](STANDARDS.md) |
| Debug or run linters in depth | [docs/source/developer_guide.rst](docs/source/developer_guide.rst) |
| Update user documentation | [docs/](docs/) |

## Commands

| Command | Purpose |
| --- | --- |
| `make build-all` | Build for all supported platforms |
| `make test` | Run the unit tests |
| `make lint` | Run golangci-lint (installs the pinned version if needed) |
| `make format` | Run `gofmt` over `cmd/` and `pkg/` |
| `make generate` | Regenerate gomock mocks from `generate.go` |
| `make generate-clean` | Remove existing `mock*` directories |
| `make container` | Build the container image |

## Rules

- Target `devel`; commits need `git commit --signoff`; rebase, don't merge.
- Make the smallest change; don't refactor unrelated code.
- Never hand-edit generated mocks; change the interface and run `make generate`.
- Run `make build-all`, `make test`, and `make lint` before finishing.

See [STANDARDS.md](STANDARDS.md) for project-specific Go rules.
