#!/bin/bash

set -ex

SOURCE_DIR=/source
BUILD_DIR=/build
ARTIFACTS_DIR=/artifacts

# Copy all content
cp -r ${SOURCE_DIR}/ ${BUILD_DIR}

# Build receptor
cd ${BUILD_DIR}
make clean # prevent fail on dev environment
make receptor

# Build receptorctl
cd ${BUILD_DIR}/receptorctl
source /opt/venv/bin/activate # uses the currect Python version
python -m build

# Move packages
mkdir -p ${ARTIFACTS_DIR}/dist
rm -f ${ARTIFACTS_DIR}/receptor
cp ${BUILD_DIR}/receptor ${ARTIFACTS_DIR}/receptor
rm -rf ${ARTIFACTS_DIR}/dist
cp -r ${BUILD_DIR}/receptorctl/dist/ ${ARTIFACTS_DIR}/dist
