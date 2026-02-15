#!/bin/sh
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
pandoc -s -f markdown -t man \
  -V title=PECO -V section=1 \
  -V header="User Commands" \
  -V footer="peco" \
  -o "$SCRIPT_DIR/peco.1" \
  "$REPO_ROOT/README.md"
