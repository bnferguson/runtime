---
title: "miren help"
sidebar_label: "help"
description: "Show help for one or more commands"
---

# miren help

Show help for one or more commands

## Usage

```bash
miren help [args...] [flags]
```

## Flags

- `--commands` — List all commands with their synopsis
- `--format` — Output format (text, json) (default: `text`)
- `--json` — Shorthand for --format json

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**List all commands:**

```bash
miren help --commands
```

**List all commands as JSON:**

```bash
miren help --commands --format json
```

**Show help for multiple commands:**

```bash
miren help app.list version sandbox.stop
```

## Subcommands

- [`miren help alias`](/command/help-alias) — Learn how to define and use CLI aliases
