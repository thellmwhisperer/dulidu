#!/bin/sh
# ATM's delivery: every run works in a fresh clone, so no-mistakes is initialised there first.
# --skip ci: GitHub Actions are paused for this repository; local test and lint gate the change until they return.
no-mistakes init || exit 1
exec no-mistakes axi run --intent "$ATM_INTENT" --closes "$ATM_ISSUE" --skip ci
