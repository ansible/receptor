import nox
import os

LATEST_PYTHON_VERSION = ["3.12"]

_PACKAGE_DIR = os.path.dirname(os.path.abspath(__file__))


@nox.session(python=False)
def coverage(session: nox.Session):
    """
    Run receptor-python-worker tests with code coverage.

    Must be invoked from the repo root so that coverage filenames are
    workspace-root-relative, which is what SonarCloud's sonar.sources=. expects:

        nox -f receptor-python-worker/noxfile.py --session coverage
    """
    session.run("pip", "install", "setuptools", "-e", f"{_PACKAGE_DIR}[test]", external=True)
    session.run(
        "python", "-m", "pytest",
        "--cov=receptor_python_worker",
        f"--cov-config={os.path.relpath(os.path.join(_PACKAGE_DIR, 'pyproject.toml'))}",
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
