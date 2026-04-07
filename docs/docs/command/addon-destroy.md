---
title: "miren addon destroy"
sidebar_label: "addon destroy"
description: "Remove an addon from an application"
---

# miren addon destroy

Remove an addon from an application

:::note
This command requires the `addons` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren addon destroy <name> [flags]
```

## Arguments

- `name` — Addon name (e.g., miren-postgresql)

## Flags

- `--force, -f` — Skip confirmation prompt

## Config Options

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file

## App Options

- `--app, -a` — Application name
- `--dir, -d` — Directory to run from (default: `.`)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Remove an addon:**

```bash
miren addon destroy miren-postgresql
```

**Remove without confirmation:**

```bash
miren addon destroy miren-postgresql --force
```

## See also

- [`miren addon`](/command/addon)
