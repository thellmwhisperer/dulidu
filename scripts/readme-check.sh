#!/bin/sh
# readme-check.sh runs the $ lines of README.md's first-run block, printing each and its output, in a new git
# repository named clay-corner holding the recorded example's brief and attachments, against a fresh build, with a
# claude that replays the example's recorded answers in order: no network. It fails unless the run folder's spec.md, tickets/,
# plan.json and brief-diseno.md are exactly docs/example-run/'s files.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
example=$root/internal/pipeline/testdata/run-example
tmp=$(mktemp -d /tmp/dulidu-readme.XXXXXX)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin" "$tmp/home" "$tmp/clay-corner"
if [ -n "${GOCOVERDIR:-}" ]; then
  (cd "$root" && go build -cover -coverpkg=./... -o "$tmp/bin/dulidu" ./cmd/dulidu)
else
  (cd "$root" && go build -o "$tmp/bin/dulidu" ./cmd/dulidu)
fi
cat > "$tmp/bin/claude" <<'EOF'
#!/bin/sh
cat > /dev/null
n=$(($(cat "$REPLAY_COUNT" 2>/dev/null || echo 0) + 1))
echo "$n" > "$REPLAY_COUNT"
f=$(ls "$REPLAY_ANSWERS"/* | sed -n "${n}p")
[ -n "$f" ] || { echo "no recorded answer $n" >&2; exit 1; }
awk 'BEGIN { printf "{\"type\":\"result\",\"result\":\"" }
{ gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); gsub(/\t/, "\\t"); gsub(/\r/, "\\r"); printf "%s\\n", $0 }
END { print "\"}" }' "$f"
EOF
chmod +x "$tmp/bin/claude"
cp "$example/brief.md" "$example/one.pdf" "$example/one.png" "$tmp/clay-corner"
git -C "$tmp/clay-corner" init -q
export PATH="$tmp/bin:$PATH" HOME="$tmp/home" REPLAY_ANSWERS="$example/answers" REPLAY_COUNT="$tmp/count"

awk '/^## First run/ { on = 1 } on && /^```/ { if (++fence == 2) exit; next } on && fence == 1 && sub(/^\$ /, "")' \
  "$root/README.md" > "$tmp/commands"
[ -s "$tmp/commands" ] || { echo "readme-check: no \$ lines in README.md's first-run block" >&2; exit 1; }
while IFS= read -r c; do
  printf '$ %s\n' "$c"
  (cd "$tmp/clay-corner" && sh -c "$c") < /dev/null > "$tmp/out" 2>&1 || { cat "$tmp/out"; echo "readme-check: failed: $c" >&2; exit 1; }
  cat "$tmp/out"
done < "$tmp/commands"

set -- "$tmp"/clay-corner/.dulidu/runs/*
[ $# -eq 1 ] && [ -d "$1" ] || { echo "readme-check: want one run folder, got $*" >&2; exit 1; }
(cd "$1" && find spec.md tickets plan.json brief-diseno.md -type f | sort) > "$tmp/got"
(cd "$root/docs/example-run" && find . -type f | sed 's|^\./||' | sort) > "$tmp/want"
diff "$tmp/want" "$tmp/got" || { echo "readme-check: the run folder does not match docs/example-run/" >&2; exit 1; }
