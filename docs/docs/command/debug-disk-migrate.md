---
title: "miren debug disk migrate"
sidebar_label: "debug disk migrate"
description: "Migrate LSVD volume to raw disk image"
---

# miren debug disk migrate

Migrate LSVD volume to raw disk image

## Usage

```bash
miren debug disk migrate [flags]
```

## Flags

- `--data-path` — Path to LSVD data directory
- `--output, -o` — Output raw disk image path
- `--volume-name` — LSVD volume name

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## See also

- [`miren debug disk`](/command/debug-disk)
