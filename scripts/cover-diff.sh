#!/bin/sh
# Compare added Go lines with blocks recorded by the built command's processes.
set -eu
export LC_ALL=C

if [ -n "${COVER_BASE:-}" ]; then
    base=$COVER_BASE
else
    base=$(git merge-base HEAD origin/main 2>/dev/null) || base=
fi
if [ -z "$base" ] || ! git rev-parse --verify "$base^{commit}" >/dev/null 2>&1; then
    printf '%s\n' 'cover-diff: cannot find the base commit'
    git branch -a
    exit 1
fi

diff=$(git -c core.quotePath=false diff --no-ext-diff --no-color --src-prefix=a/ --dst-prefix=b/ -U0 "$base" -- '*.go' ':(exclude)*_test.go' ':(exclude)**/testdata/**' ':(exclude)testdata/**')
changed=$(printf '%s\n' "$diff" | awk '
    /^diff --git / { file = ""; next }
    /^\+\+\+ b\// { file = substr($0, 7); sub(/\t$/, "", file); next }
    /^@@ / { split($3, range, ","); line = substr(range[1], 2) + 0; next }
    /^\+/ && file != "" { print file ":" line; line++; next }
    /^ / { line++ }
')
if [ -z "$changed" ]; then
    printf '%s\n' 'cover-diff: no Go changes'
    exit 0
fi

root=$(git rev-parse --show-toplevel)
scratch=$(cd "${TMPDIR:-/tmp}" && pwd -P)
case "$scratch/" in
    "$root/"*) printf '%s\n' 'cover-diff: TMPDIR must be outside the checked tree'; exit 1 ;;
esac
profile=$scratch/dulidu-cover-diff.$$.txt
(umask 077; set -C; : > "$profile")
trap 'rm -f "$profile"' 0
trap 'exit 1' HUP INT TERM
go tool covdata textfmt -i="${GOCOVERDIR:?GOCOVERDIR must name the binary coverage directory}" -o "$profile"
module=$(go list -m)
result=$(printf '%s\n' '--changed--' "$changed" | awk -v module="$module/" -v root="$root/" '
    $0 == "--changed--" { changes = 1; next }
    !changes && $1 != "mode:" {
        split($1, block, ":")
        file = block[1]
        if (index(file, module) == 1) file = substr(file, length(module) + 1)
        if (index(file, root) == 1) file = substr(file, length(root) + 1)
        split(block[2], span, ",")
        split(span[1], start, "."); split(span[2], end, ".")
        for (line = start[1]; line <= end[1]; line++) {
            key = file ":" line
            counted[key] = 1
            if ($3 > 0) executed[key] = 1
        }
        next
    }
    changes && counted[$0] && !seen[$0]++ {
        total++
        if (!executed[$0]) { print; missing++ }
    }
    END {
        if (!missing) printf "cover-diff: %d changed lines inside covered blocks, all executed by an end-to-end run\n", total
    }
' "$profile" - | sort -t : -k1,1 -k2,2n)
case "$result" in
    'cover-diff:'*) printf '%s\n' "$result" ;;
    *)
        printf '%s\n' 'cover-diff: these changed lines are not executed by any end-to-end run of the real dulidu binary. Delete them, or write the end-to-end test that reaches them from a real dulidu command.' "$result"
        exit 1
        ;;
esac
