# Go Standards

Receptor expects idiomatic Go. This file records only the project-specific rules that are easy to miss.

- **Interfaces are small and consumer-owned.** Define an interface next to the code that uses it and expose only the methods that consumer needs. Accept interfaces, return concrete types.
- **Prefer config structs over long parameter lists.** Pass a struct for related parameters instead of many positional arguments.
- **Mocks use `go.uber.org/mock`.** Generate them from interfaces with `make generate`; never edit generated mock files by hand.
- **New imports must be added to the `depguard` allowlist** in `.golangci.yml`.
- **Before opening a PR:** `make build-all`, `make test`, and `make lint` pass, and new behavior or bug fixes have tests.
