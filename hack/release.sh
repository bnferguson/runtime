#!/usr/bin/env bash
set -euo pipefail

# Release script for creating version tags following RFD-40 conventions
# Usage: hack/release.sh <version>
# Examples:
#   hack/release.sh v0.0.0-test.1    # Test release
#   hack/release.sh v0.1.0           # Preview release
#   hack/release.sh v1.0.0           # GA release

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHANGELOG="$REPO_ROOT/docs/docs/changelog.md"

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "Error: Version required"
  echo "Usage: $0 <version>"
  echo ""
  echo "Examples:"
  echo "  $0 v0.0.0-test.1    # Test release"
  echo "  $0 v0.1.0           # Preview release"
  echo "  $0 v1.0.0           # GA release"
  exit 1
fi

# Validate version format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]]; then
  echo "Error: Invalid semver format: $VERSION"
  echo "Must match: v<major>.<minor>.<patch>[-<prerelease>]"
  exit 1
fi

# Check we're on main branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "Error: Must be on main branch (currently on: $CURRENT_BRANCH)"
  echo "Run: git checkout main"
  exit 1
fi

# Check working directory is clean
if [ -n "$(git status --porcelain)" ]; then
  echo "Error: Working directory has uncommitted changes"
  git status --short
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
  exit 1
fi

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "Error: Tag $VERSION already exists"
  exit 1
fi

# Determine release type and whether to update changelog
UPDATE_CHANGELOG=true
RELEASE_TYPE=""
if [[ "$VERSION" =~ ^v0\.0\.0-test\. ]]; then
  RELEASE_TYPE="Test release"
  UPDATE_CHANGELOG=false
elif [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+- ]]; then
  RELEASE_TYPE="Prerelease"
  UPDATE_CHANGELOG=false
elif [[ "$VERSION" =~ ^v0\. ]]; then
  RELEASE_TYPE="Preview release"
else
  RELEASE_TYPE="GA release"
fi

# Validate changelog has Unreleased section (for releases that update it)
if [ "$UPDATE_CHANGELOG" = true ]; then
  if [ ! -f "$CHANGELOG" ]; then
    echo "Error: Changelog not found at $CHANGELOG"
    exit 1
  fi
  if ! grep -q "^## Unreleased" "$CHANGELOG"; then
    echo "Error: Changelog missing '## Unreleased' section"
    exit 1
  fi
fi

# Show what we're about to do
echo ""
echo "======================================"
echo "Creating $RELEASE_TYPE: $VERSION"
echo "======================================"
echo "Branch: $CURRENT_BRANCH"
echo "Commit: $(git rev-parse --short HEAD)"
if [ "$UPDATE_CHANGELOG" = true ]; then
  echo "Changelog: Will update Unreleased → $VERSION"
else
  echo "Changelog: Skipped (test/prerelease)"
fi
echo ""

# Ask for confirmation
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted"
  exit 1
fi

# Update changelog if this is a real release
if [ "$UPDATE_CHANGELOG" = true ]; then
  echo ""
  echo "Updating changelog..."

  RELEASE_DATE=$(date +%Y-%m-%d)
  TEMP_CHANGELOG=$(mktemp)
  trap '[[ -n "$TEMP_CHANGELOG" && -e "$TEMP_CHANGELOG" ]] && rm -f "$TEMP_CHANGELOG"' EXIT

  # Copy header (everything before "## Unreleased")
  sed -n '1,/^## Unreleased$/{ /^## Unreleased$/!p }' "$CHANGELOG" > "$TEMP_CHANGELOG"

  # Add fresh empty Unreleased section
  cat >> "$TEMP_CHANGELOG" << 'EOF'
## Unreleased
*main*

---

EOF

  # Append rest of changelog, converting old Unreleased to new version
  sed -n '/^## Unreleased$/,$ {
    s/^## Unreleased$/## '"$VERSION"'/
    s/^\*main\*$/*'"$RELEASE_DATE"'*/
    p
  }' "$CHANGELOG" >> "$TEMP_CHANGELOG"

  mv "$TEMP_CHANGELOG" "$CHANGELOG"
  TEMP_CHANGELOG=""  # Clear so trap doesn't try to remove moved file

  # Commit the changelog update
  git add "$CHANGELOG"
  git commit -m "Release $VERSION"

  echo "✓ Changelog updated and committed"
fi

# Create annotated tag
TAG_MESSAGE="Release $VERSION

$RELEASE_TYPE created from main branch"

git tag -a "$VERSION" -m "$TAG_MESSAGE"

echo ""
echo "✓ Created annotated tag: $VERSION"
echo ""

# Push the commit (if we made one) and tag
echo "Pushing to origin..."
if [ "$UPDATE_CHANGELOG" = true ]; then
  git push origin main
fi
git push origin "$VERSION"

echo ""
echo "======================================"
echo "✓ Release tag pushed successfully"
echo "======================================"
echo ""
echo "What happens next:"
echo "1. Test workflow runs: https://github.com/mirendev/runtime/actions"
echo "2. Once tests pass, release workflow builds and uploads"
if [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+- ]]; then
  echo "3. Artifacts uploaded to: $VERSION (prereleases don't update 'latest')"
else
  echo "3. Artifacts uploaded to: $VERSION and latest"
fi
echo "4. Slack notification sent with deployment details"
echo ""
echo "Monitor progress:"
echo "  https://github.com/mirendev/runtime/actions"
echo ""
