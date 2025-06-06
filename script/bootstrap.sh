#!/bin/bash
CURDIR=$(cd $(dirname $0); pwd)
BinaryName="essay.show"
echo "$CURDIR/bin/${BinaryName}"
exec $CURDIR/bin/${BinaryName}