---
title: "miren runner join"
sidebar_label: "runner join"
description: "Join this machine to a coordinator as a runner"
---

# miren runner join

Join this machine to a coordinator as a runner

:::note
This command requires the `distributedrunners` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren runner join <coordinatoraddr> <joincode> [flags]
```

## Arguments

- `coordinatoraddr` — Coordinator address (host:port)
- `joincode` — Join code from 'miren runner invite'

## Flags

- `--code` — Join code (or pass via stdin)
- `--config` — Path to save runner config (default: `/var/lib/miren/runner/config.yaml`)
- `--coordinator, -c` — Coordinator address (host:port)
- `--labels` — Additional labels for the runner (key=value)
- `--listen, -l` — Address this runner will listen on
- `--name` — Human-readable name for this runner (defaults to hostname)
- `--runner-id` — Specific runner ID to use (for reconnecting)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Join using a coordinator address and invite code:**

```bash
miren runner join coordinator.example.com:8443 abc123
```

## See also

- [`miren runner`](/command/runner)
