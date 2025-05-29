import os
import subprocess
from glob import iglob
from pathlib import Path

import nox.command

LATEST_PYTHON_VERSION = ["3.12"]

python_versions = ["3.8", "3.9", "3.10", "3.11", "3.12"]

LINT_FILES: tuple[str, ...] = (*iglob("**/*.py"),)

requirements_directory = Path("requirements").resolve()


def install(session: nox.Session, *args, **kwargs):
    session.install(".[test]", *args, **kwargs)


@nox.session(python=LATEST_PYTHON_VERSION)
def coverage(session: nox.Session):
    """
    Run receptorctl tests with code coverage
    """
    install(session)
    session.install("-e", ".")
    session.run(
        "pytest",
        "--cov",
        "--cov-report",
        "term-missing:skip-covered",
        "--cov-report",
        "xml:receptorctl_coverage.xml",
        "--verbose",
        "tests",
        *session.posargs,
    )


@nox.session(python=python_versions)
def tests(session: nox.Session):
    """
    Run receptorctl tests
    """
    install(session)
    session.install("-e", ".")
    session.run("pytest", "-v", "tests", *session.posargs)


@nox.session
def check_style(session: nox.Session):
    """
    Check receptorctl Python code style
    """
    install(session)
    session.run("ruff", "check", *session.posargs, *LINT_FILES)


@nox.session
def check_format(session: nox.Session):
    """
    Check receptorctl Python file formatting without making changes
    """
    install(session)
    session.run("ruff", "format", "--check", *session.posargs, *LINT_FILES)


@nox.session
def format(session: nox.Session):
    """
    Format receptorctl Python files
    """
    install(session)
    session.run("ruff", "format", *session.posargs, *LINT_FILES)


@nox.session
def lint(session: nox.Session):
    """
    Check receptorctl for code style and formatting
    """
    session.notify("check_style")
    session.notify("check_format")


@nox.session(name="pip-compile", python=["3.12"])
def pip_compile(session: nox.Session):
    """Generate lock files from input files or upgrade packages in lock files."""
    install(session)

    # Use --upgrade by default unless a user passes -P.
    upgrade_related_cli_flags = ("-P", "--upgrade-package", "--no-upgrade")
    has_upgrade_related_cli_flags = any(arg.startswith(upgrade_related_cli_flags) for arg in session.posargs)
    injected_extra_cli_args = () if has_upgrade_related_cli_flags else ("--upgrade",)

    output_file = os.path.relpath(Path(requirements_directory / "requirements.txt"))
    input_file = "pyproject.toml"

    session.run(
        "pip-compile",
        "--output-file",
        str(output_file),
        *session.posargs,
        *injected_extra_cli_args,
        str(input_file),
    )
