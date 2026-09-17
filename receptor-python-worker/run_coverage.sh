#!/usr/bin/env bash
# Run receptor-python-worker pytest coverage from the repo root so that
# relative_files=true produces workspace-root-relative filenames in the
# coverage XML (e.g. receptor-python-worker/receptor_python_worker/work.py)
# which SonarCloud can match against sonar.sources=.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

pip install setuptools -e "receptor-python-worker/[test]"
python -m pytest \
    --cov=receptor_python_worker \
    --cov-config=receptor-python-worker/pyproject.toml \
    --cov-report=term-missing:skip-covered \
    --cov-report=xml:receptor-python-worker/python_worker_coverage.xml \
    receptor-python-worker/tests/
