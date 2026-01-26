#!/bin/bash

# Use env vars if set (for CI/container builds), otherwise extract from git
# This handles cases where git isn't available (e.g., inside iso container)
build_date=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}

# Try to get git info, but don't fail if git isn't available
if [ -n "${GIT_BRANCH:-}" ]; then
  current_branch="$GIT_BRANCH"
elif current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null); then
  : # got it from git
else
  current_branch="dev"
fi

if [ -n "${GIT_COMMIT:-}" ]; then
  commit="$GIT_COMMIT"
elif commit=$(git rev-parse HEAD 2>/dev/null); then
  : # got it from git
else
  commit=""
fi

# Determine version string
if [ -n "${GIT_VERSION:-}" ]; then
  # Explicit version override
  version="$GIT_VERSION"
elif version=$(git describe --exact-match --tags HEAD 2>/dev/null); then
  : # Current commit has a tag
elif [[ $current_branch =~ ^release/(.*) ]]; then
  # On release branch
  version="${BASH_REMATCH[1]}"
elif [ -n "$commit" ]; then
  # Fall back to branch:short-commit
  version="$current_branch:${commit:0:7}"
else
  # No git info available
  version="$current_branch"
fi

echo "Building version $version"
echo "  Commit: ${commit:0:7}"
echo "  Date:   $build_date"

go build -ldflags "\
  -X miren.dev/runtime/version.Version=$version \
  -X miren.dev/runtime/version.Commit=$commit \
  -X miren.dev/runtime/version.BuildDate=$build_date" \
  -o bin/miren ./cmd/miren
