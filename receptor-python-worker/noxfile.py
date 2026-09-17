import nox

LATEST_PYTHON_VERSION = ["3.12"]


@nox.session(python=LATEST_PYTHON_VERSION)
def coverage(session: nox.Session):
    """
    Run receptor-python-worker tests with code coverage
    """
    session.install("setuptools", "-e", ".[test]")
    session.run(
        "pytest",
        "--cov=receptor_python_worker",
        "--cov-report",
        "term-missing:skip-covered",
        "--cov-report",
        "xml:python_worker_coverage.xml",
        "--verbose",
        "tests",
        *session.posargs,
    )


@nox.session(python=LATEST_PYTHON_VERSION)
def tests(session: nox.Session):
    """
    Run receptor-python-worker tests
    """
    session.install("setuptools", "-e", ".[test]")
    session.run("pytest", "-v", "tests", *session.posargs)
