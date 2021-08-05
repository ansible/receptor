# Receptor tests

All test files and test tools can be found on this directory.

```java
.
|-- artifacts     : "receptor-tester.sh builds output"
|-- environments  : "Controlled environments for testing"
|   |-- container : "Containerfile recipe"
|   `-- vagrant   : "VM recipe"
`-- functional    : "Functional test files"
    |-- cli
    |-- lib
    `-- mesh
```

## Tests

1. Functional tests and correspondent docs can be found here: [./functional/README.md](./functional/README.md)

## Tools

There are two parts of Receptor tools. Each one is used for different scenarios.

Requirements:
- podman
- make

### receptor-tester

All features will be showed on `help` argument:

```bash
./receptor-tester.sh help

# Command list:
#   list-dirs    - list all available tests directories
#   list-files   - list all available tests files
#   run          - run a specific test
#   run-all      - run all tests. Returns 0 if pass
#   help         - show this help section
```

List all available tests:

```bash
# list all available tests directories
./receptor-tester.sh list-dirs
# ./functional/mesh
# ./functional/cli
# ./functional/lib/utils

# List all available tests files
./receptor-tester.sh list-files
# ./functional/mesh/mesh_test.go
# ./functional/mesh/work_test.go
# ./functional/mesh/tls_test.go
# ./functional/cli/cli_test.go
# ./functional/lib/utils/utils_test.go
```

Run tests:

```bash
# run a specific test
./receptor-tester.sh run ./functional/cli/cli_test.go
# run all tests
./receptor-tester.sh run-all
```

### Makefile

Build artifacts (receptor and receptorctl) based on latest source code.
The container recipe used can be found at `environments/container`.

```bash
# Build artifacts
make artifacts
```

Container commands can be used in conjunction with `make artifacts` or isolated to run ad-hoc commands inside a controlled environment.

```bash
# Rebuild container image used
# by `make artifacts`
make container-image # OR
make container-image-base

# Jump into a container created from
# the same container image used by
# `make artifacts`
make container-shell-base
```

If you're using Windows or macOS, the following commands can be useful to create a virtual machine and play aroung with containers inside there.

Requirements:
- vagrant

```bash
# Creates a Vagrant VM
make vm # OR
make vm-create

# SSH into VM
make vm-shell

# Destroy VM
make vm-destroy

# Reapply Ansible playbook into VM
make vm-provision
```
