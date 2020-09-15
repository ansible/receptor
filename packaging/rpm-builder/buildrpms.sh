#!/bin/bash
set -e  # exit on any error
VERSION=$(cat $PWD/.VERSION)
mkdir -p $PWD/rpmbuild/SOURCES
git archive HEAD --format=tar.gz --prefix=receptor-$VERSION/ > $PWD/rpmbuild/SOURCES/receptor-$VERSION.tar.gz
rpmbuild -ba packaging/rpm/receptor.spec
rpmbuild -ba packaging/rpm/receptorctl.spec
rpmbuild -ba packaging/rpm/receptor-python-worker.spec
createrepo $PWD/rpmbuild/RPMS
