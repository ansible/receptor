import subprocess
from glob import iglob

import nox.command

LATEST_PYTHON_VERSION = ["3.12"]

python_versions = ["3.10", "3.11", "3.12"]

LINT_FILES: tuple[str, ...] = (*iglob("**/*.py"),)


def install(session: nox.Session, *args, **kwargs):
    """Install dependencies using uv pip."""
    # Install uv for faster pip operations
    session.install("uv")
    # Use uv pip to install from pyproject.toml (respects uv.lock)
    session.run("uv", "pip", "install", "-e", ".[test]", *args, **kwargs)


@nox.session(python=LATEST_PYTHON_VERSION)
def coverage(session: nox.Session):
    """
    Run receptorctl tests with code coverage
    """
    install(session)
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
