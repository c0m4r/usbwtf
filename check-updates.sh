#!/usr/bin/env bash
# check-updates.sh — check for available Go module updates

set -euo pipefail

if ! command -v go &>/dev/null; then
    echo "Error: go not found in PATH" >&2
    exit 1
fi

if [[ ! -f go.mod ]]; then
    echo "Error: go.mod not found. Run from the module root." >&2
    exit 1
fi

echo "Checking for module updates..."
echo

# go list -u -m all shows current and available versions
# Filter out lines with no available update (no '[' marker)
output=$(go list -u -m all 2>/dev/null)

if [[ -z "$output" ]]; then
    echo "No modules found."
    exit 0
fi

updates=""
while IFS= read -r line; do
    # Lines with an update look like: module v1.0.0 [v1.1.0]
    if [[ "$line" =~ \[(.+)\] ]]; then
        current=$(echo "$line" | awk '{print $2}')
        available="${BASH_REMATCH[1]}"
        module=$(echo "$line" | awk '{print $1}')
        updates+=$(printf "  %-50s %s → %s\n" "$module" "$current" "$available}")
        updates+=$'\n'
    fi
done <<< "$output"

if [[ -z "$updates" ]]; then
    echo "All modules are up to date."
else
    echo "Available updates:"
    echo "$updates"
    echo "Run 'go get -u ./...' to update all, or 'go get module@version' for a specific one."
fi
