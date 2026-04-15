#!/usr/bin/env bash
# Install the bridgely CLI to ~/.local/bin/bridgely
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${HOME}/.local/bin"

mkdir -p "$INSTALL_DIR"

ln -sf "${SCRIPT_DIR}/bridgely.sh" "${INSTALL_DIR}/bridgely"

echo "Installed: ${INSTALL_DIR}/bridgely -> ${SCRIPT_DIR}/bridgely.sh"

# Check if ~/.local/bin is in PATH
if ! echo "$PATH" | tr ':' '\n' | grep -q "${INSTALL_DIR}"; then
  echo ""
  echo "Add to your shell profile (~/.zshrc or ~/.bashrc):"
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi
