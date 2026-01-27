#!/usr/bin/env bash
set -euo pipefail

# Push a release tag after the release PR has been merged
# Usage: hack/release-tag.sh <version> [--force]
# Examples:
#   hack/release-tag.sh v0.3.0
#   hack/release-tag.sh v0.3.0 --force   # Skip safety checks

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHANGELOG="$REPO_ROOT/docs/docs/changelog.md"

VERSION=""
FORCE=false

# Parse arguments
for arg in "$@"; do
  case "$arg" in
    --force|-f)
      FORCE=true
      ;;
    -*)
      echo "Error: Unknown flag: $arg"
      exit 1
      ;;
    *)
      if [ -z "$VERSION" ]; then
        VERSION="$arg"
      else
        echo "Error: Unexpected argument: $arg"
        exit 1
      fi
      ;;
  esac
done

if [ -z "$VERSION" ]; then
  echo "Error: Version required"
  echo "Usage: $0 <version> [--force]"
  echo ""
  echo "Examples:"
  echo "  $0 v0.3.0"
  echo "  $0 v0.3.0 --force   # Skip safety checks"
  exit 1
fi

# Validate version format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: Invalid version format: $VERSION"
  echo "Must match: v<major>.<minor>.<patch>"
  exit 1
fi

# Check if tag already exists (always check this, even with --force)
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "Error: Tag $VERSION already exists"
  exit 1
fi

if [ "$FORCE" = true ]; then
  echo "Warning: Skipping safety checks (--force)"
else
  # Check we're on main branch
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "Error: Must be on main branch (currently on: $CURRENT_BRANCH)"
    echo "Run: git checkout main"
    echo "Or use --force to skip this check"
    exit 1
  fi

  # Check working directory is clean
  if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory has uncommitted changes"
    git status --short
    echo ""
    echo "Use --force to skip this check"
    exit 1
  fi

  # Check we're up to date with origin
  echo "Fetching from origin..."
  git fetch origin main

  LOCAL=$(git rev-parse main)
  REMOTE=$(git rev-parse origin/main)

  if [ "$LOCAL" != "$REMOTE" ]; then
    echo "Error: Local main is not up to date with origin/main"
    echo "Run: git pull origin main"
    echo "Or use --force to skip this check"
    exit 1
  fi

  # Verify changelog has this version (confirms release PR was merged)
  if [ ! -f "$CHANGELOG" ]; then
    echo "Error: Changelog not found at $CHANGELOG"
    exit 1
  fi
  if ! grep -q "^## $VERSION" "$CHANGELOG"; then
    echo "Error: Changelog does not contain '## $VERSION'"
    echo "Has the release PR been merged?"
    echo "Or use --force to skip this check"
    exit 1
  fi
fi

# Show what we're about to do
echo ""
echo "======================================"
echo "Tagging release: $VERSION"
echo "======================================"
echo "Commit: $(git rev-parse --short HEAD)"
echo ""

# Ask for confirmation
read -p "Create and push tag $VERSION? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted"
  exit 1
fi

# Create and push tag
echo ""
echo "Creating tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "Pushing tag to origin..."
git push origin "$VERSION"

echo ""
echo "======================================"
echo "✓ Tag $VERSION pushed"
echo "======================================"
echo ""
echo "The release workflow should now be running:"
echo "  https://github.com/mirendev/runtime/actions/workflows/release.yml"
echo ""
