#!/bin/sh
set -eu

GOCOVERDIR=$(mktemp -d "${TMPDIR:-/tmp}/dulidu-cover.XXXXXX")
export GOCOVERDIR
# Keep the data so a failed check or a function's coverage can be inspected.
printf 'test: GOCOVERDIR=%s\n' "$GOCOVERDIR"
go test ./... -count=1
sh scripts/cover-diff.sh
