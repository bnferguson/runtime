---
title: "miren sandbox list"
sidebar_label: "sandbox list"
description: "List sandboxes (excludes dead by default)"
---

# miren sandbox list

List sandboxes (excludes dead by default)

## Usage

```bash
miren sandbox list [flags]
```

## Flags

- `--all, -a` — Include dead sandboxes (excluded by default)
- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--format` — Output format (text, json) (default: `text`)
- `--json` — Shorthand for --format json
- `--status, -s` — Filter by status (pending, not_ready, running, stopped, dead)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**List running sandboxes:**

```bash
miren sandbox list
```

**Include dead sandboxes:**

```bash
miren sandbox list --all
```

**List as JSON:**

```bash
miren sandbox list --format json
```

## See also

- [`miren sandbox`](/command/sandbox)
