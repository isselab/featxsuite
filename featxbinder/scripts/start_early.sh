#!/bin/bash
echo "Reconfiguring early features that are selected at compile time..."

sleep 5

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Absolute path to early_loader.py
PYTHON_SCRIPT="$SCRIPT_DIR/early_loader.py"

chmod +x "$PYTHON_SCRIPT"

python3 "$PYTHON_SCRIPT"