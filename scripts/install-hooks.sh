#!/bin/bash
# scripts/install-hooks.sh
# Installs local git hooks that enforce gitflow file-scope policy.
# Run once after cloning: bash scripts/install-hooks.sh

set -e

HOOKS_DIR="$(git rev-parse --git-dir)/hooks"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$SCRIPT_DIR/hooks"

if [[ ! -d "$SOURCE_DIR" ]]; then
    echo "❌ hooks source directory not found: $SOURCE_DIR"
    exit 1
fi

installed=0
for hook in "$SOURCE_DIR"/*; do
    name=$(basename "$hook")
    dest="$HOOKS_DIR/$name"
    cp "$hook" "$dest"
    chmod +x "$dest"
    echo "✓ Installed: .git/hooks/$name"
    installed=$((installed + 1))
done

echo ""
echo "✓ $installed hook(s) installed."
echo "  To verify: ls -la .git/hooks/"
