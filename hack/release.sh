#!/usr/bin/env bash
set -euo pipefail

# Release script for stable releases following RFD-40 conventions
# Usage: hack/release.sh <version>
# Examples:
#   hack/release.sh v0.1.0           # Preview release
#   hack/release.sh v1.0.0           # GA release
# For prerelease tags, use hack/release-prerelease.sh instead

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHANGELOG="$REPO_ROOT/docs/docs/changelog.md"

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "Error: Version required"
  echo "Usage: $0 <version>"
  echo ""
  echo "Examples:"
  echo "  $0 v0.1.0           # Preview release"
  echo "  $0 v1.0.0           # GA release"
  exit 1
fi

# Validate version format (stable releases only, no prerelease suffix)
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: Invalid version format: $VERSION"
  echo "Must match: v<major>.<minor>.<patch>"
  echo "For prerelease tags, use hack/release-prerelease.sh instead"
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

# Determine release type
if [[ "$VERSION" =~ ^v0\. ]]; then
  RELEASE_TYPE="Preview release"
else
  RELEASE_TYPE="GA release"
fi

# Validate changelog has Unreleased section
if [ ! -f "$CHANGELOG" ]; then
  echo "Error: Changelog not found at $CHANGELOG"
  exit 1
fi
if ! grep -q "^## Unreleased" "$CHANGELOG"; then
  echo "Error: Changelog missing '## Unreleased' section"
  exit 1
fi

# Show what we're about to do
echo ""
echo "======================================"
echo "Creating $RELEASE_TYPE: $VERSION"
echo "======================================"
echo "Branch: $CURRENT_BRANCH"
echo "Commit: $(git rev-parse --short HEAD)"
echo "Changelog: Will update Unreleased → $VERSION"
echo ""

# Ask for confirmation
read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted"
  exit 1
fi

# Update changelog
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

# Create a release branch and commit the changelog update
RELEASE_BRANCH="release/$VERSION"
git checkout -b "$RELEASE_BRANCH"

git add "$CHANGELOG"
git commit -m "Release $VERSION"

echo "✓ Changelog updated and committed on branch $RELEASE_BRANCH"

# Push the release branch; the tag will be created automatically when the PR is merged
echo "Pushing release branch to origin..."
git push origin "$RELEASE_BRANCH"

echo ""
echo "Creating pull request..."
gh pr create --base main --head "$RELEASE_BRANCH" --title "Release $VERSION" --body "Changelog update for $VERSION"

echo ""
echo "======================================"
echo "✓ Release branch pushed and PR created"
echo "======================================"
echo ""
echo "Next steps:"
echo "1. Merge the PR"
echo "2. Run: ./hack/release-tag.sh $VERSION"
echo "3. The release workflow will then build and upload artifacts"
echo ""
echo "Monitor progress:"
echo "  https://github.com/mirendev/runtime/actions"
echo ""
