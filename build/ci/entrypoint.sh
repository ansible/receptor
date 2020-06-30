#!/bin/sh
pre-commit && \
  go build cmd/*.go && \
  go build example/*.go
