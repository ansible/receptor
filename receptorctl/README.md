Receptorctl is a front-end CLI and importable Python library that interacts with Receptor over its control socket interface.

# Setting up nox

This project includes a `nox` configuration to automate tests, checks, and other functions in a reproducible way using isolated environments.
Before you submit a PR, you should install `nox` and verify your changes.

> To run `make receptorctl-lint` and `receptorctl-test` from the repository root, you must first install `nox`.

1. Install `nox` using `python3 -m pip install nox` or your distribution's package manager.
2. Run `nox --list` from the `receptorctl` directory to view available sessions.

You can run `nox` with no arguments to execute all checks and tests.
Alternatively, you can run only certain tasks as outlined in the following sections.

# Checking changes to Receptorctl

Run the following `nox` sessions to check for code style and formatting issues:

* Run all checks.

  ``` bash
  nox -s lint
  ```

* Check code style.

  ``` bash
  nox -s check_style
  ```

* Check formatting.

  ``` bash
  nox -s check_format
  ```

* Format code if the check fails.

  ``` bash
  nox -s format
  ```

# Running Receptorctl tests

Run the following `nox` sessions to test Receptorctl changes:

* Run tests against the complete matrix of Python versions.

  ``` bash
  nox -s tests
  ```

* Run tests against a specific Python version.

  ``` bash
  # For example, this command tests Receptorctl against Python 3.11.
  nox -s tests-3.11
  ```

# Updating dependencies

Update dependencies in the `requirements` directory as follows:

1. Make sure `pip-compile` is available.

   ```
   python -m pip install --upgrade pip-tools
   ```

2. Add any packages or pins to the `*.in` file.
3. Generate the full dependency tree from the repository root, for example:

   ```
   pip-compile --output-file=receptorctl/requirements/tests.txt receptorctl/requirements/tests.in
   ```
