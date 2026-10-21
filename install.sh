#!/usr/bin/env bash
# install.sh: Idempotently symlinks workstation-setup into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/bin/workstation-setup"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/workstation-setup"
TARGET_ALIAS="${TARGET_DIR}/ws-setup"

DRY_RUN=false
for arg in "$@"; do
    case "$arg" in
        --dry-run)
            DRY_RUN=true
            ;;
        -h|--help)
            echo "Usage: ./install.sh [--dry-run]"
            echo "Symlinks bin/workstation-setup into ~/.local/bin"
            exit 0
            ;;
    esac
done

if [[ ! -f "$BIN_SRC" ]]; then
    echo -e "\033[1;31m[ERROR]\033[0m Source executable not found at: $BIN_SRC" >&2
    exit 1
fi

if [[ "$DRY_RUN" == true ]]; then
    echo -e "\033[1;33m[DRY-RUN]\033[0m Simulating workstation-setup symlink installation:"
    echo "  • Would ensure target directory exists: $TARGET_DIR"
    echo "  • Would make executable: $BIN_SRC"
    echo "  • Would symlink: $TARGET_BIN -> $BIN_SRC"
    echo "  • Would symlink: $TARGET_ALIAS -> $BIN_SRC"
    exit 0
fi

# Ensure target directory exists and source is executable
mkdir -p "$TARGET_DIR"
chmod +x "$BIN_SRC"

# Create symlinks for primary binary and convenient alias
ln -sf "$BIN_SRC" "$TARGET_BIN"
ln -sf "$BIN_SRC" "$TARGET_ALIAS"

echo -e "\033[1;32m[SUCCESS]\033[0m workstation-setup installed successfully:"
echo "  • $TARGET_BIN -> $BIN_SRC"
echo "  • $TARGET_ALIAS -> $BIN_SRC"

# Check if ~/.local/bin is present in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.profile:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
else
    echo "workstation-setup is ready to use! Try running: workstation-setup --help"
fi
