---
title: "miren completion zsh"
sidebar_label: "completion zsh"
description: "Output a zsh completion script"
---

# miren completion zsh

Output a zsh completion script

## Usage

```bash
miren completion zsh [flags]
```

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Load completion in the current shell:**

```bash
source <(miren completion zsh)
```

**Install for all sessions:**

```bash
miren completion zsh > "${fpath[1]}/_miren"
```

## See also

- [`miren completion`](/command/completion)
