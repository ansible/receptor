#!/bin/sh
export PATH=/root/go/bin:$PATH
echo "Running pre-commit" && \
  pre-commit run --all-files && \
  echo "Running Go builds" && \
  go build cmd/*.go && \
  go build example/*.go && \
  echo "All done"
