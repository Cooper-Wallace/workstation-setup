#!/usr/bin/env bash
# install.sh: Build and install workstation-setup into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/workstation-setup"
TARGET_ALIAS="${TARGET_DIR}/ws-setup"
GH_LINK="${TARGET_DIR}/gh-workstation-setup"

DRY_RUN=false
for arg in "$@"; do
    case "$arg" in
        --dry-run)
            DRY_RUN=true
            ;;
        -h|--help)
            echo "Usage: ./install.sh [--dry-run]"
            echo "Compiles and installs workstation-setup into ~/.local/bin"
            exit 0
            ;;
        *)
            echo -e "\033[1;31m[ERROR]\033[0m Unknown option: $arg" >&2
            echo "Usage: ./install.sh [--dry-run]" >&2
            exit 1
            ;;
    esac
done

# Ensure Go 1.23+ is available
GO_BIN="$(command -v go 2>/dev/null || true)"
if [[ -z "$GO_BIN" ]]; then
    for candidate in /usr/local/go/bin/go /tmp/go/bin/go; do
        if [[ -x "$candidate" ]]; then
            GO_BIN="$candidate"
            CANDIDATE_DIR="$(dirname "$candidate")"
            PATH="${CANDIDATE_DIR}:${PATH}"
            export PATH
            break
        fi
    done
fi

if [[ "$DRY_RUN" == true ]]; then
    echo -e "\033[1;33m[DRY-RUN]\033[0m Simulating workstation-setup build and installation:"
    if [[ -z "$GO_BIN" ]]; then
        echo "  • Warning: Go compiler not found in PATH or standard locations (/usr/local/go/bin, /tmp/go/bin)."
    else
        echo "  • Using Go binary: $GO_BIN"
    fi
    echo "  • Would ensure directory exists: $TARGET_DIR"
    echo "  • Would compile: go build -ldflags=\"-s -w\" -o $TARGET_BIN ./cmd/workstation-setup"
    echo "  • Would chmod +x: $TARGET_BIN"
    echo "  • Would symlink: $TARGET_ALIAS -> $TARGET_BIN"
    echo "  • Would symlink: $GH_LINK -> $TARGET_BIN"
    exit 0
fi

if [[ -z "$GO_BIN" ]]; then
    echo -e "\033[1;31m[ERROR]\033[0m Go 1.23+ is required to build workstation-setup." >&2
    echo "Please install Go or ensure it is accessible in PATH." >&2
    exit 1
fi

echo -e "\033[1;34m[INFO]\033[0m Compiling workstation-setup from source..."
mkdir -p "$TARGET_DIR"
cd "$SCRIPT_DIR"
go build -ldflags="-s -w" -o "$TARGET_BIN" ./cmd/workstation-setup
chmod +x "$TARGET_BIN"

# Create symlinks
ln -sf "$TARGET_BIN" "$TARGET_ALIAS"
ln -sf "$TARGET_BIN" "$GH_LINK"

echo -e "\033[1;32m[SUCCESS]\033[0m workstation-setup compiled and installed successfully:"
echo "  • Primary binary: $TARGET_BIN"
echo "  • Alias symlink:  $TARGET_ALIAS -> $TARGET_BIN"
echo "  • GH CLI symlink: $GH_LINK -> $TARGET_BIN"

# Check if ~/.local/bin is present in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
else
    echo "workstation-setup is ready to use! Try running: workstation-setup --help"
fi
