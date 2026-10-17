#!/bin/sh
# leak-scan.sh <file|dir>...: prints every /Users/ path, email address, IP address and token-shaped string in the
# text files given, and fails when there is one.
grep -rInE '/Users/|[[:alnum:]._%+-]+@[[:alnum:]-]+\.[[:alnum:].-]+|(^|[^0-9.])([0-9]{1,3}\.){3}[0-9]{1,3}([^0-9.]|$)|(sk-|ghp_|gho_|ghs_|github_pat_|xox[abp]-|AKIA)[[:alnum:]_-]{8,}|[[:alnum:]_-]{40,}' "$@"
[ $? -eq 1 ]
