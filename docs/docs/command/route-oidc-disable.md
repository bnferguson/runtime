---
title: "miren route oidc disable"
sidebar_label: "route oidc disable"
description: "Disable OIDC authentication for a route"
---

# miren route oidc disable

Disable OIDC authentication for a route

:::note
This command requires the `routeoidc` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren route oidc disable <host> [flags]
```

## Arguments

- `host` — Hostname for the route (e.g., example.com)

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--default` — Disable OIDC on the default route

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Disable OIDC on a route:**

```bash
miren route oidc disable example.com
```

## See also

- [`miren route oidc`](/command/route-oidc)
