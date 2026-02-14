#!/bin/bash
# Find packages with tests that are missing from test-times.json and measure them.
set -euo pipefail

TIMES_FILE="${1:-hack/test-times.json}"

if [ ! -f "$TIMES_FILE" ]; then
  echo "Error: $TIMES_FILE not found. Run a full measurement first."
  exit 1
fi

# All packages with test files
ALL_TEST_PKGS=$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... | grep -v '^$' | sort)

# Packages already in test-times.json
KNOWN_PKGS=$(jq -r '.packages[].package' "$TIMES_FILE" | sort)

# Find the difference
MISSING=$(comm -23 <(echo "$ALL_TEST_PKGS") <(echo "$KNOWN_PKGS"))

if [ -z "$MISSING" ]; then
  echo "All test packages are already in $TIMES_FILE"
  exit 0
fi

COUNT=$(echo "$MISSING" | wc -l)
echo "Found $COUNT new package(s) to measure:"
echo "$MISSING" | sed 's/^/  /'
echo

python3 hack/measure-test-times.py "$TIMES_FILE" --update $MISSING
