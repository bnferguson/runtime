---
title: "miren auth provider show"
sidebar_label: "auth provider show"
description: "Show an identity provider"
---

# miren auth provider show

Show an identity provider

:::note
This command requires the `routeoidc` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren auth provider show <name> [flags]
```

## Arguments

- `name` — Name of the identity provider

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--format` — Output format (text, json) (default: `text`)
- `--json` — Shorthand for --format json

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## See also

- [`miren auth provider`](/command/auth-provider)
