#!/bin/bash
set -e

go get -d -t $1
cd /go/src/$1

lines=`go test -covermode=count -coverprofile=coverage.out ./...`

if [ $? -gt 0 ]; then
    echo "Error: Cannot test '$1'" >&2
    exit 2
fi

if [ ! -f coverage.out ]; then
    echo "Error: No test files for '$1'" >&2
    exit 3
fi

# html=`go tool cover -html=coverage.out -o=/dev/stdout`
# | sed s/\"/\'/g)
if [ $? -gt 0 ]; then
    echo "Error: Cannot get coverage of '$1'" >&2
    exit 4
fi

echo $lines
