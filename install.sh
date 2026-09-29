#!/bin/sh
# Installs the latest DebTime CLI release.
#
#   curl -fsSL https://raw.githubusercontent.com/kubavojak/Terminal-Debate-Timer/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/kubavojak/Terminal-Debate-Timer/main/install.sh | sh -s -- --system   # /usr/local/bin
#
# Environment: DEBTIME_REPO (owner/repo), DEBTIME_VERSION (e.g. v1.2.0),
# DEBTIME_INSTALL_DIR (target directory).
set -eu

REPO="${DEBTIME_REPO:-kubavojak/Terminal-Debate-Timer}"
SYSTEM=0
for arg in "$@"; do
	case "$arg" in
		--system) SYSTEM=1 ;;
		-h|--help)
			sed -n '2,9p' "$0" 2>/dev/null || true
			exit 0 ;;
		*) echo "install.sh: unknown argument: $arg" >&2; exit 2 ;;
	esac
done

fail() { echo "install.sh: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

if have curl; then
	fetch() { curl -fsSL "$1"; }
	download() { curl -fsSL -o "$2" "$1"; }
elif have wget; then
	fetch() { wget -qO- "$1"; }
	download() { wget -qO "$2" "$1"; }
else
	fail "curl or wget is required"
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	linux|darwin) ;;
	*) fail "unsupported system: $os" ;;
esac

case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	armv7*|armv8l) arch=armv7 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

version="${DEBTIME_VERSION:-}"
if [ -z "$version" ]; then
	version=$(fetch "https://api.github.com/repos/$REPO/releases/latest" |
		sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$version" ] || fail "cannot find the latest release of $REPO"
fi

file="debtime_${version#v}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading $file ($version)…"
download "$base/$file" "$tmp/$file" || fail "download failed: $base/$file"
download "$base/checksums.txt" "$tmp/checksums.txt" || fail "download failed: checksums.txt"

expected=$(grep " $file\$" "$tmp/checksums.txt" | cut -d ' ' -f 1)
[ -n "$expected" ] || fail "$file is not listed in checksums.txt"
if have sha256sum; then
	actual=$(sha256sum "$tmp/$file" | cut -d ' ' -f 1)
elif have shasum; then
	actual=$(shasum -a 256 "$tmp/$file" | cut -d ' ' -f 1)
else
	fail "sha256sum or shasum is required to verify the download"
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $file"

tar -xzf "$tmp/$file" -C "$tmp" debtime

sudo=""
if [ -n "${DEBTIME_INSTALL_DIR:-}" ]; then
	dir="$DEBTIME_INSTALL_DIR"
elif [ "$SYSTEM" = 1 ] || [ "$(id -u)" = 0 ]; then
	dir=/usr/local/bin
	if [ "$(id -u)" != 0 ]; then
		have sudo || fail "sudo is required for --system"
		sudo=sudo
	fi
else
	dir="$HOME/.local/bin"
fi

$sudo mkdir -p "$dir"
$sudo install -m 0755 "$tmp/debtime" "$dir/debtime"
echo "Installed $dir/debtime"

case ":$PATH:" in
	*":$dir:"*) ;;
	*) echo "Note: $dir is not in your PATH. Add it, e.g.:  export PATH=\"$dir:\$PATH\"" ;;
esac
