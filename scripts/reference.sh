#!/bin/sh
# What a fresh clone needs before its tests: the Go modules and the reference checkout.
set -eu
go mod download
[ ! -d .tmp/reference ] || exit 0
reference_root=${REFERENCE_REPO:-$(git remote get-url origin)/.tmp/reference-source}
mkdir -p .tmp/reference
git -C "$reference_root" archive HEAD | tar -x -C .tmp/reference
