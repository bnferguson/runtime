---
title: "miren sandbox exec"
sidebar_label: "sandbox exec"
description: "Open interactive shell in an existing sandbox"
---

# miren sandbox exec

Open interactive shell in an existing sandbox

This command connects to an existing sandbox and runs a command inside it. Unlike `miren app run` which creates a new ephemeral sandbox, this connects to a sandbox that's already running (typically one serving production traffic).

## Finding Sandbox IDs

Use `miren sandbox list` to find the ID of a running sandbox:

```bash
$ miren sandbox list
ID                          APP       SERVICE   STATUS    NODE
sandbox/myapp-web-abc123    myapp     web       RUNNING   node-1
sandbox/myapp-web-def456    myapp     web       RUNNING   node-2
```

:::warning
When you exec into a production sandbox, you're connecting to a live instance that may be serving traffic. Be careful with commands that could affect the running application.
:::

:::tip
For debugging or one-off tasks without affecting production, use `miren app run` to create an isolated ephemeral sandbox instead.
:::

## Usage

```bash
miren sandbox exec [args...] [flags]
```

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--id, -i` — Sandbox ID (default: `miren-sandbox`)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Open a shell in a running sandbox:**

```bash
miren sandbox exec --id sb_abc123
```

**Run a command in a sandbox:**

```bash
miren sandbox exec --id sb_abc123 -- ls -la /app
```

## See also

- [`miren sandbox`](/command/sandbox)
