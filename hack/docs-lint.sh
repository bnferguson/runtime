#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../docs"
bun run lint
