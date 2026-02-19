#!/usr/bin/env bash
#
# Workspaced Installation Script
# Downloads the latest workspaced release and installs it
#
# Usage:
#   curl -sSL https://get.workspaced.dev | bash
#   # or
#   bash <(curl -sSL https://get.workspaced.dev)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
	echo -e "${GREEN}==>${NC} $1"
}

warn() {
	echo -e "${YELLOW}WARNING:${NC} $1"
}

error() {
	echo -e "${RED}ERROR:${NC} $1" >&2
	exit 1
}

# Detect OS and Architecture
case "$(uname -s)" in
Linux*) GOOS=linux ;;
Darwin*) GOOS=darwin ;;
FreeBSD*) GOOS=freebsd ;;
*) error "Unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
x86_64*) GOARCH=amd64 ;;
arm64*) GOARCH=arm64 ;;
aarch64*) GOARCH=arm64 ;;
*) error "Unsupported architecture: $(uname -m)" ;;
esac

info "Detected platform: ${GOOS}/${GOARCH}"

# Get latest version from GitHub
info "Fetching latest version..."
VERSION=$(curl -s https://api.github.com/repos/lucasew/workspaced/releases/latest | grep -oP '"tag_name":\s*"\K[^"]+')

if [[ -z ${VERSION} ]]; then
	error "Failed to fetch latest version"
fi

info "Latest version: ${VERSION}"

# Download binary
TMP=$(mktemp -d)
trap "rm -rf ${TMP}" EXIT

BINARY_NAME="workspaced-${GOOS}-${GOARCH}"
URL="https://github.com/lucasew/workspaced/releases/download/${VERSION}/${BINARY_NAME}"

info "Downloading from: ${URL}"
if ! curl -L -o "${TMP}/workspaced" "${URL}"; then
	error "Failed to download workspaced"
fi

chmod +x "${TMP}/workspaced"

info "Download complete!"

# Run self-install
info "Installing workspaced..."
if ! "${TMP}/workspaced" self-install; then
	error "Installation failed"
fi

info "Installation successful!"

# Check if ~/.local/bin is in PATH
if [[ ":${PATH}:" != *":${HOME}/.local/bin:"* ]]; then
	warn "~/.local/bin is not in your PATH"
	echo ""
	echo "Add the following to your shell configuration (~/.bashrc, ~/.zshrc, etc):"
	echo ""
	echo '    export PATH="$HOME/.local/bin:$PATH"'
	echo ""
fi

# Optional: run apply if in dotfiles directory
if [[ -d "${HOME}/.dotfiles" ]]; then
	info "Dotfiles detected at ${HOME}/.dotfiles"
	read -p "Run 'workspaced apply' now? [y/N] " -n 1 -r
	echo
	if [[ ${REPLY} =~ ^[Yy]$ ]]; then
		"${HOME}/.local/bin/workspaced" apply
	else
		info "You can run 'workspaced apply' later to apply your dotfiles"
	fi
else
	info "To initialize a new dotfiles setup, run: workspaced init"
fi

echo ""
info "🎉 Workspaced is ready! Run 'workspaced --help' to get started."
