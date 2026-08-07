#!/bin/sh
# irori installer for Linux, macOS and the BSDs.
#
#   curl -fsSL https://raw.githubusercontent.com/BX-Team/irori/main/installer/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --version 1.2.3 --bin-dir /usr/local/bin
set -eu

REPO="BX-Team/irori"
VERSION="${IRORI_VERSION:-latest}"
BIN_DIR="${IRORI_BIN_DIR:-}"

usage() {
	cat <<EOF
Install irori, the TUI for running a Minecraft server.

Usage: install.sh [options]

  --version <x.y.z>   Release to install (default: latest)
  --bin-dir <dir>     Where to put the binary
                      (default: /usr/local/bin as root, ~/.local/bin otherwise)
  -h, --help          Show this help

Environment: IRORI_VERSION, IRORI_BIN_DIR do the same as the flags.
EOF
}

die() {
	echo "irori: $*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		VERSION="${2:-}"
		shift 2
		;;
	--bin-dir)
		BIN_DIR="${2:-}"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) die "unknown option: $1 (try --help)" ;;
	esac
done

target() {
	os="$(uname -s)"
	arch="$(uname -m)"

	case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	FreeBSD) os=freebsd ;;
	*) die "unsupported operating system: $os" ;;
	esac

	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	armv7* | armv6* | armhf) arch=arm-v7 ;;
	*) die "unsupported architecture: $arch" ;;
	esac

	echo "$os-$arch"
}

fetch() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		die "neither curl nor wget is installed"
	fi
}

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		echo ""
	fi
}

TARGET="$(target)"
ASSET="irori-$TARGET.tar.gz"

case "$VERSION" in
latest) BASE="https://github.com/$REPO/releases/latest/download" ;;
v*) BASE="https://github.com/$REPO/releases/download/$VERSION" ;;
*) BASE="https://github.com/$REPO/releases/download/v$VERSION" ;;
esac

if [ -z "$BIN_DIR" ]; then
	if [ "$(id -u)" -eq 0 ]; then
		BIN_DIR="/usr/local/bin"
	else
		BIN_DIR="$HOME/.local/bin"
	fi
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

echo "irori: downloading $ASSET ($VERSION)"
fetch "$BASE/$ASSET" "$TMP/$ASSET" || die "could not download $BASE/$ASSET"

# The checksum file is best effort: an old release that predates it should not
# block an install, but a mismatch always must.
if fetch "$BASE/$ASSET.sha256" "$TMP/$ASSET.sha256" 2>/dev/null; then
	want="$(tr -d '[:space:]' <"$TMP/$ASSET.sha256")"
	got="$(sha256_of "$TMP/$ASSET")"
	if [ -z "$got" ]; then
		echo "irori: no sha256 tool found, skipping checksum verification" >&2
	elif [ "$want" != "$got" ]; then
		die "checksum mismatch: expected $want, got $got"
	else
		echo "irori: checksum ok"
	fi
fi

tar -xzf "$TMP/$ASSET" -C "$TMP" || die "could not unpack $ASSET"
[ -f "$TMP/irori" ] || die "archive did not contain an irori binary"
chmod +x "$TMP/irori"

SUDO=""
if [ ! -w "$(dirname "$BIN_DIR")" ] && [ ! -w "$BIN_DIR" ] 2>/dev/null; then
	if command -v sudo >/dev/null 2>&1; then
		SUDO="sudo"
		echo "irori: $BIN_DIR needs root, using sudo"
	fi
fi

$SUDO mkdir -p "$BIN_DIR" || die "could not create $BIN_DIR"
$SUDO cp "$TMP/irori" "$BIN_DIR/irori" || die "could not write $BIN_DIR/irori"
$SUDO chmod 755 "$BIN_DIR/irori"

echo "irori: installed to $BIN_DIR/irori"
"$BIN_DIR/irori" --version || true

case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*)
	echo
	echo "irori: $BIN_DIR is not on your PATH. Add this to your shell profile:"
	echo "  export PATH=\"$BIN_DIR:\$PATH\""
	;;
esac
