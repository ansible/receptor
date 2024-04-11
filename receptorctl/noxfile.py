from glob import iglob

import nox

python_versions = ["3.8", "3.9", "3.10", "3.11"]

LINT_FILES: tuple[str, ...] = (
    *iglob("receptorctl/*.py"),
    "setup.py",
)


@nox.session(python=python_versions)
def tests(session: nox.Session):
    """
    Run receptorctl tests
    """
    session.install("pytest")
    session.install("-e", ".")
    session.run("pytest", "-v", "tests", *session.posargs)


@nox.session
def check_style(session: nox.Session):
    """
    Check receptorctl Python code style
    """
    session.install("flake8")
    session.run("flake8", *session.posargs, *LINT_FILES)


@nox.session
def check_format(session: nox.Session):
    """
    Check receptorctl Python file formatting without making changes
    """
    session.install("black")
    session.run("black", "--check", *session.posargs, *LINT_FILES)


@nox.session
def format(session: nox.Session):
    """
    Format receptorctl Python files
    """
    session.install("black")
    session.run("black", *session.posargs, *LINT_FILES)


@nox.session
def lint(session: nox.Session):
    """
    Check receptorctl for code style and formatting
    """
    session.notify("check_style")
    session.notify("check_format")
