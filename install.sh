#!/bin/sh
# Installs dulidu, the latest release or --version vX.Y.Z, into ${DULIDU_PREFIX:-$HOME/.local/bin}:
#
#   curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh
#
# The sha256 is verified before anything is written; dulidu is replaced only by mv of a whole file, so a killed
# run leaves the previous one answering; a dulidu in the prefix that is not this product is left alone.
# DULIDU_GITHUB_API stands in for GitHub's API. POSIX sh: it runs under whatever /bin/sh a `curl | sh` gets.
# Shaped like ATM's install.sh.
set -eu

API="${DULIDU_GITHUB_API:-https://api.github.com}"
PREFIX="${DULIDU_PREFIX:-$HOME/.local/bin}"
TARGET="$PREFIX/dulidu"
RELEASE=latest

die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

case "${1:-}" in
  "") ;;
  --version) [ -n "${2:-}" ] || die "--version needs a tag, like v0.2.0"; RELEASE="tags/$2" ;;
  *) die "usage: install.sh [--version vX.Y.Z]" ;;
esac

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "dulidu installs on macOS and Linux" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "no dulidu is published for $(uname -m)" ;;
esac

if [ -e "$TARGET" ] && ! "$TARGET" --version 2>/dev/null | grep -q '^dulidu '; then
  die "$TARGET is not dulidu and is left alone: move it, or install elsewhere with DULIDU_PREFIX"
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/dulidu-install.XXXXXX")
STAGED="$PREFIX/.dulidu-install.$$"
trap 'rm -rf "$WORK" "$STAGED"' EXIT
trap 'exit 1' INT TERM

curl -fsSL -o "$WORK/release.json" "$API/repos/thellmwhisperer/dulidu/releases/$RELEASE" ||
  die "GitHub has no release at $RELEASE"
field() { grep -o "\"$1\": *\"[^\"]*\"" "$WORK/release.json" | sed 's/.*"\([^"]*\)"$/\1/'; }
TAG=$(field tag_name | head -1)
ARCHIVE="dulidu-$TAG-$os-$arch.tar.gz"

for f in "$ARCHIVE" checksums.txt; do
  url=$(field browser_download_url | grep "/$f\$" | head -1)
  [ -n "$url" ] || die "release $TAG publishes no $f"
  curl -fsSL -o "$WORK/$f" "$url" || die "cannot download $url"
done

want=$(awk -v f="$ARCHIVE" '$2 == f { print $1 }' "$WORK/checksums.txt")
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$WORK/$ARCHIVE" | awk '{ print $1 }')
else
  got=$(shasum -a 256 "$WORK/$ARCHIVE" | awk '{ print $1 }')
fi
[ -n "$want" ] && [ "$want" = "$got" ] ||
  die "the sha256 of $ARCHIVE is $got, checksums.txt says '$want': nothing was installed"

tar -xzf "$WORK/$ARCHIVE" -C "$WORK" dulidu
mkdir -p "$PREFIX"
cp "$WORK/dulidu" "$STAGED"
chmod 755 "$STAGED"
mv -f "$STAGED" "$TARGET"
"$TARGET" --version
