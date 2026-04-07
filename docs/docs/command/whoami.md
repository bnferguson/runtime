---
title: "miren whoami"
sidebar_label: "whoami"
description: "Display information about the current authenticated user"
---

# miren whoami

Display information about the current authenticated user

## Usage

```bash
miren whoami [flags]
```

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--format` — Output format (text, json) (default: `text`)
- `--json` — Shorthand for --format json

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Show current user:**

```bash
miren whoami
```

**JSON output:**

```bash
miren whoami --json
```
