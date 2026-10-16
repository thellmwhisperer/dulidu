#!/bin/sh
# Provision the read-only ATM source inside an isolated run before its worker starts.
set -eu
[ ! -d .tmp/atm-reference ] || exit 0
reference_root=${ATM_REFERENCE_REPO:-$(git remote get-url origin)/../agentic-team-member}
mkdir -p .tmp/atm-reference
git -C "$reference_root" archive HEAD | tar -x -C .tmp/atm-reference
