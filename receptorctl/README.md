# Receptorctl

Receptorctl is a front-end CLI and importable Python library that interacts with Receptor over its control socket interface.

## Setting up nox

This project includes a `nox` configuration to automate tests, checks, and other functions in a reproducible way using isolated environments.
Before you submit a PR, you should install `nox` and verify your changes.

> To run `make receptorctl-lint` and `receptorctl-test` from the repository root, you must first install `nox`.

1. Install `nox` using `python3 -m pip install nox` or your distribution's package manager.
2. Run `nox --list` from the `receptorctl` directory to view available sessions.

You can run `nox` with no arguments to execute all checks and tests.
Alternatively, you can run only certain tasks as outlined in the following sections.

> By default nox sessions install pinned dependencies from `pyproject.toml`.

## Checking changes to Receptorctl

Run the following `nox` sessions to check for code style and formatting issues:

* Run all checks.

  ```bash
  nox -s lint
  ```

* Check code style.

  ```bash
  nox -s check_style
  ```

* Check formatting.

  ```bash
  nox -s check_format
  ```

* Format code if the check fails.

  ```bash
  nox -s format
  ```

## Running Receptorctl tests

Run the following `nox` sessions to test Receptorctl changes:

* Run tests against the complete matrix of Python versions.

  ```bash
  nox -s tests
  ```

* Run tests against a specific Python version.

  ```bash
  # For example, this command tests Receptorctl against Python 3.12.
  nox -s tests-3.12
  ```

## Updating dependencies

Update dependencies using `uv` as follows:

1. Add or modify packages in `pyproject.toml` under `dependencies` or `optional-dependencies`.
2. Regenerate the lock file from the `receptorctl` directory:

    ```bash
    uv lock
    ```

> The `uv.lock` file pins all dependencies with cryptographic hashes for reproducible installs.
> Commit the updated `uv.lock` file along with your `pyproject.toml` changes.
