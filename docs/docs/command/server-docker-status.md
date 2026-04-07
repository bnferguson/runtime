---
title: "miren server docker status"
sidebar_label: "server docker status"
description: "Show status of miren server Docker container"
---

# miren server docker status

Show status of miren server Docker container

## Usage

```bash
miren server docker status [flags]
```

## Flags

- `--follow, -f` — Follow logs in real-time
- `--name, -n` — Container name (default: `miren`)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Show status:**

```bash
miren server docker status
```

**Follow logs:**

```bash
miren server docker status --follow
```

## See also

- [`miren server docker`](/command/server-docker)
