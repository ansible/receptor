import nox

LATEST_PYTHON_VERSION = ["3.12"]


@nox.session(python=False)
def coverage(session: nox.Session):
    """
    Run receptor-python-worker tests with code coverage
    """
    session.run("python", "-m", "pip", "install", "-e", ".[test]", "setuptools", external=True)
    session.run(
        "python", "-m", "pytest",
        "--cov=receptor_python_worker",
        "--cov-report", "term-missing:skip-covered",
        "--cov-report", "xml:python_worker_coverage.xml",
        "tests",
        *session.posargs,
        external=True,
    )


@nox.session(python=LATEST_PYTHON_VERSION)
def tests(session: nox.Session):
    """
    Run receptor-python-worker tests
    """
    session.install("setuptools", "-e", ".[test]")
    session.run("pytest", "-v", "tests", *session.posargs)
