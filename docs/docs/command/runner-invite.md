---
title: "miren runner invite"
sidebar_label: "runner invite"
description: "Create a join code for a new runner"
---

# miren runner invite

Create a join code for a new runner

:::note
This command requires the `distributedrunners` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren runner invite [flags]
```

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--expires, -e` — Hours until the invite expires (default: `1`)
- `--labels, -l` — Labels to apply to the runner (key=value format)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Create an invite:**

```bash
miren runner invite
```

**Create an invite with labels and custom expiry:**

```bash
miren runner invite -l region=us-east -e 24
```

## Subcommands

- [`miren runner invite list`](/command/runner-invite-list) — List all runner invitations

## See also

- [`miren runner`](/command/runner)
