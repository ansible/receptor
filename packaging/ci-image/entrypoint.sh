#!/bin/sh
export PATH=/home/test-user/receptor:/home/test-user/go/bin:$PATH
make ci
RC=$?
cat test-output.log | go-junit-report > test-junit.xml
exit $RC
