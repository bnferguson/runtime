---
title: "miren route oidc enable"
sidebar_label: "route oidc enable"
description: "Enable OIDC authentication for a route"
---

# miren route oidc enable

Enable OIDC authentication for a route

:::note
This command requires the `routeoidc` [labs feature](/labs) to be enabled.
:::

## Usage

```bash
miren route oidc enable <host> [flags]
```

## Arguments

- `host` — Hostname for the route (e.g., example.com)

## Flags

- `--claim-header` — Claim to header mapping in format 'claim:header' (e.g., 'email:X-User-Email')
- `--client-id` — OAuth2 client ID (required with --provider-url)
- `--client-secret` — OAuth2 client secret (required with --provider-url)
- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--default` — Apply to the default route
- `--provider` — Name of existing OIDC provider (use --provider-url for inline creation)
- `--provider-url` — OIDC provider URL (e.g., https://accounts.google.com) - creates provider if not exists
- `--scope` — OAuth2 scopes (can be specified multiple times)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Enable OIDC with an existing provider:**

```bash
miren route oidc enable example.com --provider my-google-oidc
```

**Enable OIDC with inline provider creation:**

```bash
miren route oidc enable example.com \
  --provider-url https://accounts.google.com \
  --client-id my-client-id \
  --client-secret my-client-secret
```

## See also

- [`miren route oidc`](/command/route-oidc)
