---
title: "miren route oidc show"
sidebar_label: "route oidc show"
description: "Show OIDC configuration for a route"
---

# miren route oidc show

Show OIDC configuration for a route

:::note
This command requires the `routeoidc` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren route oidc show <host> [flags]
```

## Arguments

- `host` — Hostname for the route (e.g., example.com)

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--default` — Show OIDC config for the default route
- `--format` — Output format (text, json) (default: `text`)
- `--json` — Shorthand for --format json

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Show OIDC config for a route:**

```bash
miren route oidc show example.com
```

## See also

- [`miren route oidc`](/command/route-oidc)
