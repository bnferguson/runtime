---
title: "miren server docker uninstall"
sidebar_label: "server docker uninstall"
description: "Uninstall miren server Docker container"
---

# miren server docker uninstall

Uninstall miren server Docker container

## Usage

```bash
miren server docker uninstall [flags]
```

## Flags

- `--force, -f` — Force removal even if container is running
- `--name, -n` — Container name (default: `miren`)
- `--remove-volume` — Remove the data volume

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Uninstall the container:**

```bash
miren server docker uninstall
```

**Uninstall and remove all data:**

```bash
miren server docker uninstall --remove-volume
```

## See also

- [`miren server docker`](/command/server-docker)
