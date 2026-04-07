---
title: "miren init"
sidebar_label: "init"
description: "Initialize a new application"
---

# miren init

Initialize a new application

## Usage

```bash
miren init [flags]
```

## Flags

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file
- `--dir, -d` — Application directory (defaults to current directory)
- `--name, -n` — Application name (defaults to directory name)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Initialize in current directory:**

```bash
miren init
```

**Initialize with a specific name:**

```bash
miren init --name myapp
```
