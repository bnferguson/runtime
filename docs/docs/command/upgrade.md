---
title: "miren upgrade"
sidebar_label: "upgrade"
description: "Upgrade miren CLI to latest version"
---

# miren upgrade

Upgrade miren CLI to latest version

## Usage

```bash
miren upgrade [flags]
```

## Flags

- `--channel` — Channel to use: 'latest' (stable releases, default) or 'main' (bleeding edge)
- `--check, -c` — Check for available updates only
- `--force, -f` — Force upgrade even if already up to date or server running
- `--user, -u` — Install to user directory (~/.miren/release/miren) instead of system location
- `--version, -V` — Specific version to upgrade to (e.g., v0.2.0)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Upgrade to latest:**

```bash
miren upgrade
```

**Check for updates without installing:**

```bash
miren upgrade --check
```

**Upgrade to a specific version:**

```bash
miren upgrade --version v0.2.0
```
