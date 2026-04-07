---
title: "miren runner invite list"
sidebar_label: "runner invite list"
description: "List all runner invitations"
---

# miren runner invite list

List all runner invitations

:::note
This command requires the `distributedrunners` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren runner invite list [flags]
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

**List invitations:**

```bash
miren runner invite list
```

## See also

- [`miren runner invite`](/command/runner-invite)
