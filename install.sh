#!/bin/sh
set -e

REPO="thavel/apidep"
BIN="apidep"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS="$(uname -s)"
case "${OS}" in
  Linux*)  OS=linux ;;
  Darwin*) OS=darwin ;;
  *)
    echo "Unsupported OS: ${OS}"
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *)
    echo "Unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

# Fetch latest release tag
echo "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "${LATEST}" ]; then
  echo "Failed to determine latest release"
  exit 1
fi

ASSET="${BIN}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}"

echo "Installing ${BIN} ${LATEST} (${OS}/${ARCH})..."
curl -fsSL "${URL}" -o "/tmp/${BIN}"
chmod +x "/tmp/${BIN}"

# Verify checksum if sha256sum is available
if command -v sha256sum > /dev/null 2>&1; then
  CHECKSUMS=$(curl -fsSL "https://github.com/${REPO}/releases/download/${LATEST}/checksums.txt")
  EXPECTED=$(echo "${CHECKSUMS}" | grep "${ASSET}" | awk '{print $1}')
  ACTUAL=$(sha256sum "/tmp/${BIN}" | awk '{print $1}')
  if [ "${EXPECTED}" != "${ACTUAL}" ]; then
    echo "Checksum mismatch! Expected ${EXPECTED}, got ${ACTUAL}"
    rm "/tmp/${BIN}"
    exit 1
  fi
  echo "Checksum verified"
fi

# Install
if [ -w "${INSTALL_DIR}" ]; then
  mv "/tmp/${BIN}" "${INSTALL_DIR}/${BIN}"
else
  sudo mv "/tmp/${BIN}" "${INSTALL_DIR}/${BIN}"
fi

echo "${BIN} ${LATEST} installed to ${INSTALL_DIR}/${BIN}"
