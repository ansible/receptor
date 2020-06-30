#!/bin/sh
pre-commit run --all-files && \
  go build cmd/*.go && \
  go build example/*.go
