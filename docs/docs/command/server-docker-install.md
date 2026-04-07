---
title: "miren server docker install"
sidebar_label: "server docker install"
description: "Install miren server using Docker"
---

# miren server docker install

Install miren server using Docker

## Usage

```bash
miren server docker install [flags]
```

## Flags

- `--cluster-name` — Cluster name for cloud registration
- `--force, -f` — Remove existing container if present
- `--host-network` — Use host networking (ignores port mappings)
- `--http-port` — HTTP port mapping (default: `80`)
- `--image, -i` — Docker image to use (default: `oci.miren.cloud/miren:latest`)
- `--labs, -l` — Miren Labs features to enable (e.g. routeoidc,adminapi). Prefix with - to disable.
- `--name, -n` — Container name
- `--url, -u` — Cloud URL for registration (default: `https://miren.cloud`)
- `--without-cloud` — Skip cloud registration setup

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Install with cloud registration:**

```bash
miren server docker install
```

**Install without cloud (local only):**

```bash
miren server docker install --without-cloud
```

**Install with a custom HTTP port:**

```bash
miren server docker install --http-port 8080
```

## See also

- [`miren server docker`](/command/server-docker)
