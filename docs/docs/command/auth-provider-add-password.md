---
title: "miren auth provider add-password"
sidebar_label: "auth provider add-password"
description: "Add a password provider for route protection"
---

# miren auth provider add-password

Add a password provider for route protection

## Usage

```bash
miren auth provider add-password <name> [flags]
```

## Arguments

- `name` — Name for this password provider

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--password` — Password to protect routes with
- `--update` — Overwrite an existing provider with the same name (rotates password)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Add a password provider:**

```bash
miren auth provider add-password my-pw --password hunter2
```

## See also

- [`miren auth provider`](/command/auth-provider)
