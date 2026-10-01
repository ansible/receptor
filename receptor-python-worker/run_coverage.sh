#!/usr/bin/env bash
# Run receptor-python-worker pytest coverage.
# CWD must be the receptor-python-worker/ subdirectory (same pattern as
# the receptorctl nox session which runs with working-directory: ./receptorctl).
# This makes coverage produce an absolute <source> path pointing at the
# package directory, with short filenames like "work.py" — the exact format
# that SonarCloud's Cobertura importer can suffix-match.
set -euo pipefail

PACKAGE_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PACKAGE_DIR"

pip install setuptools -e ".[test]"
python -m pytest \
    --cov=receptor_python_worker \
    --cov-report=term-missing:skip-covered \
    --cov-report=xml:python_worker_coverage.xml \
    tests/
