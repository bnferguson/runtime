---
title: "miren deploy"
sidebar_label: "deploy"
description: "Deploy an application"
---

# miren deploy

Deploy an application

## Usage

```bash
miren deploy [flags]
```

## Flags

- `--analyze` — Analyze the app without building (show detected stack, services, etc.)
- `--env, -e` — Set environment variable (KEY=VALUE, KEY=@file, or KEY to prompt)
- `--explain, -x` — Explain the build process
- `--explain-format` — Explain format (default: `auto`) (choices: `auto`, `plain`, `tty`, `rawjson`)
- `--force, -f` — Skip confirmation prompt
- `--sensitive, -s` — Set sensitive environment variable (masked in output)
- `--version, -V` — Deploy an existing version (skip build)

## Config Options

- `--cluster, -C` — Cluster name
- `--config` — Path to the config file

## App Options

- `--app, -a` — Application name
- `--dir, -d` — Directory to run from (default: `.`)

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Basic:**

```bash
miren deploy
```

**Analyze:**

```bash
Before deploying, the system can tell you how it's going
to treat your application by running:

miren deploy --analyze
```

**Set environment variables during deploy:**

```bash
miren deploy -e DATABASE_URL=postgres://localhost/mydb
```

**Deploy a previously built version:**

```bash
miren deploy --version v3
```

## Subcommands

- [`miren deploy cancel`](/command/deploy-cancel) — Cancel an in-progress deployment
