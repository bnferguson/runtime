#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

docs_dir="docs/docs"
sidebars="docs/sidebars.ts"
errors=0

# 1. No sidebar_position frontmatter (ordering is controlled by sidebars.ts)
if grep -rl 'sidebar_position' "$docs_dir"/*.md 2>/dev/null; then
  echo "ERROR: sidebar_position found in the above files."
  echo "  Sidebar ordering is controlled by sidebars.ts — remove sidebar_position from frontmatter."
  errors=1
fi

# 2. Every hand-written doc file must appear in sidebars.ts (and vice versa).
#    Generated files (commands.md, command/*) are excluded since they come from
#    command-sidebar.json which is imported separately.

# Doc IDs from the filesystem (filename without .md extension)
fs_ids=$(ls "$docs_dir"/*.md 2>/dev/null | xargs -I{} basename {} .md | sort)

# Doc IDs referenced as bare strings in sidebars.ts (skip command-sidebar entries)
sidebar_ids=$(grep -oP "^\s+'(\K[a-z0-9-]+)" "$sidebars" | sort)

orphaned=$(comm -23 <(echo "$fs_ids") <(echo "$sidebar_ids"))
missing=$(comm -13 <(echo "$fs_ids") <(echo "$sidebar_ids"))

if [ -n "$orphaned" ]; then
  echo "ERROR: Doc files not listed in sidebars.ts:"
  echo "$orphaned" | sed 's/^/  /'
  errors=1
fi

if [ -n "$missing" ]; then
  echo "ERROR: Sidebar entries with no matching doc file:"
  echo "$missing" | sed 's/^/  /'
  errors=1
fi

if [ "$errors" -eq 0 ]; then
  echo "✓ docs lint passed"
fi

exit "$errors"
