import os
import subprocess
from glob import iglob
from pathlib import Path

import nox.command

LATEST_PYTHON_VERSION = ["3.11"]

python_versions = ["3.8", "3.9", "3.10", "3.11"]

LINT_FILES: tuple[str, ...] = (
    *iglob("receptorctl/*.py"),
    "setup.py",
)
PINNED = os.environ.get("PINNED", "true").lower() in {"1", "true"}

requirements_directory = Path("requirements").resolve()

requirements_files = [requirements_input_file_path.stem for requirements_input_file_path in requirements_directory.glob("*.in")]


def install(session: nox.Session, *args, req: str, **kwargs):
    if PINNED:
        pip_constraint = requirements_directory / f"{req}.txt"
        kwargs.setdefault("env", {})["PIP_CONSTRAINT"] = pip_constraint
        session.log(f"export PIP_CONSTRAINT={pip_constraint!r}")
    session.install("-r", f"{requirements_directory}/{req}.in", *args, **kwargs)


def version(session: nox.Session):
    """
    Create a .VERSION file.
    """
    try:
        official_version = session.run_install(
            "git",
            "describe",
            "--exact-match",
            "--tags",
            external=True,
            stderr=subprocess.DEVNULL,
        )
    except nox.command.CommandFailed:
        official_version = None
        print("Using the closest annotated tag instead of an exact match.")

    if official_version:
        version = official_version.strip()
    else:
        tag = session.run_install("git", "describe", "--tags", "--always", silent=True, external=True)
        rev = session.run_install("git", "rev-parse", "--short", "HEAD", silent=True, external=True)
        version = tag.split("-")[0] + "+" + rev

    Path(".VERSION").write_text(version)


@nox.session(python=LATEST_PYTHON_VERSION)
def coverage(session: nox.Session):
    """
    Run receptorctl tests with code coverage
    """
    install(session, req="tests")
    version(session)
    session.install("-e", ".")
    session.run(
        "pytest", "--cov", "--cov-report", "term-missing:skip-covered", "--cov-report", "xml:receptorctl_coverage.xml", "--verbose", "tests", *session.posargs
    )


@nox.session(python=python_versions)
def tests(session: nox.Session):
    """
    Run receptorctl tests
    """
    install(session, req="tests")
    version(session)
    session.install("-e", ".")
    session.run("pytest", "-v", "tests", *session.posargs)


@nox.session
def check_style(session: nox.Session):
    """
    Check receptorctl Python code style
    """
    install(session, req="lint")
    session.run("flake8", *session.posargs, *LINT_FILES)


@nox.session
def check_format(session: nox.Session):
    """
    Check receptorctl Python file formatting without making changes
    """
    install(session, req="lint")
    session.run("black", "--check", *session.posargs, *LINT_FILES)


@nox.session
def format(session: nox.Session):
    """
    Format receptorctl Python files
    """
    install(session, req="lint")
    session.run("black", *session.posargs, *LINT_FILES)


@nox.session
def lint(session: nox.Session):
    """
    Check receptorctl for code style and formatting
    """
    session.notify("check_style")
    session.notify("check_format")


@nox.session(name="pip-compile", python=["3.11"])
@nox.parametrize(["req"], arg_values_list=requirements_files, ids=requirements_files)
def pip_compile(session: nox.Session, req: str):
    """Generate lock files from input files or upgrade packages in lock files."""
    # fmt: off
    session.install(
      "-r", str(requirements_directory / "pip-tools.in"),
      "-c", str(requirements_directory / "pip-tools.txt"),
    )
    # fmt: on

    # Use --upgrade by default unless a user passes -P.
    upgrade_related_cli_flags = ("-P", "--upgrade-package", "--no-upgrade")
    has_upgrade_related_cli_flags = any(arg.startswith(upgrade_related_cli_flags) for arg in session.posargs)
    injected_extra_cli_args = () if has_upgrade_related_cli_flags else ("--upgrade",)

    output_file = os.path.relpath(Path(requirements_directory / f"{req}.txt"))
    input_file = os.path.relpath(Path(requirements_directory / f"{req}.in"))

    session.run(
        "pip-compile",
        "--output-file",
        str(output_file),
        *session.posargs,
        *injected_extra_cli_args,
        str(input_file),
    )
