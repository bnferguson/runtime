---
title: "miren runner revoke"
sidebar_label: "runner revoke"
description: "Revoke a runner invitation"
---

# miren runner revoke

Revoke a runner invitation

:::note
This command requires the `distributedrunners` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren runner revoke <inviteid> [flags]
```

## Arguments

- `inviteid` — ID of the invite to revoke

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Revoke an invite:**

```bash
miren runner revoke inv_abc123
```

## See also

- [`miren runner`](/command/runner)
