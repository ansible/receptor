import nox
import os

LATEST_PYTHON_VERSION = ["3.12"]

_PACKAGE_DIR = os.path.dirname(os.path.abspath(__file__))
_REPO_ROOT = os.path.dirname(_PACKAGE_DIR)


@nox.session(python=False)
def coverage(session: nox.Session):
    """
    Run receptor-python-worker tests with code coverage.

    Invoke from the repo root so coverage filenames are workspace-root-relative:

        nox -f receptor-python-worker/noxfile.py --session coverage
    """
    # nox changes CWD to the noxfile directory; restore to repo root so that
    # relative_files=true produces paths like
    # receptor-python-worker/receptor_python_worker/work.py
    # (workspace-root-relative) rather than bare receptor_python_worker/work.py.
    os.chdir(_REPO_ROOT)

    session.run("pip", "install", "setuptools", "-e", f"{_PACKAGE_DIR}[test]", external=True)
    session.run(
        "python", "-m", "pytest",
        "--cov=receptor_python_worker",
        "--cov-config=receptor-python-worker/pyproject.toml",
        "--cov-report", "term-missing:skip-covered",
        "--cov-report", "xml:receptor-python-worker/python_worker_coverage.xml",
        f"{_PACKAGE_DIR}/tests",
        *session.posargs,
        external=True,
    )


@nox.session(python=LATEST_PYTHON_VERSION)
def tests(session: nox.Session):
    """
    Run receptor-python-worker tests
    """
    session.run("pip", "install", "setuptools", "-e", f"{_PACKAGE_DIR}[test]", external=True)
    session.run("pytest", "-v", f"{_PACKAGE_DIR}/tests", *session.posargs)
